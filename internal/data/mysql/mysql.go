package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-app/internal/data/rdb"
	"mygo-app/internal/domain"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// NormalizeDSN ensures MySQL DSN contains required connection options like parseTime=true.
func NormalizeDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	if !strings.Contains(dsn, "charset=") {
		dsn += "&charset=utf8mb4"
	}
	return dsn
}

// SanitizeDSN masks sensitive credentials for logging.
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

// Open connects to MySQL, runs migrations, initializes the unified RDB repo, and returns domain.Repo.
func Open(ctx context.Context, dsn string) (domain.Repo, error) {
	normDSN := NormalizeDSN(dsn)
	rawDB, err := sql.Open("mysql", normDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql %s: %w", SanitizeDSN(dsn), err)
	}

	rawDB.SetMaxOpenConns(25)
	rawDB.SetMaxIdleConns(10)
	rawDB.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := rawDB.PingContext(pingCtx); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("ping mysql %s: %w", SanitizeDSN(dsn), err)
	}

	if err := Migrate(ctx, rawDB, "up"); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("migrate mysql: %w", err)
	}

	db := sqlx.NewDb(rawDB, "mysql")
	repo, err := rdb.NewRepo(ctx, db, "mysql", "MySQL · "+SanitizeDSN(dsn))
	if err != nil {
		_ = rawDB.Close()
		return nil, err
	}

	return repo, nil
}
