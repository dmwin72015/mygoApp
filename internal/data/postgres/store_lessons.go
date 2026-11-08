package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"mygo-app/internal/domain"
)

// LessonStore manages lessons in PostgreSQL.
type LessonStore struct {
	db DBTX
}

func NewLessonStore(db DBTX) *LessonStore {
	return &LessonStore{db: db}
}

func (s *LessonStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

func (s *LessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	qb := NewQueryBuilder("SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons")
	qb.WhereSearch(q.Search, "status", "code")
	qb.OrderBy("start_at DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	rows, err := s.dbtx(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Lesson
	for rows.Next() {
		var item domain.Lesson
		if err := rows.Scan(&item.ID, &item.Code, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *LessonStore) Get(ctx context.Context, idOrCode string) (domain.Lesson, error) {
	var (
		item  domain.Lesson
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE code = $1"
		args = []any{idOrCode}
	}

	err := s.dbtx(ctx).QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return item, err
}

func (s *LessonStore) Save(ctx context.Context, v *domain.Lesson) error {
	if v.Code == "" {
		v.Code = domain.NewID("les")
	}
	if v.StartAt.IsZero() {
		v.StartAt = time.Now()
	}
	if v.Status == "" {
		v.Status = "planned"
	}

	if v.ID == 0 {
		return s.dbtx(ctx).QueryRowContext(ctx,
			`INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id`,
			v.Code, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status,
		).Scan(&v.ID)
	}

	res, err := s.dbtx(ctx).ExecContext(ctx,
		`UPDATE lessons SET
			course_id = $1, teacher_id = $2, classroom_id = $3, start_at = $4, minutes = $5, status = $6
		 WHERE id = $7`,
		v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status, v.ID,
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

func (s *LessonStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM lessons WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM lessons WHERE code = $1"
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
