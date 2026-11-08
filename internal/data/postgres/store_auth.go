package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"
)

// AuthStore handles user authentication, password resets and self-registration.
// It depends strictly on domain.UserStore and domain.Settings interfaces.
type AuthStore struct {
	db       DBTX
	users    domain.UserStore
	settings domain.Settings
}

func NewAuthStore(db DBTX, users domain.UserStore, settings domain.Settings) *AuthStore {
	return &AuthStore{
		db:       db,
		users:    users,
		settings: settings,
	}
}

func (a *AuthStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, a.db)
}

func (a *AuthStore) Login(ctx context.Context, username, password string) (domain.User, error) {
	var (
		u          domain.User
		salt, hash string
	)
	err := a.dbtx(ctx).QueryRowContext(ctx,
		"SELECT id, code, username, display_name, role, salt, hash, created_at FROM users WHERE username = $1",
		username,
	).Scan(&u.ID, &u.Code, &u.Username, &u.DisplayName, &u.Role, &salt, &hash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrBadCredentials
	}
	if err != nil {
		return domain.User{}, err
	}

	if !crypto.CheckPassword(password, salt, hash) {
		return domain.User{}, domain.ErrBadCredentials
	}
	return u, nil
}

func (a *AuthStore) ResetPassword(ctx context.Context, username, recoveryKey, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("请输入用户名")
	}
	recoveryKey = strings.TrimSpace(recoveryKey)
	if recoveryKey == "" {
		return domain.ErrInvalidRecoveryKey
	}
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 6 {
		return errors.New("新密码长度不能少于 6 位")
	}

	expectedKey := domain.DefaultRecoveryKey
	savedKey, err := a.settings.Get(ctx, domain.SettingsKeyRecoveryKey)
	if err == nil && savedKey != "" {
		expectedKey = savedKey
	}
	if recoveryKey != expectedKey {
		return domain.ErrInvalidRecoveryKey
	}

	var userID int64
	err = a.dbtx(ctx).QueryRowContext(ctx, "SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return err
	}

	salt, hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = a.dbtx(ctx).ExecContext(ctx,
		"UPDATE users SET salt = $1, hash = $2 WHERE id = $3",
		salt, hash, userID,
	)
	return err
}

func (a *AuthStore) Register(ctx context.Context, username, password, displayName string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.User{}, errors.New("用户名不能为空")
	}
	if len([]rune(password)) < 6 {
		return domain.User{}, errors.New("密码至少 6 位")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = username
	}
	user := domain.User{
		Username:    username,
		DisplayName: displayName,
		Role:        string(domain.RoleStaff),
	}
	if err := a.users.Create(ctx, &user, password); err != nil {
		return domain.User{}, err
	}
	return user, nil
}
