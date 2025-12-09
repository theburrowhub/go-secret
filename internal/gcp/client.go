package gcp

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
)

// Secret represents a GCP secret
type Secret struct {
	Name        string
	FullName    string
	CreateTime  string
	Labels      map[string]string
	Replication string
}

// SecretVersion represents a version of a secret
type SecretVersion struct {
	Name       string
	State      string
	CreateTime string
}

// Client wraps the GCP Secret Manager client
type Client struct {
	client    *secretmanager.Client
	projectID string
}

// NewClient creates a new GCP Secret Manager client
func NewClient(ctx context.Context, projectID string) (*Client, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretmanager client: %w", err)
	}

	return &Client{
		client:    client,
		projectID: projectID,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.client.Close()
}

// ProjectID returns the current project ID
func (c *Client) ProjectID() string {
	return c.projectID
}

// ListSecrets lists all secrets in the project
func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	parent := fmt.Sprintf("projects/%s", c.projectID)

	req := &secretmanagerpb.ListSecretsRequest{
		Parent: parent,
	}

	var secrets []Secret
	it := c.client.ListSecrets(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		// Extract just the secret name from the full path
		parts := strings.Split(resp.Name, "/")
		name := parts[len(parts)-1]

		replication := "automatic"
		if resp.Replication != nil {
			if resp.Replication.GetUserManaged() != nil {
				replication = "user-managed"
			}
		}

		secrets = append(secrets, Secret{
			Name:        name,
			FullName:    resp.Name,
			CreateTime:  resp.CreateTime.AsTime().Format("2006-01-02 15:04:05"),
			Labels:      resp.Labels,
			Replication: replication,
		})
	}

	return secrets, nil
}

// GetSecret retrieves a specific secret
func (c *Client) GetSecret(ctx context.Context, secretName string) (*Secret, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)

	req := &secretmanagerpb.GetSecretRequest{
		Name: name,
	}

	resp, err := c.client.GetSecret(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	replication := "automatic"
	if resp.Replication != nil {
		if resp.Replication.GetUserManaged() != nil {
			replication = "user-managed"
		}
	}

	return &Secret{
		Name:        secretName,
		FullName:    resp.Name,
		CreateTime:  resp.CreateTime.AsTime().Format("2006-01-02 15:04:05"),
		Labels:      resp.Labels,
		Replication: replication,
	}, nil
}

// ListSecretVersions lists all versions of a secret
func (c *Client) ListSecretVersions(ctx context.Context, secretName string) ([]SecretVersion, error) {
	parent := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)

	req := &secretmanagerpb.ListSecretVersionsRequest{
		Parent: parent,
	}

	var versions []SecretVersion
	it := c.client.ListSecretVersions(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list versions: %w", err)
		}

		parts := strings.Split(resp.Name, "/")
		versionName := parts[len(parts)-1]

		versions = append(versions, SecretVersion{
			Name:       versionName,
			State:      resp.State.String(),
			CreateTime: resp.CreateTime.AsTime().Format("2006-01-02 15:04:05"),
		})
	}

	return versions, nil
}

// AccessSecretVersion retrieves the payload of a secret version
func (c *Client) AccessSecretVersion(ctx context.Context, secretName, version string) ([]byte, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", c.projectID, secretName, version)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	resp, err := c.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to access secret version: %w", err)
	}

	return resp.Payload.Data, nil
}

// CreateSecret creates a new secret
func (c *Client) CreateSecret(ctx context.Context, secretName string, labels map[string]string) error {
	parent := fmt.Sprintf("projects/%s", c.projectID)

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
			Labels: labels,
		},
	}

	_, err := c.client.CreateSecret(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// AddSecretVersion adds a new version to an existing secret
func (c *Client) AddSecretVersion(ctx context.Context, secretName string, payload []byte) (*SecretVersion, error) {
	parent := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)

	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}

	resp, err := c.client.AddSecretVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to add secret version: %w", err)
	}

	parts := strings.Split(resp.Name, "/")
	versionName := parts[len(parts)-1]

	return &SecretVersion{
		Name:       versionName,
		State:      resp.State.String(),
		CreateTime: resp.CreateTime.AsTime().Format("2006-01-02 15:04:05"),
	}, nil
}

// DeleteSecret deletes a secret and all its versions
func (c *Client) DeleteSecret(ctx context.Context, secretName string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)

	req := &secretmanagerpb.DeleteSecretRequest{
		Name: name,
	}

	if err := c.client.DeleteSecret(ctx, req); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// DisableSecretVersion disables a secret version
func (c *Client) DisableSecretVersion(ctx context.Context, secretName, version string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", c.projectID, secretName, version)

	req := &secretmanagerpb.DisableSecretVersionRequest{
		Name: name,
	}

	_, err := c.client.DisableSecretVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to disable version: %w", err)
	}

	return nil
}

// EnableSecretVersion enables a secret version
func (c *Client) EnableSecretVersion(ctx context.Context, secretName, version string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", c.projectID, secretName, version)

	req := &secretmanagerpb.EnableSecretVersionRequest{
		Name: name,
	}

	_, err := c.client.EnableSecretVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to enable version: %w", err)
	}

	return nil
}

// DestroySecretVersion permanently destroys a secret version
func (c *Client) DestroySecretVersion(ctx context.Context, secretName, version string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", c.projectID, secretName, version)

	req := &secretmanagerpb.DestroySecretVersionRequest{
		Name: name,
	}

	_, err := c.client.DestroySecretVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to destroy version: %w", err)
	}

	return nil
}

// UpdateSecretLabels updates the labels of a secret
func (c *Client) UpdateSecretLabels(ctx context.Context, secretName string, labels map[string]string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretName)

	secret, err := c.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	secret.Labels = labels

	req := &secretmanagerpb.UpdateSecretRequest{
		Secret: secret,
	}

	_, err = c.client.UpdateSecret(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	return nil
}

