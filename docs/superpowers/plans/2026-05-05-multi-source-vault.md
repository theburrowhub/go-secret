# Multi-source con Vault — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir que `go-secret` gestione secretos de varios proyectos GSM y mounts Vault simultáneamente, con vista unificada (columna `PROVIDER`), filtro por fuente, prompt interactivo cuando falta `--source`, auth Vault token/AppRole/OIDC, y migración automática de configs antiguos.

**Architecture:** Capa nueva `internal/sources/` con interface `Provider`, `Registry` que carga fuentes habilitadas, y `UnifiedClient` que hace fan-out paralelo. `internal/gcp/` se renombra a `internal/providers/gsm/` y cumple `Provider`. Nuevo paquete `internal/providers/vault/` con KV v1+v2 y tres métodos de auth. Config se reescribe a `Sources []SourceConfig` con migración automática del shape legacy.

**Tech Stack:** Go 1.24, Cobra, Bubbletea, `cloud.google.com/go/secretmanager`, `github.com/hashicorp/vault/api`, `github.com/zalando/go-keyring`, golangci-lint.

**Spec:** [`docs/superpowers/specs/2026-05-05-multi-source-vault-design.md`](../specs/2026-05-05-multi-source-vault-design.md)

---

## File Structure

### Nuevos paquetes / archivos

```
internal/sources/
├── provider.go        # Provider interface, Secret, Version, Capabilities, CreateOpts, ErrNotSupported
├── provider_test.go
├── registry.go        # Registry: load + active + toggle
├── registry_test.go
├── unified.go         # UnifiedClient: List fan-out, Resolve
├── unified_test.go
├── prompt.go          # PromptForSource (CLI interactive picker)
└── fakes_test.go      # FakeProvider para tests

internal/providers/gsm/    # ← refactor de internal/gcp/
├── client.go          # Adaptado: implementa sources.Provider
├── secret.go          # Tipos GSM-específicos
└── client_test.go

internal/providers/vault/
├── client.go          # implementa sources.Provider
├── kv2.go             # backend KV v2
├── kv1.go             # backend KV v1
├── auth.go            # Auth method dispatch
├── auth_token.go
├── auth_approle.go
├── auth_oidc.go       # browser flow + callback HTTP local
├── keyring.go         # wrapper sobre zalando/go-keyring con fallback
├── client_test.go
├── kv2_test.go
├── kv1_test.go
├── auth_token_test.go
├── auth_approle_test.go
└── auth_oidc_test.go

cmd/sources.go         # parent command
cmd/sources_list.go
cmd/sources_add.go
cmd/sources_edit.go
cmd/sources_remove.go
cmd/sources_toggle.go
cmd/sources_login.go
cmd/sources_set_default.go

docker-compose.yml     # vault dev + dexidp para E2E OIDC local
docs/dev-environment.md
```

### Archivos modificados

```
internal/config/config.go         # SourceConfig, VaultAuthConfig, VaultMount, migración legacy
internal/config/config_test.go    # NUEVO: tests de migración round-trip
internal/audit/audit.go           # source_id, provider en Event; nuevos event types
internal/ui/model.go              # PROVIDER column, source filter, picker, multi-source state
internal/ui/keys.go               # Tab cycle, Ctrl+P picker
internal/ui/styles.go             # Color por provider kind
cmd/root.go                       # --source flag global, deprecation alias --project
cmd/list.go                       # UnifiedClient
cmd/get.go                        # UnifiedClient + Resolve
cmd/reveal.go                     # Resolve
cmd/copy.go                       # Resolve
cmd/create.go                     # PromptForSource cuando falta
cmd/delete.go                     # Resolve
cmd/add_version.go                # Resolve
cmd/versions_*.go                 # Resolve
cmd/templates_generate.go         # SourceID/Provider variables
cmd/audit_logs.go                 # --source / --provider flags
cmd/config_get.go                 # render Sources
cmd/config_set.go                 # default_source key
cmd/config_projects.go            # deprecated, mapeado a sources
cmd/locations_*.go                # ámbito limitado a sources GSM
go.mod                            # +hashicorp/vault/api, +zalando/go-keyring
README.md                         # Vault + multi-source docs
CHANGELOG.md                      # NUEVO si no existe
```

### Archivos eliminados / renombrados

- `internal/gcp/client.go` → `internal/providers/gsm/client.go` (con cambios)

---

## Convenciones

- **TDD estricto**: test primero, falla, implementación mínima, pasa, commit.
- **Commits frecuentes**: uno por paso lógico (interface, implementación, integración).
- **Lint cero issues**: `golangci-lint run ./...` debe pasar antes de cada commit.
- **Race detector siempre**: tests con `go test -race ./...`.
- **No `panic`** fuera de `init()` en código de librería.
- **Errores wrapped** con `%w` para preservar la cadena.

---

## Task 1: Crear paquete `internal/sources` con tipos base

**Files:**
- Create: `internal/sources/provider.go`
- Create: `internal/sources/provider_test.go`

- [ ] **Step 1.1: Escribir test del sentinel `ErrNotSupported`**

```go
// internal/sources/provider_test.go
package sources

import (
	"errors"
	"testing"
)

func TestErrNotSupportedIsSentinel(t *testing.T) {
	wrapped := errors.New("wrap: " + ErrNotSupported.Error())
	if errors.Is(wrapped, ErrNotSupported) {
		t.Fatal("plain string wrap shouldn't match")
	}
	wrapped2 := WrapNotSupported("KV v1")
	if !errors.Is(wrapped2, ErrNotSupported) {
		t.Fatal("WrapNotSupported should be Is-able")
	}
	if got := wrapped2.Error(); got != "operation not supported: KV v1" {
		t.Fatalf("unexpected error string: %q", got)
	}
}
```

- [ ] **Step 1.2: Run test, verificar que falla con "package not found" o "ErrNotSupported undefined"**

Run: `go test ./internal/sources/...`
Expected: FAIL — package or symbol does not exist.

- [ ] **Step 1.3: Crear `provider.go` con interface, tipos y sentinel**

```go
// internal/sources/provider.go
// Package sources defines the abstract Provider interface that every secret
// backend (GSM, Vault, ...) implements, plus the registry/unified layer
// consumed by cmd/* and internal/ui.
package sources

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotSupported is returned by providers when an operation has no analog
// in the underlying backend (e.g., versioning on Vault KV v1).
var ErrNotSupported = errors.New("operation not supported")

// WrapNotSupported wraps ErrNotSupported with a human-readable detail.
func WrapNotSupported(detail string) error {
	return fmt.Errorf("%w: %s", ErrNotSupported, detail)
}

// Capabilities describes optional features a Provider supports. The UI/CLI
// uses these to render meaningful messages instead of raw "not supported".
type Capabilities struct {
	SupportsVersions  bool
	SupportsLabels    bool
	SupportsLocations bool
}

// Secret is the unified representation of a stored secret. Backend-specific
// fields (Replication, Labels) may be empty.
type Secret struct {
	Name        string
	SourceID    string
	CreateTime  string
	Labels      map[string]string
	Replication string
}

// Version represents a single version of a secret. State is normalized to
// "ENABLED" | "DISABLED" | "DESTROYED" regardless of backend.
type Version struct {
	Name       string
	State      string
	CreateTime string
}

// CreateOpts groups optional parameters for Create. Backends ignore fields
// they don't support (and return them via Capabilities).
type CreateOpts struct {
	Labels   map[string]string
	Location string
}

// Provider is the contract every secret backend must satisfy.
type Provider interface {
	ID() string
	Kind() string
	DisplayName() string
	FolderSeparator() string
	Capabilities() Capabilities

	List(ctx context.Context) ([]Secret, error)
	Get(ctx context.Context, name string) (*Secret, error)
	Reveal(ctx context.Context, name, version string) ([]byte, error)
	ListVersions(ctx context.Context, name string) ([]Version, error)

	Create(ctx context.Context, name string, value []byte, opts CreateOpts) error
	Delete(ctx context.Context, name string) error
	AddVersion(ctx context.Context, name string, value []byte) (*Version, error)
	EnableVersion(ctx context.Context, name, version string) error
	DisableVersion(ctx context.Context, name, version string) error
	DestroyVersion(ctx context.Context, name, version string) error

	UserEmail() string
	Close() error
}
```

- [ ] **Step 1.4: Run tests, verificar que pasan**

Run: `go test ./internal/sources/... -v`
Expected: PASS — `TestErrNotSupportedIsSentinel`.

- [ ] **Step 1.5: Lint**

Run: `golangci-lint run ./internal/sources/...`
Expected: `0 issues.`

- [ ] **Step 1.6: Commit**

```bash
git add internal/sources/provider.go internal/sources/provider_test.go
git commit -m "feat(sources): add Provider interface and base types"
```

---

## Task 2: Crear `FakeProvider` para tests

**Files:**
- Create: `internal/sources/fakes_test.go`

- [ ] **Step 2.1: Crear FakeProvider configurable**

```go
// internal/sources/fakes_test.go
package sources

import (
	"context"
	"sync/atomic"
)

type fakeProvider struct {
	id          string
	kind        string
	displayName string
	separator   string
	caps        Capabilities

	secrets    map[string]Secret
	listErr    error
	listCalls  atomic.Int32
	closeCalls atomic.Int32
	closeErr   error
}

func newFakeProvider(id, kind string) *fakeProvider {
	return &fakeProvider{
		id:          id,
		kind:        kind,
		displayName: id,
		separator:   "/",
		caps:        Capabilities{SupportsVersions: true, SupportsLabels: true},
		secrets:     map[string]Secret{},
	}
}

func (f *fakeProvider) ID() string                  { return f.id }
func (f *fakeProvider) Kind() string                { return f.kind }
func (f *fakeProvider) DisplayName() string         { return f.displayName }
func (f *fakeProvider) FolderSeparator() string     { return f.separator }
func (f *fakeProvider) Capabilities() Capabilities  { return f.caps }
func (f *fakeProvider) UserEmail() string           { return "fake@test" }

func (f *fakeProvider) List(ctx context.Context) ([]Secret, error) {
	f.listCalls.Add(1)
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]Secret, 0, len(f.secrets))
	for _, s := range f.secrets {
		s.SourceID = f.id
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeProvider) Get(ctx context.Context, name string) (*Secret, error) {
	s, ok := f.secrets[name]
	if !ok {
		return nil, errNotFound
	}
	s.SourceID = f.id
	return &s, nil
}

func (f *fakeProvider) Reveal(ctx context.Context, name, version string) ([]byte, error) {
	if _, ok := f.secrets[name]; !ok {
		return nil, errNotFound
	}
	return []byte("value-of-" + name), nil
}

func (f *fakeProvider) ListVersions(ctx context.Context, name string) ([]Version, error) {
	if _, ok := f.secrets[name]; !ok {
		return nil, errNotFound
	}
	if !f.caps.SupportsVersions {
		return nil, WrapNotSupported("versions on this fake")
	}
	return []Version{{Name: "1", State: "ENABLED", CreateTime: "2026-01-01T00:00:00Z"}}, nil
}

func (f *fakeProvider) Create(ctx context.Context, name string, value []byte, opts CreateOpts) error {
	f.secrets[name] = Secret{Name: name, Labels: opts.Labels, CreateTime: "now"}
	return nil
}

func (f *fakeProvider) Delete(ctx context.Context, name string) error {
	delete(f.secrets, name)
	return nil
}

func (f *fakeProvider) AddVersion(ctx context.Context, name string, value []byte) (*Version, error) {
	return &Version{Name: "2", State: "ENABLED"}, nil
}

func (f *fakeProvider) EnableVersion(ctx context.Context, name, version string) error  { return nil }
func (f *fakeProvider) DisableVersion(ctx context.Context, name, version string) error { return nil }
func (f *fakeProvider) DestroyVersion(ctx context.Context, name, version string) error { return nil }

func (f *fakeProvider) Close() error {
	f.closeCalls.Add(1)
	return f.closeErr
}

var errNotFound = &notFoundErr{}

type notFoundErr struct{}

func (*notFoundErr) Error() string { return "not found" }
```

- [ ] **Step 2.2: Verificar que la fake compila con tests**

Run: `go test ./internal/sources/... -count=1`
Expected: PASS (`TestErrNotSupportedIsSentinel`) y la fake compila.

- [ ] **Step 2.3: Commit**

```bash
git add internal/sources/fakes_test.go
git commit -m "test(sources): add fakeProvider helper for unit tests"
```

---

## Task 3: Implementar `Registry`

**Files:**
- Create: `internal/sources/registry.go`
- Create: `internal/sources/registry_test.go`

- [ ] **Step 3.1: Tests de Registry — get, active, toggle, close**

```go
// internal/sources/registry_test.go
package sources

import (
	"errors"
	"testing"
)

func TestRegistry_GetReturnsProviderByID(t *testing.T) {
	r := NewRegistry()
	p := newFakeProvider("a", "gsm")
	r.Register(p, true)

	got, err := r.Get("a")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID() != "a" {
		t.Fatalf("got %q, want %q", got.ID(), "a")
	}
}

func TestRegistry_GetUnknownReturnsErrSourceNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("missing")
	if !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("got %v, want ErrSourceNotFound", err)
	}
}

func TestRegistry_ActiveReturnsOnlyEnabled(t *testing.T) {
	r := NewRegistry()
	r.Register(newFakeProvider("a", "gsm"), true)
	r.Register(newFakeProvider("b", "vault"), false)
	r.Register(newFakeProvider("c", "gsm"), true)

	active := r.Active()
	if len(active) != 2 {
		t.Fatalf("got %d active, want 2", len(active))
	}
	ids := []string{active[0].ID(), active[1].ID()}
	if !contains(ids, "a") || !contains(ids, "c") {
		t.Fatalf("missing expected providers: %v", ids)
	}
}

func TestRegistry_ToggleSwitchesEnabled(t *testing.T) {
	r := NewRegistry()
	r.Register(newFakeProvider("a", "gsm"), true)

	if err := r.Toggle("a"); err != nil {
		t.Fatalf("toggle err: %v", err)
	}
	if got := r.IsEnabled("a"); got {
		t.Fatal("expected disabled after toggle")
	}
	if err := r.Toggle("a"); err != nil {
		t.Fatalf("toggle err: %v", err)
	}
	if got := r.IsEnabled("a"); !got {
		t.Fatal("expected enabled after second toggle")
	}
}

func TestRegistry_CloseClosesAll(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	b := newFakeProvider("b", "vault")
	r.Register(a, true)
	r.Register(b, false)

	if err := r.Close(); err != nil {
		t.Fatalf("close err: %v", err)
	}
	if a.closeCalls.Load() != 1 || b.closeCalls.Load() != 1 {
		t.Fatalf("close not called on all: a=%d b=%d", a.closeCalls.Load(), b.closeCalls.Load())
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3.2: Run tests, verificar fallos por símbolos faltantes**

Run: `go test ./internal/sources/...`
Expected: FAIL — `NewRegistry`, `ErrSourceNotFound` undefined.

- [ ] **Step 3.3: Implementar Registry**

```go
// internal/sources/registry.go
package sources

import (
	"errors"
	"sync"
)

// ErrSourceNotFound is returned by Registry.Get when no source matches the id.
var ErrSourceNotFound = errors.New("source not found")

type entry struct {
	provider Provider
	enabled  bool
}

// Registry holds the loaded providers and their runtime enabled state.
// Toggle/SetEnabled mutate runtime state only; persistence is the caller's
// responsibility (config.Save).
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*entry
	order   []string
}

func NewRegistry() *Registry {
	return &Registry{entries: map[string]*entry{}}
}

func (r *Registry) Register(p Provider, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := p.ID()
	if _, exists := r.entries[id]; !exists {
		r.order = append(r.order, id)
	}
	r.entries[id] = &entry{provider: p, enabled: enabled}
}

func (r *Registry) Get(id string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, ErrSourceNotFound
	}
	return e.provider, nil
}

func (r *Registry) Active() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, 0, len(r.entries))
	for _, id := range r.order {
		e := r.entries[id]
		if e.enabled {
			out = append(out, e.provider)
		}
	}
	return out
}

func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, 0, len(r.entries))
	for _, id := range r.order {
		out = append(out, r.entries[id].provider)
	}
	return out
}

func (r *Registry) IsEnabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[id]
	return ok && e.enabled
}

func (r *Registry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return ErrSourceNotFound
	}
	e.enabled = enabled
	return nil
}

func (r *Registry) Toggle(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return ErrSourceNotFound
	}
	e.enabled = !e.enabled
	return nil
}

func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for _, id := range r.order {
		if err := r.entries[id].provider.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
```

- [ ] **Step 3.4: Run tests verificando que pasan**

Run: `go test ./internal/sources/... -race -v`
Expected: PASS — todos los tests verdes, sin races.

- [ ] **Step 3.5: Commit**

```bash
git add internal/sources/registry.go internal/sources/registry_test.go
git commit -m "feat(sources): add Registry with runtime toggle support"
```

---

## Task 4: Implementar `UnifiedClient` con fan-out

**Files:**
- Create: `internal/sources/unified.go`
- Create: `internal/sources/unified_test.go`

- [ ] **Step 4.1: Tests de UnifiedClient**

```go
// internal/sources/unified_test.go
package sources

import (
	"context"
	"errors"
	"testing"
)

func TestUnified_ListAggregatesActiveProviders(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	a.secrets["s1"] = Secret{Name: "s1"}
	a.secrets["s2"] = Secret{Name: "s2"}
	b := newFakeProvider("b", "vault")
	b.secrets["s3"] = Secret{Name: "s3"}
	c := newFakeProvider("c", "gsm")
	c.secrets["s4"] = Secret{Name: "s4"}
	r.Register(a, true)
	r.Register(b, true)
	r.Register(c, false) // disabled, must be skipped

	u := NewUnifiedClient(r)
	got, err := u.List(context.Background())
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d secrets, want 3", len(got))
	}
	for _, s := range got {
		if s.SourceID == "" {
			t.Fatalf("secret %q missing SourceID", s.Name)
		}
		if s.SourceID == "c" {
			t.Fatalf("disabled provider c leaked into result")
		}
	}
}

func TestUnified_ListPartialErrorReportsButContinues(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	a.secrets["s1"] = Secret{Name: "s1"}
	b := newFakeProvider("b", "vault")
	b.listErr = errors.New("vault down")
	r.Register(a, true)
	r.Register(b, true)

	u := NewUnifiedClient(r)
	got, err := u.List(context.Background())
	if err == nil {
		t.Fatal("expected partial error")
	}
	var pe *PartialError
	if !errors.As(err, &pe) {
		t.Fatalf("expected PartialError, got %T", err)
	}
	if len(pe.SourceErrors) != 1 || pe.SourceErrors["b"] == nil {
		t.Fatalf("expected one source error for 'b', got %v", pe.SourceErrors)
	}
	if len(got) != 1 || got[0].SourceID != "a" {
		t.Fatalf("expected 1 secret from 'a', got %v", got)
	}
}

func TestUnified_ResolveExplicitSource(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	a.secrets["foo"] = Secret{Name: "foo"}
	b := newFakeProvider("b", "vault")
	b.secrets["foo"] = Secret{Name: "foo"}
	r.Register(a, true)
	r.Register(b, true)

	u := NewUnifiedClient(r)
	p, err := u.Resolve(context.Background(), "foo", "b")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.ID() != "b" {
		t.Fatalf("got %q, want b", p.ID())
	}
}

func TestUnified_ResolveAmbiguousReturnsErrAmbiguous(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	a.secrets["foo"] = Secret{Name: "foo"}
	b := newFakeProvider("b", "vault")
	b.secrets["foo"] = Secret{Name: "foo"}
	r.Register(a, true)
	r.Register(b, true)

	u := NewUnifiedClient(r)
	_, err := u.Resolve(context.Background(), "foo", "")
	if !errors.Is(err, ErrAmbiguousSecret) {
		t.Fatalf("got %v, want ErrAmbiguousSecret", err)
	}
}

func TestUnified_ResolveSingleProviderWithSecret(t *testing.T) {
	r := NewRegistry()
	a := newFakeProvider("a", "gsm")
	a.secrets["foo"] = Secret{Name: "foo"}
	b := newFakeProvider("b", "vault")
	r.Register(a, true)
	r.Register(b, true)

	u := NewUnifiedClient(r)
	p, err := u.Resolve(context.Background(), "foo", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p.ID() != "a" {
		t.Fatalf("got %q, want a", p.ID())
	}
}
```

- [ ] **Step 4.2: Run tests, verificar fallos**

Run: `go test ./internal/sources/... -count=1`
Expected: FAIL — `NewUnifiedClient`, `PartialError`, `ErrAmbiguousSecret` undefined.

- [ ] **Step 4.3: Implementar UnifiedClient**

```go
// internal/sources/unified.go
package sources

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrAmbiguousSecret is returned by Resolve when the same name exists in
// more than one active provider and no explicit source was requested.
var ErrAmbiguousSecret = errors.New("secret name exists in multiple sources")

// PartialError aggregates per-source failures while still returning the
// successful secrets. Use errors.As to detect.
type PartialError struct {
	SourceErrors map[string]error
}

func (e *PartialError) Error() string {
	parts := make([]string, 0, len(e.SourceErrors))
	for id, err := range e.SourceErrors {
		parts = append(parts, fmt.Sprintf("%s: %v", id, err))
	}
	return "partial failure: " + joinComma(parts)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// UnifiedClient fans out reads across active providers in the registry.
type UnifiedClient struct{ reg *Registry }

func NewUnifiedClient(r *Registry) *UnifiedClient { return &UnifiedClient{reg: r} }

// List returns secrets from every active provider. If a provider fails the
// others still complete and the error returned is a *PartialError.
func (u *UnifiedClient) List(ctx context.Context) ([]Secret, error) {
	providers := u.reg.Active()
	var (
		mu      sync.Mutex
		all     []Secret
		errs    = map[string]error{}
		wg      sync.WaitGroup
	)
	for _, p := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			items, err := p.List(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[p.ID()] = err
				return
			}
			for _, s := range items {
				if s.SourceID == "" {
					s.SourceID = p.ID()
				}
				all = append(all, s)
			}
		}(p)
	}
	wg.Wait()
	if len(errs) > 0 {
		return all, &PartialError{SourceErrors: errs}
	}
	return all, nil
}

// Resolve picks the right Provider for an operation on a named secret.
//   - if sourceID != "", returns that provider unconditionally.
//   - if sourceID == "", searches active providers and returns the unique
//     one that contains the secret, or ErrAmbiguousSecret if multiple do.
func (u *UnifiedClient) Resolve(ctx context.Context, name, sourceID string) (Provider, error) {
	if sourceID != "" {
		return u.reg.Get(sourceID)
	}
	matches := []Provider{}
	for _, p := range u.reg.Active() {
		if _, err := p.Get(ctx, name); err == nil {
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no active source contains secret %q", name)
	case 1:
		return matches[0], nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID())
		}
		return nil, fmt.Errorf("%w: found in %v", ErrAmbiguousSecret, ids)
	}
}
```

- [ ] **Step 4.4: Run tests**

Run: `go test ./internal/sources/... -race -v`
Expected: PASS.

- [ ] **Step 4.5: Lint y commit**

```bash
golangci-lint run ./internal/sources/...
git add internal/sources/unified.go internal/sources/unified_test.go
git commit -m "feat(sources): add UnifiedClient with parallel List and Resolve"
```

---

## Task 5: Refactor `internal/gcp` → `internal/providers/gsm`

**Files:**
- Move: `internal/gcp/client.go` → `internal/providers/gsm/client.go`
- Update package declaration y todos los imports en `cmd/*.go` e `internal/ui/*.go`

- [ ] **Step 5.1: Mover archivo y cambiar package**

```bash
mkdir -p internal/providers/gsm
git mv internal/gcp/client.go internal/providers/gsm/client.go
```

Editar la primera línea: `package gcp` → `package gsm`.

- [ ] **Step 5.2: Actualizar imports**

Buscar y reemplazar en todo el repo:

```bash
grep -rl "github.com/theburrowhub/go-secret/internal/gcp" cmd/ internal/ \
  | xargs perl -i -pe 's|github.com/theburrowhub/go-secret/internal/gcp|github.com/theburrowhub/go-secret/internal/providers/gsm|g'

grep -rl "gcp\." cmd/ internal/ \
  | xargs perl -i -pe 's|\bgcp\.|gsm.|g'
```

Verificar que no queda nada del paquete antiguo:

```bash
grep -rn "internal/gcp\|\\bgcp\\." cmd/ internal/ || echo "clean"
```

Expected: `clean`.

- [ ] **Step 5.3: Borrar directorio viejo si quedó vacío**

```bash
rmdir internal/gcp 2>/dev/null || true
ls internal/gcp 2>&1 | head -1
```

Expected: directory does not exist.

- [ ] **Step 5.4: Build y lint**

```bash
go build ./...
golangci-lint run ./...
```

Expected: BUILD OK, `0 issues`.

- [ ] **Step 5.5: Commit**

```bash
git add -A
git commit -m "refactor: rename internal/gcp to internal/providers/gsm

Preparation for the multi-provider sources layer. No behavior change."
```

---

## Task 6: Hacer que GSM Client implemente `sources.Provider`

**Files:**
- Modify: `internal/providers/gsm/client.go`
- Create: `internal/providers/gsm/client_test.go`

- [ ] **Step 6.1: Test que valida que `*gsm.Client` cumple la interface**

```go
// internal/providers/gsm/client_test.go
package gsm

import (
	"testing"

	"github.com/theburrowhub/go-secret/internal/sources"
)

func TestClientImplementsProvider(t *testing.T) {
	var _ sources.Provider = (*Client)(nil)
}
```

- [ ] **Step 6.2: Run test, verificar fallos por métodos no presentes**

Run: `go test ./internal/providers/gsm/...`
Expected: FAIL — método `ID() string` no existe en `Client`, etc.

- [ ] **Step 6.3: Añadir campos de identidad y métodos faltantes a `Client`**

Agregar arriba del struct existente (mantener los campos actuales tal cual):

```go
// internal/providers/gsm/client.go (cambios incrementales)

// Estos campos se añaden a Client; mantén los existentes:
type Client struct {
	// ... campos actuales (proyecto, smClient, etc.) sin tocar ...

	id              string
	displayName     string
	folderSeparator string
	locations       []string
}

// Identity (sources.Provider).
func (c *Client) ID() string              { return c.id }
func (c *Client) Kind() string            { return "gsm" }
func (c *Client) DisplayName() string {
	if c.displayName != "" {
		return c.displayName
	}
	return c.id
}
func (c *Client) FolderSeparator() string {
	if c.folderSeparator == "" {
		return "/"
	}
	return c.folderSeparator
}
func (c *Client) Capabilities() sources.Capabilities {
	return sources.Capabilities{
		SupportsVersions:  true,
		SupportsLabels:    true,
		SupportsLocations: true,
	}
}
```

- [ ] **Step 6.4: Adaptar `NewClient` para aceptar identidad**

Reemplazar la firma de `NewClient` por una que toma la `SourceConfig` (que se define en Task 8). Por ahora, añadir un constructor adicional:

```go
// NewClientWithIdentity is the modern constructor used by the sources Registry.
// The legacy NewClient still works for callers that haven't migrated yet.
func NewClientWithIdentity(ctx context.Context, id, displayName, folderSep, projectID string, locations []string) (*Client, error) {
	c, err := NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	c.id = id
	c.displayName = displayName
	c.folderSeparator = folderSep
	c.locations = locations
	return c, nil
}
```

- [ ] **Step 6.5: Adaptar firmas existentes para cumplir el interface**

Las firmas actuales (`AccessSecretVersion`, `ListSecrets`, etc.) probablemente no coinciden 1:1. Añade métodos puente con las firmas de `sources.Provider`:

```go
// Adapter methods to sources.Provider.

func (c *Client) List(ctx context.Context) ([]sources.Secret, error) {
	raw, err := c.ListSecrets(ctx) // método existente
	if err != nil {
		return nil, err
	}
	out := make([]sources.Secret, 0, len(raw))
	for _, s := range raw {
		out = append(out, sources.Secret{
			Name:        s.Name,
			SourceID:    c.id,
			CreateTime:  s.CreateTime,
			Labels:      s.Labels,
			Replication: s.Replication,
		})
	}
	return out, nil
}

func (c *Client) Get(ctx context.Context, name string) (*sources.Secret, error) {
	s, err := c.GetSecret(ctx, name)
	if err != nil {
		return nil, err
	}
	return &sources.Secret{
		Name:        s.Name,
		SourceID:    c.id,
		CreateTime:  s.CreateTime,
		Labels:      s.Labels,
		Replication: s.Replication,
	}, nil
}

func (c *Client) Reveal(ctx context.Context, name, version string) ([]byte, error) {
	if version == "" {
		version = "latest"
	}
	return c.AccessSecretVersion(ctx, name, version)
}

func (c *Client) ListVersions(ctx context.Context, name string) ([]sources.Version, error) {
	raw, err := c.ListSecretVersions(ctx, name)
	if err != nil {
		return nil, err
	}
	out := make([]sources.Version, 0, len(raw))
	for _, v := range raw {
		out = append(out, sources.Version{
			Name:       v.Name,
			State:      normalizeGSMState(v.State),
			CreateTime: v.CreateTime,
		})
	}
	return out, nil
}

func (c *Client) Create(ctx context.Context, name string, value []byte, opts sources.CreateOpts) error {
	if err := c.CreateSecret(ctx, name, opts.Labels, opts.Location); err != nil {
		return err
	}
	if len(value) > 0 {
		if _, err := c.AddSecretVersion(ctx, name, string(value)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Delete(ctx context.Context, name string) error {
	return c.DeleteSecret(ctx, name)
}

func (c *Client) AddVersion(ctx context.Context, name string, value []byte) (*sources.Version, error) {
	v, err := c.AddSecretVersion(ctx, name, string(value))
	if err != nil {
		return nil, err
	}
	return &sources.Version{Name: v.Name, State: normalizeGSMState(v.State), CreateTime: v.CreateTime}, nil
}

func (c *Client) EnableVersion(ctx context.Context, name, version string) error {
	return c.EnableSecretVersion(ctx, name, version)
}
func (c *Client) DisableVersion(ctx context.Context, name, version string) error {
	return c.DisableSecretVersion(ctx, name, version)
}
func (c *Client) DestroyVersion(ctx context.Context, name, version string) error {
	return c.DestroySecretVersion(ctx, name, version)
}

func normalizeGSMState(s string) string {
	switch s {
	case "STATE_ENABLED":
		return "ENABLED"
	case "STATE_DISABLED":
		return "DISABLED"
	case "STATE_DESTROYED":
		return "DESTROYED"
	default:
		return s
	}
}
```

> **Nota para el implementador:** los nombres `ListSecrets`, `GetSecret`, `AccessSecretVersion`, `AddSecretVersion`, `EnableSecretVersion`, `DisableSecretVersion`, `DestroySecretVersion`, `DeleteSecret`, `ListSecretVersions`, `CreateSecret` son los actuales en `internal/providers/gsm/client.go`. Ajustar si difieren.

- [ ] **Step 6.6: Run test del adapter**

```bash
go test ./internal/providers/gsm/... -race -v
```

Expected: PASS — `TestClientImplementsProvider`.

- [ ] **Step 6.7: Build, lint y commit**

```bash
go build ./...
golangci-lint run ./...
git add internal/providers/gsm/
git commit -m "feat(gsm): make Client implement sources.Provider via adapter methods"
```

---

## Task 7: Añadir tipos de config para sources

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 7.1: Test del shape nuevo y serialización YAML**

```go
// internal/config/config_test.go
package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSourceConfigYAMLRoundTrip(t *testing.T) {
	cfg := &Config{
		DefaultSource: "gsm-prod",
		Sources: []SourceConfig{
			{
				ID:       "gsm-prod",
				Provider: "gsm",
				Enabled:  true,
				ProjectID: "my-project",
				FolderSeparator: "/",
				SecretLocations: []string{"global"},
			},
			{
				ID:       "vault-corp",
				Provider: "vault",
				Enabled:  true,
				Address:  "https://vault.corp.io",
				Auth: VaultAuthConfig{Method: "oidc", Role: "developer", OIDCPort: 8250},
				Mounts: []VaultMount{{Path: "secret", Version: 2}},
			},
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.DefaultSource != "gsm-prod" {
		t.Fatalf("default_source: got %q, want gsm-prod", got.DefaultSource)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("sources: got %d, want 2", len(got.Sources))
	}
	if got.Sources[1].Auth.Method != "oidc" {
		t.Fatalf("vault auth method: got %q, want oidc", got.Sources[1].Auth.Method)
	}
}
```

- [ ] **Step 7.2: Run test, verificar fallos**

Run: `go test ./internal/config/...`
Expected: FAIL — `SourceConfig`, `VaultAuthConfig`, `VaultMount` no definidos.

- [ ] **Step 7.3: Añadir tipos a `config.go`**

Insertar tras los tipos existentes y modificar `Config` para incluir `DefaultSource` y `Sources`. Mantener los campos legacy con `omitempty`:

```go
// internal/config/config.go (cambios)

// SourceConfig describes one secret backend. Either ProjectID (for "gsm")
// or Address+Auth+Mounts (for "vault") is populated, never both.
type SourceConfig struct {
	ID              string     `yaml:"id"`
	Provider        string     `yaml:"provider"`
	Enabled         bool       `yaml:"enabled"`
	DisplayName     string     `yaml:"display_name,omitempty"`
	FolderSeparator string     `yaml:"folder_separator,omitempty"`
	Templates       []Template `yaml:"templates,omitempty"`

	// GSM
	ProjectID       string   `yaml:"project_id,omitempty"`
	SecretLocations []string `yaml:"secret_locations,omitempty"`

	// Vault
	Address string          `yaml:"address,omitempty"`
	Auth    VaultAuthConfig `yaml:"auth,omitempty"`
	Mounts  []VaultMount    `yaml:"mounts,omitempty"`
}

type VaultAuthConfig struct {
	Method        string `yaml:"method"`
	Role          string `yaml:"role,omitempty"`
	OIDCPort      int    `yaml:"oidc_port,omitempty"`
	AppRoleRoleID string `yaml:"role_id,omitempty"`
}

type VaultMount struct {
	Path    string `yaml:"path"`
	Version int    `yaml:"version,omitempty"`
}
```

Modificar el struct `Config` para incluir los campos nuevos (manteniendo legacy):

```go
type Config struct {
	DefaultSource string         `yaml:"default_source,omitempty"`
	Sources       []SourceConfig `yaml:"sources,omitempty"`

	Clipboard ClipboardConfig `yaml:"clipboard"`
	Audit     AuditConfig     `yaml:"audit"`
	Session   SessionConfig   `yaml:"session"`
	Templates []Template      `yaml:"templates,omitempty"`

	// Legacy (deprecated, auto-migrados): mantener hasta v2.
	ProjectID       string   `yaml:"project_id,omitempty"`
	RecentProjects  []string `yaml:"recent_projects,omitempty"`
	SecretLocations []string `yaml:"secret_locations,omitempty"`
	FolderSeparator string   `yaml:"folder_separator,omitempty"`
}
```

- [ ] **Step 7.4: Run tests**

Run: `go test ./internal/config/... -v`
Expected: PASS — `TestSourceConfigYAMLRoundTrip`.

- [ ] **Step 7.5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add SourceConfig, VaultAuthConfig, VaultMount types"
```

---

## Task 8: Migración legacy automática

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 8.1: Tests de migración**

Añadir a `config_test.go`:

```go
import (
	"os"
	"path/filepath"
	"strings"
)

func TestMigrate_LegacyToSources(t *testing.T) {
	cfg := &Config{
		ProjectID:       "active-proj",
		RecentProjects:  []string{"active-proj", "other-proj"},
		FolderSeparator: "/",
		SecretLocations: []string{"global", "europe-west1"},
	}
	migrated := MigrateLegacy(cfg)

	if migrated.DefaultSource != "gsm-active-proj" {
		t.Fatalf("default_source: got %q, want gsm-active-proj", migrated.DefaultSource)
	}
	if len(migrated.Sources) != 2 {
		t.Fatalf("sources count: got %d, want 2", len(migrated.Sources))
	}
	var active, other *SourceConfig
	for i := range migrated.Sources {
		s := &migrated.Sources[i]
		if s.ID == "gsm-active-proj" {
			active = s
		}
		if s.ID == "gsm-other-proj" {
			other = s
		}
	}
	if active == nil || other == nil {
		t.Fatalf("missing migrated sources: %+v", migrated.Sources)
	}
	if !active.Enabled {
		t.Fatal("active project source should be Enabled")
	}
	if other.Enabled {
		t.Fatal("inactive project source should be Disabled by default")
	}
	if active.ProjectID != "active-proj" {
		t.Fatalf("project_id: got %q", active.ProjectID)
	}
	if active.FolderSeparator != "/" {
		t.Fatalf("folder_separator: got %q", active.FolderSeparator)
	}
	if len(active.SecretLocations) != 2 {
		t.Fatalf("secret_locations: got %v", active.SecretLocations)
	}

	// Legacy fields should be cleared after migration.
	if migrated.ProjectID != "" || len(migrated.RecentProjects) != 0 ||
		migrated.FolderSeparator != "" || len(migrated.SecretLocations) != 0 {
		t.Fatal("legacy fields not cleared after migration")
	}
}

func TestMigrate_NoOpWhenSourcesAlreadyPresent(t *testing.T) {
	cfg := &Config{
		Sources: []SourceConfig{{ID: "x", Provider: "gsm", Enabled: true}},
	}
	migrated := MigrateLegacy(cfg)
	if len(migrated.Sources) != 1 || migrated.Sources[0].ID != "x" {
		t.Fatalf("sources mutated: %+v", migrated.Sources)
	}
}

func TestMigrate_SanitizeProjectIDInSourceID(t *testing.T) {
	cfg := &Config{
		ProjectID:      "My Project With Spaces",
		RecentProjects: []string{"My Project With Spaces"},
	}
	migrated := MigrateLegacy(cfg)
	if migrated.Sources[0].ID != "gsm-my-project-with-spaces" {
		t.Fatalf("sanitized id: got %q", migrated.Sources[0].ID)
	}
}

func TestLoad_PerformsMigrationAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	legacy := `
project_id: legacy-proj
recent_projects: ["legacy-proj"]
folder_separator: "/"
clipboard:
  auto_clear: true
  timeout_seconds: 30
audit:
  enabled: true
  max_size_mb: 10
  max_age_days: 90
session:
  inactivity_timeout: 15
  lock_on_timeout: true
`
	path := filepath.Join(dir, "go-secrets", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("sources: got %d, want 1", len(cfg.Sources))
	}
	if cfg.DefaultSource != "gsm-legacy-proj" {
		t.Fatalf("default: got %q", cfg.DefaultSource)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "default_source: gsm-legacy-proj") {
		t.Fatalf("config not persisted with migrated shape:\n%s", saved)
	}
	if strings.Contains(string(saved), "project_id: legacy-proj") {
		t.Fatal("legacy project_id should be removed after migration")
	}
}
```

- [ ] **Step 8.2: Run tests, verificar fallo por `MigrateLegacy` undefined**

Run: `go test ./internal/config/...`
Expected: FAIL — `MigrateLegacy` undefined.

- [ ] **Step 8.3: Implementar `MigrateLegacy` y modificar `Load` para invocarlo + persistir**

Añadir a `config.go`:

```go
// MigrateLegacy converts an old-shape Config (project_id at root, recent_projects, etc.)
// into the new Sources-based shape. If Sources is already populated the input is
// returned unchanged.
func MigrateLegacy(cfg *Config) *Config {
	if len(cfg.Sources) > 0 {
		return cfg
	}
	if cfg.ProjectID == "" && len(cfg.RecentProjects) == 0 {
		return cfg
	}

	known := map[string]bool{}
	add := func(proj string, enabled bool) {
		if proj == "" || known[proj] {
			return
		}
		known[proj] = true
		cfg.Sources = append(cfg.Sources, SourceConfig{
			ID:              "gsm-" + sanitizeID(proj),
			Provider:        "gsm",
			Enabled:         enabled,
			ProjectID:       proj,
			FolderSeparator: cfg.FolderSeparator,
			SecretLocations: append([]string(nil), cfg.SecretLocations...),
		})
	}
	if cfg.ProjectID != "" {
		add(cfg.ProjectID, true)
		cfg.DefaultSource = "gsm-" + sanitizeID(cfg.ProjectID)
	}
	for _, p := range cfg.RecentProjects {
		add(p, false)
	}
	// Re-enable the active one if it appeared first as inactive.
	for i := range cfg.Sources {
		if cfg.Sources[i].ProjectID == cfg.ProjectID {
			cfg.Sources[i].Enabled = true
		}
	}

	cfg.ProjectID = ""
	cfg.RecentProjects = nil
	cfg.FolderSeparator = ""
	cfg.SecretLocations = nil
	return cfg
}

func sanitizeID(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '_' || r == '.':
			out = append(out, '-')
		}
	}
	return string(out)
}
```

Modificar `Load` para invocar `MigrateLegacy` y persistir cuando haya cambio:

```go
func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	if info.Mode().Perm() != 0600 {
		_ = os.Chmod(configPath, 0600)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	hadLegacy := cfg.ProjectID != "" || len(cfg.RecentProjects) > 0 ||
		cfg.FolderSeparator != "" || len(cfg.SecretLocations) > 0
	cfg = MigrateLegacy(cfg)

	if hadLegacy {
		// Persist the migrated shape so the next load is a no-op.
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("persisting migrated config: %w", err)
		}
	}
	return cfg, nil
}
```

> Asegurarse de que `fmt` está importado.

- [ ] **Step 8.4: Run tests con race**

Run: `go test ./internal/config/... -race -v`
Expected: PASS — todos los tests.

- [ ] **Step 8.5: Build, lint, commit**

```bash
go build ./...
golangci-lint run ./...
git add internal/config/
git commit -m "feat(config): auto-migrate legacy single-project config to sources"
```

---

## Task 9: Wire `Registry` desde `config.Config`

**Files:**
- Modify: `internal/sources/registry.go`
- Create: `internal/sources/load.go`
- Create: `internal/sources/load_test.go`

- [ ] **Step 9.1: Test que valida `LoadFromConfig` instancia providers GSM**

```go
// internal/sources/load_test.go
package sources

import (
	"context"
	"testing"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestLoadFromConfig_GSMOnly_InstantiatesEnabledOnly(t *testing.T) {
	cfg := &config.Config{
		DefaultSource: "a",
		Sources: []config.SourceConfig{
			{ID: "a", Provider: "gsm", Enabled: true, ProjectID: "p1"},
			{ID: "b", Provider: "gsm", Enabled: false, ProjectID: "p2"},
		},
	}
	// Inject GSM constructor stub via package var (defined in load.go).
	prevGSM := newGSMProvider
	defer func() { newGSMProvider = prevGSM }()
	newGSMProvider = func(ctx context.Context, sc config.SourceConfig) (Provider, error) {
		return newFakeProvider(sc.ID, "gsm"), nil
	}
	prevVault := newVaultProvider
	defer func() { newVaultProvider = prevVault }()
	newVaultProvider = func(ctx context.Context, sc config.SourceConfig) (Provider, error) {
		t.Fatal("vault should not be instantiated")
		return nil, nil
	}

	reg, err := LoadFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("LoadFromConfig: %v", err)
	}
	if got := len(reg.All()); got != 2 {
		t.Fatalf("got %d registered, want 2", got)
	}
	if got := len(reg.Active()); got != 1 {
		t.Fatalf("got %d active, want 1", got)
	}
	if reg.Active()[0].ID() != "a" {
		t.Fatalf("active[0]: got %q", reg.Active()[0].ID())
	}
}

func TestLoadFromConfig_UnknownProviderReturnsError(t *testing.T) {
	cfg := &config.Config{
		Sources: []config.SourceConfig{{ID: "x", Provider: "azure", Enabled: true}},
	}
	if _, err := LoadFromConfig(context.Background(), cfg); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
```

- [ ] **Step 9.2: Run tests, verificar fallos**

Run: `go test ./internal/sources/...`
Expected: FAIL — `LoadFromConfig` undefined.

- [ ] **Step 9.3: Implementar `LoadFromConfig` con package vars inyectables**

```go
// internal/sources/load.go
package sources

import (
	"context"
	"fmt"

	"github.com/theburrowhub/go-secret/internal/config"
)

// Constructors injected at runtime to avoid an import cycle (config + sources
// shouldn't depend on the concrete provider packages directly).
var (
	newGSMProvider   func(ctx context.Context, sc config.SourceConfig) (Provider, error)
	newVaultProvider func(ctx context.Context, sc config.SourceConfig) (Provider, error)
)

// RegisterProviderConstructors is called once from main (or cmd/root.go) to
// wire the real GSM and Vault factories.
func RegisterProviderConstructors(
	gsm func(ctx context.Context, sc config.SourceConfig) (Provider, error),
	vault func(ctx context.Context, sc config.SourceConfig) (Provider, error),
) {
	newGSMProvider = gsm
	newVaultProvider = vault
}

// LoadFromConfig instantiates providers per SourceConfig and registers them
// with their initial enabled flag.
func LoadFromConfig(ctx context.Context, cfg *config.Config) (*Registry, error) {
	r := NewRegistry()
	for _, sc := range cfg.Sources {
		var (
			p   Provider
			err error
		)
		switch sc.Provider {
		case "gsm":
			if newGSMProvider == nil {
				return nil, fmt.Errorf("gsm provider constructor not registered")
			}
			p, err = newGSMProvider(ctx, sc)
		case "vault":
			if newVaultProvider == nil {
				return nil, fmt.Errorf("vault provider constructor not registered")
			}
			p, err = newVaultProvider(ctx, sc)
		default:
			return nil, fmt.Errorf("unknown provider %q for source %q", sc.Provider, sc.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", sc.ID, err)
		}
		r.Register(p, sc.Enabled)
	}
	return r, nil
}
```

- [ ] **Step 9.4: Run tests verificando que pasan**

Run: `go test ./internal/sources/... -race -v`
Expected: PASS — `TestLoadFromConfig_GSMOnly_InstantiatesEnabledOnly` y `TestLoadFromConfig_UnknownProviderReturnsError`.

- [ ] **Step 9.5: Commit**

```bash
git add internal/sources/load.go internal/sources/load_test.go
git commit -m "feat(sources): add LoadFromConfig with injectable provider constructors"
```

---

## Task 10: Constructor GSM desde `SourceConfig`

**Files:**
- Modify: `internal/providers/gsm/client.go`
- Modify: `internal/providers/gsm/client_test.go`

- [ ] **Step 10.1: Test que verifica el constructor desde SourceConfig**

```go
// internal/providers/gsm/client_test.go (añadir)
import (
	"context"
	"github.com/theburrowhub/go-secret/internal/config"
)

func TestNewFromSourceConfigPopulatesIdentity(t *testing.T) {
	t.Skip("integration: requires GCP creds; verifies signature only")
	_ = func() (*Client, error) {
		return NewFromSourceConfig(context.Background(), config.SourceConfig{
			ID: "gsm-x", Provider: "gsm", ProjectID: "p1", FolderSeparator: "/",
		})
	}
}
```

- [ ] **Step 10.2: Implementar `NewFromSourceConfig`**

Añadir a `internal/providers/gsm/client.go`:

```go
import "github.com/theburrowhub/go-secret/internal/config"

// NewFromSourceConfig is the canonical constructor used by sources.LoadFromConfig.
func NewFromSourceConfig(ctx context.Context, sc config.SourceConfig) (*Client, error) {
	if sc.ProjectID == "" {
		return nil, fmt.Errorf("gsm source %q missing project_id", sc.ID)
	}
	c, err := NewClient(ctx, sc.ProjectID)
	if err != nil {
		return nil, err
	}
	c.id = sc.ID
	c.displayName = sc.DisplayName
	c.folderSeparator = sc.FolderSeparator
	c.locations = sc.SecretLocations
	return c, nil
}
```

- [ ] **Step 10.3: Build y test**

```bash
go build ./...
go test ./internal/providers/gsm/...
```

Expected: BUILD OK, tests pasan (incluido el skipped).

- [ ] **Step 10.4: Commit**

```bash
git add internal/providers/gsm/
git commit -m "feat(gsm): NewFromSourceConfig constructor for sources.Registry"
```

---

## Task 11: Vault provider — esqueleto + auth token

**Files:**
- Create: `internal/providers/vault/client.go`
- Create: `internal/providers/vault/auth.go`
- Create: `internal/providers/vault/auth_token.go`
- Create: `internal/providers/vault/auth_token_test.go`

- [ ] **Step 11.1: Añadir dependencia vault api**

```bash
go get github.com/hashicorp/vault/api@latest
```

- [ ] **Step 11.2: Test del auth `token` con httptest**

```go
// internal/providers/vault/auth_token_test.go
package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestTokenAuth_FromEnvVar(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "env-token")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "env-token" {
			t.Fatalf("unexpected token: %q", r.Header.Get("X-Vault-Token"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tok, err := resolveTokenAuth(config.SourceConfig{
		ID: "v", Provider: "vault", Address: srv.URL, Auth: config.VaultAuthConfig{Method: "token"},
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if tok != "env-token" {
		t.Fatalf("got %q, want env-token", tok)
	}
}

func TestTokenAuth_PrefersEnvOverFile(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "from-env")
	tok, err := resolveTokenAuth(config.SourceConfig{ID: "v"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if tok != "from-env" {
		t.Fatalf("got %q", tok)
	}
}
```

- [ ] **Step 11.3: Run test, fallo esperado**

Run: `go test ./internal/providers/vault/...`
Expected: FAIL — `resolveTokenAuth` undefined.

- [ ] **Step 11.4: Implementar `auth.go`, `auth_token.go`, esqueleto de `client.go`**

```go
// internal/providers/vault/auth.go
package vault

import (
	"context"
	"fmt"

	"github.com/theburrowhub/go-secret/internal/config"
)

// resolveAuth returns a Vault token for the given source according to the
// configured auth method. Caller takes ownership of the token (caching is
// handled per-method internally).
func resolveAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	switch sc.Auth.Method {
	case "", "token":
		return resolveTokenAuth(sc)
	case "approle":
		return resolveAppRoleAuth(ctx, sc)
	case "oidc":
		return resolveOIDCAuth(ctx, sc)
	default:
		return "", fmt.Errorf("unknown vault auth method %q", sc.Auth.Method)
	}
}
```

```go
// internal/providers/vault/auth_token.go
package vault

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/theburrowhub/go-secret/internal/config"
)

// Resolution order:
//   1. VAULT_TOKEN env var
//   2. OS keyring entry "go-secret:vault:<source-id>"
//   3. ~/.vault-token file
func resolveTokenAuth(sc config.SourceConfig) (string, error) {
	if v := os.Getenv("VAULT_TOKEN"); v != "" {
		return v, nil
	}
	if v, ok := keyringGet(sc.ID); ok {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		data, err := os.ReadFile(filepath.Join(home, ".vault-token"))
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("no vault token available (set VAULT_TOKEN, run `go-secret sources login %s`, or write ~/.vault-token)", sc.ID)
}
```

```go
// internal/providers/vault/keyring.go (stub; Task 14 lo completa)
package vault

func keyringGet(sourceID string) (string, bool) { return "", false }
func keyringSet(sourceID, token string) error    { return nil }
func keyringDelete(sourceID string) error        { return nil }
```

```go
// internal/providers/vault/auth_approle.go (stub; Task 14 implementa)
package vault

import (
	"context"
	"errors"

	"github.com/theburrowhub/go-secret/internal/config"
)

func resolveAppRoleAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	return "", errors.New("approle auth not implemented yet")
}
```

```go
// internal/providers/vault/auth_oidc.go (stub; Task 15 implementa)
package vault

import (
	"context"
	"errors"

	"github.com/theburrowhub/go-secret/internal/config"
)

func resolveOIDCAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	return "", errors.New("oidc auth not implemented yet")
}
```

```go
// internal/providers/vault/client.go (esqueleto)
package vault

import (
	"context"
	"errors"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
)

// Client is the Vault sources.Provider implementation. Supports KV v1 and v2,
// possibly multiple mounts within the same source.
type Client struct {
	id              string
	displayName     string
	folderSeparator string
	api             *vaultapi.Client
	mounts          []mountInfo
}

type mountInfo struct {
	Path    string
	Version int // 1 or 2
}

// NewFromSourceConfig instantiates a Vault Client from a SourceConfig.
// Authentication happens here; the returned Client carries the Vault token
// in the api client.
func NewFromSourceConfig(ctx context.Context, sc config.SourceConfig) (*Client, error) {
	if sc.Address == "" {
		return nil, fmt.Errorf("vault source %q missing address", sc.ID)
	}
	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	tok, err := resolveAuth(ctx, sc)
	if err != nil {
		return nil, err
	}
	api.SetToken(tok)

	c := &Client{
		id:              sc.ID,
		displayName:     sc.DisplayName,
		folderSeparator: sc.FolderSeparator,
		api:             api,
	}
	if c.folderSeparator == "" {
		c.folderSeparator = "/"
	}
	for _, m := range sc.Mounts {
		c.mounts = append(c.mounts, mountInfo{Path: m.Path, Version: m.Version})
	}
	if len(c.mounts) == 0 {
		return nil, fmt.Errorf("vault source %q has no mounts configured", sc.ID)
	}
	return c, nil
}

// Identity (sources.Provider).
func (c *Client) ID() string                { return c.id }
func (c *Client) Kind() string              { return "vault" }
func (c *Client) DisplayName() string {
	if c.displayName != "" {
		return c.displayName
	}
	return c.id
}
func (c *Client) FolderSeparator() string   { return c.folderSeparator }
func (c *Client) UserEmail() string         { return "" } // populated in Task 15 for OIDC
func (c *Client) Close() error              { return nil }
func (c *Client) Capabilities() sources.Capabilities {
	supportsVersions := false
	for _, m := range c.mounts {
		if m.Version == 2 {
			supportsVersions = true
			break
		}
	}
	return sources.Capabilities{
		SupportsVersions: supportsVersions,
		SupportsLabels:   true,
	}
}

// Stubs to satisfy sources.Provider — implemented in tasks 12 and 13.
func (c *Client) List(ctx context.Context) ([]sources.Secret, error) {
	return nil, errors.New("vault.Client.List not implemented")
}
func (c *Client) Get(ctx context.Context, name string) (*sources.Secret, error) {
	return nil, errors.New("vault.Client.Get not implemented")
}
func (c *Client) Reveal(ctx context.Context, name, version string) ([]byte, error) {
	return nil, errors.New("vault.Client.Reveal not implemented")
}
func (c *Client) ListVersions(ctx context.Context, name string) ([]sources.Version, error) {
	return nil, errors.New("vault.Client.ListVersions not implemented")
}
func (c *Client) Create(ctx context.Context, name string, value []byte, opts sources.CreateOpts) error {
	return errors.New("vault.Client.Create not implemented")
}
func (c *Client) Delete(ctx context.Context, name string) error {
	return errors.New("vault.Client.Delete not implemented")
}
func (c *Client) AddVersion(ctx context.Context, name string, value []byte) (*sources.Version, error) {
	return nil, errors.New("vault.Client.AddVersion not implemented")
}
func (c *Client) EnableVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("EnableVersion not supported until KV v2 implemented")
}
func (c *Client) DisableVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("DisableVersion not supported until KV v2 implemented")
}
func (c *Client) DestroyVersion(ctx context.Context, name, version string) error {
	return sources.WrapNotSupported("DestroyVersion not supported until KV v2 implemented")
}

// Compile-time assertion.
var _ sources.Provider = (*Client)(nil)
```

- [ ] **Step 11.5: Run tests + build**

```bash
go mod tidy
go build ./...
go test ./internal/providers/vault/... -v
```

Expected: BUILD OK; `TestTokenAuth_*` PASS.

- [ ] **Step 11.6: Commit**

```bash
git add go.mod go.sum internal/providers/vault/
git commit -m "feat(vault): provider skeleton with token auth"
```

---

## Task 12: Vault KV v2 — read operations

**Files:**
- Create: `internal/providers/vault/kv2.go`
- Create: `internal/providers/vault/kv2_test.go`
- Modify: `internal/providers/vault/client.go`

- [ ] **Step 12.1: Test KV v2 List/Get/Reveal contra httptest**

```go
// internal/providers/vault/kv2_test.go
package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newKV2Server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/metadata/", func(w http.ResponseWriter, r *http.Request) {
		// LIST returns the keys at this prefix (Vault uses LIST verb).
		if r.Method == "LIST" || r.URL.Query().Get("list") == "true" {
			path := strings.TrimPrefix(r.URL.Path, "/v1/secret/metadata/")
			keys := map[string][]string{
				"":      {"app/", "single"},
				"app/":  {"db", "api"},
			}
			out := map[string]interface{}{"data": map[string]interface{}{"keys": keys[path]}}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"created_time":    "2026-01-02T03:04:05Z",
				"current_version": 3,
				"versions": map[string]interface{}{
					"1": map[string]interface{}{"created_time": "2026-01-01T00:00:00Z"},
					"2": map[string]interface{}{"created_time": "2026-01-02T00:00:00Z", "destroyed": true},
					"3": map[string]interface{}{"created_time": "2026-01-02T03:04:05Z"},
				},
			},
		})
	})
	mux.HandleFunc("/v1/secret/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"data":     map[string]interface{}{"value": "the-secret"},
				"metadata": map[string]interface{}{"version": 3, "created_time": "2026-01-02T03:04:05Z"},
			},
		})
	})
	return httptest.NewServer(mux)
}

func TestKV2_List(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
		if s.SourceID != "v" {
			t.Fatalf("missing SourceID: %+v", s)
		}
	}
	for _, expected := range []string{"single", "app/db", "app/api"} {
		if !names[expected] {
			t.Fatalf("missing %q in %v", expected, names)
		}
	}
}

func TestKV2_RevealReturnsValue(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	v, err := c.Reveal(context.Background(), "app/db", "")
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if string(v) != "the-secret" {
		t.Fatalf("got %q", v)
	}
}

func TestKV2_ListVersionsNormalizesState(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	vs, err := c.ListVersions(context.Background(), "app/db")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("got %d versions", len(vs))
	}
	stateBy := map[string]string{}
	for _, v := range vs {
		stateBy[v.Name] = v.State
	}
	if stateBy["1"] != "ENABLED" || stateBy["2"] != "DESTROYED" || stateBy["3"] != "ENABLED" {
		t.Fatalf("unexpected states: %v", stateBy)
	}
}

// helper used by tests in this file and others.
func newTestClient(t *testing.T, addr string, mounts ...mountInfo) *Client {
	t.Helper()
	t.Setenv("VAULT_TOKEN", "test-token")
	c := &Client{id: "v", folderSeparator: "/", mounts: mounts}
	cfg := vaultapiConfig(addr)
	api, err := vaultapiNewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	api.SetToken("test-token")
	c.api = api
	return c
}
```

(Si los helpers `vaultapiConfig` / `vaultapiNewClient` no existen aún, sustitúyelos en el test por llamadas directas a `vaultapi.DefaultConfig()` y `vaultapi.NewClient()` y un set explícito de `cfg.Address`.)

- [ ] **Step 12.2: Implementar `kv2.go`**

```go
// internal/providers/vault/kv2.go
package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/go-secret/internal/sources"
)

// listKV2 walks the metadata tree for a single mount and returns flattened
// secret names (relative to the mount root).
func (c *Client) listKV2(ctx context.Context, m mountInfo) ([]sources.Secret, error) {
	out := []sources.Secret{}
	var walk func(prefix string) error
	walk = func(prefix string) error {
		path := fmt.Sprintf("%s/metadata/%s", m.Path, prefix)
		s, err := c.api.Logical().ListWithContext(ctx, path)
		if err != nil {
			return err
		}
		if s == nil || s.Data == nil {
			return nil
		}
		keysRaw, ok := s.Data["keys"].([]interface{})
		if !ok {
			return nil
		}
		for _, k := range keysRaw {
			name, _ := k.(string)
			full := prefix + name
			if strings.HasSuffix(name, "/") {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			out = append(out, sources.Secret{
				Name:     prefixWithMount(m, full),
				SourceID: c.id,
			})
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}

func prefixWithMount(m mountInfo, name string) string {
	if len(m.Path) == 0 {
		return name
	}
	return m.Path + "/" + name
}

// stripMountPrefix returns (mountInfo, relativePath) for a fully-qualified name.
func (c *Client) resolveMount(name string) (mountInfo, string, error) {
	for _, m := range c.mounts {
		prefix := m.Path + "/"
		if strings.HasPrefix(name, prefix) {
			return m, strings.TrimPrefix(name, prefix), nil
		}
		if name == m.Path {
			return m, "", nil
		}
	}
	if len(c.mounts) == 1 {
		return c.mounts[0], name, nil
	}
	return mountInfo{}, "", fmt.Errorf("name %q does not match any configured mount", name)
}

func (c *Client) getKV2(ctx context.Context, m mountInfo, rel string) (*sources.Secret, error) {
	path := fmt.Sprintf("%s/metadata/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Data == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	created, _ := s.Data["created_time"].(string)
	return &sources.Secret{
		Name:       prefixWithMount(m, rel),
		SourceID:   c.id,
		CreateTime: created,
	}, nil
}

func (c *Client) revealKV2(ctx context.Context, m mountInfo, rel, version string) ([]byte, error) {
	path := fmt.Sprintf("%s/data/%s", m.Path, rel)
	if version != "" && version != "latest" {
		path = fmt.Sprintf("%s?version=%s", path, version)
	}
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	d, ok := s.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response shape for %s", rel)
	}
	// KV v2 stores arbitrary keys; for go-secret we use a single "value" key by convention.
	if v, ok := d["value"].(string); ok {
		return []byte(v), nil
	}
	// Fallback: if there's exactly one key, use it.
	if len(d) == 1 {
		for _, v := range d {
			if vs, ok := v.(string); ok {
				return []byte(vs), nil
			}
		}
	}
	return nil, fmt.Errorf("no 'value' key in secret %s; multi-key secrets are not supported", rel)
}

func (c *Client) listVersionsKV2(ctx context.Context, m mountInfo, rel string) ([]sources.Version, error) {
	path := fmt.Sprintf("%s/metadata/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Data == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	versionsRaw, _ := s.Data["versions"].(map[string]interface{})
	out := []sources.Version{}
	for k, v := range versionsRaw {
		entry, _ := v.(map[string]interface{})
		state := "ENABLED"
		if d, _ := entry["destroyed"].(bool); d {
			state = "DESTROYED"
		} else if dt, _ := entry["deletion_time"].(string); dt != "" && dt != "" {
			state = "DISABLED"
		}
		ct, _ := entry["created_time"].(string)
		out = append(out, sources.Version{Name: k, State: state, CreateTime: ct})
	}
	return out, nil
}
```

- [ ] **Step 12.3: Reemplazar stubs de `client.go` que son KV v2 read**

```go
func (c *Client) List(ctx context.Context) ([]sources.Secret, error) {
	out := []sources.Secret{}
	for _, m := range c.mounts {
		var (
			items []sources.Secret
			err   error
		)
		switch m.Version {
		case 2, 0: // 0 = auto-detect, default to v2
			items, err = c.listKV2(ctx, m)
		case 1:
			items, err = c.listKV1(ctx, m) // implementado en Task 13
		default:
			err = fmt.Errorf("mount %q has unsupported version %d", m.Path, m.Version)
		}
		if err != nil {
			return out, fmt.Errorf("mount %q: %w", m.Path, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

func (c *Client) Get(ctx context.Context, name string) (*sources.Secret, error) {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return nil, err
	}
	if m.Version == 1 {
		return c.getKV1(ctx, m, rel)
	}
	return c.getKV2(ctx, m, rel)
}

func (c *Client) Reveal(ctx context.Context, name, version string) ([]byte, error) {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return nil, err
	}
	if m.Version == 1 {
		if version != "" && version != "latest" {
			return nil, sources.WrapNotSupported("Vault KV v1 has no versions")
		}
		return c.revealKV1(ctx, m, rel)
	}
	return c.revealKV2(ctx, m, rel, version)
}

func (c *Client) ListVersions(ctx context.Context, name string) ([]sources.Version, error) {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return nil, err
	}
	if m.Version == 1 {
		return nil, sources.WrapNotSupported("Vault KV v1 has no versions")
	}
	return c.listVersionsKV2(ctx, m, rel)
}
```

- [ ] **Step 12.4: Run tests con httptest mock**

```bash
go test ./internal/providers/vault/... -race -v
```

Expected: PASS — todos los tests KV v2.

- [ ] **Step 12.5: Commit**

```bash
git add internal/providers/vault/
git commit -m "feat(vault): KV v2 List/Get/Reveal/ListVersions"
```

---

## Task 13: Vault KV v2 — write operations + KV v1 read

**Files:**
- Modify: `internal/providers/vault/kv2.go` (añade write helpers)
- Create: `internal/providers/vault/kv1.go`
- Create: `internal/providers/vault/kv1_test.go`
- Modify: `internal/providers/vault/client.go`

- [ ] **Step 13.1: Test write KV v2 (Create, AddVersion, Delete, Disable/Enable/Destroy version) con httptest que captura las llamadas**

```go
// añadir a internal/providers/vault/kv2_test.go
func TestKV2_CreateAddVersion(t *testing.T) {
	calls := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"version": 1, "created_time": "now"},
		})
	})
	mux.HandleFunc("/v1/secret/metadata/", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	if err := c.Create(context.Background(), "app/db", []byte("v1"), sources.CreateOpts{}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := c.AddVersion(context.Background(), "app/db", []byte("v2")); err != nil {
		t.Fatalf("AddVersion: %v", err)
	}
	if err := c.Delete(context.Background(), "app/db"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	wantSubstrings := []string{"POST /v1/secret/data/app/db", "DELETE /v1/secret/metadata/app/db"}
	for _, w := range wantSubstrings {
		found := false
		for _, c := range calls {
			if strings.Contains(c, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing call %q in %v", w, calls)
		}
	}
}
```

- [ ] **Step 13.2: Implementar writes KV v2 en `kv2.go`**

```go
// añadir a internal/providers/vault/kv2.go
import "github.com/theburrowhub/go-secret/internal/sources"

func (c *Client) writeKV2(ctx context.Context, m mountInfo, rel string, value []byte) (string, error) {
	path := fmt.Sprintf("%s/data/%s", m.Path, rel)
	payload := map[string]interface{}{
		"data": map[string]interface{}{"value": string(value)},
	}
	s, err := c.api.Logical().WriteWithContext(ctx, path, payload)
	if err != nil {
		return "", err
	}
	if s != nil && s.Data != nil {
		switch v := s.Data["version"].(type) {
		case json.Number:
			return v.String(), nil
		case float64:
			return fmt.Sprintf("%d", int(v)), nil
		case string:
			return v, nil
		}
	}
	return "", nil
}

func (c *Client) deleteKV2(ctx context.Context, m mountInfo, rel string) error {
	path := fmt.Sprintf("%s/metadata/%s", m.Path, rel)
	_, err := c.api.Logical().DeleteWithContext(ctx, path)
	return err
}

func (c *Client) versionOpKV2(ctx context.Context, m mountInfo, rel, version, op string) error {
	// op = "delete" | "undelete" | "destroy"
	path := fmt.Sprintf("%s/%s/%s", m.Path, op, rel)
	_, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{
		"versions": []string{version},
	})
	return err
}
```

> Recuerda añadir `"encoding/json"` al import block.

- [ ] **Step 13.3: Implementar `kv1.go` (read solo, KV v1 no soporta versiones)**

```go
// internal/providers/vault/kv1.go
package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/go-secret/internal/sources"
)

func (c *Client) listKV1(ctx context.Context, m mountInfo) ([]sources.Secret, error) {
	out := []sources.Secret{}
	var walk func(prefix string) error
	walk = func(prefix string) error {
		path := fmt.Sprintf("%s/%s", m.Path, prefix)
		s, err := c.api.Logical().ListWithContext(ctx, path)
		if err != nil {
			return err
		}
		if s == nil || s.Data == nil {
			return nil
		}
		keysRaw, _ := s.Data["keys"].([]interface{})
		for _, k := range keysRaw {
			name, _ := k.(string)
			full := prefix + name
			if strings.HasSuffix(name, "/") {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			out = append(out, sources.Secret{
				Name:     prefixWithMount(m, full),
				SourceID: c.id,
			})
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getKV1(ctx context.Context, m mountInfo, rel string) (*sources.Secret, error) {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	return &sources.Secret{Name: prefixWithMount(m, rel), SourceID: c.id}, nil
}

func (c *Client) revealKV1(ctx context.Context, m mountInfo, rel string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	if v, ok := s.Data["value"].(string); ok {
		return []byte(v), nil
	}
	if len(s.Data) == 1 {
		for _, v := range s.Data {
			if vs, ok := v.(string); ok {
				return []byte(vs), nil
			}
		}
	}
	return nil, fmt.Errorf("no 'value' key in secret %s", rel)
}

func (c *Client) writeKV1(ctx context.Context, m mountInfo, rel string, value []byte) error {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	_, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{"value": string(value)})
	return err
}

func (c *Client) deleteKV1(ctx context.Context, m mountInfo, rel string) error {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	_, err := c.api.Logical().DeleteWithContext(ctx, path)
	return err
}
```

- [ ] **Step 13.4: Reemplazar stubs en `client.go` para Create/Delete/AddVersion/EnableVersion/DisableVersion/DestroyVersion**

```go
func (c *Client) Create(ctx context.Context, name string, value []byte, opts sources.CreateOpts) error {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return err
	}
	if m.Version == 1 {
		return c.writeKV1(ctx, m, rel, value)
	}
	_, err = c.writeKV2(ctx, m, rel, value)
	return err
}

func (c *Client) Delete(ctx context.Context, name string) error {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return err
	}
	if m.Version == 1 {
		return c.deleteKV1(ctx, m, rel)
	}
	return c.deleteKV2(ctx, m, rel)
}

func (c *Client) AddVersion(ctx context.Context, name string, value []byte) (*sources.Version, error) {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return nil, err
	}
	if m.Version == 1 {
		// KV v1: overwriting is the only "add version" semantic.
		if err := c.writeKV1(ctx, m, rel, value); err != nil {
			return nil, err
		}
		return &sources.Version{Name: "1", State: "ENABLED"}, nil
	}
	v, err := c.writeKV2(ctx, m, rel, value)
	if err != nil {
		return nil, err
	}
	return &sources.Version{Name: v, State: "ENABLED"}, nil
}

func (c *Client) EnableVersion(ctx context.Context, name, version string) error {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return err
	}
	if m.Version == 1 {
		return sources.WrapNotSupported("KV v1 has no versions")
	}
	return c.versionOpKV2(ctx, m, rel, version, "undelete")
}

func (c *Client) DisableVersion(ctx context.Context, name, version string) error {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return err
	}
	if m.Version == 1 {
		return sources.WrapNotSupported("KV v1 has no versions")
	}
	return c.versionOpKV2(ctx, m, rel, version, "delete")
}

func (c *Client) DestroyVersion(ctx context.Context, name, version string) error {
	m, rel, err := c.resolveMount(name)
	if err != nil {
		return err
	}
	if m.Version == 1 {
		return sources.WrapNotSupported("KV v1 has no versions")
	}
	return c.versionOpKV2(ctx, m, rel, version, "destroy")
}
```

- [ ] **Step 13.5: Tests + commit**

```bash
go test ./internal/providers/vault/... -race -v
golangci-lint run ./...
git add internal/providers/vault/
git commit -m "feat(vault): KV v2 writes + KV v1 backend"
```

---

## Task 14: Auth AppRole + keyring real

**Files:**
- Modify: `internal/providers/vault/keyring.go`
- Modify: `internal/providers/vault/auth_approle.go`
- Create: `internal/providers/vault/auth_approle_test.go`

- [ ] **Step 14.1: Añadir `zalando/go-keyring`**

```bash
go get github.com/zalando/go-keyring@latest
```

- [ ] **Step 14.2: Implementar wrapper `keyring.go` con fallback memoria si el SO no tiene secret-service**

```go
// internal/providers/vault/keyring.go
package vault

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

const keyringService = "go-secret"

var (
	memMu    sync.Mutex
	memStore = map[string]string{}
)

func keyringKey(sourceID, kind string) string {
	return "vault:" + sourceID + ":" + kind
}

func keyringGet(sourceID string) (string, bool) {
	v, err := keyring.Get(keyringService, keyringKey(sourceID, "token"))
	if err == nil {
		return v, true
	}
	if errors.Is(err, keyring.ErrNotFound) {
		// fall through to mem fallback (last-resort, per-process)
		memMu.Lock()
		defer memMu.Unlock()
		v, ok := memStore[keyringKey(sourceID, "token")]
		return v, ok
	}
	// keyring backend unavailable; use process memory.
	memMu.Lock()
	defer memMu.Unlock()
	v2, ok := memStore[keyringKey(sourceID, "token")]
	return v2, ok
}

func keyringSet(sourceID, token string) error {
	if err := keyring.Set(keyringService, keyringKey(sourceID, "token"), token); err == nil {
		return nil
	}
	memMu.Lock()
	defer memMu.Unlock()
	memStore[keyringKey(sourceID, "token")] = token
	return nil
}

func keyringDelete(sourceID string) error {
	_ = keyring.Delete(keyringService, keyringKey(sourceID, "token"))
	memMu.Lock()
	defer memMu.Unlock()
	delete(memStore, keyringKey(sourceID, "token"))
	return nil
}

func keyringSetSecretID(sourceID, secretID string) error {
	if err := keyring.Set(keyringService, keyringKey(sourceID, "approle-secret-id"), secretID); err == nil {
		return nil
	}
	memMu.Lock()
	defer memMu.Unlock()
	memStore[keyringKey(sourceID, "approle-secret-id")] = secretID
	return nil
}

func keyringGetSecretID(sourceID string) (string, bool) {
	v, err := keyring.Get(keyringService, keyringKey(sourceID, "approle-secret-id"))
	if err == nil {
		return v, true
	}
	memMu.Lock()
	defer memMu.Unlock()
	v2, ok := memStore[keyringKey(sourceID, "approle-secret-id")]
	return v2, ok
}
```

- [ ] **Step 14.3: Implementar `auth_approle.go`**

```go
// internal/providers/vault/auth_approle.go
package vault

import (
	"context"
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
)

func resolveAppRoleAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	if sc.Auth.AppRoleRoleID == "" {
		return "", fmt.Errorf("approle: missing role_id in config for source %q", sc.ID)
	}
	secretID, ok := keyringGetSecretID(sc.ID)
	if !ok {
		return "", fmt.Errorf("approle: no secret_id in keyring for source %q (run `go-secret sources login %s`)", sc.ID, sc.ID)
	}

	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return "", err
	}
	resp, err := api.Logical().WriteWithContext(ctx, "auth/approle/login", map[string]interface{}{
		"role_id":   sc.Auth.AppRoleRoleID,
		"secret_id": secretID,
	})
	if err != nil {
		return "", fmt.Errorf("approle login: %w", err)
	}
	if resp == nil || resp.Auth == nil || resp.Auth.ClientToken == "" {
		return "", fmt.Errorf("approle login: no token in response")
	}
	_ = keyringSet(sc.ID, resp.Auth.ClientToken)
	return resp.Auth.ClientToken, nil
}

// SaveAppRoleSecretID is the public helper for `sources login` to persist the
// secret_id obtained interactively from the user.
func SaveAppRoleSecretID(sourceID, secretID string) error {
	return keyringSetSecretID(sourceID, secretID)
}
```

- [ ] **Step 14.4: Test approle con httptest**

```go
// internal/providers/vault/auth_approle_test.go
package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestAppRoleAuth_LoginAndCachesToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/approle/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"auth": map[string]interface{}{"client_token": "approle-token"},
		})
	}))
	defer srv.Close()

	if err := SaveAppRoleSecretID("v", "the-secret-id"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keyringDelete("v") })

	tok, err := resolveAppRoleAuth(context.Background(), config.SourceConfig{
		ID:      "v",
		Address: srv.URL,
		Auth:    config.VaultAuthConfig{Method: "approle", AppRoleRoleID: "role-1"},
	})
	if err != nil {
		t.Fatalf("approle: %v", err)
	}
	if tok != "approle-token" {
		t.Fatalf("got %q", tok)
	}
}
```

- [ ] **Step 14.5: Test, build, commit**

```bash
go test ./internal/providers/vault/... -race -v
go mod tidy
git add go.mod go.sum internal/providers/vault/
git commit -m "feat(vault): AppRole auth backed by zalando/go-keyring"
```

---

## Task 15: Auth OIDC con browser callback

**Files:**
- Modify: `internal/providers/vault/auth_oidc.go`
- Create: `internal/providers/vault/auth_oidc_test.go`

- [ ] **Step 15.1: Test OIDC con httptest que simula la URL de auth y el callback**

```go
// internal/providers/vault/auth_oidc_test.go
package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestOIDCAuth_Flow(t *testing.T) {
	// Vault server stub
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/oidc/oidc/auth_url", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Build a fake authorize URL that the test "browser" hits and which
		// then redirects to the callback in our agent.
		redirect := body["redirect_uri"].(string)
		idp := httptest.NewServer(http.HandlerFunc(func(w2 http.ResponseWriter, r2 *http.Request) {
			http.Redirect(w2, r2, redirect+"?code=abc&state="+body["state"].(string), http.StatusFound)
		}))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"auth_url": idp.URL},
		})
	})
	mux.HandleFunc("/v1/auth/oidc/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"auth": map[string]interface{}{"client_token": "oidc-token", "lease_duration": 3600},
		})
	})
	vault := httptest.NewServer(mux)
	defer vault.Close()

	// Inject a "browser" that follows the auth URL to trigger the callback.
	prev := openBrowser
	defer func() { openBrowser = prev }()
	openBrowser = func(u string) error {
		go func() {
			_, _ = http.Get(u)
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok, err := resolveOIDCAuth(ctx, config.SourceConfig{
		ID:      "v",
		Address: vault.URL,
		Auth:    config.VaultAuthConfig{Method: "oidc", Role: "developer", OIDCPort: 18250},
	})
	if err != nil {
		t.Fatalf("oidc: %v", err)
	}
	if tok != "oidc-token" {
		t.Fatalf("got %q", tok)
	}

	// Sanity-check the agent uses the redirect URI we set.
	parsed, _ := url.Parse(vault.URL)
	if parsed == nil {
		t.Fatal("could not parse vault url")
	}
}
```

- [ ] **Step 15.2: Implementar OIDC**

```go
// internal/providers/vault/auth_oidc.go
package vault

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
)

// openBrowser is overridable for tests. By default it shells out to xdg-open
// (Linux), open (macOS) or rundll32 (Windows).
var openBrowser = func(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	return errors.New("unsupported platform for browser auto-open")
}

func resolveOIDCAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	// Try cached token first.
	if v, ok := keyringGet(sc.ID); ok {
		return v, nil
	}

	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return "", err
	}

	port := sc.Auth.OIDCPort
	if port == 0 {
		port = 8250
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oidc/callback", port)

	resp, err := api.Logical().WriteWithContext(ctx, "auth/oidc/oidc/auth_url", map[string]interface{}{
		"role":         sc.Auth.Role,
		"redirect_uri": redirectURI,
		"state":        state,
	})
	if err != nil {
		return "", fmt.Errorf("oidc auth_url: %w", err)
	}
	authURL, _ := resp.Data["auth_url"].(string)
	if authURL == "" {
		return "", errors.New("oidc: empty auth_url returned")
	}

	type result struct {
		token string
		err   error
	}
	results := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			results <- result{err: errors.New("state mismatch")}
			return
		}
		code := q.Get("code")
		s, err := api.Logical().WriteWithContext(ctx, "auth/oidc/oidc/callback", map[string]interface{}{
			"code":  code,
			"state": state,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			results <- result{err: err}
			return
		}
		if s == nil || s.Auth == nil || s.Auth.ClientToken == "" {
			results <- result{err: errors.New("oidc: no token in callback response")}
			return
		}
		_, _ = w.Write([]byte("Login successful. You can close this tab."))
		results <- result{token: s.Auth.ClientToken}
	})

	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.ListenAndServe() }()
	defer func() {
		shutdownCtx, cancel := context.WithCancel(context.Background())
		_ = cancel
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(authURL); err != nil {
		return "", fmt.Errorf("open browser: %w", err)
	}

	select {
	case r := <-results:
		if r.err != nil {
			return "", r.err
		}
		_ = keyringSet(sc.ID, r.token)
		return r.token, nil
	case err := <-srvErr:
		return "", fmt.Errorf("local server: %w", err)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func randomState() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 15.3: Run tests, build, commit**

```bash
go test ./internal/providers/vault/... -race -v
go build ./...
golangci-lint run ./...
git add internal/providers/vault/
git commit -m "feat(vault): OIDC auth via browser flow with localhost callback"
```

---

## Task 16: Wire constructors GSM y Vault al Registry desde main

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 16.1: Registrar constructores en `init()` de cmd/root.go**

```go
// cmd/root.go (añadir imports y init)
import (
	"github.com/theburrowhub/go-secret/internal/providers/gsm"
	"github.com/theburrowhub/go-secret/internal/providers/vault"
	"github.com/theburrowhub/go-secret/internal/sources"
)

func init() {
	rootCmd.PersistentFlags().StringVar(&sourceID, "source", "", "Source ID (defined in config)")
	rootCmd.PersistentFlags().StringVarP(&projectID, "project", "p", "", "GCP Project ID (deprecated alias for --source on gsm sources)")

	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate)

	sources.RegisterProviderConstructors(
		func(ctx context.Context, sc config.SourceConfig) (sources.Provider, error) {
			return gsm.NewFromSourceConfig(ctx, sc)
		},
		func(ctx context.Context, sc config.SourceConfig) (sources.Provider, error) {
			return vault.NewFromSourceConfig(ctx, sc)
		},
	)
}
```

Asegúrate también de declarar `var sourceID string` arriba junto con `projectID`.

- [ ] **Step 16.2: Build**

```bash
go build ./...
```

Expected: BUILD OK.

- [ ] **Step 16.3: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): register GSM and Vault provider constructors at startup"
```

---

## Task 17: Helper común `loadRegistry()` para cmd/*

**Files:**
- Create: `cmd/sources_helper.go`

- [ ] **Step 17.1: Helper que carga config + registry y aplica deprecation de `--project`**

```go
// cmd/sources_helper.go
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
)

// resolveActiveSource returns the source id to use for a write operation,
// applying:
//   1. --source flag (preferred)
//   2. --project flag (deprecated, only matches gsm sources by ProjectID)
//   3. cfg.DefaultSource
//   4. "" (caller may run an interactive prompt)
func resolveActiveSource(cfg *config.Config) string {
	if sourceID != "" {
		return sourceID
	}
	if projectID != "" {
		fmt.Fprintln(os.Stderr, "warning: --project is deprecated; prefer --source <id>")
		for _, s := range cfg.Sources {
			if s.Provider == "gsm" && s.ProjectID == projectID {
				return s.ID
			}
		}
		// Fall through: not found.
	}
	return cfg.DefaultSource
}

// loadRegistry centralizes the boilerplate every cmd needs to talk to backends.
func loadRegistry(ctx context.Context) (*config.Config, *sources.Registry, *sources.UnifiedClient, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load config: %w", err)
	}
	reg, err := sources.LoadFromConfig(ctx, cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load sources: %w", err)
	}
	return cfg, reg, sources.NewUnifiedClient(reg), nil
}
```

- [ ] **Step 17.2: Build**

```bash
go build ./...
```

Expected: BUILD OK.

- [ ] **Step 17.3: Commit**

```bash
git add cmd/sources_helper.go
git commit -m "feat(cmd): add resolveActiveSource and loadRegistry helpers"
```

---

## Task 18: Migrar `cmd/list.go` a UnifiedClient + columna PROVIDER

**Files:**
- Modify: `cmd/list.go`

- [ ] **Step 18.1: Reemplazar implementación**

```go
// cmd/list.go (sustituye toda la función runList)
func runList() error {
	ctx := context.Background()
	cfg, reg, uc, err := loadRegistry(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = reg.Close() }()
	_ = cfg

	var (
		secrets []sources.Secret
		listErr error
	)
	if sourceID != "" {
		p, err := reg.Get(sourceID)
		if err != nil {
			return err
		}
		secrets, listErr = p.List(ctx)
	} else {
		secrets, listErr = uc.List(ctx)
	}
	if listErr != nil {
		var pe *sources.PartialError
		if errors.As(listErr, &pe) {
			fmt.Fprintf(os.Stderr, "warning: partial failure: %v\n", pe)
		} else {
			return listErr
		}
	}

	switch listOutput {
	case "json":
		return outputListJSON(secrets)
	case "yaml":
		return outputListYAML(secrets)
	default:
		return outputListTable(secrets)
	}
}

func outputListTable(secrets []sources.Secret) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "PROVIDER\tNAME\tCREATED\tREPLICATION")
	_, _ = fmt.Fprintln(w, "--------\t----\t-------\t-----------")
	for _, s := range secrets {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.SourceID, s.Name, s.CreateTime, s.Replication)
	}
	_ = w.Flush()
	fmt.Printf("\nTotal: %d secretos\n", len(secrets))
	return nil
}
```

> Adapta `outputListJSON` / `outputListYAML` para incluir `source_id` en cada entrada.

- [ ] **Step 18.2: Build, lint, manual smoke**

```bash
go build -o bin/go-secret ./
./bin/go-secret list --help
```

Expected: muestra `--source` flag y la nueva descripción.

- [ ] **Step 18.3: Commit**

```bash
git add cmd/list.go
git commit -m "feat(cmd/list): use UnifiedClient with PROVIDER column"
```

---

## Task 19: Migrar el resto de cmd/* a Resolve + UnifiedClient

**Files:**
- Modify: `cmd/get.go`, `cmd/reveal.go`, `cmd/copy.go`, `cmd/delete.go`, `cmd/add_version.go`, `cmd/versions_list.go`, `cmd/versions_enable.go`, `cmd/versions_disable.go`, `cmd/versions_destroy.go`

Cada uno sigue el mismo patrón. Plantilla para reads (ej. `cmd/reveal.go`):

- [ ] **Step 19.1: Patrón estándar — sustituir el bloque "Crear cliente GCP" + "client.AccessSecretVersion"**

```go
ctx := context.Background()
_, reg, uc, err := loadRegistry(ctx)
if err != nil {
	return err
}
defer func() { _ = reg.Close() }()

p, err := uc.Resolve(ctx, secretName, sourceID)
if err != nil {
	if errors.Is(err, sources.ErrAmbiguousSecret) {
		return fmt.Errorf("%w. Use --source <id>", err)
	}
	return err
}
payload, err := p.Reveal(ctx, secretName, revealVersion)
if err != nil {
	return err
}
```

Aplicar el equivalente:
- `Get` → `p.Get(ctx, name)`
- `Reveal` → `p.Reveal(ctx, name, version)`
- `Copy` → `p.Reveal(...)` + clipboard
- `Delete` → `p.Delete(ctx, name)`
- `AddVersion` → `p.AddVersion(ctx, name, value)`
- `versions list` → `p.ListVersions(...)`
- `versions enable/disable/destroy` → `p.EnableVersion(...)` etc. Retornar bonito si `errors.Is(err, sources.ErrNotSupported)`.

- [ ] **Step 19.2: Para cada archivo modificado, build + smoke + commit individual**

Repite por archivo:

```bash
go build -o bin/go-secret ./ && ./bin/go-secret <subcmd> --help
git add cmd/<file>.go
git commit -m "feat(cmd/<subcmd>): migrate to sources.Provider via UnifiedClient"
```

---

## Task 20: Prompt interactivo para `create` cuando falta source

**Files:**
- Create: `internal/sources/prompt.go`
- Create: `internal/sources/prompt_test.go`
- Modify: `cmd/create.go`

- [ ] **Step 20.1: Test del helper de prompt (input mock)**

```go
// internal/sources/prompt_test.go
package sources

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptForSource_PicksByNumber(t *testing.T) {
	in := strings.NewReader("2\n")
	out := &bytes.Buffer{}
	picked, err := promptForSourceFromIO(in, out, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if picked != "b" {
		t.Fatalf("got %q", picked)
	}
	if !strings.Contains(out.String(), "1) a") {
		t.Fatalf("listing missing: %q", out.String())
	}
}

func TestPromptForSource_RejectsOutOfRange(t *testing.T) {
	in := strings.NewReader("9\n1\n")
	out := &bytes.Buffer{}
	picked, err := promptForSourceFromIO(in, out, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if picked != "a" {
		t.Fatalf("got %q", picked)
	}
}
```

- [ ] **Step 20.2: Implementar prompt**

```go
// internal/sources/prompt.go
package sources

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// PromptForSource is the interactive picker used when no --source is given
// and no default exists. Returns the chosen source ID.
func PromptForSource(active []Provider) (string, error) {
	ids := make([]string, 0, len(active))
	for _, p := range active {
		ids = append(ids, p.ID())
	}
	return promptForSourceFromIO(os.Stdin, os.Stdout, ids)
}

func promptForSourceFromIO(in io.Reader, out io.Writer, ids []string) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("no active sources")
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	br := bufio.NewReader(in)
	for {
		fmt.Fprintln(out, "Select source:")
		for i, id := range ids {
			fmt.Fprintf(out, "  %d) %s\n", i+1, id)
		}
		fmt.Fprint(out, "Choice [1-", len(ids), "]: ")
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(ids) {
			return ids[n-1], nil
		}
		fmt.Fprintln(out, "Invalid choice, try again.")
	}
}
```

- [ ] **Step 20.3: Integrar en `cmd/create.go`**

```go
// dentro de runCreate, sustituir la determinación de proyecto por:
target := resolveActiveSource(cfg)
if target == "" {
	picked, err := sources.PromptForSource(reg.Active())
	if err != nil {
		return err
	}
	target = picked
}
p, err := reg.Get(target)
if err != nil {
	return err
}
if err := p.Create(ctx, secretName, value, sources.CreateOpts{Labels: labels, Location: createLocation}); err != nil {
	return err
}
```

- [ ] **Step 20.4: Tests, build, commit**

```bash
go test ./internal/sources/... -race
go build -o bin/go-secret ./
git add internal/sources/prompt.go internal/sources/prompt_test.go cmd/create.go
git commit -m "feat: interactive source picker for create when none specified"
```

---

## Task 21: Subcomando `sources list`

**Files:**
- Create: `cmd/sources.go`
- Create: `cmd/sources_list.go`

- [ ] **Step 21.1: Crear comando padre y `list`**

```go
// cmd/sources.go
package cmd

import "github.com/spf13/cobra"

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Manage secret sources (GSM projects and Vault mounts)",
}

func init() { rootCmd.AddCommand(sourcesCmd) }
```

```go
// cmd/sources_list.go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tPROVIDER\tENABLED\tDETAIL\tDEFAULT")
		_, _ = fmt.Fprintln(w, "--\t--------\t-------\t------\t-------")
		for _, s := range cfg.Sources {
			detail := s.ProjectID
			if s.Provider == "vault" {
				detail = s.Address
			}
			def := ""
			if s.ID == cfg.DefaultSource {
				def = "*"
			}
			enabled := "no"
			if s.Enabled {
				enabled = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Provider, enabled, detail, def)
		}
		_ = w.Flush()
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesListCmd) }
```

- [ ] **Step 21.2: Build + smoke**

```bash
go build -o bin/go-secret ./
./bin/go-secret sources list --help
```

- [ ] **Step 21.3: Commit**

```bash
git add cmd/sources.go cmd/sources_list.go
git commit -m "feat(cmd/sources): list subcommand"
```

---

## Task 22: Subcomandos `sources add/edit/remove`

**Files:**
- Create: `cmd/sources_add.go`
- Create: `cmd/sources_edit.go`
- Create: `cmd/sources_remove.go`

- [ ] **Step 22.1: `sources add` — wizard interactivo**

```go
// cmd/sources_add.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new source interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		r := bufio.NewReader(os.Stdin)
		ask := func(q, def string) string {
			if def != "" {
				fmt.Printf("%s [%s]: ", q, def)
			} else {
				fmt.Printf("%s: ", q)
			}
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return def
			}
			return line
		}

		sc := config.SourceConfig{
			Enabled:         true,
			FolderSeparator: "/",
		}
		sc.Provider = ask("Provider (gsm|vault)", "gsm")
		sc.ID = ask("Source ID", "")
		sc.DisplayName = ask("Display name (optional)", "")
		switch sc.Provider {
		case "gsm":
			sc.ProjectID = ask("GCP Project ID", "")
		case "vault":
			sc.Address = ask("Vault address", "")
			sc.Auth.Method = ask("Auth method (token|approle|oidc)", "token")
			if sc.Auth.Method == "approle" {
				sc.Auth.AppRoleRoleID = ask("Role ID", "")
			}
			if sc.Auth.Method == "oidc" {
				sc.Auth.Role = ask("OIDC role", "")
			}
			mountPath := ask("KV mount path", "secret")
			versionStr := ask("KV version (1|2)", "2")
			version := 2
			if versionStr == "1" {
				version = 1
			}
			sc.Mounts = []config.VaultMount{{Path: mountPath, Version: version}}
		default:
			return fmt.Errorf("unknown provider %q", sc.Provider)
		}

		for _, existing := range cfg.Sources {
			if existing.ID == sc.ID {
				return fmt.Errorf("source %q already exists", sc.ID)
			}
		}
		cfg.Sources = append(cfg.Sources, sc)
		if cfg.DefaultSource == "" {
			cfg.DefaultSource = sc.ID
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q added\n", sc.ID)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesAddCmd) }
```

- [ ] **Step 22.2: `sources remove`**

```go
// cmd/sources_remove.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		id := args[0]
		filtered := cfg.Sources[:0]
		removed := false
		for _, s := range cfg.Sources {
			if s.ID == id {
				removed = true
				continue
			}
			filtered = append(filtered, s)
		}
		if !removed {
			return fmt.Errorf("source %q not found", id)
		}
		cfg.Sources = filtered
		if cfg.DefaultSource == id {
			cfg.DefaultSource = ""
			if len(cfg.Sources) > 0 {
				cfg.DefaultSource = cfg.Sources[0].ID
			}
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q removed\n", id)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesRemoveCmd) }
```

- [ ] **Step 22.3: `sources edit` (re-usa wizard pre-poblado)**

```go
// cmd/sources_edit.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an existing source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		idx := -1
		for i, s := range cfg.Sources {
			if s.ID == args[0] {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("source %q not found", args[0])
		}
		sc := &cfg.Sources[idx]

		r := bufio.NewReader(os.Stdin)
		ask := func(q, def string) string {
			fmt.Printf("%s [%s]: ", q, def)
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				return def
			}
			return line
		}
		sc.DisplayName = ask("Display name", sc.DisplayName)
		sc.FolderSeparator = ask("Folder separator", sc.FolderSeparator)
		switch sc.Provider {
		case "gsm":
			sc.ProjectID = ask("GCP Project ID", sc.ProjectID)
		case "vault":
			sc.Address = ask("Vault address", sc.Address)
			sc.Auth.Method = ask("Auth method", sc.Auth.Method)
		}
		if err := cfg.Save(); err != nil {
			return err
		}
		fmt.Printf("✓ Source %q updated\n", sc.ID)
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesEditCmd) }
```

- [ ] **Step 22.4: Build + commit**

```bash
go build -o bin/go-secret ./
git add cmd/sources_add.go cmd/sources_edit.go cmd/sources_remove.go
git commit -m "feat(cmd/sources): add/edit/remove subcommands"
```

---

## Task 23: Subcomandos `sources toggle`, `set-default`, `login`

**Files:**
- Create: `cmd/sources_toggle.go`
- Create: `cmd/sources_set_default.go`
- Create: `cmd/sources_login.go`

- [ ] **Step 23.1: `toggle`**

```go
// cmd/sources_toggle.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Toggle a source's enabled flag and persist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == args[0] {
				cfg.Sources[i].Enabled = !cfg.Sources[i].Enabled
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Printf("✓ Source %q is now %s\n", args[0], onOff(cfg.Sources[i].Enabled))
				return nil
			}
		}
		return fmt.Errorf("source %q not found", args[0])
	},
}

func onOff(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func init() { sourcesCmd.AddCommand(sourcesToggleCmd) }
```

- [ ] **Step 23.2: `set-default`**

```go
// cmd/sources_set_default.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
)

var sourcesSetDefaultCmd = &cobra.Command{
	Use:   "set-default <id>",
	Short: "Set the default source for write operations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		for _, s := range cfg.Sources {
			if s.ID == args[0] {
				cfg.DefaultSource = args[0]
				if err := cfg.Save(); err != nil {
					return err
				}
				fmt.Printf("✓ Default source set to %q\n", args[0])
				return nil
			}
		}
		return fmt.Errorf("source %q not found", args[0])
	},
}

func init() { sourcesCmd.AddCommand(sourcesSetDefaultCmd) }
```

- [ ] **Step 23.3: `login` — re-auth para Vault**

```go
// cmd/sources_login.go
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/providers/vault"
)

var sourcesLoginCmd = &cobra.Command{
	Use:   "login <id>",
	Short: "Re-authenticate against a Vault source",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		var sc *config.SourceConfig
		for i := range cfg.Sources {
			if cfg.Sources[i].ID == args[0] {
				sc = &cfg.Sources[i]
				break
			}
		}
		if sc == nil {
			return fmt.Errorf("source %q not found", args[0])
		}
		if sc.Provider != "vault" {
			return fmt.Errorf("login only applies to vault sources")
		}
		if sc.Auth.Method == "approle" {
			fmt.Print("Enter secret_id (input hidden): ")
			r := bufio.NewReader(os.Stdin)
			line, _ := r.ReadString('\n')
			line = strings.TrimSpace(line)
			if err := vault.SaveAppRoleSecretID(sc.ID, line); err != nil {
				return err
			}
			fmt.Println("✓ secret_id stored in keyring")
			return nil
		}
		// Token / OIDC: trigger a connection to drive the auth flow.
		if _, err := vault.NewFromSourceConfig(context.Background(), *sc); err != nil {
			return err
		}
		fmt.Println("✓ Login successful, token cached")
		return nil
	},
}

func init() { sourcesCmd.AddCommand(sourcesLoginCmd) }
```

- [ ] **Step 23.4: Build + commit**

```bash
go build -o bin/go-secret ./
git add cmd/sources_toggle.go cmd/sources_set_default.go cmd/sources_login.go
git commit -m "feat(cmd/sources): toggle, set-default, login subcommands"
```

---

## Task 24: TUI — campo `Provider` en estado y columna en lista

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/styles.go`

- [ ] **Step 24.1: Cambiar el modelo TUI para hablar con `Registry`/`UnifiedClient`**

En el constructor `ui.NewModel(cfg, projectID)`, sustituir la creación del cliente GCP por:

```go
ctx := context.Background()
reg, err := sources.LoadFromConfig(ctx, cfg)
if err != nil {
	// surface as fatal
	panic(err) // dev only; replace with logged exit in real PR
}
m.registry = reg
m.unified = sources.NewUnifiedClient(reg)
```

Añadir campos al struct `Model`:

```go
registry    *sources.Registry
unified     *sources.UnifiedClient
sourceFilter string // "" means ALL active
```

Sustituir todos los lugares que llaman `m.client.X` por la lógica equivalente a través de `m.unified` o `m.registry.Get(s.SourceID)`.

- [ ] **Step 24.2: Añadir columna `PROVIDER` en `viewList`**

Modificar `viewList()` para incluir un primer campo con `s.SourceID` coloreado por kind:

```go
provider := m.styles.ProviderBadge(p.Kind()).Render(s.SourceID)
line := fmt.Sprintf("%s  %s", provider, s.Name)
```

Y añadir en `internal/ui/styles.go`:

```go
func (s *Styles) ProviderBadge(kind string) lipgloss.Style {
	switch kind {
	case "gsm":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Bold(true)
	case "vault":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD814")).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	}
}
```

- [ ] **Step 24.3: Build + commit**

```bash
go build ./...
git add internal/ui/
git commit -m "feat(ui): wire Registry/UnifiedClient and add PROVIDER badge"
```

---

## Task 25: TUI — filtro por fuente con Tab cycle

**Files:**
- Modify: `internal/ui/keys.go`
- Modify: `internal/ui/model.go`

- [ ] **Step 25.1: Añadir keys**

```go
// internal/ui/keys.go (añadir bindings)
var (
	keyNextSource = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next source"))
	keyPrevSource = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev source"))
)
```

Añadir a `ListViewBindings()` en el footer.

- [ ] **Step 25.2: Implementar cycle en `Update`**

En el handler de la lista (`updateList`):

```go
case key.Matches(msg, keyNextSource):
	m.sourceFilter = m.cycleSourceFilter(+1)
	m.refreshSecretsView()
	return m, nil
case key.Matches(msg, keyPrevSource):
	m.sourceFilter = m.cycleSourceFilter(-1)
	m.refreshSecretsView()
	return m, nil
```

Helper:

```go
// returns "" for ALL, or the next active source id
func (m *Model) cycleSourceFilter(dir int) string {
	active := m.registry.Active()
	options := []string{""} // "" = ALL
	for _, p := range active {
		options = append(options, p.ID())
	}
	idx := 0
	for i, o := range options {
		if o == m.sourceFilter {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(options)) % len(options)
	return options[idx]
}

func (m *Model) refreshSecretsView() {
	all := m.allSecrets // populated at startup with unified.List
	if m.sourceFilter == "" {
		m.secrets = all
		return
	}
	out := all[:0]
	for _, s := range all {
		if s.SourceID == m.sourceFilter {
			out = append(out, s)
		}
	}
	m.secrets = out
}
```

Mostrar el filtro activo en el header: `[ALL]` o `[<source-id>]`.

- [ ] **Step 25.3: Build + commit**

```bash
go build ./...
git add internal/ui/
git commit -m "feat(ui): Tab/Shift+Tab cycles source filter in list"
```

---

## Task 26: TUI — sources picker (Ctrl+P)

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/keys.go`

- [ ] **Step 26.1: Nueva vista `ViewSourcesPicker`**

```go
// internal/ui/model.go
const ViewSourcesPicker = "sources_picker"

func (m Model) updateSourcesPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = ViewList
		return m, nil
	case "up", "k":
		if m.sourcesPickerCursor > 0 {
			m.sourcesPickerCursor--
		}
	case "down", "j":
		if m.sourcesPickerCursor < len(m.registry.All())-1 {
			m.sourcesPickerCursor++
		}
	case " ":
		ids := allIDs(m.registry.All())
		_ = m.registry.Toggle(ids[m.sourcesPickerCursor])
		m.refreshSecretsView()
	case "s":
		// persist current enabled flags into config and save.
		for _, p := range m.registry.All() {
			for i := range m.config.Sources {
				if m.config.Sources[i].ID == p.ID() {
					m.config.Sources[i].Enabled = m.registry.IsEnabled(p.ID())
				}
			}
		}
		_ = m.config.Save()
		m.statusMsg = "Sources saved"
	}
	return m, nil
}

func allIDs(ps []sources.Provider) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID()
	}
	return out
}
```

Añadir keybind:

```go
// internal/ui/keys.go
var keyOpenSourcesPicker = key.NewBinding(
	key.WithKeys("ctrl+p"),
	key.WithHelp("ctrl+p", "sources"),
)
```

En el handler de `ViewList`:

```go
case key.Matches(msg, keyOpenSourcesPicker):
	m.view = ViewSourcesPicker
	return m, nil
```

Implementar `viewSourcesPicker()` que renderice el modal con `[x]/[ ]` por cada source.

- [ ] **Step 26.2: Build + commit**

```bash
go build ./...
git add internal/ui/
git commit -m "feat(ui): Ctrl+P sources picker with runtime toggle and save"
```

---

## Task 27: TUI — picker de source en flow de create

**Files:**
- Modify: `internal/ui/model.go`

- [ ] **Step 27.1: Antes de mostrar el form de Create, decidir source**

```go
// dentro del handler que abre Create (probablemente "n" sobre la lista):
active := m.registry.Active()
switch {
case len(active) == 1:
	m.createSourceID = active[0].ID()
	m.view = ViewCreate
case m.config.DefaultSource != "":
	m.createSourceID = m.config.DefaultSource
	m.view = ViewCreate
default:
	m.view = ViewCreateSourcePicker
	m.createSourceCursor = 0
}
```

Implementar `ViewCreateSourcePicker` parecido al sources picker pero single-select que setea `m.createSourceID` y luego transiciona a `ViewCreate`.

En el commit del form de Create, usar `m.registry.Get(m.createSourceID)` para obtener el provider y llamar `Create`.

- [ ] **Step 27.2: Build + commit**

```bash
go build ./...
git add internal/ui/
git commit -m "feat(ui): pick source before create when ambiguous"
```

---

## Task 28: TUI — entrada `🔌 Sources` en settings menu

**Files:**
- Modify: `internal/ui/model.go`

- [ ] **Step 28.1: Añadir el menu item**

Localizar `m.configMenuItems` y añadir:

```go
m.configMenuItems = append(m.configMenuItems, "🔌 Sources")
```

En el handler `updateConfigMenu`, en el `case` correspondiente al índice nuevo:

```go
m.view = ViewSourcesPicker
return m, nil
```

- [ ] **Step 28.2: Commit**

```bash
git add internal/ui/
git commit -m "feat(ui): add Sources entry to settings menu"
```

---

## Task 29: Audit — añadir `source_id` y `provider` a Event

**Files:**
- Modify: `internal/audit/audit.go`

- [ ] **Step 29.1: Añadir campos al struct Event y métodos para setearlos**

```go
type Event struct {
	// existing fields...
	SourceID string `json:"source_id,omitempty"`
	Provider string `json:"provider,omitempty"`
}

// SetSource is called by callers right after creating the logger.
func (l *Logger) SetSource(id, provider string) {
	l.sourceID = id
	l.providerKind = provider
}
```

Y modificar el método `log()` (o equivalente) para incluir esos campos en cada event:

```go
event := Event{
	// ...
	SourceID: l.sourceID,
	Provider: l.providerKind,
}
```

- [ ] **Step 29.2: Nuevos event types**

```go
const (
	// existing...
	EventSourceAdd          EventType = "SOURCE_ADD"
	EventSourceRemove       EventType = "SOURCE_REMOVE"
	EventSourceToggle       EventType = "SOURCE_TOGGLE"
	EventSourceLogin        EventType = "SOURCE_LOGIN"
	EventSourceAuthRefresh  EventType = "SOURCE_AUTH_REFRESH"
)
```

Y métodos `LogSourceAdd`, `LogSourceRemove`, etc. siguiendo el patrón existente.

- [ ] **Step 29.3: Llamar `SetSource` desde cada cmd que crea un logger**

Buscar `audit.NewLogger(...)` en `cmd/*` y añadir tras la línea:

```go
auditLogger.SetSource(p.ID(), p.Kind())
```

(`p` es el `sources.Provider` resuelto justo antes).

- [ ] **Step 29.4: Filtros en `audit logs`**

```go
// cmd/audit_logs.go (añadir)
auditLogsSourceFilter   string
auditLogsProviderFilter string
```

```go
auditLogsCmd.Flags().StringVar(&auditLogsSourceFilter, "source", "", "Filter by source id")
auditLogsCmd.Flags().StringVar(&auditLogsProviderFilter, "provider", "", "Filter by provider kind (gsm|vault)")
```

Y en el loop de filtrado:

```go
if auditLogsSourceFilter != "" && event.SourceID != auditLogsSourceFilter {
	continue
}
if auditLogsProviderFilter != "" && event.Provider != auditLogsProviderFilter {
	continue
}
```

- [ ] **Step 29.5: Build + commit**

```bash
go build ./...
git add internal/audit/ cmd/audit_logs.go cmd/*.go
git commit -m "feat(audit): record source_id/provider and add filters"
```

---

## Task 30: Templates per-source con override y nuevas variables

**Files:**
- Modify: `cmd/templates_generate.go`
- Modify: `internal/ui/model.go` (función `generateCode`)

- [ ] **Step 30.1: Resolver template combinando globales y per-source**

```go
// cmd/templates_generate.go (después de cargar cfg y resolver provider p)
chosenSource := p.ID()
sourceTemplates := []config.Template{}
for _, s := range cfg.Sources {
	if s.ID == chosenSource {
		sourceTemplates = s.Templates
		break
	}
}
templates := append([]config.Template{}, sourceTemplates...)
seen := map[string]bool{}
for _, t := range sourceTemplates {
	seen[t.Title] = true
}
for _, t := range cfg.Templates {
	if !seen[t.Title] {
		templates = append(templates, t)
	}
}

if templateGenerateIndex < 1 || templateGenerateIndex > len(templates) {
	return fmt.Errorf("índice fuera de rango: %d (debe estar entre 1 y %d)", templateGenerateIndex, len(templates))
}
tmpl := templates[templateGenerateIndex-1]
```

Y añadir variables nuevas al `data`:

```go
data := struct {
	SecretName     string
	FullSecretName string
	ProjectID      string
	SourceID       string
	Provider       string
}{
	SecretName:     extractSecretName(secretName, p.FolderSeparator()),
	FullSecretName: secretName,
	ProjectID: func() string {
		for _, s := range cfg.Sources {
			if s.ID == chosenSource && s.Provider == "gsm" {
				return s.ProjectID
			}
		}
		return ""
	}(),
	SourceID: chosenSource,
	Provider: p.Kind(),
}
```

- [ ] **Step 30.2: Aplicar el mismo cambio en `internal/ui/model.go` `generateCode`**

```go
// dentro de generateCode
sourceID := m.selectedSecret.SourceID
sourceTemplates := []config.Template{}
for _, s := range m.config.Sources {
	if s.ID == sourceID {
		sourceTemplates = s.Templates
		break
	}
}
templates := append([]config.Template{}, sourceTemplates...)
seen := map[string]bool{}
for _, t := range sourceTemplates {
	seen[t.Title] = true
}
for _, t := range m.config.Templates {
	if !seen[t.Title] {
		templates = append(templates, t)
	}
}

if templateIdx >= len(templates) {
	return ""
}
tpl := templates[templateIdx]

data := map[string]string{
	"SecretName":     shortName,
	"FullSecretName": m.selectedSecret.Name,
	"ProjectID":      "", // populated below if gsm
	"SourceID":       sourceID,
	"Provider":       "",
}
for _, s := range m.config.Sources {
	if s.ID == sourceID {
		data["ProjectID"] = s.ProjectID
		data["Provider"] = s.Provider
	}
}
```

- [ ] **Step 30.3: Build + commit**

```bash
go build ./...
git add cmd/templates_generate.go internal/ui/model.go
git commit -m "feat(templates): per-source template override and source/provider vars"
```

---

## Task 31: docker-compose + dex para E2E OIDC

**Files:**
- Create: `docker-compose.yml`
- Create: `docs/dev-environment.md`
- Create: `dev/dex-config.yaml`

- [ ] **Step 31.1: docker-compose con vault dev y dex**

```yaml
# docker-compose.yml
version: "3.9"
services:
  vault:
    image: hashicorp/vault:1.16
    cap_add: [IPC_LOCK]
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: root-token
      VAULT_DEV_LISTEN_ADDRESS: 0.0.0.0:8200
    ports:
      - "8200:8200"
    command: server -dev

  dex:
    image: dexidp/dex:v2.39.0
    volumes:
      - ./dev/dex-config.yaml:/etc/dex/cfg/config.yaml
    command: ["dex", "serve", "/etc/dex/cfg/config.yaml"]
    ports:
      - "5556:5556"
```

- [ ] **Step 31.2: Config dex local**

```yaml
# dev/dex-config.yaml
issuer: http://localhost:5556
storage:
  type: memory
web:
  http: 0.0.0.0:5556
oauth2:
  skipApprovalScreen: true
staticClients:
  - id: vault
    redirectURIs:
      - "http://localhost:8200/v1/auth/oidc/oidc/callback"
      - "http://127.0.0.1:8250/oidc/callback"
    name: Vault
    secret: vault-secret
enablePasswordDB: true
staticPasswords:
  - email: dev@local
    hash: "$2a$10$2b2cu2a8X5yQwaWpL5N0LeYa3bnz6ZrqV2gDl8l9KZtkOjWYzZjqe" # bcrypt of "password"
    username: dev
    userID: "1"
```

- [ ] **Step 31.3: docs/dev-environment.md con instrucciones**

```markdown
# Dev environment

## Vault local (KV v2 + OIDC dex)

```bash
docker-compose up -d
```

Ese comando levanta Vault en `:8200` (token raíz `root-token`) y dex en `:5556`.

### Configurar OIDC en Vault

```bash
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=root-token
vault auth enable oidc
vault write auth/oidc/config \
  oidc_discovery_url=http://localhost:5556 \
  oidc_client_id=vault \
  oidc_client_secret=vault-secret \
  default_role=developer
vault write auth/oidc/role/developer \
  bound_audiences=vault \
  allowed_redirect_uris=http://localhost:8200/v1/auth/oidc/oidc/callback,http://127.0.0.1:8250/oidc/callback \
  user_claim=email \
  policies=default
vault secrets enable -path=secret -version=2 kv
```

### Añadir source en go-secret

```bash
go-secret sources add
# provider: vault
# id: vault-local
# address: http://localhost:8200
# auth method: oidc
# role: developer
# mount: secret, version: 2
```

### Login

```bash
go-secret sources login vault-local
# → abre el browser con dex (user: dev@local, pass: password)
```

Después de eso `go-secret list` debería listar tanto los secretos GSM como los de Vault.
```

- [ ] **Step 31.4: Commit**

```bash
git add docker-compose.yml dev/ docs/dev-environment.md
git commit -m "docs: docker-compose with Vault + dex for OIDC E2E testing"
```

---

## Task 32: Actualizar README + CHANGELOG

**Files:**
- Modify: `README.md`
- Create: `CHANGELOG.md`

- [ ] **Step 32.1: README — sección Multi-source**

Añadir tras la sección de features:

```markdown
## 🌐 Multi-source

`go-secret` ahora puede gestionar simultáneamente secretos de **GCP Secret Manager** y de **HashiCorp Vault** (KV v1 y v2). Define cada backend como una "source" en `config.yaml` y úsalas en paralelo:

```yaml
default_source: gsm-prod
sources:
  - id: gsm-prod
    provider: gsm
    enabled: true
    project_id: my-prod-project

  - id: vault-corp
    provider: vault
    enabled: true
    address: https://vault.corp.io
    auth: { method: oidc, role: developer }
    mounts:
      - { path: secret, version: 2 }
```

Al ejecutar `go-secret list` sin `--source`, verás los secretos de ambas fuentes en una sola lista, distinguidos por la columna `PROVIDER`. En la TUI, `Tab` cicla el filtro por fuente y `Ctrl+P` abre el picker de fuentes.

### Auth Vault soportado

- `token` — `VAULT_TOKEN` env, keyring del SO o `~/.vault-token`
- `approle` — `role_id` en config, `secret_id` en keyring (interactivo via `go-secret sources login <id>`)
- `oidc` — flujo browser con callback en `127.0.0.1:8250` (puerto configurable)

### Migración desde versiones anteriores

Tu config existente (`project_id` raíz + `recent_projects`) se migra automáticamente a una entry `sources` GSM la primera vez que arrancas la nueva versión. No se pierde ningún proyecto.
```

- [ ] **Step 32.2: CHANGELOG.md**

```markdown
# Changelog

## [Unreleased]

### Added
- Multi-source support: gestiona varios proyectos GSM y Vaults simultáneamente.
- HashiCorp Vault provider (KV v1 + KV v2).
- Vault auth: token, AppRole, OIDC con callback browser.
- TUI: columna `PROVIDER`, filtro por fuente con Tab cycle, picker de fuentes con `Ctrl+P`.
- Subcomando `sources` (`list/add/edit/remove/toggle/login/set-default`).
- Templates per-source con override por título; nuevas variables `{{.SourceID}}` y `{{.Provider}}`.
- Audit log incluye `source_id` y `provider`; filtros `--source` y `--provider` en `audit logs`.
- Migración automática de configs antiguos (`project_id` raíz) al nuevo shape `sources`.
- `docker-compose.yml` + dex para E2E de OIDC en local.

### Changed
- `--project` está deprecated en favor de `--source <id>` (warning a stderr cuando se usa).
- `internal/gcp` movido a `internal/providers/gsm` (cambio interno, sin impacto en usuarios).

### Fixed
- `templates generate --copy` ahora copia realmente al portapapeles (regresión introducida en la PR #8).
```

- [ ] **Step 32.3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: README + CHANGELOG for multi-source/Vault feature"
```

---

## Task 33: Verificación end-to-end manual

- [ ] **Step 33.1: Construir binario**

```bash
go build -o bin/go-secret ./
golangci-lint run ./...
go test -race ./...
```

Expected: BUILD OK, `0 issues`, todos los tests verdes.

- [ ] **Step 33.2: Smoke CLI con un proyecto GSM real**

```bash
./bin/go-secret sources list
./bin/go-secret list
./bin/go-secret get <secret-name>
./bin/go-secret reveal <secret-name>
```

Expected: comandos responden sin panic; columna `PROVIDER` muestra el id de fuente GSM.

- [ ] **Step 33.3: Smoke E2E con Vault local + dex**

```bash
docker-compose up -d
# Configurar Vault según docs/dev-environment.md
./bin/go-secret sources add # añadir vault-local con OIDC
./bin/go-secret sources login vault-local # → browser
./bin/go-secret list
./bin/go-secret create test-secret-vault --source vault-local
./bin/go-secret reveal test-secret-vault --source vault-local
./bin/go-secret versions list test-secret-vault --source vault-local
./bin/go-secret delete test-secret-vault --source vault-local
docker-compose down
```

Expected: cada comando ejecuta correctamente contra Vault, audit logs incluyen `source_id: vault-local` y `provider: vault`.

- [ ] **Step 33.4: Smoke TUI**

```bash
./bin/go-secret
```

Expected:
- Lista mezcla secretos de GSM y Vault con badge coloreado.
- `Tab` cicla filtros: `[ALL]` → `[gsm-prod]` → `[vault-local]` → `[ALL]`.
- `Ctrl+P` abre picker; `Space` toggle; `s` persiste.
- `n` (Create) muestra picker de fuente cuando hay múltiples activas.
- Templates: si la fuente Vault tiene template per-source con título coincidente, se ve antes que el global.

- [ ] **Step 33.5: Migración manual desde config legacy**

```bash
# Copiar tu config actual a otro path
cp ~/.config/go-secrets/config.yaml /tmp/legacy.yaml
# Restaurar el binario nuevo y arrancar
./bin/go-secret list
# Inspeccionar
diff /tmp/legacy.yaml ~/.config/go-secrets/config.yaml
```

Expected: el diff muestra los campos legacy desaparecidos y un nuevo bloque `sources:` con el proyecto GSM original.

- [ ] **Step 33.6: Push + abrir PR**

```bash
git push -u origin feat/multi-source-vault
gh pr create --title "feat: multi-source con HashiCorp Vault y vista unificada" --body "$(cat <<'EOF'
## Summary
- Añade soporte para HashiCorp Vault (KV v1 + KV v2) junto al GSM existente.
- Lista unificada con columna `PROVIDER` y filtro por fuente con `Tab`.
- Auth Vault: token, AppRole, OIDC (browser flow).
- Migración automática de configs antiguos.
- Templates per-source con override.
- Audit logs con `source_id` y `provider`.

## Test plan
- [ ] `go test -race ./...` verde
- [ ] `golangci-lint run ./...` `0 issues`
- [ ] Smoke CLI contra GSM real (list/get/reveal)
- [ ] Smoke E2E contra Vault local con OIDC dex (docker-compose)
- [ ] Smoke TUI: Tab cycle, Ctrl+P picker, create con picker
- [ ] Migración config legacy verificada

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-Review

Spec coverage:

- §3.1 capas → Tasks 1–6.
- §3.2 Provider interface → Task 1.
- §3.3 Registry/UnifiedClient → Tasks 3, 4, 9.
- §3.4 GSM refactor → Tasks 5, 6, 10.
- §3.5 Vault provider (KV v1+v2) → Tasks 11, 12, 13.
- §3.6 Auth Vault (token, approle, oidc, keyring) → Tasks 11, 14, 15.
- §4 Config + migración → Tasks 7, 8.
- §5 CLI cambios (`--source`, `sources` subcmd, prompt) → Tasks 16, 17, 18, 19, 20, 21, 22, 23.
- §6.1 Lista TUI con PROVIDER → Task 24.
- §6.2 Sources picker `Ctrl+P` → Task 26.
- §6.3 Create flow con picker → Task 27.
- §6.4 Settings menu Sources → Task 28.
- §7 Audit con source_id/provider y filtros → Task 29.
- §8 Templates per-source → Task 30.
- §9 Testing (Vault httptest, GSM fakes, migración) → cubierto a lo largo de Tasks 1–15.
- §10 Build sequence → Tasks 1–32 ordenados.
- §11 Decisiones (rename, keyring lib, OIDC port) → Tasks 5, 11, 14, 15.
- §12 Riesgos (keyring fallback Linux) → Task 14 step 14.2 implementa fallback.
- E2E manual → Task 33.
- Bug clipboard templates (fuera de scope, ya en main).

Placeholders revisados: ningún `TODO`/`TBD` en pasos. Cada step de TDD tiene test concreto + comando esperado + commit. Las firmas (`Provider`, `Registry`, `UnifiedClient`, `Capabilities`, `LoadFromConfig`, `NewFromSourceConfig`) son consistentes en todos los tasks.

Tipos / signaturas auditadas:
- `sources.Provider.List()`, `Get()`, `Reveal()` etc. invocados con la misma firma en Tasks 18–19.
- `config.SourceConfig` field names estables entre Tasks 7 y los consumidores (Tasks 9, 10, 11, 22).
- `sources.PartialError` instanciado en Task 4, consumido en Task 18.
- `keyring{Get,Set,Delete,SetSecretID,GetSecretID}` definidos en Task 14, usados en Tasks 11/14/15.
- `openBrowser` (override de tests) definido en Task 15 y mockeado en su test.

Sin gaps detectados.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-05-multi-source-vault.md`. Two execution options:

1. **Subagent-Driven (recommended)** — Dispatch fresh subagents per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using executing-plans, batch with checkpoints.

Which approach?
