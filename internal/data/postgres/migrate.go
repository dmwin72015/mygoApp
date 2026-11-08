package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"strings"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// MigrationFS returns the embedded migration filesystem.
func MigrationFS() embed.FS {
	return embedMigrations
}

// Migrate executes schema migrations on the provided database using goose.
// Supported actions: "up", "down", "redo", "reset", "status", "version", "none".
func Migrate(ctx context.Context, db *sql.DB, action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "none" || action == "off" || action == "false" {
		return nil
	}
	if action == "" {
		action = "up"
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect postgres: %w", err)
	}

	switch action {
	case "up", "up-only":
		if err := goose.UpContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose up: %w", err)
		}
	case "down":
		if err := goose.DownContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose down: %w", err)
		}
	case "redo":
		if err := goose.RedoContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose redo: %w", err)
		}
	case "reset":
		if err := goose.ResetContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose reset: %w", err)
		}
	case "status":
		if err := goose.StatusContext(ctx, db, "migrations"); err != nil {
			return fmt.Errorf("goose status: %w", err)
		}
	case "version":
		v, err := goose.GetDBVersionContext(ctx, db)
		if err != nil {
			return fmt.Errorf("goose version: %w", err)
		}
		log.Printf("[MIGRATE] Current DB version: %d", v)
	default:
		return fmt.Errorf("unknown migration action %q (supported: up, down, redo, reset, status, version, none)", action)
	}

	return nil
}
