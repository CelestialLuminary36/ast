package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/CelestialLuminary36/ast/internal/config"
)

const defaultAnthropicEndpoint = "https://api.anthropic.com/v1/messages"

// AnthropicProvider wraps the anthropic-sdk-go to implement the Provider interface.
type AnthropicProvider struct {
	client    anthropic.Client
	model     string
	maxTokens int
}

// NewAnthropic creates a provider backed by the Anthropic Messages API.
func NewAnthropic(cfg config.ProviderConfig) (*AnthropicProvider, error) {
	apiKey := cfg.Key
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing API key: set provider.key in ast.yaml or ANTHROPIC_API_KEY env var")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultAnthropicEndpoint
	}
	if endpoint != defaultAnthropicEndpoint {
		opts = append(opts, option.WithBaseURL(normalizeEndpointForBaseURL(endpoint)))
	}

	model := cfg.Model
	if model == "" {
		model = anthropic.ModelClaudeSonnet4_6
	}

	maxTokens := 4096

	return &AnthropicProvider{
		client:    anthropic.NewClient(opts...),
		model:     model,
		maxTokens: maxTokens,
	}, nil
}

func (p *AnthropicProvider) Send(ctx context.Context, req Request) (*Response, error) {
	systemBlocks := []anthropic.TextBlockParam{
		{Text: req.SystemPrompt},
	}

	tools := canonicalToolsToAnthropic(req.Tools)
	messages, err := canonicalMessagesToAnthropic(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("convert messages: %w", err)
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = p.maxTokens
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(maxTokens),
		System:    systemBlocks,
		Messages:  messages,
		Tools:     tools,
	})
	if err != nil {
		return nil, err
	}

	return anthropicResponseToCanonical(resp), nil
}

// canonicalToolsToAnthropic converts canonical ToolDef values to Anthropic SDK types.
func canonicalToolsToAnthropic(tools []ToolDef) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, td := range tools {
		tp := anthropic.ToolParam{
			Name:        td.Name,
			Description: anthropic.String(td.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: td.InputSchema["properties"],
				Required:   stringSlice(td.InputSchema["required"]),
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return out
}

// canonicalMessagesToAnthropic converts canonical Message values to Anthropic SDK types.
func canonicalMessagesToAnthropic(msgs []Message) ([]anthropic.MessageParam, error) {
	out := make([]anthropic.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "user":
			var blocks []anthropic.ContentBlockParamUnion
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					blocks = append(blocks, anthropic.NewTextBlock(cb.Text))
				case "tool_result":
					blocks = append(blocks, anthropic.NewToolResultBlock(cb.ToolID, cb.ToolOutput, cb.IsError))
				case "tool_use":
					return nil, fmt.Errorf("unexpected tool_use block in user message")
				}
			}
			out = append(out, anthropic.NewUserMessage(blocks...))
		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					blocks = append(blocks, anthropic.NewTextBlock(cb.Text))
				case "tool_use":
					blocks = append(blocks, anthropic.NewToolUseBlock(cb.ToolID, cb.ToolInput, cb.ToolName))
				case "tool_result":
					return nil, fmt.Errorf("unexpected tool_result block in assistant message")
				}
			}
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		default:
			return nil, fmt.Errorf("unknown message role %q", m.Role)
		}
	}
	return out, nil
}

// anthropicResponseToCanonical converts an Anthropic SDK response to the canonical Response type.
func anthropicResponseToCanonical(resp *anthropic.Message) *Response {
	r := &Response{StopReason: string(resp.StopReason)}
	for _, block := range resp.Content {
		switch v := block.AsAny().(type) {
		case anthropic.TextBlock:
			if strings.TrimSpace(v.Text) != "" {
				r.TextBlocks = append(r.TextBlocks, v.Text)
			}
		case anthropic.ToolUseBlock:
			var input map[string]any
			if len(v.Input) > 0 {
				_ = json.Unmarshal(v.Input, &input)
			}
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:    v.ID,
				Name:  v.Name,
				Input: input,
			})
		}
	}
	return r
}

// normalizeEndpointForBaseURL converts a Messages-API URL into the base URL
// format expected by the SDK. The SDK appends "/v1/messages" internally.
func normalizeEndpointForBaseURL(endpoint string) string {
	s := strings.TrimSuffix(endpoint, "/")
	s = strings.TrimSuffix(s, "/v1/messages")
	if s == "" {
		return "/"
	}
	return s + "/"
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
