package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-app/internal/data/rdb"
	"mygo-app/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// SanitizeDSN masks password from database connection string for logs.
func SanitizeDSN(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.User != nil {
		return u.Redacted()
	}
	atIdx := strings.LastIndex(raw, "@")
	if atIdx != -1 {
		userInfo := raw[:atIdx]
		colonIdx := strings.Index(userInfo, ":")
		if colonIdx != -1 {
			return userInfo[:colonIdx] + ":xxxxx@" + raw[atIdx+1:]
		}
	}
	return raw
}

// Open connects to PostgreSQL, runs schema migrations, initializes unified RDB repo, and returns domain.Repo.
func Open(ctx context.Context, dsn string) (domain.Repo, error) {
	rawDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", SanitizeDSN(dsn), err)
	}

	rawDB.SetMaxOpenConns(25)
	rawDB.SetMaxIdleConns(10)
	rawDB.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := rawDB.PingContext(pingCtx); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("ping postgres %s: %w", SanitizeDSN(dsn), err)
	}

	if err := Migrate(ctx, rawDB, "up"); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}

	db := sqlx.NewDb(rawDB, "postgres")
	repo, err := rdb.NewRepo(ctx, db, "postgres", "PostgreSQL · "+SanitizeDSN(dsn))
	if err != nil {
		_ = rawDB.Close()
		return nil, err
	}

	return repo, nil
}
