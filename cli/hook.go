package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// hookInput is the stdin payload Cursor sends to every hook. Fields are a
// superset across events; unknown fields are ignored on purpose so newer
// Cursor versions keep working.
type hookInput struct {
	HookEventName  string   `json:"hook_event_name"`
	ConversationID string   `json:"conversation_id"`
	GenerationID   string   `json:"generation_id"`
	WorkspaceRoots []string `json:"workspace_roots"`

	// beforeSubmitPrompt
	Prompt string `json:"prompt"`

	// beforeShellExecution
	Command string `json:"command"`
	CWD     string `json:"cwd"`

	// beforeMCPExecution — Cursor builds have shipped the call arguments under
	// different keys, so accept the known spellings.
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Arguments json.RawMessage `json:"arguments"`

	// beforeReadFile
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// hookOutput is the stdout answer. Control hooks read `permission`
// (allow|ask|deny); beforeSubmitPrompt reads `continue`.
type hookOutput struct {
	Continue     *bool  `json:"continue,omitempty"`
	Permission   string `json:"permission,omitempty"`
	UserMessage  string `json:"user_message,omitempty"`
	AgentMessage string `json:"agent_message,omitempty"`
}

const (
	permissionAllow = "allow"
	permissionAsk   = "ask"
	permissionDeny  = "deny"
)

// verdict is the event-agnostic decision derived from an evaluate response.
type verdict struct {
	permission   string
	userMessage  string
	agentMessage string
}

func runHook(stdin io.Reader, stdout io.Writer, cfg Config) error {
	// Decode incrementally: the decoder returns as soon as the top-level JSON
	// value is complete. Cursor may keep the stdin pipe open after writing the
	// event, so waiting for EOF (io.ReadAll) would hang the hook forever.
	var in hookInput
	if err := json.NewDecoder(io.LimitReader(stdin, 16<<20)).Decode(&in); err != nil {
		return fmt.Errorf("decode hook input: %w", err)
	}

	out := decideEvent(cfg, in)
	if err := json.NewEncoder(stdout).Encode(out); err != nil {
		return fmt.Errorf("write hook output: %w", err)
	}
	return nil
}

func decideEvent(cfg Config, in hookInput) hookOutput {
	if cfg.APIKey == "" {
		// Unconfigured installs must never brick the editor: allow and say why.
		logf("TRUSTGUARD_API_KEY missing; allowing %s without evaluation", in.HookEventName)
		return allowOutput(in)
	}
	if !cfg.eventEnabled(in.HookEventName) {
		return allowOutput(in)
	}

	req, ok := buildEvaluateRequest(cfg, in)
	if !ok {
		// Event without evaluable content (or unknown event): nothing to score.
		return allowOutput(in)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout())
	defer cancel()
	res, err := newGuardClient(cfg).Evaluate(ctx, req)
	if err != nil {
		return failModeOutput(cfg, in, err)
	}
	return toHookOutput(in, applyVerdict(cfg, res))
}

// buildEvaluateRequest maps one Cursor event onto the /v1/evaluate contract.
// Shapes follow docs/api/provider-signatures.md: `all` only accepts
// {"input": …}; chat shapes need `llm`; JSON-RPC needs `mcp`. Tool-call
// arguments are only analyzed on the MCP path, hence the tools/call envelope.
func buildEvaluateRequest(cfg Config, in hookInput) (EvaluateRequest, bool) {
	base := EvaluateRequest{
		Direction:  "input",
		SessionID:  in.ConversationID,
		ConsumerID: cfg.ConsumerID,
		Attributes: map[string]any{
			"collector": map[string]any{"type": "ide"},
			"cursor": map[string]any{
				"event":     in.HookEventName,
				"workspace": firstOrEmpty(in.WorkspaceRoots),
			},
		},
	}

	switch in.HookEventName {
	case "beforeSubmitPrompt":
		if strings.TrimSpace(in.Prompt) == "" {
			return base, false
		}
		base.Protocol = "llm"
		base.Payload = map[string]any{
			"messages": []any{map[string]any{"role": "user", "content": in.Prompt}},
		}
		return base, true

	case "beforeShellExecution":
		if strings.TrimSpace(in.Command) == "" {
			return base, false
		}
		base.Protocol = "all"
		base.Payload = map[string]any{"input": in.Command}
		return base, true

	case "beforeMCPExecution":
		if in.ToolName == "" {
			return base, false
		}
		base.Protocol = "mcp"
		base.Payload = map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      in.ToolName,
				"arguments": decodeToolArguments(in),
			},
		}
		return base, true

	case "beforeReadFile":
		if in.Content == "" {
			return base, false
		}
		content := clip(in.Content, cfg.MaxContentBytes)
		// File content is model-bound external context, so it is scored as an
		// MCP tool result (direction=output) where indirect_prompt_injection
		// and DLP apply.
		base.Direction = "output"
		base.Protocol = "mcp"
		base.Payload = map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []any{map[string]any{"type": "text", "text": content}},
			},
		}
		return base, true
	}
	return base, false
}

func decodeToolArguments(in hookInput) any {
	raw := in.Arguments
	if len(raw) == 0 {
		raw = in.ToolInput
	}
	if len(raw) == 0 {
		return map[string]any{}
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		return asMap
	}
	// tools/call arguments must be an object; wrap scalar/array/string forms.
	var asAny any
	if err := json.Unmarshal(raw, &asAny); err == nil {
		return map[string]any{"input": asAny}
	}
	return map[string]any{"input": string(raw)}
}

// applyVerdict folds the guard status into a hook decision:
// block → deny · transform → configured (ask by default, hooks cannot rewrite)
// · report → allow with notice · allow/skip/unknown → allow.
func applyVerdict(cfg Config, res *EvaluateResponse) verdict {
	reason := primaryReason(res.Findings)
	switch res.Status {
	case "block":
		msg := "TrustGuard blocked this action"
		if reason != "" {
			msg += ": " + reason
		}
		return verdict{permission: permissionDeny, userMessage: msg, agentMessage: msg}
	case "transform":
		msg := "TrustGuard detected sensitive data"
		if reason != "" {
			msg = "TrustGuard detected sensitive data: " + reason
		}
		permission := permissionAsk
		switch cfg.TransformAction {
		case "deny":
			permission = permissionDeny
		case "allow":
			permission = permissionAllow
		}
		return verdict{permission: permission, userMessage: msg, agentMessage: msg}
	case "report":
		v := verdict{permission: permissionAllow}
		if cfg.reportNotice() && reason != "" {
			v.userMessage = "TrustGuard flagged (report-only): " + reason
		}
		return v
	default:
		// "allow", gate "skip", or any future status: do not get in the way.
		return verdict{permission: permissionAllow}
	}
}

// primaryReason mirrors TrustGate's selectPrimaryFinding: prefer enforced
// findings, then highest signal confidence.
func primaryReason(findings []Finding) string {
	var best *Finding
	bestScore := -1.0
	for i := range findings {
		f := &findings[i]
		score := 0.0
		if f.Signal != nil {
			score = f.Signal.Confidence
		}
		if f.Outcome != nil && (f.Outcome.Action == "block" || f.Outcome.Action == "transform") {
			score += 10 // enforced findings outrank observational ones
		}
		if score > bestScore {
			best, bestScore = f, score
		}
	}
	if best == nil {
		return ""
	}
	label := ""
	if best.Signal != nil {
		label = best.Signal.Type
	}
	source := best.Source.DetectorName
	if source == "" {
		source = best.Source.GateName
	}
	if source == "" {
		source = best.Source.Plugin
	}
	switch {
	case label != "" && source != "":
		return fmt.Sprintf("%s (%s)", label, source)
	case label != "":
		return label
	default:
		return source
	}
}

func failModeOutput(cfg Config, in hookInput, err error) hookOutput {
	logf("evaluate failed (%s): %v", in.HookEventName, err)
	if cfg.FailMode == "closed" {
		msg := "TrustGuard is unreachable and fail_mode is closed; action denied."
		return toHookOutput(in, verdict{permission: permissionDeny, userMessage: msg, agentMessage: msg})
	}
	return allowOutput(in)
}

func allowOutput(in hookInput) hookOutput {
	return toHookOutput(in, verdict{permission: permissionAllow})
}

// toHookOutput adapts a verdict to the event's response contract:
// beforeSubmitPrompt answers {continue}, control hooks answer {permission}.
func toHookOutput(in hookInput, v verdict) hookOutput {
	out := hookOutput{
		UserMessage:  v.userMessage,
		AgentMessage: v.agentMessage,
	}
	if in.HookEventName == "beforeSubmitPrompt" {
		allowed := v.permission == permissionAllow
		out.Continue = &allowed
		return out
	}
	out.Permission = v.permission
	return out
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func firstOrEmpty(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

// logf writes to stderr only — stdout is reserved for the hook response and
// payload contents are never logged.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "trustguard-cursor: "+format+"\n", args...)
}
