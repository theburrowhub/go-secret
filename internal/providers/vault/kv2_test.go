// internal/providers/vault/kv2_test.go
package vault

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
)

func newKV2Server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/metadata/", func(w http.ResponseWriter, r *http.Request) {
		// LIST returns the keys at this prefix (Vault API uses GET with ?list=true).
		if r.Method == "LIST" || r.URL.Query().Get("list") == "true" {
			// Vault API strips trailing slash from paths, so normalize for lookup.
			path := strings.TrimPrefix(r.URL.Path, "/v1/secret/metadata/")
			path = strings.TrimSuffix(path, "/")
			keys := map[string][]string{
				"":    {"app/", "single"},
				"app": {"db", "api"},
			}
			out := map[string]interface{}{"data": map[string]interface{}{"keys": keys[path]}}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"created_time":    "2026-01-02T03:04:05Z",
				"current_version": 3,
				"versions": map[string]interface{}{
					"1": map[string]interface{}{"created_time": "2026-01-01T00:00:00Z"},
					"2": map[string]interface{}{"created_time": "2026-01-02T00:00:00Z", "destroyed": true},
					"3": map[string]interface{}{"created_time": "2026-01-02T03:04:05Z"},
				},
			},
		})
	})
	mux.HandleFunc("/v1/secret/data/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"data":     map[string]interface{}{"value": "the-secret"},
				"metadata": map[string]interface{}{"version": 3, "created_time": "2026-01-02T03:04:05Z"},
			},
		})
	})
	return httptest.NewServer(mux)
}

func TestKV2_List(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	got, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
		if s.SourceID != "v" {
			t.Fatalf("missing SourceID: %+v", s)
		}
	}
	for _, expected := range []string{"single", "app/db", "app/api"} {
		if !names[expected] {
			t.Fatalf("missing %q in %v", expected, names)
		}
	}
}

func TestKV2_RevealReturnsValue(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	v, err := c.Reveal(context.Background(), "app/db", "")
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if string(v) != "the-secret" {
		t.Fatalf("got %q", v)
	}
}

func TestKV2_ListVersionsNormalizesState(t *testing.T) {
	srv := newKV2Server(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL, mountInfo{Path: "secret", Version: 2})

	vs, err := c.ListVersions(context.Background(), "app/db")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("got %d versions", len(vs))
	}
	stateBy := map[string]string{}
	for _, v := range vs {
		stateBy[v.Name] = v.State
	}
	if stateBy["1"] != "ENABLED" || stateBy["2"] != "DESTROYED" || stateBy["3"] != "ENABLED" {
		t.Fatalf("unexpected states: %v", stateBy)
	}
}

// newTestClient builds a *Client wired to the given httptest server address.
// It uses direct vaultapi calls (no wrapper helpers needed).
func newTestClient(t *testing.T, addr string, mounts ...mountInfo) *Client {
	t.Helper()
	t.Setenv("VAULT_TOKEN", "test-token")
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	api, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	api.SetToken("test-token")
	return &Client{id: "v", folderSeparator: "/", mounts: mounts, api: api}
}
