package rdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"mygo-app/internal/domain"
)

// LessonStore manages lessons across relational databases.
type LessonStore struct {
	db      DBTX
	dialect *Dialect
}

func NewLessonStore(db DBTX, dialect *Dialect) *LessonStore {
	return &LessonStore{db: db, dialect: dialect}
}

func (s *LessonStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

func (s *LessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	qb := NewQueryBuilder(s.dialect, "SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons")
	qb.WhereSearch(q.Search, "status", "code")
	qb.OrderBy("start_at DESC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	var list []domain.Lesson
	err := s.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.Lesson{}
	}
	return list, err
}

func (s *LessonStore) Get(ctx context.Context, idOrCode string) (domain.Lesson, error) {
	var (
		item  domain.Lesson
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = s.dialect.Rebind("SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = s.dialect.Rebind("SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE code = ?")
		args = []any{idOrCode}
	}

	err := s.dbtx(ctx).GetContext(ctx, &item, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return item, err
}

func (s *LessonStore) Save(ctx context.Context, v *domain.Lesson) error {
	if v.CourseID == 0 {
		return errors.New("课程不能为空")
	}
	if v.TeacherID == 0 {
		return errors.New("教师不能为空")
	}
	if v.StartAt.IsZero() {
		return errors.New("开始时间不能为空")
	}
	if v.Minutes <= 0 {
		v.Minutes = 60
	}
	if v.Status == "" {
		v.Status = "planned"
	}
	if v.Code == "" {
		v.Code = domain.NewID("les")
	}

	if v.ID == 0 {
		newID, err := s.dialect.InsertAndGetID(ctx, s.dbtx(ctx),
			`INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status,
		)
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	query := s.dialect.Rebind(
		`UPDATE lessons SET
			course_id = ?, teacher_id = ?, classroom_id = ?, start_at = ?, minutes = ?, status = ?
		 WHERE id = ?`,
	)
	res, err := s.dbtx(ctx).ExecContext(ctx, query,
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
		query = s.dialect.Rebind("DELETE FROM lessons WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = s.dialect.Rebind("DELETE FROM lessons WHERE code = ?")
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
