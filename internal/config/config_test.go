package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `{
		"port": "9090",
		"users": {
			"tok1": {"name": "Alice", "hebbanToken": "h1"}
		}
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	entry, ok := cfg.Users["tok1"]
	if !ok {
		t.Fatal("expected users[\"tok1\"] to be present")
	}
	if entry.Name != "Alice" || entry.HebbanToken != "h1" {
		t.Errorf("got entry %+v, want Name=Alice HebbanToken=h1", entry)
	}
}

func TestLoad_DefaultsPortWhenMissing(t *testing.T) {
	path := writeConfig(t, `{"users": {}}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("got err=%v, want a \"read config\" error", err)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	path := writeConfig(t, "not json")

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("got err=%v, want a \"parse config\" error", err)
	}
}

func TestLoad_EmptyUsersMap(t *testing.T) {
	path := writeConfig(t, `{"port": "9090"}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Users) != 0 {
		t.Errorf("got %d users, want 0", len(cfg.Users))
	}
}
