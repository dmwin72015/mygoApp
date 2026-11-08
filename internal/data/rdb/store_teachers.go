package rdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"mygo-app/internal/domain"
)

// TeacherStore manages teachers across relational databases.
type TeacherStore struct {
	db      DBTX
	dialect *Dialect
}

func NewTeacherStore(db DBTX, dialect *Dialect) *TeacherStore {
	return &TeacherStore{db: db, dialect: dialect}
}

func (t *TeacherStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, t.db)
}

func (t *TeacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	qb := NewQueryBuilder(t.dialect, "SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers")
	qb.WhereSearch(q.Search, "name", "phone", "subject", "code")
	qb.OrderBy("id DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	var list []domain.Teacher
	err := t.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.Teacher{}
	}
	return list, err
}

func (t *TeacherStore) Get(ctx context.Context, idOrCode string) (domain.Teacher, error) {
	var (
		item  domain.Teacher
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = t.dialect.Rebind("SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = t.dialect.Rebind("SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE code = ?")
		args = []any{idOrCode}
	}

	err := t.dbtx(ctx).GetContext(ctx, &item, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Teacher{}, domain.ErrNotFound
	}
	return item, err
}

func (t *TeacherStore) Save(ctx context.Context, v *domain.Teacher) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("tch")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	if v.ID == 0 {
		newID, err := t.dialect.InsertAndGetID(ctx, t.dbtx(ctx),
			`INSERT INTO teachers (code, name, phone, subject, rate_cents, active, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.CreatedAt,
		)
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	query := t.dialect.Rebind(
		`UPDATE teachers SET
			name = ?, phone = ?, subject = ?, rate_cents = ?, active = ?
		 WHERE id = ?`,
	)
	res, err := t.dbtx(ctx).ExecContext(ctx, query,
		v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.ID,
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

func (t *TeacherStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = t.dialect.Rebind("DELETE FROM teachers WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = t.dialect.Rebind("DELETE FROM teachers WHERE code = ?")
		args = []any{idOrCode}
	}

	res, err := t.dbtx(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
