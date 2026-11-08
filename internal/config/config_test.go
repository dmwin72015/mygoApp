package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected default driver postgres, got %s", cfg.Database.Driver)
	}
	if cfg.Migrate.Action != "up" {
		t.Errorf("expected default migrate action up, got %s", cfg.Migrate.Action)
	}
}

func TestLoadYAML(t *testing.T) {
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "test.yaml")
	content := `
server:
  host: "127.0.0.1"
  port: 9090
  mode: "debug"
  read_timeout: "20s"
database:
  driver: "sqlite"
  dsn: "test_custom.db"
migrate:
  auto: false
  action: "none"
`
	if err := os.WriteFile(yamlFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfg, err := Load(yamlFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Mode != "debug" {
		t.Errorf("expected mode debug, got %s", cfg.Server.Mode)
	}
	if cfg.Server.ReadTimeout != 20*time.Second {
		t.Errorf("expected read_timeout 20s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected driver sqlite, got %s", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "test_custom.db" {
		t.Errorf("expected dsn test_custom.db, got %s", cfg.Database.DSN)
	}
	if cfg.Migrate.Action != "none" {
		t.Errorf("expected migrate action none, got %s", cfg.Migrate.Action)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("SCHOOL_DRIVER", "postgres")
	t.Setenv("SCHOOL_DSN", "postgres://testuser:testpass@localhost:5432/testdb")
	t.Setenv("SCHOOL_MIGRATE", "status")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("expected driver postgres, got %s", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "postgres://testuser:testpass@localhost:5432/testdb" {
		t.Errorf("unexpected dsn: %s", cfg.Database.DSN)
	}
	if cfg.Migrate.Action != "status" {
		t.Errorf("expected migrate action status, got %s", cfg.Migrate.Action)
	}
}

func TestLoadClientConfig(t *testing.T) {
	tempDir := t.TempDir()
	clientYaml := filepath.Join(tempDir, "client.yaml")
	content := `
driver: "remote"
api_url: "http://127.0.0.1:8099"
token: "my-test-token"
`
	if err := os.WriteFile(clientYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write client yaml: %v", err)
	}

	cfg, err := LoadClient(clientYaml)
	if err != nil {
		t.Fatalf("LoadClient returned error: %v", err)
	}
	if cfg.Driver != "remote" {
		t.Errorf("expected driver remote, got %s", cfg.Driver)
	}
	if cfg.APIURL != "http://127.0.0.1:8099" {
		t.Errorf("expected api_url http://127.0.0.1:8099, got %s", cfg.APIURL)
	}
	if cfg.Token != "my-test-token" {
		t.Errorf("expected token my-test-token, got %s", cfg.Token)
	}

	// Test env override
	t.Setenv("SCHOOL_API", "http://server.internal:9999")
	cfgEnv, err := LoadClient(clientYaml)
	if err != nil {
		t.Fatalf("LoadClient with env returned error: %v", err)
	}
	if cfgEnv.APIURL != "http://server.internal:9999" {
		t.Errorf("expected env override http://server.internal:9999, got %s", cfgEnv.APIURL)
	}
}

func TestSaveClientConfig(t *testing.T) {
	tempDir := t.TempDir()
	outYaml := filepath.Join(tempDir, "client_saved.yaml")

	toSave := &ClientConfig{
		Driver:     "remote",
		APIURL:     "http://192.168.1.50:8099",
		SQLitePath: "custom.db",
	}

	if err := SaveClient(outYaml, toSave); err != nil {
		t.Fatalf("SaveClient returned error: %v", err)
	}

	loaded, err := LoadClient(outYaml)
	if err != nil {
		t.Fatalf("LoadClient after save returned error: %v", err)
	}
	if loaded.Driver != "remote" || loaded.APIURL != "http://192.168.1.50:8099" || loaded.SQLitePath != "custom.db" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}

func TestMySQLDriverConfig(t *testing.T) {
	tempDir := t.TempDir()
	yamlFile := filepath.Join(tempDir, "mysql_test.yaml")
	content := `
database:
  dsn: "root:secret@tcp(127.0.0.1:3306)/school?charset=utf8mb4"
`
	if err := os.WriteFile(yamlFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfg, err := Load(yamlFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Database.Driver != "mysql" {
		t.Errorf("expected driver mysql auto-detected from DSN, got %s", cfg.Database.Driver)
	}
}



