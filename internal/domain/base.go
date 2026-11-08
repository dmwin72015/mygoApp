package domain

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
)

// Identifiable represents an entity that can bind its identity (ID or Code) from a raw string.
type Identifiable interface {
	SetIdentity(rawID string)
}

// SetEntityIdentity parses rawID: if numeric positive int, sets idField; otherwise sets codeField.
func SetEntityIdentity(idField *int64, codeField *string, rawID string) {
	if num, err := strconv.ParseInt(rawID, 10, 64); err == nil && num > 0 {
		*idField = num
	} else {
		*codeField = rawID
	}
}

// ErrNotFound is what a store returns for an ID it does not hold.
var ErrNotFound = errors.New("not found")

// ErrBadCredentials is what Login returns for an unknown user or a wrong password.
var ErrBadCredentials = errors.New("用户名或密码不正确")

// ErrInvalidRecoveryKey is what ResetPassword returns when the recovery key is wrong.
var ErrInvalidRecoveryKey = errors.New("安全恢复密钥不正确")

// ErrUserNotFound is returned when the specified username does not exist.
var ErrUserNotFound = errors.New("账号不存在")

// DefaultRecoveryKey is the default fallback security recovery key.
const DefaultRecoveryKey = "qm-recovery-2026"

// SettingsKeyRecoveryKey is the key in settings table storing custom security recovery key.
const SettingsKeyRecoveryKey = "security_recovery_key"

// Query narrows a list: Search matches name or phone, Limit 0 means no limit.
type Query struct {
	Search string
	Limit  int
}

// Store is the generic CRUD interface every entity has.
type Store[T any] interface {
	List(ctx context.Context, q Query) ([]T, error)
	Get(ctx context.Context, id string) (T, error)
	// Save inserts when the ID is empty, and updates otherwise.
	Save(ctx context.Context, v *T) error
	Delete(ctx context.Context, id string) error
}

// Repo is everything a page or service needs from storage.
type Repo interface {
	Auth() Auth
	Settings() Settings
	Users() UserStore
	Students() Store[Student]
	Teachers() Store[Teacher]
	Classrooms() Store[Classroom]
	Courses() Store[Course]
	Lessons() Store[Lesson]
	Ledger() Ledger
	Stats(ctx context.Context) (Stats, error)
	Close() error
	// Source names the store for system settings / health.
	Source() string
}

var idSeq atomic.Uint64

// NewID creates a timestamp-ordered unique ID without external dependencies.
func NewID(prefix string) string {
	return fmt.Sprintf("%s_%x%03x", prefix, time.Now().UnixMilli(), idSeq.Add(1)&0xfff)
}
