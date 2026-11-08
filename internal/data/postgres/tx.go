package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX is the common interface implemented by *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// TxRunner executes a database transaction with automatic commit/rollback and panic safety.
type TxRunner interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
	WithTxLevel(ctx context.Context, level sql.IsolationLevel, fn func(tx *sql.Tx) error) error
}

// WithTx executes fn inside a transaction with default isolation level.
func (r *Repo) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return r.WithTxLevel(ctx, sql.LevelDefault, fn)
}

// WithTxLevel executes fn inside a transaction with the given isolation level.
// If fn returns an error or panics, the transaction is automatically rolled back.
// If fn succeeds without error, the transaction is committed.
func (r *Repo) WithTxLevel(ctx context.Context, level sql.IsolationLevel, fn func(tx *sql.Tx) error) (err error) {
	var opts *sql.TxOptions
	if level != sql.LevelDefault {
		opts = &sql.TxOptions{Isolation: level}
	}
	tx, beginErr := r.db.BeginTx(ctx, opts)
	if beginErr != nil {
		return fmt.Errorf("begin transaction: %w", beginErr)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				err = fmt.Errorf("commit transaction: %w", commitErr)
			}
		}
	}()

	err = fn(tx)
	return err
}
