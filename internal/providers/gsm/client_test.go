package gsm

import (
	"context"
	"testing"

	"github.com/theburrowhub/go-secret/internal/config"
	"github.com/theburrowhub/go-secret/internal/sources"
)

func TestClientImplementsProvider(t *testing.T) {
	var _ sources.Provider = (*Client)(nil)
}

func TestNewFromSourceConfigPopulatesIdentity(t *testing.T) {
	t.Skip("integration: requires GCP creds; verifies signature only")
	_ = func() (*Client, error) {
		return NewFromSourceConfig(context.Background(), config.SourceConfig{
			ID: "gsm-x", Provider: "gsm", ProjectID: "p1", FolderSeparator: "/",
		})
	}
}
