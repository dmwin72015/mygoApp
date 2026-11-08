package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mygo-app/internal/data/postgres"

	"github.com/pressly/goose/v3"
)

func TestMigrateCreateCommand(t *testing.T) {
	tempDir := t.TempDir()

	goose.SetSequential(true)
	if err := goose.Create(nil, tempDir, "test_add_field", "sql"); err != nil {
		t.Fatalf("goose.Create failed: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 migration file created, got %d", len(entries))
	}

	content, err := os.ReadFile(filepath.Join(tempDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	s := string(content)
	if len(s) == 0 {
		t.Fatalf("expected non-empty migration template")
	}
}

func TestLivePostgresMigrateWorkflow(t *testing.T) {
	dsn := "postgres://postgres:root1234@127.0.0.1:15432/school?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping live postgres migrate test: driver error")
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping live postgres migrate test: cannot reach %s", postgres.SanitizeDSN(dsn))
	}

	// Test status
	if err := postgres.Migrate(ctx, db, "status"); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	// Test version
	if err := postgres.Migrate(ctx, db, "version"); err != nil {
		t.Fatalf("version failed: %v", err)
	}
}
