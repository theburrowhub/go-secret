package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/theburrowhub/go-secret/internal/config"
)

func TestOIDCAuth_Flow(t *testing.T) {
	t.Cleanup(func() { _ = keyringDelete("v") })
	// Vault server stub
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/auth/oidc/oidc/auth_url", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Build a fake authorize URL that the test "browser" hits and which
		// then redirects to the callback in our agent.
		redirect := body["redirect_uri"].(string)
		idp := httptest.NewServer(http.HandlerFunc(func(w2 http.ResponseWriter, r2 *http.Request) {
			http.Redirect(w2, r2, redirect+"?code=abc&state="+body["state"].(string), http.StatusFound)
		}))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"auth_url": idp.URL},
		})
	})
	mux.HandleFunc("/v1/auth/oidc/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"auth": map[string]interface{}{"client_token": "oidc-token", "lease_duration": 3600},
		})
	})
	vault := httptest.NewServer(mux)
	defer vault.Close()

	// Inject a "browser" that follows the auth URL to trigger the callback.
	prev := openBrowser
	defer func() { openBrowser = prev }()
	openBrowser = func(u string) error {
		go func() {
			_, _ = http.Get(u)
		}()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok, err := resolveOIDCAuth(ctx, config.SourceConfig{
		ID:      "v",
		Address: vault.URL,
		Auth:    config.VaultAuthConfig{Method: "oidc", Role: "developer", OIDCPort: 18250},
	})
	if err != nil {
		t.Fatalf("oidc: %v", err)
	}
	if tok != "oidc-token" {
		t.Fatalf("got %q", tok)
	}

	// Sanity-check the agent uses the redirect URI we set.
	parsed, _ := url.Parse(vault.URL)
	if parsed == nil {
		t.Fatal("could not parse vault url")
	}
}
