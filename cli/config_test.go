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
	if cfg.managed {
		t.Fatal("BYO install must not be treated as managed")
	}
}

func TestLoadConfigManagedLocksOrgCredentials(t *testing.T) {
	dir := t.TempDir()
	systemPath := filepath.Join(dir, "system.json")
	userPath := filepath.Join(dir, "user.json")
	system := `{"data_url":"https://managed:1","api_key":"tgk_org","fail_mode":"closed"}`
	userFile := `{"data_url":"https://evil","api_key":"tgk_personal","fail_mode":"open","timeout_ms":99,"transform_action":"deny"}`
	if err := os.WriteFile(systemPath, []byte(system), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte(userFile), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRUSTGUARD_CURSOR_SYSTEM_CONFIG", systemPath)
	t.Setenv("TRUSTGUARD_CURSOR_CONFIG", userPath)
	t.Setenv("TRUSTGUARD_DATA_URL", "https://env-evil")
	t.Setenv("TRUSTGUARD_API_KEY", "tgk_env")
	t.Setenv("TRUSTGUARD_FAIL_MODE", "open")
	t.Setenv("TRUSTGUARD_TIMEOUT_MS", "")
	t.Setenv("TRUSTGUARD_TRANSFORM_ACTION", "")
	t.Setenv("TRUSTGUARD_CONSUMER_ID", "")

	cfg := loadConfig()
	if !cfg.managed {
		t.Fatal("expected managed mode when the system file ships an api_key")
	}
	if cfg.APIKey != "tgk_org" {
		t.Fatalf("managed org key must win, got %q", cfg.APIKey)
	}
	if cfg.DataURL != "https://managed:1" {
		t.Fatalf("managed data_url must win, got %q", cfg.DataURL)
	}
	if cfg.FailMode != "closed" {
		t.Fatalf("managed fail_mode must win, got %q", cfg.FailMode)
	}
	if cfg.TimeoutMS != 99 || cfg.TransformAction != "deny" {
		t.Fatalf("soft prefs must still layer from the user file: %+v", cfg)
	}
}

func TestLoadConfigSystemWithoutKeyIsNotManaged(t *testing.T) {
	dir := t.TempDir()
	systemPath := filepath.Join(dir, "system.json")
	userPath := filepath.Join(dir, "user.json")
	if err := os.WriteFile(systemPath, []byte(`{"data_url":"https://managed:1","fail_mode":"closed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte(`{"api_key":"tgk_personal"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRUSTGUARD_CURSOR_SYSTEM_CONFIG", systemPath)
	t.Setenv("TRUSTGUARD_CURSOR_CONFIG", userPath)
	for _, env := range []string{"TRUSTGUARD_DATA_URL", "TRUSTGUARD_API_KEY", "TRUSTGUARD_FAIL_MODE", "TRUSTGUARD_TIMEOUT_MS", "TRUSTGUARD_TRANSFORM_ACTION", "TRUSTGUARD_CONSUMER_ID"} {
		t.Setenv(env, "")
	}

	cfg := loadConfig()
	if cfg.managed {
		t.Fatal("system file without api_key is not managed mode")
	}
	if cfg.APIKey != "tgk_personal" {
		t.Fatalf("user key must apply when MDM only ships soft prefs, got %q", cfg.APIKey)
	}
	if cfg.DataURL != "https://managed:1" {
		t.Fatalf("system data_url should still apply, got %q", cfg.DataURL)
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

func TestConsumerIDForPrefersUserEmail(t *testing.T) {
	cfg := Config{ConsumerID: "cursor:fallback"}
	got := consumerIDFor(cfg, hookInput{UserEmail: "alice@acme.com"})
	if got != "cursor:alice@acme.com" {
		t.Fatalf("expected cursor email consumer, got %q", got)
	}
	got = consumerIDFor(cfg, hookInput{UserEmail: "  "})
	if got != "cursor:fallback" {
		t.Fatalf("blank email must fall back to config, got %q", got)
	}
}
