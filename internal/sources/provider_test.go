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
