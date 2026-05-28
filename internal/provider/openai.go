package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CelestialLuminary36/ast/internal/config"
)

const defaultOpenAIEndpoint = "https://api.openai.com/v1"

// OpenAIProvider implements Provider via raw HTTP to the OpenAI chat completions API.
type OpenAIProvider struct {
	apiKey   string
	endpoint string
	model    string
	client   *http.Client
}

// NewOpenAI creates a provider backed by the OpenAI API.
func NewOpenAI(cfg config.ProviderConfig) (*OpenAIProvider, error) {
	apiKey := cfg.Key
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("missing API key: set provider.key in ast.yaml or OPENAI_API_KEY env var")
	}

	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultOpenAIEndpoint
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	model := cfg.Model
	if model == "" {
		model = "gpt-4o"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120
	}

	return &OpenAIProvider{
		apiKey:   apiKey,
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *OpenAIProvider) Send(ctx context.Context, req Request) (*Response, error) {
	messages := canonicalToOpenAIMessages(req.SystemPrompt, req.Messages)
	tools := canonicalToOpenAITools(req.Tools)

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := map[string]any{
		"model":      p.model,
		"messages":   messages,
		"tools":      tools,
		"max_tokens": maxTokens,
	}
	if len(tools) == 0 {
		delete(body, "tools")
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/chat/completions", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("api error %d: %s", httpResp.StatusCode, string(respBody))
	}

	return parseOpenAIResponse(respBody)
}

func canonicalToOpenAIMessages(systemPrompt string, msgs []Message) []map[string]any {
	var out []map[string]any

	if systemPrompt != "" {
		out = append(out, map[string]any{"role": "system", "content": systemPrompt})
	}

	for _, m := range msgs {
		switch m.Role {
		case "user":
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					out = append(out, map[string]any{"role": "user", "content": cb.Text})
				case "tool_result":
					out = append(out, map[string]any{
						"role":         "tool",
						"tool_call_id": cb.ToolID,
						"content":      cb.ToolOutput,
					})
				}
			}
		case "assistant":
			msg := map[string]any{"role": "assistant"}
			if m.ReasoningContent != "" {
				msg["reasoning_content"] = m.ReasoningContent
			}
			var texts []string
			var toolCalls []map[string]any
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					texts = append(texts, cb.Text)
				case "tool_use":
					argsJSON, _ := json.Marshal(cb.ToolInput)
					toolCalls = append(toolCalls, map[string]any{
						"id":   cb.ToolID,
						"type": "function",
						"function": map[string]any{
							"name":      cb.ToolName,
							"arguments": string(argsJSON),
						},
					})
				}
			}
			if len(texts) > 0 {
				msg["content"] = strings.Join(texts, "\n")
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			out = append(out, msg)
		}
	}
	return out
}

func canonicalToOpenAITools(tools []ToolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, td := range tools {
		schema := td.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        td.Name,
				"description": td.Description,
				"parameters":  schema,
			},
		})
	}
	return out
}

func parseOpenAIResponse(body []byte) (*Response, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content          *string `json:"content"`
				ReasoningContent *string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	msg := raw.Choices[0].Message

	r := &Response{StopReason: raw.Choices[0].FinishReason}
	if msg.ReasoningContent != nil {
		r.ReasoningContent = *msg.ReasoningContent
	}

	if msg.Content != nil && strings.TrimSpace(*msg.Content) != "" {
		r.TextBlocks = append(r.TextBlocks, *msg.Content)
	}

	for _, tc := range msg.ToolCalls {
		var input map[string]any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			input = map[string]any{"_raw": tc.Function.Arguments}
		}
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return r, nil
}
