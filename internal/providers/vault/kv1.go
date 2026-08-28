// internal/providers/vault/kv1.go
package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/theburrowhub/go-secret/internal/sources"
)

func (c *Client) listKV1(ctx context.Context, m mountInfo) ([]sources.Secret, error) {
	out := []sources.Secret{}
	var walk func(prefix string) error
	walk = func(prefix string) error {
		path := fmt.Sprintf("%s/%s", m.Path, prefix)
		s, err := c.api.Logical().ListWithContext(ctx, path)
		if err != nil {
			return err
		}
		if s == nil || s.Data == nil {
			return nil
		}
		keysRaw, _ := s.Data["keys"].([]interface{})
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
				Name:     prefixWithMount(m, full),
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

func (c *Client) getKV1(ctx context.Context, m mountInfo, rel string) (*sources.Secret, error) {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	return &sources.Secret{Name: prefixWithMount(m, rel), SourceID: c.id}, nil
}

func (c *Client) revealKV1(ctx context.Context, m mountInfo, rel string) ([]byte, error) {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	s, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("not found: %s", rel)
	}
	if v, ok := s.Data["value"].(string); ok {
		return []byte(v), nil
	}
	if len(s.Data) == 1 {
		for _, v := range s.Data {
			if vs, ok := v.(string); ok {
				return []byte(vs), nil
			}
		}
	}
	return nil, fmt.Errorf("no 'value' key in secret %s", rel)
}

func (c *Client) writeKV1(ctx context.Context, m mountInfo, rel string, value []byte) error {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	_, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{"value": string(value)})
	return err
}

func (c *Client) deleteKV1(ctx context.Context, m mountInfo, rel string) error {
	path := fmt.Sprintf("%s/%s", m.Path, rel)
	_, err := c.api.Logical().DeleteWithContext(ctx, path)
	return err
}
