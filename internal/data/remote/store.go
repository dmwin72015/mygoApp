package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"mygo-app/internal/domain"
)

// restStore provides generic REST CRUD operations for domain entity collections.
type restStore[T any] struct {
	c    *Client
	path string
	id   func(*T) string
	new  func() *T
}

func newRESTStore[T any](c *Client, path string, id func(*T) string, newFn func() *T) domain.Store[T] {
	return restStore[T]{c: c, path: path, id: id, new: newFn}
}

func (r restStore[T]) List(ctx context.Context, q domain.Query) ([]T, error) {
	v := url.Values{}
	if q.Search != "" {
		v.Set("search", q.Search)
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	path := r.path
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	// Accept both {"items": [...]} and a bare [...].
	var wrapped struct {
		Items []T `json:"items"`
	}
	var buf json.RawMessage
	if err := r.c.Do(ctx, http.MethodGet, path, nil, &buf); err != nil {
		return nil, err
	}
	if len(buf) > 0 && buf[0] == '[' {
		var items []T
		if err := json.Unmarshal(buf, &items); err != nil {
			return nil, err
		}
		return items, nil
	}
	if err := json.Unmarshal(buf, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Items, nil
}

func (r restStore[T]) Get(ctx context.Context, id string) (T, error) {
	v := r.new()
	if err := r.c.Do(ctx, http.MethodGet, r.path+"/"+url.PathEscape(id), nil, v); err != nil {
		return *r.new(), err
	}
	return *v, nil
}

func (r restStore[T]) Save(ctx context.Context, v *T) error {
	if id := r.id(v); id == "" {
		return r.c.Do(ctx, http.MethodPost, r.path, v, v)
	}
	return r.c.Do(ctx, http.MethodPut, r.path+"/"+url.PathEscape(r.id(v)), v, v)
}

func (r restStore[T]) Delete(ctx context.Context, id string) error {
	return r.c.Do(ctx, http.MethodDelete, r.path+"/"+url.PathEscape(id), nil, nil)
}
