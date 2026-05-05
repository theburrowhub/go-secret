package gsm

import (
	"testing"

	"github.com/theburrowhub/go-secret/internal/sources"
)

func TestClientImplementsProvider(t *testing.T) {
	var _ sources.Provider = (*Client)(nil)
}
