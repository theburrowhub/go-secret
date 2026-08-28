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
