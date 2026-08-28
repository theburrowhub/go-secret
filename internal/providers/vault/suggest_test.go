package vault

import (
	"testing"
)

func TestSuggestSourceID(t *testing.T) {
	cases := []struct {
		addr string
		want string
	}{
		{"http://localhost:8200", "vault-localhost"},
		{"https://vault.corp.io", "vault-vault-corp-io"},
		{"https://vault.corp.io:8200", "vault-vault-corp-io"},
		{"https://my_vault.example.com", "vault-my-vault-example-com"},
	}
	for _, c := range cases {
		got := SuggestSourceID(c.addr)
		if got != c.want {
			t.Errorf("SuggestSourceID(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}
