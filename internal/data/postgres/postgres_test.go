package postgres

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

func TestStoresCRUD_LivePostgres(t *testing.T) {
	dsn := "postgres://postgres:root1234@127.0.0.1:15432/school?sslmode=disable"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repo, err := Open(ctx, dsn)
	if err != nil {
		t.Skipf("skipping live postgres repo test: %v", err)
	}
	defer repo.Close()

	// 1. Students CRUD with sqlx
	stu := domain.Student{
		Name:     "测试学员sqlx",
		Phone:    "13888888888",
		Guardian: "家长sqlx",
		Active:   true,
	}
	if err := repo.Students().Save(ctx, &stu); err != nil {
		t.Fatalf("save student failed: %v", err)
	}
	if stu.ID == 0 || stu.Code == "" {
		t.Fatalf("expected student ID and Code to be populated, got ID=%d, Code=%q", stu.ID, stu.Code)
	}

	fetchedStu, err := repo.Students().Get(ctx, stu.Code)
	if err != nil || fetchedStu.Name != stu.Name {
		t.Fatalf("get student failed: %v (got %+v)", err, fetchedStu)
	}

	listStu, err := repo.Students().List(ctx, domain.Query{Search: "测试学员sqlx"})
	if err != nil || len(listStu) == 0 {
		t.Fatalf("list students search failed: %v (found %d)", err, len(listStu))
	}

	// 2. Teachers CRUD with sqlx
	tch := domain.Teacher{
		Name:      "测试教师sqlx",
		Phone:     "13999999999",
		Subject:   "物理",
		RateCents: 20000,
		Active:    true,
	}
	if err := repo.Teachers().Save(ctx, &tch); err != nil {
		t.Fatalf("save teacher failed: %v", err)
	}
	fetchedTch, err := repo.Teachers().Get(ctx, tch.Code)
	if err != nil || fetchedTch.Name != tch.Name {
		t.Fatalf("get teacher failed: %v (got %+v)", err, fetchedTch)
	}

	// 3. Ledger TopUp and Consume inside Tx
	topUpEntry, err := repo.Ledger().TopUp(ctx, stu.ID, 100000, "报名预存")
	if err != nil {
		t.Fatalf("ledger topup failed: %v", err)
	}
	if topUpEntry.Balance != 100000 {
		t.Fatalf("expected balance 100000, got %d", topUpEntry.Balance)
	}

	consumeEntry, err := repo.Ledger().Consume(ctx, stu.ID, 30000, "课时扣费")
	if err != nil {
		t.Fatalf("ledger consume failed: %v", err)
	}
	if consumeEntry.Balance != 70000 {
		t.Fatalf("expected balance 70000, got %d", consumeEntry.Balance)
	}

	// 4. Stats with sqlx GetContext
	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if stats.Students == 0 || stats.Revenue == 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	// Cleanup test entries
	_ = repo.Students().Delete(ctx, stu.Code)
	_ = repo.Teachers().Delete(ctx, tch.Code)
}


