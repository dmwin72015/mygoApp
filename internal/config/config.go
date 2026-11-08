package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server network and timeout settings.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	Mode         string        `yaml:"mode"` // "debug" | "release"
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// DatabaseConfig holds connection parameters and pool settings.
type DatabaseConfig struct {
	Driver          string        `yaml:"driver"` // "postgres" | "sqlite"
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// MigrateConfig holds database migration options.
type MigrateConfig struct {
	Auto   bool   `yaml:"auto"`   // Auto-run migrations on server start
	Action string `yaml:"action"` // "up", "down", "status", "version", "none"
}

// Config is the top-level configuration for the server.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Migrate  MigrateConfig  `yaml:"migrate"`
}

// Default returns a Config populated with sensible production-ready defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			Mode:         "release",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Database: DatabaseConfig{
			Driver:          "postgres",
			DSN:             "postgres://postgres:password@127.0.0.1:5432/school?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Migrate: MigrateConfig{
			Auto:   true,
			Action: "up",
		},
	}
}

// Load loads configuration from a YAML file at filePath (if provided and exists),
// then overlays environment variables and CLI overrides.
func Load(filePath string) (*Config, error) {
	cfg := Default()

	// Try reading file if given, or fallback to default paths
	targetPath := filePath
	if targetPath == "" {
		for _, cand := range []string{"configs/server.yaml", "config.yaml", "server.yaml"} {
			if _, err := os.Stat(cand); err == nil {
				targetPath = cand
				break
			}
		}
	}

	if targetPath != "" {
		data, err := os.ReadFile(targetPath)
		if err != nil {
			if filePath != "" {
				// User explicitly specified a file that couldn't be read
				return nil, fmt.Errorf("read config file %s: %w", targetPath, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse yaml config %s: %w", targetPath, err)
			}
		}
	}

	// Environment variable overrides
	if val := os.Getenv("PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = p
		}
	}
	if val := os.Getenv("GIN_MODE"); val != "" {
		cfg.Server.Mode = val
	}
	if val := os.Getenv("SCHOOL_DRIVER"); val != "" {
		cfg.Database.Driver = strings.ToLower(strings.TrimSpace(val))
	}
	if val := os.Getenv("SCHOOL_DSN"); val != "" {
		cfg.Database.DSN = val
	} else if val := os.Getenv("SCHOOL_DB"); val != "" {
		cfg.Database.DSN = val
	}
	if val := os.Getenv("SCHOOL_MIGRATE"); val != "" {
		cfg.Migrate.Action = strings.ToLower(strings.TrimSpace(val))
	}

	// Auto-detect driver from DSN prefix if driver is not explicitly set
	if cfg.Database.Driver == "" {
		if strings.HasPrefix(cfg.Database.DSN, "postgres://") || strings.HasPrefix(cfg.Database.DSN, "postgresql://") {
			cfg.Database.Driver = "postgres"
		} else {
			cfg.Database.Driver = "sqlite"
		}
	}

	return cfg, nil
}
