package settings

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testDirPerm  = 0o755
	testFilePerm = 0o644
)

func TestLoadConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	writeConfigFile(t, path, `{
  "format": "json",
  "production": true,
  "rules": {"unused-function": "off"},
  "ignorePatterns": ["internal/generated/**"]
}`)

	cfg, ok, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() ok = false; want true")
	}
	if cfg.Format != "json" {
		t.Fatalf("Format = %q; want json", cfg.Format)
	}
	if cfg.Production == nil || !*cfg.Production {
		t.Fatalf("Production = %v; want true", cfg.Production)
	}
	if got := cfg.Rules["unused-function"]; got != RuleOff {
		t.Fatalf("rule severity = %q; want %q", got, RuleOff)
	}
}

func TestLoadMissingConfig(t *testing.T) {
	_, ok, err := Load(filepath.Join(t.TempDir(), DefaultConfigFile))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ok {
		t.Fatal("Load() ok = true; want false")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	writeConfigFile(t, path, `{"unknown": true}`)

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil; want error")
	}
}

func TestLoadRejectsUnknownRuleSeverity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, DefaultConfigFile)
	writeConfigFile(t, path, `{"rules": {"unused-function": "ignore"}}`)

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil; want error")
	}
}

func writeConfigFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), testDirPerm); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), testFilePerm); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
