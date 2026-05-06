package vault

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/theburrowhub/go-secret/internal/config"
)

// openBrowser is overridable for tests. By default it shells out to xdg-open
// (Linux), open (macOS) or rundll32 (Windows).
var openBrowser = func(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
	return errors.New("unsupported platform for browser auto-open")
}

func resolveOIDCAuth(ctx context.Context, sc config.SourceConfig) (string, error) {
	// Try cached token first.
	if v, ok := keyringGet(sc.ID); ok {
		return v, nil
	}

	cfg := vaultapi.DefaultConfig()
	cfg.Address = sc.Address
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		return "", err
	}

	port := sc.Auth.OIDCPort
	if port == 0 {
		port = 8250
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oidc/callback", port)

	resp, err := api.Logical().WriteWithContext(ctx, "auth/oidc/oidc/auth_url", map[string]interface{}{
		"role":         sc.Auth.Role,
		"redirect_uri": redirectURI,
		"state":        state,
	})
	if err != nil {
		return "", fmt.Errorf("oidc auth_url: %w", err)
	}
	authURL, _ := resp.Data["auth_url"].(string)
	if authURL == "" {
		return "", errors.New("oidc: empty auth_url returned")
	}

	type result struct {
		token string
		err   error
	}
	results := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			results <- result{err: errors.New("state mismatch")}
			return
		}
		code := q.Get("code")
		s, err := api.Logical().WriteWithContext(ctx, "auth/oidc/oidc/callback", map[string]interface{}{
			"code":  code,
			"state": state,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			results <- result{err: err}
			return
		}
		if s == nil || s.Auth == nil || s.Auth.ClientToken == "" {
			results <- result{err: errors.New("oidc: no token in callback response")}
			return
		}
		_, _ = w.Write([]byte("Login successful. You can close this tab."))
		results <- result{token: s.Auth.ClientToken}
	})

	srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.ListenAndServe() }()
	defer func() {
		shutdownCtx, cancel := context.WithCancel(context.Background())
		_ = cancel
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(authURL); err != nil {
		return "", fmt.Errorf("open browser: %w", err)
	}

	select {
	case r := <-results:
		if r.err != nil {
			return "", r.err
		}
		_ = keyringSet(sc.ID, r.token)
		return r.token, nil
	case err := <-srvErr:
		return "", fmt.Errorf("local server: %w", err)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func randomState() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
