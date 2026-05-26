package provider

import (
	"testing"
)

func TestNormalizeEndpointForBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com/"},
		{"https://api.anthropic.com/v1/messages/", "https://api.anthropic.com/"},
		{"https://proxy.example.com/v1/messages", "https://proxy.example.com/"},
		{"https://proxy.example.com/anthropic/v1/messages", "https://proxy.example.com/anthropic/"},
		{"https://proxy.example.com/", "https://proxy.example.com/"},
		{"https://proxy.example.com", "https://proxy.example.com/"},
	}

	for _, c := range cases {
		got := normalizeEndpointForBaseURL(c.in)
		if got != c.want {
			t.Errorf("normalizeEndpointForBaseURL(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalToolsToAnthropic(t *testing.T) {
	tools := []ToolDef{
		{Name: "read_file", Description: "Read a file", InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
			"required":   []any{"path"},
		}},
	}
	result := canonicalToolsToAnthropic(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].OfTool == nil {
		t.Fatal("OfTool is nil")
	}
	if result[0].OfTool.Name != "read_file" {
		t.Errorf("name = %q, want read_file", result[0].OfTool.Name)
	}
}

func TestCanonicalMessagesToAnthropic_UserText(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "text", Text: "hello"}}},
	}
	result, err := canonicalMessagesToAnthropic(msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
}

func TestCanonicalMessagesToAnthropic_ToolUseInUserErrors(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: []ContentBlock{{Type: "tool_use"}}},
	}
	_, err := canonicalMessagesToAnthropic(msgs)
	if err == nil {
		t.Fatal("expected error for tool_use in user message")
	}
}

func TestCanonicalMessagesToAnthropic_ToolResultInAssistantErrors(t *testing.T) {
	msgs := []Message{
		{Role: "assistant", Content: []ContentBlock{{Type: "tool_result"}}},
	}
	_, err := canonicalMessagesToAnthropic(msgs)
	if err == nil {
		t.Fatal("expected error for tool_result in assistant message")
	}
}
