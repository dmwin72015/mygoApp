package mysql

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed seeds/*.sql
var embedSeeds embed.FS

// SeedFS returns the embedded seeds filesystem.
func SeedFS() embed.FS {
	return embedSeeds
}

// Seed executes the demo seed SQL script against the MySQL database.
func Seed(ctx context.Context, db *sql.DB) error {
	content, err := embedSeeds.ReadFile("seeds/demo.sql")
	if err != nil {
		return fmt.Errorf("read mysql demo seed: %w", err)
	}
	_, err = db.ExecContext(ctx, string(content))
	if err != nil {
		return fmt.Errorf("execute mysql demo seed: %w", err)
	}
	return nil
}
