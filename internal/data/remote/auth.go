package remote

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"mygo-app/internal/captcha"
	"mygo-app/internal/domain"
)

// remoteAuth handles authentication, registration, and password recovery via the REST API.
type remoteAuth struct {
	c *Client
}

func newAuth(c *Client) domain.Auth {
	return remoteAuth{c: c}
}

func (a remoteAuth) Login(ctx context.Context, username, password string) (domain.User, error) {
	var res struct {
		Token string      `json:"token"`
		User  domain.User `json:"user"`
	}
	body := map[string]string{
		"username": username,
		"password": password,
	}
	if cred, ok := captcha.CredentialsFromContext(ctx); ok {
		body["captcha_id"] = cred.ID
		body["captcha_code"] = cred.Code
	}
	err := a.c.Do(ctx, http.MethodPost, "/auth/login", body, &res)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
			return domain.User{}, domain.ErrBadCredentials
		}
		return domain.User{}, err
	}
	if res.Token == "" {
		return domain.User{}, errors.New("auth: the API returned no token")
	}
	a.c.Token = res.Token
	return res.User, nil
}

func (a remoteAuth) ResetPassword(ctx context.Context, username, recoveryKey, newPassword string) error {
	var res struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	err := a.c.Do(ctx, http.MethodPost, "/auth/reset-password", map[string]string{
		"username":     username,
		"recovery_key": recoveryKey,
		"new_password": newPassword,
	}, &res)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Status == http.StatusForbidden || strings.Contains(apiErr.Message, "安全恢复密钥") {
				return domain.ErrInvalidRecoveryKey
			}
			if apiErr.Status == http.StatusNotFound || strings.Contains(apiErr.Message, "不存在") {
				return domain.ErrUserNotFound
			}
			if apiErr.Message != "" {
				return errors.New(apiErr.Message)
			}
		}
		return err
	}
	return nil
}

func (a remoteAuth) Register(ctx context.Context, username, password, displayName string) (domain.User, error) {
	var res struct {
		Token string      `json:"token"`
		User  domain.User `json:"user"`
	}
	err := a.c.Do(ctx, http.MethodPost, "/auth/register", map[string]string{
		"username":     username,
		"password":     password,
		"display_name": displayName,
	}, &res)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Message != "" {
			return domain.User{}, errors.New(apiErr.Message)
		}
		return domain.User{}, err
	}
	if res.Token != "" {
		a.c.Token = res.Token
	}
	return res.User, nil
}
