package remote

import (
	"context"
	"net/http"
	"strconv"

	"mygo-app/internal/captcha"
	"mygo-app/internal/domain"
)

// Repo implements domain.Repo over the HTTP REST API.
type Repo struct {
	c *Client
}

// Open connects to a remote REST API service and returns a domain.Repo.
func Open(baseURL, token string) domain.Repo {
	return &Repo{c: NewClient(baseURL, token)}
}

func (r *Repo) Close() error   { return nil }
func (r *Repo) Source() string { return "远程 API · " + r.c.BaseURL }

// Client returns the underlying Client instance, useful in tests.
func (r *Repo) Client() *Client {
	return r.c
}

// GetCaptcha requests a fresh math arithmetic captcha challenge from the API.
func (r *Repo) GetCaptcha(ctx context.Context) (captcha.Item, error) {
	var item captcha.Item
	err := r.c.Do(ctx, http.MethodGet, "/auth/captcha", nil, &item)
	return item, err
}

func (r *Repo) Auth() domain.Auth         { return newAuth(r.c) }
func (r *Repo) Settings() domain.Settings { return newSettings(r.c) }
func (r *Repo) Users() domain.UserStore   { return newUsers(r.c) }
func (r *Repo) Ledger() domain.Ledger     { return newLedger(r.c) }

func (r *Repo) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	err := r.c.Do(ctx, http.MethodGet, "/stats", nil, &st)
	return st, err
}

func (r *Repo) Students() domain.Store[domain.Student] {
	return newRESTStore[domain.Student](r.c, "/students", func(v *domain.Student) string {
		if v.ID > 0 {
			return strconv.FormatInt(v.ID, 10)
		}
		return v.Code
	}, func() *domain.Student { return &domain.Student{} })
}

func (r *Repo) Teachers() domain.Store[domain.Teacher] {
	return newRESTStore[domain.Teacher](r.c, "/teachers", func(v *domain.Teacher) string {
		if v.ID > 0 {
			return strconv.FormatInt(v.ID, 10)
		}
		return v.Code
	}, func() *domain.Teacher { return &domain.Teacher{} })
}

func (r *Repo) Classrooms() domain.Store[domain.Classroom] {
	return newRESTStore[domain.Classroom](r.c, "/classrooms", func(v *domain.Classroom) string {
		if v.ID > 0 {
			return strconv.FormatInt(v.ID, 10)
		}
		return v.Code
	}, func() *domain.Classroom { return &domain.Classroom{} })
}

func (r *Repo) Courses() domain.Store[domain.Course] {
	return newRESTStore[domain.Course](r.c, "/courses", func(v *domain.Course) string {
		if v.ID > 0 {
			return strconv.FormatInt(v.ID, 10)
		}
		return v.Code
	}, func() *domain.Course { return &domain.Course{} })
}

func (r *Repo) Lessons() domain.Store[domain.Lesson] {
	return newRESTStore[domain.Lesson](r.c, "/lessons", func(v *domain.Lesson) string {
		if v.ID > 0 {
			return strconv.FormatInt(v.ID, 10)
		}
		return v.Code
	}, func() *domain.Lesson { return &domain.Lesson{} })
}
