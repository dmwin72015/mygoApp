package postgres

import (
	"context"
	"database/sql"
	"errors"
)

type pgSettingsStore struct{ r *Repo }

func (s pgSettingsStore) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := s.r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s pgSettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.r.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}
