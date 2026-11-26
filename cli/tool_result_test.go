package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestToolOutputBlocks(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"plain string", "hello world", []string{"hello world"}},
		{"plain multiline text", "line1\nline2", []string{"line1\nline2"}},
		{"json string literal", `"hello"`, []string{"hello"}},
		{"stringified mcp array", `[{"type":"text","text":"echo: hello probe"}]`, []string{"echo: hello probe"}},
		{"multi block keeps order and extra keys", `[{"type":"text","text":"a","annotations":{}},{"type":"text","text":"b"}]`, []string{"a", "b"}},
		{"content object", `{"content":[{"type":"text","text":"foo"},{"type":"text","text":"bar"}],"isError":false}`, []string{"foo", "bar"}},
		{"mixed text and image skips image", `[{"type":"text","text":"a"},{"type":"image","data":"AAAA"},{"type":"text","text":"b"}]`, []string{"a", "b"}},
		{"only images keeps the original string", `[{"type":"image","data":"AAAA"}]`, []string{`[{"type":"image","data":"AAAA"}]`}},
		{"other object keeps the original string", "{\n  \"matches\": [\"a\"]\n}", []string{"{\n  \"matches\": [\"a\"]\n}"}},
		{"array without block types", `[1,2]`, []string{`[1,2]`}},
		{"number", `42`, []string{`42`}},
		{"invalid JSON kept as is", `{"a":`, []string{`{"a":`}},
		{"empty", ``, nil},
		{"blank", `   `, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolOutputBlocks(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("toolOutputBlocks(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestClipIsUTF8Safe(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"fits", "abc", 3, "abc"},
		{"ascii cut", "abcdef", 4, "abcd"},
		{"cut inside rune backs off", "aéb", 2, "a"},
		{"cut on rune boundary", "aéb", 3, "aé"},
		{"four byte rune", "😀😀", 5, "😀"},
		{"zero", "abc", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clip(tt.in, tt.n); got != tt.want {
				t.Fatalf("clip(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

func TestPostToolUsePayloadBlocks(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		maxBytes int
		want     []string
		skipped  bool
	}{
		{"plain output is one block", "package main", 0, []string{"package main"}, false},
		{"mcp result sent as its blocks", `[{"type":"text","text":"foo"},{"type":"text","text":"bar"}]`, 0, []string{"foo", "bar"}, false},
		{"content object", `{"content":[{"type":"text","text":"x"},{"type":"text","text":"y"}]}`, 0, []string{"x", "y"}, false},
		{"budget spans blocks and truncates the last", `[{"type":"text","text":"aaaa"},{"type":"text","text":"bbbb"},{"type":"text","text":"cccc"}]`, 6, []string{"aaaa", "bb"}, false},
		{"budget exhausted drops later blocks", `[{"type":"text","text":"aaaa"},{"type":"text","text":"bbbb"}]`, 4, []string{"aaaa"}, false},
		{"clip does not split a rune", `[{"type":"text","text":"ééé"}]`, 3, []string{"é"}, false},
		{"other object stays the original string", `{"query":"q"}`, 0, []string{`{"query":"q"}`}, false},
		{"blank blocks skip the evaluate", `[{"type":"text","text":"  "},{"type":"text","text":""}]`, 0, nil, true},
		{"blank output skips the evaluate", "  ", 0, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, captured := stubGuard(t, EvaluateResponse{Status: "allow"})
			cfg := testConfig(srv.URL)
			if tt.maxBytes > 0 {
				cfg.MaxContentBytes = tt.maxBytes
			}
			invokeHook(t, cfg, map[string]any{
				"hook_event_name": "postToolUse",
				"tool_name":       "mcp__echo__echo",
				"tool_output":     tt.output,
			})
			if tt.skipped {
				if len(*captured) != 0 {
					t.Fatalf("expected no evaluate, got %v", *captured)
				}
				return
			}
			if (*captured)["direction"] != "output" || (*captured)["protocol"] != "mcp" {
				t.Fatalf("unexpected envelope: %v", *captured)
			}
			content := (*captured)["payload"].(map[string]any)["result"].(map[string]any)["content"].([]any)
			got := make([]string, 0, len(content))
			for _, c := range content {
				block := c.(map[string]any)
				if block["type"] != "text" {
					t.Fatalf("expected text block, got %v", block)
				}
				got = append(got, block["text"].(string))
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("blocks = %q, want %q", got, tt.want)
			}
			if total := len(strings.Join(got, "")); tt.maxBytes > 0 && total > tt.maxBytes {
				t.Fatalf("sent %d bytes, budget %d", total, tt.maxBytes)
			}
		})
	}
}
