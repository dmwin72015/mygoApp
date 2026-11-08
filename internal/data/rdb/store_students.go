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

// StudentStore manages students across relational databases (Postgres, MySQL, SQLite).
type StudentStore struct {
	db      DBTX
	dialect *Dialect
}

func NewStudentStore(db DBTX, dialect *Dialect) *StudentStore {
	return &StudentStore{db: db, dialect: dialect}
}

func (s *StudentStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

// ResolveID resolves a student numeric ID or string code to its primary key int64.
func (s *StudentStore) ResolveID(ctx context.Context, studentID any) (int64, error) {
	switch v := studentID.(type) {
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
		query := s.dialect.Rebind("SELECT id FROM students WHERE code = ?")
		err := s.dbtx(ctx).GetContext(ctx, &realID, query, v)
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

// LockAndGetBalance retrieves student's current balance with a row-level lock (FOR UPDATE).
func (s *StudentStore) LockAndGetBalance(ctx context.Context, id int64) (int64, error) {
	var currentBalance int64
	query := s.dialect.Rebind("SELECT balance_cents FROM students WHERE id = ? FOR UPDATE")
	err := s.dbtx(ctx).GetContext(ctx, &currentBalance, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return currentBalance, err
}

// UpdateBalance updates the student's balance_cents.
func (s *StudentStore) UpdateBalance(ctx context.Context, id int64, newBalance int64) error {
	query := s.dialect.Rebind("UPDATE students SET balance_cents = ? WHERE id = ?")
	res, err := s.dbtx(ctx).ExecContext(ctx, query, newBalance, id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s *StudentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	qb := NewQueryBuilder(s.dialect, "SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students")
	qb.WhereSearch(q.Search, "name", "phone", "guardian", "code")
	qb.OrderBy("id DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	var list []domain.Student
	err := s.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.Student{}
	}
	return list, err
}

func (s *StudentStore) Get(ctx context.Context, idOrCode string) (domain.Student, error) {
	var (
		item  domain.Student
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = s.dialect.Rebind("SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = s.dialect.Rebind("SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE code = ?")
		args = []any{idOrCode}
	}

	err := s.dbtx(ctx).GetContext(ctx, &item, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	return item, err
}

func (s *StudentStore) Save(ctx context.Context, v *domain.Student) error {
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
		newID, err := s.dialect.InsertAndGetID(ctx, s.dbtx(ctx),
			`INSERT INTO students (code, name, guardian, phone, balance_cents, note, active, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.CreatedAt,
		)
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	query := s.dialect.Rebind(
		`UPDATE students SET
			name = ?, guardian = ?, phone = ?, balance_cents = ?, note = ?, active = ?
		 WHERE id = ?`,
	)
	res, err := s.dbtx(ctx).ExecContext(ctx, query,
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

func (s *StudentStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = s.dialect.Rebind("DELETE FROM students WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = s.dialect.Rebind("DELETE FROM students WHERE code = ?")
		args = []any{idOrCode}
	}

	res, err := s.dbtx(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
