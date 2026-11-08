package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"mygo-app/internal/domain"
)

type pgClassroomStore struct{ r *Repo }

func (c pgClassroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	qb := NewQueryBuilder("SELECT id, code, name, capacity, location, note FROM classrooms")
	qb.WhereSearch(q.Search, "name", "location", "code")
	qb.OrderBy("id ASC")
	qb.Limit(q.Limit)

	query, args := qb.Build()
	rows, err := c.r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Classroom
	for rows.Next() {
		var item domain.Classroom
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Capacity, &item.Location, &item.Note); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (c pgClassroomStore) Get(ctx context.Context, idOrCode string) (domain.Classroom, error) {
	var (
		item  domain.Classroom
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, capacity, location, note FROM classrooms WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, capacity, location, note FROM classrooms WHERE code = $1"
		args = []any{idOrCode}
	}

	err := c.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Capacity, &item.Location, &item.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return item, err
}

func (c pgClassroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("教室名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("rom")
	}

	if v.ID == 0 {
		return c.r.db.QueryRowContext(ctx,
			`INSERT INTO classrooms (code, name, capacity, location, note)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id`,
			v.Code, v.Name, v.Capacity, v.Location, v.Note,
		).Scan(&v.ID)
	}

	res, err := c.r.db.ExecContext(ctx,
		`UPDATE classrooms SET
			name = $1, capacity = $2, location = $3, note = $4
		 WHERE id = $5`,
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

func (c pgClassroomStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM classrooms WHERE id = $1 OR code = $2"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM classrooms WHERE code = $1"
		args = []any{idOrCode}
	}

	res, err := c.r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}
