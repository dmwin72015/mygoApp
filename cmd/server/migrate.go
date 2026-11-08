package main

import (
	"context"
	"database/sql"
	"log"
	"strings"

	"mygo-app/internal/config"
	"mygo-app/internal/data/postgres"
)

// handleMigrationFlags checks if a dedicated migration action was requested via CLI flags.
// If so, it executes the action and returns true (indicating main should exit).
func handleMigrationFlags(ctx context.Context, cfg *config.Config, migrateFlag string, migrateOnly bool) bool {
	action := strings.ToLower(strings.TrimSpace(cfg.Migrate.Action))
	if migrateFlag != "" {
		action = strings.ToLower(strings.TrimSpace(migrateFlag))
	}

	isDedicatedMigrate := migrateOnly ||
		action == "status" ||
		action == "down" ||
		action == "redo" ||
		action == "reset" ||
		action == "version" ||
		action == "up-only"

	if !isDedicatedMigrate {
		return false
	}

	driver := cfg.Database.Driver
	dsn := cfg.Database.DSN

	log.Printf("Executing SQL migration [%s] on %s (%s)...", action, driver, postgres.SanitizeDSN(dsn))
	if driver != "postgres" && driver != "postgresql" && driver != "pgx" {
		log.Printf("Migration action [%s] is currently designed for PostgreSQL.", action)
		return true
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	if err := postgres.Migrate(ctx, db, action); err != nil {
		log.Fatalf("Migration [%s] failed: %v", action, err)
	}
	log.Printf("Migration [%s] completed successfully.", action)
	return true
}
