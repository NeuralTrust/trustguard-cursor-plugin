package main

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Config drives the hook runtime. Values resolve as env > config file > default.
type Config struct {
	// DataURL is the TrustGuard data-plane base URL (serves /v1/evaluate).
	DataURL string `json:"data_url"`
	// APIKey is a collector API key (tgk_…); with it no routing key is needed.
	APIKey string `json:"api_key"`
	// FailMode decides the verdict when TrustGuard is unreachable or errors:
	// "open" allows, "closed" denies.
	FailMode string `json:"fail_mode"`
	// TransformAction maps a `transform` verdict (DLP found PII/secrets; hooks
	// cannot rewrite content): "ask" (default), "deny" or "allow".
	TransformAction string `json:"transform_action"`
	// ReportNotice attaches a user-visible warning when findings are report-only.
	ReportNotice *bool `json:"report_notice"`
	// TimeoutMS bounds each /v1/evaluate call.
	TimeoutMS int `json:"timeout_ms"`
	// MaxContentBytes truncates file/tool content sent to the guard.
	MaxContentBytes int `json:"max_content_bytes"`
	// ConsumerID anchors anomaly detection and policy routing (default OS user).
	ConsumerID string `json:"consumer_id"`
	// Events disables individual hook events, e.g. {"beforeReadFile": false}.
	Events map[string]bool `json:"events"`
}

const (
	defaultDataURL         = "http://localhost:8081"
	defaultFailMode        = "open"
	defaultTransformAction = "ask"
	defaultTimeoutMS       = 5000
	defaultMaxContentBytes = 256 * 1024
)

func defaultConfigPath() string {
	if p := os.Getenv("TRUSTGUARD_CURSOR_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".trustguard", "cursor.json")
}

// systemConfigPath is the managed (MDM-deployed) config location, mirroring
// the system paths Cursor itself uses for enterprise hooks.
func systemConfigPath() string {
	if p := os.Getenv("TRUSTGUARD_CURSOR_SYSTEM_CONFIG"); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/TrustGuard/cursor.json"
	case "windows":
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "TrustGuard", "cursor.json")
	default:
		return "/etc/trustguard/cursor.json"
	}
}

// loadConfig layers configuration: managed system file, then the user file
// (fields present in it override), then environment variables.
func loadConfig() Config {
	cfg := Config{}
	for _, path := range []string{systemConfigPath(), defaultConfigPath()} {
		if path == "" {
			continue
		}
		if raw, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(raw, &cfg)
		}
	}
	if v := os.Getenv("TRUSTGUARD_DATA_URL"); v != "" {
		cfg.DataURL = v
	}
	if v := os.Getenv("TRUSTGUARD_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("TRUSTGUARD_FAIL_MODE"); v != "" {
		cfg.FailMode = v
	}
	if v := os.Getenv("TRUSTGUARD_TRANSFORM_ACTION"); v != "" {
		cfg.TransformAction = v
	}
	if v := os.Getenv("TRUSTGUARD_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			cfg.TimeoutMS = ms
		}
	}
	if v := os.Getenv("TRUSTGUARD_CONSUMER_ID"); v != "" {
		cfg.ConsumerID = v
	}
	cfg.applyDefaults()
	return cfg
}

func (c *Config) applyDefaults() {
	if c.DataURL == "" {
		c.DataURL = defaultDataURL
	}
	if c.FailMode != "closed" {
		c.FailMode = defaultFailMode
	}
	switch c.TransformAction {
	case "ask", "deny", "allow":
	default:
		c.TransformAction = defaultTransformAction
	}
	if c.TimeoutMS <= 0 {
		c.TimeoutMS = defaultTimeoutMS
	}
	if c.MaxContentBytes <= 0 {
		c.MaxContentBytes = defaultMaxContentBytes
	}
	if c.ConsumerID == "" {
		c.ConsumerID = currentUser()
	}
}

func (c *Config) timeout() time.Duration {
	return time.Duration(c.TimeoutMS) * time.Millisecond
}

func (c *Config) eventEnabled(name string) bool {
	if c.Events == nil {
		return true
	}
	enabled, found := c.Events[name]
	return !found || enabled
}

func (c *Config) reportNotice() bool {
	return c.ReportNotice == nil || *c.ReportNotice
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return "cursor:" + u.Username
	}
	host, _ := os.Hostname()
	return "cursor:" + host
}
