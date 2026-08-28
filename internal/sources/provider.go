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
