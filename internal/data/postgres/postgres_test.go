package postgres

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"time"
)

func TestMigrationFSEmbedded(t *testing.T) {
	mfs := MigrationFS()
	entries, err := fs.ReadDir(mfs, "migrations")
	if err != nil {
		t.Fatalf("failed to read embedded migrations dir: %v", err)
	}

	if len(entries) == 0 {
		t.Fatalf("expected embedded migration files, got 0")
	}

	foundInitial := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "00001_initial_schema.sql") {
			foundInitial = true
			content, err := fs.ReadFile(mfs, "migrations/"+entry.Name())
			if err != nil {
				t.Fatalf("failed to read migration file %s: %v", entry.Name(), err)
			}
			s := string(content)
			if !strings.Contains(s, "-- +goose Up") {
				t.Errorf("migration file missing '-- +goose Up'")
			}
			if !strings.Contains(s, "-- +goose Down") {
				t.Errorf("migration file missing '-- +goose Down'")
			}
		}
	}

	if !foundInitial {
		t.Fatalf("00001_initial_schema.sql was not found in embedded filesystem")
	}
}

func TestSanitizeDSN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "postgres://user:secret123@localhost:5432/school?sslmode=disable",
			expected: "postgres://user:xxxxx@localhost:5432/school?sslmode=disable",
		},
		{
			input:    "postgresql://admin:p%40ssword%21@db.internal:5432/prod_db",
			expected: "postgresql://admin:xxxxx@db.internal:5432/prod_db",
		},
		{
			input:    "school.db",
			expected: "school.db",
		},
	}

	for _, tc := range tests {
		got := SanitizeDSN(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeDSN(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestPGXDriverRegistered(t *testing.T) {
	drivers := sql.Drivers()
	hasPGX := false
	for _, d := range drivers {
		if d == "pgx" {
			hasPGX = true
			break
		}
	}
	if !hasPGX {
		t.Fatalf("expected 'pgx' driver to be registered in database/sql, registered: %v", drivers)
	}
}

func TestOpenPostgresFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dsn := "postgres://nonexistent:fake@127.0.0.1:59999/nonexistent?sslmode=disable&connect_timeout=1"
	_, err := Open(ctx, dsn)
	if err == nil {
		t.Fatalf("expected error connecting to non-existent postgres, got nil")
	}
}

func TestLivePostgresMigration(t *testing.T) {
	dsn := "postgres://postgres:root1234@127.0.0.1:15432/school?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping live postgres test: driver error")
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping live postgres test: cannot reach %s (%v)", SanitizeDSN(dsn), err)
	}

	// Verify migrate up executes without error
	if err := Migrate(ctx, db, "up"); err != nil {
		t.Fatalf("Migrate up failed: %v", err)
	}

	// Verify tables exist
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version").Scan(&count)
	if err != nil {
		t.Fatalf("Query goose_db_version failed: %v", err)
	}
	if count == 0 {
		t.Fatalf("Expected migration records in goose_db_version")
	}
}

