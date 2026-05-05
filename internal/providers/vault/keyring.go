// Package vault implements the sources.Provider interface for HashiCorp Vault.
// keyring.go — stub implementation; real OS keyring integration added in Task 14.
package vault

func keyringGet(sourceID string) (string, bool) { return "", false }
func keyringSet(sourceID, token string) error    { return nil } //nolint:unused // referenced by Task 14 login flow
func keyringDelete(sourceID string) error        { return nil } //nolint:unused // referenced by Task 14 logout flow
