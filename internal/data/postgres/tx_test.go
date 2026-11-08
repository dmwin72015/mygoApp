package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestTxManager_LivePostgres(t *testing.T) {
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

	txm := NewTxManager(db)
	settings := NewSettingsStore(db)

	var key = "test_tx_key"

	// 1. Commit on success
	err = txm.RunInTx(ctx, func(txCtx context.Context) error {
		// Notice settings.Set automatically runs inside the transaction via txCtx!
		return settings.Set(txCtx, key, "committed_val")
	})
	if err != nil {
		t.Fatalf("RunInTx commit failed: %v", err)
	}

	val, err := settings.Get(ctx, key)
	if err != nil || val != "committed_val" {
		t.Fatalf("expected 'committed_val', got %q (err: %v)", val, err)
	}

	// 2. Rollback on error
	err = txm.RunInTx(ctx, func(txCtx context.Context) error {
		if err := settings.Set(txCtx, key, "should_rollback"); err != nil {
			return err
		}
		return errors.New("simulated error")
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	val, err = settings.Get(ctx, key)
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
		_ = txm.RunInTx(ctx, func(txCtx context.Context) error {
			_ = settings.Set(txCtx, key, "panic_val")
			panic("something went wrong")
		})
	}()

	val, err = settings.Get(ctx, key)
	if err != nil || val != "committed_val" {
		t.Fatalf("expected rollback after panic to preserve 'committed_val', got %q", val)
	}

	// 4. Nested / Reentrant transaction joins parent
	err = txm.RunInTx(ctx, func(txCtx context.Context) error {
		if err := settings.Set(txCtx, key, "parent_val"); err != nil {
			return err
		}
		return txm.RunInTx(txCtx, func(nestedCtx context.Context) error {
			return settings.Set(nestedCtx, key, "nested_val")
		})
	})
	if err != nil {
		t.Fatalf("nested RunInTx failed: %v", err)
	}

	val, err = settings.Get(ctx, key)
	if err != nil || val != "nested_val" {
		t.Fatalf("expected 'nested_val', got %q", val)
	}

	// Cleanup
	_, _ = db.ExecContext(ctx, "DELETE FROM settings WHERE key = $1", key)
}
