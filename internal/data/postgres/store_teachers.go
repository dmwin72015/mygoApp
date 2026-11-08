package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"mygo-app/internal/domain"
)

type pgTeacherStore struct{ r *Repo }

func (t pgTeacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	qb := NewQueryBuilder("SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers")
	qb.WhereSearch(q.Search, "name", "phone", "subject", "code")
	qb.OrderBy("id DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	rows, err := t.r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Teacher
	for rows.Next() {
		var item domain.Teacher
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (t pgTeacherStore) Get(ctx context.Context, idOrCode string) (domain.Teacher, error) {
	var (
		item  domain.Teacher
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE code = $1"
		args = []any{idOrCode}
	}

	err := t.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Teacher{}, domain.ErrNotFound
	}
	return item, err
}

func (t pgTeacherStore) Save(ctx context.Context, v *domain.Teacher) error {
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
		return t.r.db.QueryRowContext(ctx,
			`INSERT INTO teachers (code, name, phone, subject, rate_cents, active, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id`,
			v.Code, v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.CreatedAt,
		).Scan(&v.ID)
	}

	res, err := t.r.db.ExecContext(ctx,
		`UPDATE teachers SET
			name = $1, phone = $2, subject = $3, rate_cents = $4, active = $5
		 WHERE id = $6`,
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

func (t pgTeacherStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM teachers WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM teachers WHERE code = $1"
		args = []any{idOrCode}
	}

	res, err := t.r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
