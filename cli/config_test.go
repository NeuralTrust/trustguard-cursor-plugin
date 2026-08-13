package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cursor.json")
	file := `{"data_url":"http://file:1","api_key":"tgk_file","fail_mode":"closed","timeout_ms":100}`
	if err := os.WriteFile(path, []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRUSTGUARD_CURSOR_SYSTEM_CONFIG", filepath.Join(dir, "missing-system.json"))
	t.Setenv("TRUSTGUARD_CURSOR_CONFIG", path)
	t.Setenv("TRUSTGUARD_DATA_URL", "http://env:2")
	t.Setenv("TRUSTGUARD_API_KEY", "")
	t.Setenv("TRUSTGUARD_FAIL_MODE", "")
	t.Setenv("TRUSTGUARD_TIMEOUT_MS", "")
	t.Setenv("TRUSTGUARD_TRANSFORM_ACTION", "")
	t.Setenv("TRUSTGUARD_CONSUMER_ID", "")

	cfg := loadConfig()
	if cfg.DataURL != "http://env:2" {
		t.Fatalf("env must win over file, got %q", cfg.DataURL)
	}
	if cfg.APIKey != "tgk_file" {
		t.Fatalf("file value must apply when env is empty, got %q", cfg.APIKey)
	}
	if cfg.FailMode != "closed" || cfg.TimeoutMS != 100 {
		t.Fatalf("file values not applied: %+v", cfg)
	}
	if cfg.TransformAction != "ask" || cfg.MaxContentBytes != defaultMaxContentBytes {
		t.Fatalf("defaults not applied: %+v", cfg)
	}
}

func TestLoadConfigUserFileLayersOverSystemFile(t *testing.T) {
	dir := t.TempDir()
	systemPath := filepath.Join(dir, "system.json")
	userPath := filepath.Join(dir, "user.json")
	system := `{"data_url":"https://managed:1","api_key":"tgk_shared","fail_mode":"closed"}`
	userFile := `{"api_key":"tgk_personal"}`
	if err := os.WriteFile(systemPath, []byte(system), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte(userFile), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRUSTGUARD_CURSOR_SYSTEM_CONFIG", systemPath)
	t.Setenv("TRUSTGUARD_CURSOR_CONFIG", userPath)
	for _, env := range []string{"TRUSTGUARD_DATA_URL", "TRUSTGUARD_API_KEY", "TRUSTGUARD_FAIL_MODE", "TRUSTGUARD_TIMEOUT_MS", "TRUSTGUARD_TRANSFORM_ACTION", "TRUSTGUARD_CONSUMER_ID"} {
		t.Setenv(env, "")
	}

	cfg := loadConfig()
	if cfg.APIKey != "tgk_personal" {
		t.Fatalf("user file must override the managed key, got %q", cfg.APIKey)
	}
	if cfg.DataURL != "https://managed:1" || cfg.FailMode != "closed" {
		t.Fatalf("managed values must survive when the user file omits them: %+v", cfg)
	}
}

func TestApplyDefaultsNormalizesInvalidValues(t *testing.T) {
	cfg := Config{FailMode: "bogus", TransformAction: "bogus", TimeoutMS: -1}
	cfg.applyDefaults()
	if cfg.FailMode != "open" || cfg.TransformAction != "ask" || cfg.TimeoutMS != defaultTimeoutMS {
		t.Fatalf("invalid values must normalize to defaults: %+v", cfg)
	}
	if cfg.DataURL != defaultDataURL || cfg.ConsumerID == "" {
		t.Fatalf("missing defaults: %+v", cfg)
	}
}
