package provider

import "context"

// Provider abstracts an LLM backend behind a single stateless call.
type Provider interface {
	Send(ctx context.Context, req Request) (*Response, error)
}

// Request is the canonical input for one chat completion call.
type Request struct {
	SystemPrompt string
	Messages     []Message
	Tools        []ToolDef
	MaxTokens    int
}

// Response is the canonical output from one chat completion call.
type Response struct {
	TextBlocks       []string
	ToolCalls        []ToolCall
	StopReason       string
	ReasoningContent string // DeepSeek thinking mode
}

// Message is a single conversation turn.
type Message struct {
	Role             string         // "user" | "assistant"
	Content          []ContentBlock
	ReasoningContent string         // DeepSeek thinking mode
}

// ContentBlock is one unit of content in a message.
type ContentBlock struct {
	Type       string         // "text" | "tool_use" | "tool_result"
	Text       string
	ToolID     string
	ToolName   string
	ToolInput  map[string]any
	ToolOutput string
	IsError    bool
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolDef is a canonical tool definition.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}
