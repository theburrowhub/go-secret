// internal/providers/vault/kv1.go
// KV v1 read operations — implemented in Task 13.
package vault

import (
	"context"
	"errors"

	"github.com/theburrowhub/go-secret/internal/sources"
)

// listKV1 lists secrets under a KV v1 mount. Implemented in Task 13.
func (c *Client) listKV1(_ context.Context, _ mountInfo) ([]sources.Secret, error) {
	return nil, errors.New("kv1 not implemented in this task")
}

// getKV1 fetches metadata for a secret under a KV v1 mount. Implemented in Task 13.
func (c *Client) getKV1(_ context.Context, _ mountInfo, _ string) (*sources.Secret, error) {
	return nil, errors.New("kv1 not implemented in this task")
}

// revealKV1 reads the value of a secret under a KV v1 mount. Implemented in Task 13.
func (c *Client) revealKV1(_ context.Context, _ mountInfo, _ string) ([]byte, error) {
	return nil, errors.New("kv1 not implemented in this task")
}
