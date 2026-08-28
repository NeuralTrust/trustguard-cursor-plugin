package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config drives the hook runtime. Values resolve as env > user file > managed
// file > default — except in managed mode (see loadConfig), where the org key
// and its data-plane URL cannot be overridden by the developer.
type Config struct {
	// DataURL is the TrustGuard data-plane base URL (serves /v1/evaluate).
	DataURL string `json:"data_url"`
	// APIKey is a collector API key (tgk_…); with it no routing key is needed.
	// In enterprise deployments this is the org-wide Cursor collector key,
	// provisioned by MDM — employees do not need a NeuralTrust account.
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
	// ConsumerID is set only by MDM / TRUSTGUARD_CONSUMER_ID. When empty the
	// evaluate request omits consumer_id; the account email still travels in
	// attributes.user.email.
	ConsumerID string `json:"consumer_id"`
	// Events disables individual hook events, e.g. {"postToolUse": false}.
	Events map[string]bool `json:"events"`

	// managed is set when the MDM system file shipped an org API key. Locked
	// fields then refuse user-file and env overrides so a developer cannot
	// disable or redirect the org firewall.
	managed bool
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

// loadConfig layers configuration for two deployment modes:
//
//   - Managed (enterprise): the MDM system file carries an api_key. That key,
//     its data_url and fail_mode are locked — user file and env cannot replace
//     them. Soft prefs (timeout, transform_action, events, consumer_id) still
//     layer normally.
//   - Local / BYO: no managed key. User file then env win, as before.
func loadConfig() Config {
	cfg := Config{}
	if raw, err := os.ReadFile(systemConfigPath()); err == nil {
		_ = json.Unmarshal(raw, &cfg)
		if strings.TrimSpace(cfg.APIKey) != "" {
			cfg.managed = true
		}
	}

	overlay := Config{}
	if path := defaultConfigPath(); path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(raw, &overlay)
		}
	}
	applyOverlay(&cfg, overlay)
	applyEnv(&cfg)
	cfg.applyDefaults()
	return cfg
}

// applyOverlay merges non-empty overlay fields into cfg, skipping the locked
// enterprise fields when cfg is managed.
func applyOverlay(cfg *Config, overlay Config) {
	if overlay.DataURL != "" && !cfg.managed {
		cfg.DataURL = overlay.DataURL
	}
	if overlay.APIKey != "" && !cfg.managed {
		cfg.APIKey = overlay.APIKey
	}
	if overlay.FailMode != "" && !cfg.managed {
		cfg.FailMode = overlay.FailMode
	}
	if overlay.TransformAction != "" {
		cfg.TransformAction = overlay.TransformAction
	}
	if overlay.ReportNotice != nil {
		cfg.ReportNotice = overlay.ReportNotice
	}
	if overlay.TimeoutMS > 0 {
		cfg.TimeoutMS = overlay.TimeoutMS
	}
	if overlay.MaxContentBytes > 0 {
		cfg.MaxContentBytes = overlay.MaxContentBytes
	}
	if overlay.ConsumerID != "" {
		cfg.ConsumerID = overlay.ConsumerID
	}
	if overlay.Events != nil {
		cfg.Events = overlay.Events
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("TRUSTGUARD_DATA_URL"); v != "" && !cfg.managed {
		cfg.DataURL = v
	}
	if v := os.Getenv("TRUSTGUARD_API_KEY"); v != "" && !cfg.managed {
		cfg.APIKey = v
	}
	if v := os.Getenv("TRUSTGUARD_FAIL_MODE"); v != "" && !cfg.managed {
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

func looksLikeEmail(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !strings.Contains(s, "@") || strings.ContainsAny(s, " \t\n") {
		return ""
	}
	return s
}
