package postgres

import (
	"context"
	"database/sql"
	"errors"
)

// SettingsStore manages key-value configurations in PostgreSQL.
type SettingsStore struct {
	db DBTX
}

func NewSettingsStore(db DBTX) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, s.db)
}

func (s *SettingsStore) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := s.dbtx(ctx).GetContext(ctx, &val, "SELECT value FROM settings WHERE key = $1", key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s *SettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.dbtx(ctx).ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}
