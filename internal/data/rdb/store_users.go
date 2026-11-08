package rdb

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

// UserStore manages users across relational databases.
type UserStore struct {
	db      DBTX
	dialect *Dialect
}

func NewUserStore(db DBTX, dialect *Dialect) *UserStore {
	return &UserStore{db: db, dialect: dialect}
}

func (u *UserStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, u.db)
}

func (u *UserStore) List(ctx context.Context) ([]domain.User, error) {
	var list []domain.User
	query := u.dialect.Rebind("SELECT id, code, username, display_name, role, created_at FROM users ORDER BY id ASC")
	err := u.dbtx(ctx).SelectContext(ctx, &list, query)
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

	newID, err := u.dialect.InsertAndGetID(ctx, u.dbtx(ctx),
		`INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.Code, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "Duplicate entry") {
			return errors.New("用户名已存在")
		}
		return err
	}
	v.ID = newID
	return nil
}

func (u *UserStore) Update(ctx context.Context, v domain.User) error {
	if v.Role != string(domain.RoleAdmin) {
		if err := u.keepOneAdmin(ctx, v.ID); err != nil {
			return err
		}
	}
	query := u.dialect.Rebind("UPDATE users SET display_name = ?, role = ? WHERE id = ?")
	res, err := u.dbtx(ctx).ExecContext(ctx, query, v.DisplayName, v.Role, v.ID)
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
		query := u.dialect.Rebind("SELECT id FROM users WHERE code = ? OR username = ?")
		err := u.dbtx(ctx).GetContext(ctx, &realID, query, v, v)
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
	query := u.dialect.Rebind("UPDATE users SET salt = ?, hash = ? WHERE id = ?")
	res, err := u.dbtx(ctx).ExecContext(ctx, query, salt, hash, userID)
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
	query := u.dialect.Rebind("DELETE FROM users WHERE id = ?")
	res, err := u.dbtx(ctx).ExecContext(ctx, query, userID)
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
	query := u.dialect.Rebind("SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?")
	err := u.dbtx(ctx).GetContext(ctx, &remainingAdminCount, query, id)
	if err != nil {
		return err
	}
	if remainingAdminCount > 0 {
		return nil
	}

	var currentRole string
	roleQuery := u.dialect.Rebind("SELECT role FROM users WHERE id = ?")
	err = u.dbtx(ctx).GetContext(ctx, &currentRole, roleQuery, id)
	if err != nil {
		return err
	}
	if currentRole == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}
