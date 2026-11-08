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

type pgStudentStore struct{ r *Repo }

func (s pgStudentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	qb := NewQueryBuilder("SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students")
	qb.WhereSearch(q.Search, "name", "phone", "guardian", "code")
	qb.OrderBy("id DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	rows, err := s.r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Student
	for rows.Next() {
		var item domain.Student
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s pgStudentStore) Get(ctx context.Context, idOrCode string) (domain.Student, error) {
	var (
		item  domain.Student
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE code = $1"
		args = []any{idOrCode}
	}

	err := s.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	return item, err
}

func (s pgStudentStore) Save(ctx context.Context, v *domain.Student) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("stu")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	if v.ID == 0 {
		return s.r.db.QueryRowContext(ctx,
			`INSERT INTO students (code, name, guardian, phone, balance_cents, note, active, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id`,
			v.Code, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.CreatedAt,
		).Scan(&v.ID)
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE students SET
			name = $1, guardian = $2, phone = $3, balance_cents = $4, note = $5, active = $6
		 WHERE id = $7`,
		v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.ID,
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

func (s pgStudentStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM students WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM students WHERE code = $1"
		args = []any{idOrCode}
	}

	res, err := s.r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
