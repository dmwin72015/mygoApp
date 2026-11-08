package remote

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"mygo-app/internal/domain"
)

// remoteSettings manages application configuration key-value pairs over REST at /settings/{key}.
type remoteSettings struct {
	c *Client
}

func newSettings(c *Client) domain.Settings {
	return remoteSettings{c: c}
}

func (s remoteSettings) Get(ctx context.Context, key string) (string, error) {
	var res struct {
		Value string `json:"value"`
	}
	err := s.c.Do(ctx, http.MethodGet, "/settings/"+url.PathEscape(key), nil, &res)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return "", nil
	}
	return res.Value, err
}

func (s remoteSettings) Set(ctx context.Context, key, value string) error {
	return s.c.Do(ctx, http.MethodPut, "/settings/"+url.PathEscape(key),
		map[string]string{"value": value}, nil)
}
