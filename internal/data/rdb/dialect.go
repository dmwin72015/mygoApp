package rdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// Dialect abstracts database-specific dialect behaviors.
type Dialect struct {
	driverName string
	bindType   int
}

// NewDialect creates a Dialect instance for the given driver ("postgres", "mysql", "sqlite3").
func NewDialect(driverName string) *Dialect {
	driverName = strings.ToLower(strings.TrimSpace(driverName))
	bindType := sqlx.QUESTION
	if driverName == "postgres" || driverName == "postgresql" || driverName == "pgx" {
		bindType = sqlx.DOLLAR
		driverName = "postgres"
	}
	return &Dialect{
		driverName: driverName,
		bindType:   bindType,
	}
}

func (d *Dialect) DriverName() string {
	return d.driverName
}

func (d *Dialect) IsPostgres() bool {
	return d.driverName == "postgres"
}

func (d *Dialect) IsMySQL() bool {
	return d.driverName == "mysql"
}

// Rebind adapts query placeholders (? -> $1 for Postgres, ? for MySQL/SQLite).
func (d *Dialect) Rebind(query string) string {
	return sqlx.Rebind(d.bindType, query)
}

// InsertAndGetID executes an INSERT statement and retrieves the newly generated primary key ID.
// For PostgreSQL, it appends " RETURNING id" and retrieves the ID via GetContext.
// For MySQL and SQLite, it uses LastInsertId() from sql.Result.
func (d *Dialect) InsertAndGetID(ctx context.Context, dbtx DBTX, query string, args ...any) (int64, error) {
	if d.IsPostgres() {
		q := d.Rebind(strings.TrimSpace(query) + " RETURNING id")
		var id int64
		err := dbtx.GetContext(ctx, &id, q, args...)
		return id, err
	}

	q := d.Rebind(query)
	res, err := dbtx.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpsertSettingsSQL returns the dialect-specific UPSERT statement for the settings table.
func (d *Dialect) UpsertSettingsSQL() string {
	if d.IsPostgres() {
		return `INSERT INTO settings (key, value) VALUES ($1, $2)
		        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	}
	return "INSERT INTO settings (`key`, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)"
}

// GetSettingsSQL returns the dialect-specific query for settings by key.
func (d *Dialect) GetSettingsSQL() string {
	if d.IsPostgres() {
		return "SELECT value FROM settings WHERE key = $1"
	}
	return "SELECT value FROM settings WHERE `key` = ?"
}

// UpsertAdminSQL returns the dialect-specific statement to safely seed the initial admin account.
func (d *Dialect) UpsertAdminSQL() string {
	if d.IsPostgres() {
		return `INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		        VALUES ($1, $2, $3, $4, $5, $6, $7)
		        ON CONFLICT (username) DO NOTHING`
	}
	return `INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
	        VALUES (?, ?, ?, ?, ?, ?, ?)
	        ON DUPLICATE KEY UPDATE id=id`
}

// SearchCond formats a single search condition for column and placeholder.
func (d *Dialect) SearchCond(col string, placeholder string) string {
	if d.IsPostgres() {
		return fmt.Sprintf("%s ILIKE %s", col, placeholder)
	}
	return fmt.Sprintf("%s LIKE %s", col, placeholder)
}
