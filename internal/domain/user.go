package domain

import (
	"context"
	"time"
)

// User represents an operator or administrator in the system.
type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Name returns DisplayName if available, falling back to Username.
func (u User) Name() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// Auth signs a user in.
type Auth interface {
	Login(ctx context.Context, username, password string) (User, error)
}

// UserStore manages user accounts.
type UserStore interface {
	List(ctx context.Context) ([]User, error)
	// Create adds an account, hashing its initial password.
	Create(ctx context.Context, u *User, password string) error
	// Update changes display name and role.
	Update(ctx context.Context, u User) error
	SetPassword(ctx context.Context, id, password string) error
	Delete(ctx context.Context, id string) error
}
