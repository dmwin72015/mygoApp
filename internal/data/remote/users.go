package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"mygo-app/internal/domain"
)

// remoteUsers manages operator accounts over the REST API at /users.
type remoteUsers struct {
	c *Client
}

func newUsers(c *Client) domain.UserStore {
	return remoteUsers{c: c}
}

func (u remoteUsers) List(ctx context.Context) ([]domain.User, error) {
	var resp struct {
		Items []domain.User `json:"items"`
	}
	if err := u.c.Do(ctx, http.MethodGet, "/users", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (u remoteUsers) Create(ctx context.Context, v *domain.User, password string) error {
	body := struct {
		domain.User
		Password string `json:"password"`
	}{User: *v, Password: password}
	return u.c.Do(ctx, http.MethodPost, "/users", body, v)
}

func (u remoteUsers) Update(ctx context.Context, v domain.User) error {
	idStr := strconv.FormatInt(v.ID, 10)
	if v.ID == 0 {
		idStr = v.Code
	}
	return u.c.Do(ctx, http.MethodPut, "/users/"+url.PathEscape(idStr), v, nil)
}

func (u remoteUsers) SetPassword(ctx context.Context, id any, password string) error {
	idStr := fmt.Sprint(id)
	return u.c.Do(ctx, http.MethodPost, "/users/"+url.PathEscape(idStr)+"/password",
		map[string]string{"password": password}, nil)
}

func (u remoteUsers) Delete(ctx context.Context, id any) error {
	idStr := fmt.Sprint(id)
	return u.c.Do(ctx, http.MethodDelete, "/users/"+url.PathEscape(idStr), nil, nil)
}
