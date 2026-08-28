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

	const sourceID = "v-approle-test"

	if err := SaveAppRoleSecretID(sourceID, "the-secret-id"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keyringDelete(sourceID) })

	tok, err := resolveAppRoleAuth(context.Background(), config.SourceConfig{
		ID:      sourceID,
		Address: srv.URL,
		Auth:    config.VaultAuthConfig{Method: "approle", AppRoleRoleID: "role-1"},
	})
	if err != nil {
		t.Fatalf("approle: %v", err)
	}
	if tok != "approle-token" {
		t.Fatalf("got %q, want %q", tok, "approle-token")
	}

	// Verify token was cached in keyring.
	cached, ok := keyringGet(sourceID)
	if !ok {
		t.Fatal("expected token to be cached in keyring after login")
	}
	if cached != "approle-token" {
		t.Fatalf("cached token: got %q, want %q", cached, "approle-token")
	}
}

func TestAppRoleAuth_MissingRoleID(t *testing.T) {
	_, err := resolveAppRoleAuth(context.Background(), config.SourceConfig{
		ID:   "v-no-role",
		Auth: config.VaultAuthConfig{Method: "approle"},
	})
	if err == nil {
		t.Fatal("expected error for missing role_id")
	}
}

func TestAppRoleAuth_MissingSecretID(t *testing.T) {
	// Ensure no secret_id stored for this source.
	const sourceID = "v-no-secret"
	_ = keyringDelete(sourceID)

	_, err := resolveAppRoleAuth(context.Background(), config.SourceConfig{
		ID:   sourceID,
		Auth: config.VaultAuthConfig{Method: "approle", AppRoleRoleID: "role-x"},
	})
	if err == nil {
		t.Fatal("expected error when secret_id missing from keyring")
	}
}
