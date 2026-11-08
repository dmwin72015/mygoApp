package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"mygo-app/internal/domain"
)

// CourseStore manages courses in PostgreSQL.
type CourseStore struct {
	db DBTX
}

func NewCourseStore(db DBTX) *CourseStore {
	return &CourseStore{db: db}
}

func (s *CourseStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

func (s *CourseStore) List(ctx context.Context, q domain.Query) ([]domain.Course, error) {
	qb := NewQueryBuilder("SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses")
	qb.WhereSearch(q.Search, "name", "subject", "code")
	qb.OrderBy("id DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	var list []domain.Course
	err := s.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.Course{}
	}
	return list, err
}

func (s *CourseStore) Get(ctx context.Context, idOrCode string) (domain.Course, error) {
	var (
		item  domain.Course
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses WHERE code = $1"
		args = []any{idOrCode}
	}

	err := s.dbtx(ctx).GetContext(ctx, &item, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Course{}, domain.ErrNotFound
	}
	return item, err
}

func (s *CourseStore) Save(ctx context.Context, v *domain.Course) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("课程名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("crs")
	}

	if v.ID == 0 {
		return s.dbtx(ctx).GetContext(ctx, &v.ID,
			`INSERT INTO courses (code, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id`,
			v.Code, v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, v.Active,
		)
	}

	res, err := s.dbtx(ctx).ExecContext(ctx,
		`UPDATE courses SET
			name = $1, subject = $2, teacher_id = $3, total_lessons = $4, price_cents = $5, minutes = $6, active = $7
		 WHERE id = $8`,
		v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, v.Active, v.ID,
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

func (s *CourseStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM courses WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM courses WHERE code = $1"
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
