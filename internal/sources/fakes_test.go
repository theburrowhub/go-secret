// internal/sources/fakes_test.go
package sources

import (
	"context"
	"sync/atomic"
)

// Compile-time assertion: fakeProvider must satisfy the Provider interface.
var _ Provider = (*fakeProvider)(nil)

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

func newFakeProvider(id, kind string) *fakeProvider { //nolint:unused // consumed by registry_test.go and unified_client_test.go (Tasks 3 & 4)
	return &fakeProvider{
		id:          id,
		kind:        kind,
		displayName: id,
		separator:   "/",
		caps:        Capabilities{SupportsVersions: true, SupportsLabels: true},
		secrets:     map[string]Secret{},
	}
}

func (f *fakeProvider) ID() string                 { return f.id }
func (f *fakeProvider) Kind() string               { return f.kind }
func (f *fakeProvider) DisplayName() string        { return f.displayName }
func (f *fakeProvider) FolderSeparator() string    { return f.separator }
func (f *fakeProvider) Capabilities() Capabilities { return f.caps }
func (f *fakeProvider) UserEmail() string          { return "fake@test" }

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
