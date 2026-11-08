package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mygo-app/internal/i18n"
)

// Client is the HTTP request transport layer shared by remote stores:
// manages BaseURL, Bearer tokens, timeouts, and error handling.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient builds an HTTP Client with a 15-second timeout.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// APIError represents a non-2xx HTTP response from the server.
type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
	}
	return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
}

// Do sends an HTTP request, injects common headers (Accept, Accept-Language,
// Authorization, Content-Type), and decodes the JSON response into out.
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", i18n.Current())
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connect: connection refused") {
			return fmt.Errorf("无法连接到服务端 (%s)，请确认后端服务已启动", c.BaseURL)
		}
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("%s %s: read: %w", method, path, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		var e struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := e.Message
		if msg == "" {
			msg = e.Error
		}
		if msg == "" {
			msg = string(raw)
		}
		return &APIError{Method: method, Path: path, Status: res.StatusCode, Message: msg}
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%s %s: decode: %w", method, path, err)
	}
	return nil
}
