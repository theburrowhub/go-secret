// Package vault implements the sources.Provider interface for HashiCorp Vault.
// keyring.go — real OS keyring wrapper using zalando/go-keyring with in-memory
// fallback for headless / CI environments that have no secret-service daemon.
package vault

import (
	"errors"
	"sync"

	"github.com/zalando/go-keyring"
)

const keyringService = "go-secret"

var (
	memMu    sync.Mutex
	memStore = map[string]string{}
)

func keyringKey(sourceID, kind string) string {
	return "vault:" + sourceID + ":" + kind
}

func keyringGet(sourceID string) (string, bool) {
	v, err := keyring.Get(keyringService, keyringKey(sourceID, "token"))
	if err == nil {
		return v, true
	}
	// Both ErrNotFound and any backend-unavailable error fall back to process memory.
	if errors.Is(err, keyring.ErrNotFound) {
		memMu.Lock()
		defer memMu.Unlock()
		v2, ok := memStore[keyringKey(sourceID, "token")]
		return v2, ok
	}
	// keyring backend unavailable; use process memory.
	memMu.Lock()
	defer memMu.Unlock()
	v2, ok := memStore[keyringKey(sourceID, "token")]
	return v2, ok
}

func keyringSet(sourceID, token string) error {
	if err := keyring.Set(keyringService, keyringKey(sourceID, "token"), token); err == nil {
		return nil
	}
	memMu.Lock()
	defer memMu.Unlock()
	memStore[keyringKey(sourceID, "token")] = token
	return nil
}

func keyringDelete(sourceID string) error {
	_ = keyring.Delete(keyringService, keyringKey(sourceID, "token"))
	memMu.Lock()
	defer memMu.Unlock()
	delete(memStore, keyringKey(sourceID, "token"))
	return nil
}

func keyringSetSecretID(sourceID, secretID string) error {
	if err := keyring.Set(keyringService, keyringKey(sourceID, "approle-secret-id"), secretID); err == nil {
		return nil
	}
	memMu.Lock()
	defer memMu.Unlock()
	memStore[keyringKey(sourceID, "approle-secret-id")] = secretID
	return nil
}

func keyringGetSecretID(sourceID string) (string, bool) {
	v, err := keyring.Get(keyringService, keyringKey(sourceID, "approle-secret-id"))
	if err == nil {
		return v, true
	}
	memMu.Lock()
	defer memMu.Unlock()
	v2, ok := memStore[keyringKey(sourceID, "approle-secret-id")]
	return v2, ok
}
