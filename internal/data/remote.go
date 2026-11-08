package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mygo-app/internal/captcha"
	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"
)

// Client is the request layer shared by every remote store: one base URL,
// one bearer token, one timeout, and one place where a response that is
// not what the app expects becomes an error the UI can show.
//
// Every request goes through do, so adding retries, logging or a refresh
// token is a change in one function, not in fifty call sites.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient builds a client with a sensible timeout. An empty token sends
// no Authorization header.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// APIError is what a client returns for a status other than 2xx.
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

// do sends one request and decodes the JSON reply into out, which may be
// nil. A non-2xx reply becomes an *APIError carrying the server's message
// field when it sends one.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
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

// remoteRepo is the same domain.Repo as SQLite, over HTTP. The server is
// expected to speak REST: GET /students?search=&limit=, GET/PUT/DELETE
// /students/{id}, POST /students, and the same for every collection.
type remoteRepo struct {
	c *Client
}

// OpenRemote connects to an API with the conventions of Client.
func OpenRemote(baseURL, token string) domain.Repo {
	return &remoteRepo{c: NewClient(baseURL, token)}
}

func (r *remoteRepo) Close() error   { return nil }
func (r *remoteRepo) Source() string { return "远程 API · " + r.c.BaseURL }

// GetCaptcha requests a fresh math arithmetic captcha challenge from the API.
func (r *remoteRepo) GetCaptcha(ctx context.Context) (captcha.Item, error) {
	var item captcha.Item
	err := r.c.do(ctx, http.MethodGet, "/auth/captcha", nil, &item)
	return item, err
}

func (r *remoteRepo) Auth() domain.Auth { return remoteAuth{r.c} }

// remoteSettings keeps the app's preferences on the API, at /settings/{key}.
type remoteSettings struct{ c *Client }

func (r *remoteRepo) Settings() domain.Settings { return remoteSettings{r.c} }

// remoteUsers manages the accounts over REST, at /users.
type remoteUsers struct{ c *Client }

func (r *remoteRepo) Users() domain.UserStore { return remoteUsers{r.c} }

func (u remoteUsers) List(ctx context.Context) ([]domain.User, error) {
	var resp struct {
		Items []domain.User `json:"items"`
	}
	if err := u.c.do(ctx, http.MethodGet, "/users", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (u remoteUsers) Create(ctx context.Context, v *domain.User, password string) error {
	body := struct {
		domain.User
		Password string `json:"password"`
	}{User: *v, Password: password}
	return u.c.do(ctx, http.MethodPost, "/users", body, v)
}

func (u remoteUsers) Update(ctx context.Context, v domain.User) error {
	idStr := strconv.FormatInt(v.ID, 10)
	if v.ID == 0 {
		idStr = v.Code
	}
	return u.c.do(ctx, http.MethodPut, "/users/"+url.PathEscape(idStr), v, nil)
}

func (u remoteUsers) SetPassword(ctx context.Context, id any, password string) error {
	idStr := fmt.Sprint(id)
	return u.c.do(ctx, http.MethodPost, "/users/"+url.PathEscape(idStr)+"/password",
		map[string]string{"password": password}, nil)
}

func (u remoteUsers) Delete(ctx context.Context, id any) error {
	idStr := fmt.Sprint(id)
	return u.c.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(idStr), nil, nil)
}

func (s remoteSettings) Get(ctx context.Context, key string) (string, error) {
	var res struct {
		Value string `json:"value"`
	}
	err := s.c.do(ctx, http.MethodGet, "/settings/"+url.PathEscape(key), nil, &res)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return "", nil
	}
	return res.Value, err
}

func (s remoteSettings) Set(ctx context.Context, key, value string) error {
	return s.c.do(ctx, http.MethodPut, "/settings/"+url.PathEscape(key),
		map[string]string{"value": value}, nil)
}

// remoteAuth posts the credentials and keeps the token the API returns,
// so that every request after the login carries it.
type remoteAuth struct{ c *Client }

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
	err := a.c.do(ctx, http.MethodPost, "/auth/login", body, &res)
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
	err := a.c.do(ctx, http.MethodPost, "/auth/reset-password", map[string]string{
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
	err := a.c.do(ctx, http.MethodPost, "/auth/register", map[string]string{
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


func (r *remoteRepo) Students() domain.Store[domain.Student] {
	return restStore[domain.Student]{
		c: r.c, path: "/students",
		id: func(v *domain.Student) string {
			if v.ID > 0 {
				return strconv.FormatInt(v.ID, 10)
			}
			return v.Code
		},
		new: func() *domain.Student { return &domain.Student{} },
	}
}

func (r *remoteRepo) Teachers() domain.Store[domain.Teacher] {
	return restStore[domain.Teacher]{
		c: r.c, path: "/teachers",
		id: func(v *domain.Teacher) string {
			if v.ID > 0 {
				return strconv.FormatInt(v.ID, 10)
			}
			return v.Code
		},
		new: func() *domain.Teacher { return &domain.Teacher{} },
	}
}

func (r *remoteRepo) Classrooms() domain.Store[domain.Classroom] {
	return restStore[domain.Classroom]{
		c: r.c, path: "/classrooms",
		id: func(v *domain.Classroom) string {
			if v.ID > 0 {
				return strconv.FormatInt(v.ID, 10)
			}
			return v.Code
		},
		new: func() *domain.Classroom { return &domain.Classroom{} },
	}
}

func (r *remoteRepo) Courses() domain.Store[domain.Course] {
	return restStore[domain.Course]{
		c: r.c, path: "/courses",
		id: func(v *domain.Course) string {
			if v.ID > 0 {
				return strconv.FormatInt(v.ID, 10)
			}
			return v.Code
		},
		new: func() *domain.Course { return &domain.Course{} },
	}
}

func (r *remoteRepo) Lessons() domain.Store[domain.Lesson] {
	return restStore[domain.Lesson]{
		c: r.c, path: "/lessons",
		id: func(v *domain.Lesson) string {
			if v.ID > 0 {
				return strconv.FormatInt(v.ID, 10)
			}
			return v.Code
		},
		new: func() *domain.Lesson { return &domain.Lesson{} },
	}
}

func (r *remoteRepo) Ledger() domain.Ledger { return remoteLedger{r.c} }

func (r *remoteRepo) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	err := r.c.do(ctx, http.MethodGet, "/stats", nil, &st)
	return st, err
}

// restStore is every remote collection: the generic half of the store, so
// that a new entity needs no HTTP code at all.
type restStore[T any] struct {
	c    *Client
	path string
	id   func(*T) string
	new  func() *T
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
	if err := r.c.do(ctx, http.MethodGet, path, nil, &buf); err != nil {
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
	if err := r.c.do(ctx, http.MethodGet, r.path+"/"+url.PathEscape(id), nil, v); err != nil {
		return *r.new(), err
	}
	return *v, nil
}

func (r restStore[T]) Save(ctx context.Context, v *T) error {
	if id := r.id(v); id == "" {
		return r.c.do(ctx, http.MethodPost, r.path, v, v)
	}
	return r.c.do(ctx, http.MethodPut, r.path+"/"+url.PathEscape(r.id(v)), v, v)
}

func (r restStore[T]) Delete(ctx context.Context, id string) error {
	return r.c.do(ctx, http.MethodDelete, r.path+"/"+url.PathEscape(id), nil, nil)
}

type remoteLedger struct{ c *Client }

func (l remoteLedger) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	path := "/ledger"
	v := url.Values{}
	if studentID != nil && studentID != "" && studentID != int64(0) {
		v.Set("studentId", fmt.Sprint(studentID))
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	var resp struct {
		Items []domain.LedgerEntry `json:"items"`
	}
	if err := l.c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (l remoteLedger) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	return l.move(ctx, "topup", studentID, amountCents, note)
}

func (l remoteLedger) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	return l.move(ctx, "consume", studentID, amountCents, note)
}

func (l remoteLedger) move(ctx context.Context, kind string, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	var e domain.LedgerEntry
	err := l.c.do(ctx, http.MethodPost, "/ledger/"+kind, map[string]any{
		"studentId": fmt.Sprint(studentID),
		"amount":    amountCents,
		"note":      note,
	}, &e)
	return e, err
}
