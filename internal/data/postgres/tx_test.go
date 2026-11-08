package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestTxRunner_LivePostgres(t *testing.T) {
	dsn := "postgres://postgres:root1234@127.0.0.1:15432/school?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping live postgres test: driver error")
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping live postgres test: cannot reach %s (%v)", SanitizeDSN(dsn), err)
	}

	repo := &Repo{db: db, dsn: dsn}

	// 1. Commit on success
	var key = "test_tx_key"
	err = repo.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value", key, "committed_val")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx commit failed: %v", err)
	}

	var val string
	err = db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err != nil || val != "committed_val" {
		t.Fatalf("expected 'committed_val', got %q (err: %v)", val, err)
	}

	// 2. Rollback on error
	err = repo.WithTx(ctx, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(ctx, "UPDATE settings SET value = 'should_rollback' WHERE key = $1", key)
		if execErr != nil {
			return execErr
		}
		return errors.New("simulated failure")
	})
	if err == nil {
		t.Fatalf("expected simulated failure error, got nil")
	}

	err = db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err != nil || val != "committed_val" {
		t.Fatalf("expected rollback to preserve 'committed_val', got %q", val)
	}

	// 3. Rollback on panic
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic to be rethrown")
			}
		}()
		_ = repo.WithTx(ctx, func(tx *sql.Tx) error {
			_, _ = tx.ExecContext(ctx, "UPDATE settings SET value = 'panic_val' WHERE key = $1", key)
			panic("something went terribly wrong")
		})
	}()

	err = db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err != nil || val != "committed_val" {
		t.Fatalf("expected rollback after panic to preserve 'committed_val', got %q", val)
	}

	// Cleanup
	_, _ = db.ExecContext(ctx, "DELETE FROM settings WHERE key = $1", key)
}
