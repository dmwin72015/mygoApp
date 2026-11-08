package rdb

import (
	"context"
	"database/sql"
	"errors"
)

// SettingsStore manages key-value configurations across relational databases.
type SettingsStore struct {
	db      DBTX
	dialect *Dialect
}

func NewSettingsStore(db DBTX, dialect *Dialect) *SettingsStore {
	return &SettingsStore{db: db, dialect: dialect}
}

func (s *SettingsStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

func (s *SettingsStore) Get(ctx context.Context, key string) (string, error) {
	var val string
	query := s.dialect.GetSettingsSQL()
	err := s.dbtx(ctx).GetContext(ctx, &val, query, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s *SettingsStore) Set(ctx context.Context, key, value string) error {
	query := s.dialect.UpsertSettingsSQL()
	_, err := s.dbtx(ctx).ExecContext(ctx, query, key, value)
	return err
}
