package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mygo-app/internal/domain"
)

// Config chooses and configures client-side data access.
type Config struct {
	// Driver is "sqlite" (default local) or "remote" (HTTP API).
	Driver string
	// DSN is the SQLite file path or remote API base URL.
	DSN string
	// Token is the bearer token the remote API expects.
	Token string
}

// Open returns the client repository specified by the config.
func Open(ctx context.Context, cfg Config) (domain.Repo, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	switch driver {
	case "", "sqlite":
		dsn := cfg.DSN
		if dsn == "" {
			dsn = "school.db"
		}
		path, err := filepath.Abs(dsn)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		return openSQLite(ctx, path)

	case "remote", "http", "api":
		if cfg.DSN == "" {
			return nil, errors.New("data: remote needs a base URL (SCHOOL_API)")
		}
		return OpenRemote(strings.TrimRight(cfg.DSN, "/"), cfg.Token), nil

	default:
		return nil, fmt.Errorf("data: unknown client driver %q, use sqlite or remote", cfg.Driver)
	}
}
