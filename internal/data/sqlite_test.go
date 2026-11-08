package data

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mygo-app/internal/domain"
)

// TestLogin checks the default account the seed adds, and that a wrong
// password and an unknown user are the same error.
func TestLogin(t *testing.T) {
	ctx := context.Background()
	repo, err := Open(ctx, Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "auth.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()

	u, err := repo.Auth().Login(ctx, DefaultUsername, DefaultPassword)
	if err != nil {
		t.Fatalf("login with the seeded account: %v", err)
	}
	if u.Username != DefaultUsername || u.Name() == "" || u.Role == "" {
		t.Errorf("login returned %+v", u)
	}

	if _, err := repo.Auth().Login(ctx, DefaultUsername, "wrong-password"); !errors.Is(err, domain.ErrBadCredentials) {
		t.Errorf("a wrong password returned %v, want ErrBadCredentials", err)
	}
	if _, err := repo.Auth().Login(ctx, "nobody", DefaultPassword); !errors.Is(err, domain.ErrBadCredentials) {
		t.Errorf("an unknown user returned %v, want ErrBadCredentials", err)
	}
	// The username is trimmed, so a space typed before it still works.
	if _, err := repo.Auth().Login(ctx, "  "+DefaultUsername+"  ", DefaultPassword); err != nil {
		t.Errorf("a padded username: %v", err)
	}
}

// TestUserStore checks the accounts an admin hands out: a new login works,
// a duplicate username and a short password are refused, a reset password
// replaces the old one, and the database always keeps one admin.
func TestUserStore(t *testing.T) {
	ctx := context.Background()
	repo, err := Open(ctx, Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "users.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	users := repo.Users()

	all, err := users.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].Username != DefaultUsername {
		t.Fatalf("a fresh database has %+v, want only the admin", all)
	}
	admin := all[0]

	staff := domain.User{Username: "teacher1", DisplayName: "王老师", Role: string(domain.RoleStaff)}
	if err := users.Create(ctx, &staff, "secret123"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if staff.ID == "" {
		t.Error("create left the account without an ID")
	}
	if _, err := repo.Auth().Login(ctx, "teacher1", "secret123"); err != nil {
		t.Errorf("the new account cannot sign in: %v", err)
	}

	dup := domain.User{Username: "teacher1", Role: string(domain.RoleStaff)}
	if err := users.Create(ctx, &dup, "secret123"); err == nil {
		t.Error("a duplicate username was allowed")
	}
	short := domain.User{Username: "teacher2", Role: string(domain.RoleStaff)}
	if err := users.Create(ctx, &short, "123"); err == nil {
		t.Error("a 3-character password was allowed")
	}

	if err := users.SetPassword(ctx, staff.ID, "better123"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if _, err := repo.Auth().Login(ctx, "teacher1", "secret123"); !errors.Is(err, domain.ErrBadCredentials) {
		t.Error("the old password still works after a reset")
	}
	if _, err := repo.Auth().Login(ctx, "teacher1", "better123"); err != nil {
		t.Errorf("the new password does not work: %v", err)
	}

	// The admin is the only one: neither demoting nor deleting may leave
	// the app without an administrator.
	demoted := admin
	demoted.Role = string(domain.RoleStaff)
	if err := users.Update(ctx, demoted); err == nil {
		t.Error("the last admin was demoted")
	}
	if err := users.Delete(ctx, admin.ID); err == nil {
		t.Error("the last admin was deleted")
	}

	if err := users.Delete(ctx, staff.ID); err != nil {
		t.Fatalf("delete a staff account: %v", err)
	}
	if _, err := repo.Auth().Login(ctx, "teacher1", "better123"); !errors.Is(err, domain.ErrBadCredentials) {
		t.Error("a deleted account can still sign in")
	}

	// With a second admin, demoting the first is fine.
	second := domain.User{Username: "boss2", Role: string(domain.RoleAdmin)}
	if err := users.Create(ctx, &second, "secret123"); err != nil {
		t.Fatalf("create a second admin: %v", err)
	}
	if err := users.Update(ctx, demoted); err != nil {
		t.Errorf("demoting an admin while another exists: %v", err)
	}
}

// TestSQLiteStore exercises the whole local store: the seed of a fresh
// file, the ledger moving a balance, the overdraft it refuses, the profile
// write that must not touch the balance, and a reopen that must not seed a
// second time.
func TestSQLiteStore(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "test.db")

	repo, err := Open(ctx, Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	students, err := repo.Students().List(ctx, domain.Query{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(students) == 0 {
		t.Fatal("a fresh database seeded nothing")
	}
	v := students[0]
	before := v.Balance

	top, err := repo.Ledger().TopUp(ctx, v.ID, 10000, "测试充值")
	if err != nil {
		t.Fatalf("top up: %v", err)
	}
	if top.Balance != before+10000 || top.Amount != 10000 {
		t.Errorf("top up left %d with amount %d, want %d and 10000", top.Balance, top.Amount, before+10000)
	}

	use, err := repo.Ledger().Consume(ctx, v.ID, 4000, "测试消课")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if use.Balance != before+6000 {
		t.Errorf("consume left %d, want %d", use.Balance, before+6000)
	}

	if _, err := repo.Ledger().Consume(ctx, v.ID, 1<<40, "透支"); err == nil {
		t.Error("an overdraft was allowed")
	}

	v.Name = "改名后的学生"
	if err := repo.Students().Save(ctx, &v); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := repo.Students().Get(ctx, v.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "改名后的学生" {
		t.Errorf("name %q after save", got.Name)
	}
	if got.Balance != use.Balance {
		t.Errorf("balance %d after a profile save, want %d: it moves only through the ledger",
			got.Balance, use.Balance)
	}

	entries, err := repo.Ledger().List(ctx, v.ID, 10)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("%d entries, want the seed's plus two", len(entries))
	}

	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.Students != len(students) {
		t.Errorf("stats count %d students, the list has %d", stats.Students, len(students))
	}
	if stats.Revenue <= 0 || stats.Outstanding <= 0 {
		t.Errorf("stats revenue %d, outstanding %d", stats.Revenue, stats.Outstanding)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopening the same file must find what was written, not a new seed.
	again, err := Open(ctx, Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	rows, err := again.Students().List(ctx, domain.Query{})
	if err != nil {
		t.Fatalf("list after reopen: %v", err)
	}
	if len(rows) != len(students) {
		t.Errorf("the reopened store has %d students, want %d", len(rows), len(students))
	}
	kept, err := again.Students().Get(ctx, v.ID)
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if kept.Name != "改名后的学生" || kept.Balance != use.Balance {
		t.Errorf("reopen found %q with %d, want %q with %d", kept.Name, kept.Balance, "改名后的学生", use.Balance)
	}
}
