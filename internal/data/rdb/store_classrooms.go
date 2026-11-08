package rdb

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"mygo-app/internal/domain"
)

// ClassroomStore manages classrooms across relational databases.
type ClassroomStore struct {
	db      DBTX
	dialect *Dialect
}

func NewClassroomStore(db DBTX, dialect *Dialect) *ClassroomStore {
	return &ClassroomStore{db: db, dialect: dialect}
}

func (c *ClassroomStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, c.db)
}

func (c *ClassroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	qb := NewQueryBuilder(c.dialect, "SELECT id, code, name, capacity, location, note FROM classrooms")
	qb.WhereSearch(q.Search, "name", "location", "code")
	qb.OrderBy("id ASC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	var list []domain.Classroom
	err := c.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.Classroom{}
	}
	return list, err
}

func (c *ClassroomStore) Get(ctx context.Context, idOrCode string) (domain.Classroom, error) {
	var (
		item  domain.Classroom
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = c.dialect.Rebind("SELECT id, code, name, capacity, location, note FROM classrooms WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = c.dialect.Rebind("SELECT id, code, name, capacity, location, note FROM classrooms WHERE code = ?")
		args = []any{idOrCode}
	}

	err := c.dbtx(ctx).GetContext(ctx, &item, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return item, err
}

func (c *ClassroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("教室名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("rom")
	}

	if v.ID == 0 {
		newID, err := c.dialect.InsertAndGetID(ctx, c.dbtx(ctx),
			`INSERT INTO classrooms (code, name, capacity, location, note)
			 VALUES (?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Capacity, v.Location, v.Note,
		)
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	query := c.dialect.Rebind(
		`UPDATE classrooms SET
			name = ?, capacity = ?, location = ?, note = ?
		 WHERE id = ?`,
	)
	res, err := c.dbtx(ctx).ExecContext(ctx, query,
		v.Name, v.Capacity, v.Location, v.Note, v.ID,
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

func (c *ClassroomStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = c.dialect.Rebind("DELETE FROM classrooms WHERE id = ? OR code = ?")
		args = []any{numID, idOrCode}
	} else {
		query = c.dialect.Rebind("DELETE FROM classrooms WHERE code = ?")
		args = []any{idOrCode}
	}

	res, err := c.dbtx(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
