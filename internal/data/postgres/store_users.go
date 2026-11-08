package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"
)

// UserStore manages users in PostgreSQL.
type UserStore struct {
	db DBTX
}

func NewUserStore(db DBTX) *UserStore {
	return &UserStore{db: db}
}

func (u *UserStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, u.db)
}

func (u *UserStore) List(ctx context.Context) ([]domain.User, error) {
	var list []domain.User
	err := u.dbtx(ctx).SelectContext(ctx, &list,
		"SELECT id, code, username, display_name, role, created_at FROM users ORDER BY id ASC",
	)
	if list == nil {
		list = []domain.User{}
	}
	return list, err
}

func (u *UserStore) Create(ctx context.Context, v *domain.User, password string) error {
	if strings.TrimSpace(v.Username) == "" {
		return errors.New("用户名不能为空")
	}
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	salt, hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	if v.Code == "" {
		v.Code = domain.NewID("usr")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	err = u.dbtx(ctx).GetContext(ctx, &v.ID,
		`INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		v.Code, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt,
	)
	if err != nil && (strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate")) {
		return errors.New("用户名已存在")
	}
	return err
}

func (u *UserStore) Update(ctx context.Context, v domain.User) error {
	if v.Role != string(domain.RoleAdmin) {
		if err := u.keepOneAdmin(ctx, v.ID); err != nil {
			return err
		}
	}
	res, err := u.dbtx(ctx).ExecContext(ctx,
		"UPDATE users SET display_name = $1, role = $2 WHERE id = $3",
		v.DisplayName, v.Role, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u *UserStore) resolveUserID(ctx context.Context, id any) (int64, error) {
	switch v := id.(type) {
	case int64:
		if v > 0 {
			return v, nil
		}
	case int:
		if v > 0 {
			return int64(v), nil
		}
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil && num > 0 {
			return num, nil
		}
		var realID int64
		err := u.dbtx(ctx).GetContext(ctx, &realID, "SELECT id FROM users WHERE code = $1 OR username = $2", v, v)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, domain.ErrNotFound
			}
			return 0, err
		}
		return realID, nil
	}
	return 0, domain.ErrNotFound
}

func (u *UserStore) SetPassword(ctx context.Context, id any, password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	userID, err := u.resolveUserID(ctx, id)
	if err != nil {
		return err
	}

	salt, hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	res, err := u.dbtx(ctx).ExecContext(ctx, "UPDATE users SET salt = $1, hash = $2 WHERE id = $3", salt, hash, userID)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u *UserStore) Delete(ctx context.Context, id any) error {
	userID, err := u.resolveUserID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.keepOneAdmin(ctx, userID); err != nil {
		return err
	}
	res, err := u.dbtx(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u *UserStore) keepOneAdmin(ctx context.Context, id int64) error {
	var remainingAdminCount int64
	err := u.dbtx(ctx).GetContext(ctx, &remainingAdminCount,
		"SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> $1",
		id,
	)
	if err != nil {
		return err
	}
	if remainingAdminCount > 0 {
		return nil
	}

	var currentRole string
	err = u.dbtx(ctx).GetContext(ctx, &currentRole, "SELECT role FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	if currentRole == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}
