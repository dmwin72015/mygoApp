package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// txContextKey is the private context key for active transaction propagation.
type txContextKey struct{}

// DBTX is the common interface implemented by *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// TxManager manages atomic database transactions with context propagation,
// panic recovery, automatic rollback on failure, and commit on success.
type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
	RunInTxLevel(ctx context.Context, level sql.IsolationLevel, fn func(ctx context.Context) error) error
}

type txManager struct {
	db *sql.DB
}

// NewTxManager creates a new transaction manager bound to the DB connection pool.
func NewTxManager(db *sql.DB) TxManager {
	return &txManager{db: db}
}

// WithTx returns a child context carrying the active transaction.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext retrieves an active transaction from context, if any.
func TxFromContext(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(txContextKey{}).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// GetDBTX returns the active transaction from context if present; otherwise returns fallback.
func GetDBTX(ctx context.Context, fallback DBTX) DBTX {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return fallback
}

// RunInTx executes fn inside a transaction with LevelDefault isolation.
func (tm *txManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return tm.RunInTxLevel(ctx, sql.LevelDefault, fn)
}

// RunInTxLevel executes fn inside a transaction with the given isolation level.
func (tm *txManager) RunInTxLevel(ctx context.Context, level sql.IsolationLevel, fn func(ctx context.Context) error) (err error) {
	// Nested transaction support: participate in existing active transaction
	if existingTx := TxFromContext(ctx); existingTx != nil {
		return fn(ctx)
	}

	var opts *sql.TxOptions
	if level != sql.LevelDefault {
		opts = &sql.TxOptions{Isolation: level}
	}

	tx, beginErr := tm.db.BeginTx(ctx, opts)
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

	txCtx := WithTx(ctx, tx)
	err = fn(txCtx)
	return err
}
