package mysql

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"time"

	"mygo-app/internal/domain"
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
			if !strings.Contains(s, "ENGINE=InnoDB") {
				t.Errorf("migration file missing 'ENGINE=InnoDB' table options")
			}
		}
	}

	if !foundInitial {
		t.Fatalf("00001_initial_schema.sql was not found in embedded filesystem")
	}
}

func TestNormalizeDSN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:     "bare dsn",
			input:    "root:secret@tcp(127.0.0.1:3306)/school",
			contains: []string{"parseTime=true", "charset=utf8mb4"},
		},
		{
			name:     "dsn with existing params",
			input:    "root:secret@tcp(127.0.0.1:3306)/school?timeout=5s",
			contains: []string{"parseTime=true", "charset=utf8mb4", "timeout=5s"},
		},
		{
			name:     "dsn already has parseTime",
			input:    "root:secret@tcp(127.0.0.1:3306)/school?parseTime=true",
			contains: []string{"parseTime=true", "charset=utf8mb4"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeDSN(tc.input)
			for _, item := range tc.contains {
				if !strings.Contains(got, item) {
					t.Errorf("NormalizeDSN(%q) = %q; expected to contain %q", tc.input, got, item)
				}
			}
		})
	}
}

func TestSanitizeDSN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "root:secret123@tcp(127.0.0.1:3306)/school?charset=utf8mb4",
			expected: "root:xxxxx@tcp(127.0.0.1:3306)/school?charset=utf8mb4",
		},
		{
			input:    "mysql://admin:p%40ssword%21@db.internal:3306/prod_db",
			expected: "mysql://admin:xxxxx@db.internal:3306/prod_db",
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

func TestMySQLDriverRegistered(t *testing.T) {
	drivers := sql.Drivers()
	hasMySQL := false
	for _, d := range drivers {
		if d == "mysql" {
			hasMySQL = true
			break
		}
	}
	if !hasMySQL {
		t.Fatalf("expected 'mysql' driver to be registered in database/sql, registered: %v", drivers)
	}
}

func TestOpenMySQLFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	dsn := "root:wrong@tcp(127.0.0.1:59999)/nonexistent?timeout=1s"
	_, err := Open(ctx, dsn)
	if err == nil {
		t.Fatalf("expected error connecting to non-existent mysql, got nil")
	}
}

func TestLiveMySQLMigrationAndRepo(t *testing.T) {
	dsn := "root:root1234@tcp(127.0.0.1:33306)/school_test"
	normDSN := NormalizeDSN(dsn)

	db, err := sql.Open("mysql", normDSN)
	if err != nil {
		t.Skip("skipping live MySQL test: unable to initialize driver")
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping live MySQL test: no MySQL instance at %s (%v)", dsn, err)
	}

	// Test Migrate Up
	if err := Migrate(ctx, db, "up"); err != nil {
		t.Fatalf("Migrate up failed: %v", err)
	}

	// Test Open Repo
	repo, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open repo failed: %v", err)
	}
	defer repo.Close()

	// Verify Seeding
	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.Teachers == 0 || stats.Students == 0 || stats.Courses == 0 {
		t.Fatalf("Expected seeded data, got %+v", stats)
	}

	// Test CRUD on Settings
	if err := repo.Settings().Set(ctx, "test_compat_key", "compat_value_123"); err != nil {
		t.Fatalf("Settings.Set failed: %v", err)
	}
	val, err := repo.Settings().Get(ctx, "test_compat_key")
	if err != nil || val != "compat_value_123" {
		t.Fatalf("Settings.Get = %q, %v; want compat_value_123", val, err)
	}

	// Test Ledger TopUp
	students, err := repo.Students().List(ctx, domain.Query{Limit: 1})
	if err != nil || len(students) == 0 {
		t.Fatalf("Students.List failed: %v", err)
	}
	stuID := students[0].ID
	origBal := students[0].Balance
	entry, err := repo.Ledger().TopUp(ctx, stuID, 10000, "测试充值")
	if err != nil {
		t.Fatalf("Ledger.TopUp failed: %v", err)
	}
	if entry.Balance != origBal+10000 {
		t.Errorf("Expected balance %d, got %d", origBal+10000, entry.Balance)
	}
}

