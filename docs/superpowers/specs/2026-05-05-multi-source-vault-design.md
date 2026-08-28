# Multi-source con HashiCorp Vault y vista unificada

**Fecha:** 2026-05-05
**Rama:** `feat/multi-source-vault`
**Estado:** Diseño aprobado, pendiente de plan de implementación

---

## 1. Objetivo

Permitir que `go-secret` gestione secretos en **GCP Secret Manager (GSM)** y en **HashiCorp Vault** simultáneamente, presentándolos en una **vista unificada** con una columna `PROVIDER` que distinga el origen.

El usuario podrá:

- Definir varias **fuentes** (sources) en config (varios proyectos GSM, varios Vaults).
- Activar/desactivar fuentes en caliente desde TUI/CLI sin editar el YAML.
- Ver la lista combinada de secretos de las fuentes activas en la TUI.
- Crear/leer/borrar secretos pasando `--source <id>` (o seleccionando con prompt si falta).
- Configurar parámetros per-source (ej. `folder_separator`) y compartir parámetros globales (clipboard, audit, session).

---

## 2. Decisiones tomadas durante el brainstorming

| # | Pregunta | Respuesta |
|---|---|---|
| 1 | Modelo de activación | **Híbrido**: config define fuentes disponibles; UI/CLI hace toggle de activas |
| 2 | Engines Vault | **KV v1 + KV v2**; otros engines se añaden iterativamente |
| 3 | Auth Vault | **Token + AppRole + OIDC** (OIDC requerido en v1) |
| 4 | Vista unificada | **B**: lista única con cycle de filtro por fuente vía `Tab` |
| 5 | Config per-source | **B**: per-source incluye templates (override de globales por nombre) |
| 6 | Escritura sin fuente implícita | **Prompt interactivo siempre** que falte; `--source <id>` lo salta |

Bug colateral arreglado fuera de scope (en la PR de CLI commands `#8`): `templates generate --copy` no copiaba realmente al portapapeles.

---

## 3. Arquitectura

### 3.1 Capas

```
cmd/  (CLI Cobra)        internal/ui/  (TUI Bubbletea)
       │                          │
       └────── ambos consumen ────┘
                       │
        ┌──────────────▼──────────────┐
        │  internal/sources           │
        │   - Provider interface      │
        │   - Registry                │
        │   - UnifiedClient           │
        └──┬────────────┬─────────────┘
           │            │
   ┌───────▼─────┐ ┌────▼─────┐ (futuros: AWS SM, Azure KV, ...)
   │ providers/  │ │ providers/│
   │   gsm       │ │   vault   │
   │ (refactor   │ │  (KV v1   │
   │  internal/  │ │   + KV v2)│
   │  gcp)       │ │           │
   └─────────────┘ └───────────┘
```

### 3.2 Interface `Provider`

`internal/sources/provider.go`:

```go
type Capabilities struct {
    SupportsVersions bool   // KV v1 = false, GSM/KV v2 = true
    SupportsLabels   bool   // GSM = true, Vault = via metadata
    SupportsLocations bool  // GSM = true, Vault = false
}

type Secret struct {
    Name        string  // path completo dentro de la fuente
    SourceID    string  // populated por UnifiedClient
    CreateTime  string
    Labels      map[string]string
    Replication string  // GSM-specific, vacío en Vault
}

type Version struct {
    Name       string
    State      string  // ENABLED | DISABLED | DESTROYED (mapping per provider)
    CreateTime string
}

type Provider interface {
    ID() string                // alias estable (de SourceConfig.ID)
    Kind() string              // "gsm" | "vault"
    DisplayName() string       // texto en columna PROVIDER
    FolderSeparator() string
    Capabilities() Capabilities

    // Reads
    List(ctx context.Context) ([]Secret, error)
    Get(ctx context.Context, name string) (*Secret, error)
    Reveal(ctx context.Context, name, version string) ([]byte, error)
    ListVersions(ctx context.Context, name string) ([]Version, error)

    // Writes
    Create(ctx context.Context, name string, value []byte, opts CreateOpts) error
    Delete(ctx context.Context, name string) error
    AddVersion(ctx context.Context, name string, value []byte) (*Version, error)
    EnableVersion(ctx context.Context, name, version string) error
    DisableVersion(ctx context.Context, name, version string) error
    DestroyVersion(ctx context.Context, name, version string) error

    UserEmail() string  // para audit log; "" si no aplica
    Close() error
}
```

Operaciones no soportadas por un provider devuelven `ErrNotSupported` (sentinel) y la UI/CLI lo presenta como mensaje claro.

### 3.3 Registry y UnifiedClient

`internal/sources/registry.go`:

- `Load(cfg *Config) (*Registry, error)` — instancia un Provider por cada `SourceConfig` con `Enabled=true`. Cachea credenciales (token Vault) en keyring.
- `Get(id string) (Provider, error)`
- `Active() []Provider` — lista actual filtrada (incluye toggle runtime).
- `Toggle(id string)` — toggle in-memory para la sesión. No persiste a YAML salvo `SaveActive()` explícito.
- `SetEnabledRuntime(id, enabled)`, `SaveActive() error`.

`internal/sources/unified.go`:

```go
type UnifiedClient struct { reg *Registry }

func (u *UnifiedClient) List(ctx) ([]Secret, error) {
    // errgroup paralelo: cada provider activo aporta sus secretos
    // cada Secret retornado lleva SourceID populado
}

func (u *UnifiedClient) Resolve(ctx, name, source string) (Provider, error) {
    // si source!="", devuelve ese provider
    // si source=="" y name solo existe en un provider, devuelve ese
    // si name existe en varios → error con sugerencia "--source"
}
```

### 3.4 Provider GSM (refactor)

- Renombrar `internal/gcp/` → `internal/providers/gsm/`.
- Adaptar el `Client` actual al interface `Provider`.
- Mantener semánticas existentes (replication, locations) detrás de `Capabilities` y `CreateOpts`.

### 3.5 Provider Vault

`internal/providers/vault/`:

- Cliente: `github.com/hashicorp/vault/api`.
- Detecta KV v1 vs v2 leyendo el mount info al iniciar; respeta `version` de config como override.
- KV v2: usa `data.metadata.versions` para `ListVersions`. `Reveal` con versión específica.
- KV v1: `Capabilities.SupportsVersions=false`. `ListVersions` retorna lista vacía. `EnableVersion`/`DisableVersion`/`DestroyVersion` retornan `ErrNotSupported`.
- Si una `SourceConfig` define **varios mounts**, el Provider los expone como prefijo en `Secret.Name`: `<mount>/<path>`. Operaciones detectan el mount por prefijo.

### 3.6 Auth Vault

`internal/providers/vault/auth.go`:

- **Token**: orden de resolución → env `VAULT_TOKEN` → keyring (clave `go-secret:vault:<source-id>`) → `~/.vault-token`.
- **AppRole**: `role_id` en config (texto plano OK), `secret_id` en keyring. `go-secret sources login <id>` solicita el `secret_id` interactivamente la primera vez.
- **OIDC**: flujo browser via `auth/oidc/oidc_auth_url` + callback HTTP local en `127.0.0.1:8250/oidc/callback` (puerto configurable). Token resultante en keyring con TTL del propio Vault. Auto-renew silencioso si quedan <5min.
- Keyring backend: `github.com/zalando/go-keyring`.

---

## 4. Cambios en `Config`

`internal/config/config.go`:

```go
type Config struct {
    DefaultSource string         `yaml:"default_source,omitempty"`
    Sources       []SourceConfig `yaml:"sources"`

    Clipboard ClipboardConfig
    Audit     AuditConfig
    Session   SessionConfig
    Templates []Template

    // Legacy (auto-migrados al primer Load tras upgrade):
    ProjectID       string   `yaml:"project_id,omitempty"`
    RecentProjects  []string `yaml:"recent_projects,omitempty"`
    SecretLocations []string `yaml:"secret_locations,omitempty"`
    FolderSeparator string   `yaml:"folder_separator,omitempty"`
}

type SourceConfig struct {
    ID              string     `yaml:"id"`
    Provider        string     `yaml:"provider"`         // "gsm" | "vault"
    Enabled         bool       `yaml:"enabled"`
    DisplayName     string     `yaml:"display_name,omitempty"`
    FolderSeparator string     `yaml:"folder_separator,omitempty"`
    Templates       []Template `yaml:"templates,omitempty"`

    // GSM-only
    ProjectID       string   `yaml:"project_id,omitempty"`
    SecretLocations []string `yaml:"secret_locations,omitempty"`

    // Vault-only
    Address string           `yaml:"address,omitempty"`
    Auth    VaultAuthConfig  `yaml:"auth,omitempty"`
    Mounts  []VaultMount     `yaml:"mounts,omitempty"`
}

type VaultAuthConfig struct {
    Method        string `yaml:"method"`           // "token" | "approle" | "oidc"
    Role          string `yaml:"role,omitempty"`
    OIDCPort      int    `yaml:"oidc_port,omitempty"` // default 8250
    AppRoleRoleID string `yaml:"role_id,omitempty"`
    // secret_id NUNCA en YAML — siempre keyring
}

type VaultMount struct {
    Path    string `yaml:"path"`
    Version int    `yaml:"version,omitempty"` // 1 | 2; auto-detect si vacío
}
```

### 4.1 Migración legacy

En `Load()`, si detecta los campos raíz `project_id` o `recent_projects` y `Sources` está vacío:

1. Crea una `SourceConfig` por cada proyecto en `RecentProjects` (todas con `Enabled=false`).
2. La que coincida con `ProjectID` queda con `Enabled=true` y `DefaultSource = ID`.
3. ID generado: `gsm-<projectID-sanitizado>`.
4. `FolderSeparator` raíz se copia a la fuente migrada.
5. Marca el archivo: añade comentario `# migrated to sources on YYYY-MM-DD` arriba del YAML.
6. Conserva los campos legacy en el struct (con `omitempty`) durante una versión, deprecated. Eliminar en v2.

Tests obligatorios: round-trip de un config legacy → migrado → guardado → cargado debe ser equivalente al esperado.

---

## 5. Cambios en CLI

| Antes | Después | Notas |
|---|---|---|
| `--project p1` | `--source <id>` | `--project` queda alias deprecated (warning a stderr) |
| (n/a) | sin `--source` y sin default → **prompt interactivo** | Lista fuentes activas; flecha+enter |
| (n/a) | `go-secret sources list` | Tabla con id, provider, enabled, display_name |
| (n/a) | `go-secret sources add` | Wizard interactivo |
| (n/a) | `go-secret sources edit <id>` | Wizard sobre fuente existente |
| (n/a) | `go-secret sources remove <id>` | Confirmación + limpia keyring |
| (n/a) | `go-secret sources toggle <id>` | Persiste `enabled` |
| (n/a) | `go-secret sources login <id>` | Re-auth para Vault (OIDC reabre browser, AppRole pide secret_id) |
| (n/a) | `go-secret sources set-default <id>` | Cambia `default_source` |

Comandos existentes (`list`, `get`, `reveal`, `create`, `delete`, `add-version`, `copy`, `versions *`) ganan `--source <id>` global.

`list` sin `--source` lista de **todas las fuentes activas** con columna `PROVIDER`.

---

## 6. Cambios en TUI

### 6.1 ViewList (lista de secretos)

- Nueva columna `PROVIDER` (alias de fuente). Color por kind (`gsm` azul, `vault` verde) para escaneo rápido.
- Header muestra el filtro activo: `[ALL]` por defecto, o `[gsm-prod]`, etc.
- Keymap nuevo: `Tab` cicla forward entre `[ALL]` → fuentes activas; `Shift+Tab` cicla backward.
- Filtro por texto (buscador existente) opera sobre la lista filtrada por fuente.

### 6.2 Sources picker (`Ctrl+P`)

Modal con lista de fuentes config y checkbox por cada una:

```
┌─ Sources ─────────────────────┐
│ [x] gsm-prod      (gsm)       │
│ [x] gsm-staging   (gsm)       │
│ [ ] vault-corp    (vault)     │
│ [x] vault-dev     (vault)     │
├───────────────────────────────┤
│ Space: toggle  s: save  Esc   │
└───────────────────────────────┘
```

- `Space` toggle runtime (no persiste).
- `s` persiste `enabled` al YAML.
- `l` (lowercase L) sobre fuente Vault dispara re-login.
- `Esc` cierra sin guardar (mantiene cambios runtime).

### 6.3 Create flow

Antes del form actual de `name/value/location`:

- Si solo hay 1 fuente activa → la usa, salta el picker.
- Si hay default y >1 activa → la usa, muestra hint `Tab para cambiar`.
- Si no hay default → muestra picker compacto con fuentes activas.

### 6.4 Settings menu

Nueva entrada `🔌 Sources` que abre el sources picker en modo edición (añadir/editar/borrar). Acceso también con `Ctrl+S` → `Sources`.

---

## 7. Audit log

Cada evento existente añade dos campos:

```json
{
  "timestamp": "...",
  "event_type": "SECRET_REVEAL",
  "source_id": "vault-corp",
  "provider": "vault",
  "user": "...",
  "secret_name": "...",
  "result": "..."
}
```

Nuevos eventos:

- `SOURCE_ADD`, `SOURCE_REMOVE`, `SOURCE_TOGGLE`, `SOURCE_LOGIN`
- `SOURCE_AUTH_REFRESH` (Vault OIDC token renew)

Filtro en `audit logs`: `--source <id>` y `--provider <kind>`.

---

## 8. Templates per-source

- Las `Templates` globales en config siguen funcionando.
- Cada `SourceConfig.Templates` puede definir templates con el **mismo `title`** que sobrescriben los globales **solo para esa fuente**.
- Render: cuando se genera un template para un secreto de `source X`, primero busca en `X.Templates`, si no encuentra busca en globales.
- Variables nuevas disponibles en templates: `{{.SourceID}}`, `{{.Provider}}` (kind), además de las existentes `{{.SecretName}}`, `{{.FullSecretName}}`, `{{.ProjectID}}` (vacío en Vault).

---

## 9. Testing

- **Unit**: cada provider con cliente mockeado (`net/http/httptest` para Vault, fake server `cloud.google.com/go/secretmanager/.../fake` para GSM).
- **Migración config**: tests de round-trip legacy → sources.
- **UnifiedClient**: tests con providers fake validando fan-out, agregación, errores parciales (un provider cae, los demás siguen).
- **Auth Vault**: tests de cada método con servidor mock. OIDC se testa con un servidor que simula el callback HTTP.
- **E2E manual**: `docker-compose.yml` con `vault:1.x` en dev mode + `dexidp/dex` para OIDC. Documentado en `docs/dev-environment.md`.

---

## 10. Build sequence

1. Refactor `internal/gcp/` → `internal/providers/gsm/` cumpliendo `Provider`.
2. Crear `internal/sources/{provider.go,registry.go,unified.go}` + tests.
3. Migración `Config` (struct + Load + tests round-trip).
4. Provider Vault KV v2 + auth `token`.
5. Provider Vault KV v1.
6. Auth Vault `approle` + integración keyring.
7. Auth Vault `oidc` (browser + callback HTTP local + keyring).
8. CLI: flag `--source` global + prompt interactivo + alias deprecated `--project`.
9. CLI: subcomandos `sources` (`list/add/edit/remove/toggle/login/set-default`).
10. TUI: columna `PROVIDER` + filtro `Tab` cycle.
11. TUI: sources picker `Ctrl+P` (toggle runtime + save).
12. TUI: integración picker en flow de create.
13. TUI: entrada `🔌 Sources` en settings menu.
14. Audit: campos `source_id`/`provider` + filtros + nuevos eventos.
15. Templates per-source con override por título.
16. Docs: README, CHANGELOG, `docs/dev-environment.md` con compose para Vault+dex.
17. E2E manual contra real GSM + Vault local + Vault corporativo (si disponible).

---

## 11. Decisiones explícitas

- **Rename `internal/gcp` → `internal/providers/gsm`**: aprobado en brainstorming. Cambia imports en cmd/* y internal/ui/*.
- **Keyring lib**: `github.com/zalando/go-keyring` (cross-platform, sin CGO obligatorio en macOS/Windows; en Linux requiere DBus + secret service o `pass`).
- **OIDC callback port**: `8250` por defecto, configurable per-source vía `auth.oidc_port`.

---

## 12. Riesgos y consideraciones

- **Breaking changes**: el rename `internal/gcp → internal/providers/gsm` es interno (no exportado al usuario). El config legacy se migra automáticamente. Aún así, anunciar en CHANGELOG.
- **OIDC en corporativo**: depende del provider configurado en Vault. Documentar troubleshooting (errores típicos: redirect_uri mismatch, scope insuficiente).
- **Keyring en Linux**: si el sistema no tiene secret-service, fallback a fichero cifrado en `~/.config/go-secrets/keyring.enc` (con master key del SO o passphrase). **Decisión**: si keyring falla, mostrar warning y caer a `~/.vault-token` (token) o env var (approle/secret_id) — no escribir secret_id a disco sin cifrar.
- **Escalabilidad de la lista unificada**: con muchas fuentes (>5) y muchos secretos (>1000 por fuente), la lista puede ser lenta. Mitigación: paginación lazy en TUI (existente para una fuente, extender), spinner por fuente que aún esté cargando.

---

## 13. Fuera de scope (para iteraciones futuras)

- Otros engines Vault (database, AWS, transit). Cuando alguien lo pida.
- Otros providers (AWS Secrets Manager, Azure Key Vault, 1Password). El interface `Provider` lo permite sin cambios.
- Sincronización entre fuentes (copiar secreto de GSM a Vault). Posible feature `go-secret sync`.
- RBAC interno (qué fuentes ve quién). Hoy es per-usuario por su propio config.
