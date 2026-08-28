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
