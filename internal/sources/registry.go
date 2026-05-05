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
