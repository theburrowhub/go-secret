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

// NewUnifiedClient returns a UnifiedClient backed by the given Registry.
func NewUnifiedClient(r *Registry) *UnifiedClient { return &UnifiedClient{reg: r} }

// List returns secrets from every active provider. If a provider fails the
// others still complete and the error returned is a *PartialError.
func (u *UnifiedClient) List(ctx context.Context) ([]Secret, error) {
	providers := u.reg.Active()
	var (
		mu   sync.Mutex
		all  []Secret
		errs = map[string]error{}
		wg   sync.WaitGroup
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
