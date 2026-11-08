package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClientConfig holds configuration for the desktop GUI client.
type ClientConfig struct {
	// Driver is "remote" (HTTP REST API) or "sqlite" (local offline file).
	Driver string `yaml:"driver"`
	// APIURL is the remote server base URL (e.g. "http://127.0.0.1:8099").
	APIURL string `yaml:"api_url"`
	// SQLitePath is the local SQLite file path (default: "school.db").
	SQLitePath string `yaml:"sqlite_path"`
	// Token is the optional pre-configured bearer token.
	Token string `yaml:"token"`
}

// DefaultClient returns a default ClientConfig (sqlite, school.db).
func DefaultClient() *ClientConfig {
	return &ClientConfig{
		Driver:     "sqlite",
		APIURL:     "http://127.0.0.1:8099",
		SQLitePath: "school.db",
	}
}

// LoadClient loads client configuration from YAML, then applies env overrides.
func LoadClient(filePath string) (*ClientConfig, error) {
	cfg := DefaultClient()

	targetPath := filePath
	if targetPath == "" {
		for _, cand := range []string{"configs/client.yaml", "client.yaml", "configs/client.yml"} {
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
				return nil, fmt.Errorf("read client config %s: %w", targetPath, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse client yaml %s: %w", targetPath, err)
			}
		}
	}

	// Environment variable overrides
	if api := os.Getenv("SCHOOL_API"); api != "" {
		cfg.Driver = "remote"
		cfg.APIURL = strings.TrimRight(api, "/")
	}
	if driver := os.Getenv("SCHOOL_DRIVER"); driver != "" {
		cfg.Driver = strings.ToLower(strings.TrimSpace(driver))
	}
	if dsn := os.Getenv("SCHOOL_DB"); dsn != "" {
		if cfg.Driver == "remote" || strings.HasPrefix(dsn, "http://") || strings.HasPrefix(dsn, "https://") {
			cfg.Driver = "remote"
			cfg.APIURL = strings.TrimRight(dsn, "/")
		} else {
			cfg.SQLitePath = dsn
		}
	}
	if token := os.Getenv("SCHOOL_TOKEN"); token != "" {
		cfg.Token = token
	}

	return cfg, nil
}

// SaveClient writes ClientConfig to a YAML file (defaults to configs/client.yaml).
func SaveClient(targetPath string, cfg *ClientConfig) error {
	if targetPath == "" {
		targetPath = "configs/client.yaml"
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0o644)
}
