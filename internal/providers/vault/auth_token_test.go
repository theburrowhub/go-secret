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
