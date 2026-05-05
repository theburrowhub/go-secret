// internal/providers/vault/kv2.go
package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/go-secret/internal/sources"
)

// listKV2 walks the metadata tree for a single mount and returns flattened
// secret names (relative to the mount root).
func (c *Client) listKV2(ctx context.Context, m mountInfo) ([]sources.Secret, error) {
	out := []sources.Secret{}
	var walk func(prefix string) error
	walk = func(prefix string) error {
		path := fmt.Sprintf("%s/metadata/%s", m.Path, prefix)
		s, err := c.api.Logical().ListWithContext(ctx, path)
		if err != nil {
			return err
		}
		if s == nil || s.Data == nil {
			return nil
		}
		keysRaw, ok := s.Data["keys"].([]interface{})
		if !ok {
			return nil
		}
		for _, k := range keysRaw {
			name, _ := k.(string)
			full := prefix + name
			if strings.HasSuffix(name, "/") {
				if err := walk(full); err != nil {
					return err
				}
				continue
			}
			out = append(out, sources.Secret{
				Name:     full,
				SourceID: c.id,
			})
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
}

func prefixWithMount(m mountInfo, name string) string {
	if len(m.Path) == 0 {
		return name
	}
	return m.Path + "/" + name
}

// resolveMount returns (mountInfo, relativePath) for a fully-qualified name.
func (c *Client) resolveMount(name string) (mountInfo, string, error) {
	for _, m := range c.mounts {
		prefix := m.Path + "/"
		if strings.HasPrefix(name, prefix) {
			return m, strings.TrimPrefix(name, prefix), nil
		}
		if name == m.Path {
			return m, "", nil
		}
	}
	if len(c.mounts) == 1 {
		return c.mounts[0], name, nil
	}
	return mountInfo{}, "", fmt.Errorf("name %q does not match any configured mount", name)
}

func (c *Client) getKV2(ctx context.Context, m mountInfo, rel string) (*sources.Secret, error) {
	path := fmt.Sprintf("%s/metadata/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Data == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	created, _ := s.Data["created_time"].(string)
	return &sources.Secret{
		Name:       prefixWithMount(m, rel),
		SourceID:   c.id,
		CreateTime: created,
	}, nil
}

func (c *Client) revealKV2(ctx context.Context, m mountInfo, rel, version string) ([]byte, error) {
	path := fmt.Sprintf("%s/data/%s", m.Path, rel)
	if version != "" && version != "latest" {
		path = fmt.Sprintf("%s?version=%s", path, version)
	}
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	d, ok := s.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response shape for %s", rel)
	}
	// KV v2 stores arbitrary keys; for go-secret we use a single "value" key by convention.
	if v, ok := d["value"].(string); ok {
		return []byte(v), nil
	}
	// Fallback: if there's exactly one key, use it.
	if len(d) == 1 {
		for _, v := range d {
			if vs, ok := v.(string); ok {
				return []byte(vs), nil
			}
		}
	}
	return nil, fmt.Errorf("no 'value' key in secret %s; multi-key secrets are not supported", rel)
}

func (c *Client) listVersionsKV2(ctx context.Context, m mountInfo, rel string) ([]sources.Version, error) {
	path := fmt.Sprintf("%s/metadata/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Data == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	versionsRaw, _ := s.Data["versions"].(map[string]interface{})
	out := []sources.Version{}
	for k, v := range versionsRaw {
		entry, _ := v.(map[string]interface{})
		state := "ENABLED"
		if d, _ := entry["destroyed"].(bool); d {
			state = "DESTROYED"
		} else if dt, _ := entry["deletion_time"].(string); dt != "" {
			state = "DISABLED"
		}
		ct, _ := entry["created_time"].(string)
		out = append(out, sources.Version{Name: k, State: state, CreateTime: ct})
	}
	return out, nil
}
