package domain

import (
	"context"
	"time"
)

// User represents an operator or administrator in the system.
type User struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
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

// Auth signs a user in, registers new accounts, or resets credentials with a recovery key.
type Auth interface {
	Login(ctx context.Context, username, password string) (User, error)
	ResetPassword(ctx context.Context, username, recoveryKey, newPassword string) error
	Register(ctx context.Context, username, password, displayName string) (User, error)
}

// UserStore manages user accounts.
type UserStore interface {
	List(ctx context.Context) ([]User, error)
	// Create adds an account, hashing its initial password.
	Create(ctx context.Context, u *User, password string) error
	// Update changes display name and role.
	Update(ctx context.Context, u User) error
	SetPassword(ctx context.Context, id any, password string) error
	Delete(ctx context.Context, id any) error
}
