package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hhy/ast/internal/config"
)

const defaultOllamaEndpoint = "http://localhost:11434"

// OllamaProvider implements Provider via raw HTTP to the Ollama /api/chat endpoint.
type OllamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewOllama creates a provider backed by a local Ollama instance.
func NewOllama(cfg config.ProviderConfig) (*OllamaProvider, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	model := cfg.Model
	if model == "" {
		model = "llama3.2"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 300 // Ollama can be slow on first load
	}

	return &OllamaProvider{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

func (p *OllamaProvider) Send(ctx context.Context, req Request) (*Response, error) {
	messages := canonicalToOllamaMessages(req.SystemPrompt, req.Messages)
	tools := canonicalToOllamaTools(req.Tools)

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := map[string]any{
		"model":    p.model,
		"messages": messages,
		"tools":    tools,
		"stream":   false,
		"options": map[string]any{
			"num_predict": maxTokens,
		},
	}
	if len(tools) == 0 {
		delete(body, "tools")
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint+"/api/chat", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
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

	return parseOllamaResponse(respBody)
}

func canonicalToOllamaMessages(systemPrompt string, msgs []Message) []map[string]any {
	var out []map[string]any

	if systemPrompt != "" {
		out = append(out, map[string]any{"role": "system", "content": systemPrompt})
	}

	for _, m := range msgs {
		switch m.Role {
		case "user":
			// Ollama has no "tool" role — tool results are embedded as user content.
			var texts []string
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					texts = append(texts, cb.Text)
				case "tool_result":
					resultJSON, _ := json.Marshal(map[string]string{
						"tool_call_id": cb.ToolID,
						"content":      cb.ToolOutput,
					})
					texts = append(texts, string(resultJSON))
				}
			}
			out = append(out, map[string]any{"role": "user", "content": strings.Join(texts, "\n")})
		case "assistant":
			msg := map[string]any{"role": "assistant"}
			var texts []string
			var toolCalls []map[string]any
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					texts = append(texts, cb.Text)
				case "tool_use":
					toolCalls = append(toolCalls, map[string]any{
						"function": map[string]any{
							"name":      cb.ToolName,
							"arguments": cb.ToolInput,
						},
					})
				}
			}
			if len(texts) > 0 {
				msg["content"] = strings.Join(texts, "\n")
			} else {
				msg["content"] = ""
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			out = append(out, msg)
		}
	}
	return out
}

func canonicalToOllamaTools(tools []ToolDef) []map[string]any {
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

func parseOllamaResponse(body []byte) (*Response, error) {
	var raw struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments any    `json:"arguments"` // can be string or pre-parsed object
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		DoneReason string `json:"done_reason"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	r := &Response{StopReason: raw.DoneReason}

	if strings.TrimSpace(raw.Message.Content) != "" {
		r.TextBlocks = append(r.TextBlocks, raw.Message.Content)
	}

	for _, tc := range raw.Message.ToolCalls {
		input := parseOllamaArgs(tc.Function.Arguments)
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:    "", // Ollama doesn't always include an ID
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return r, nil
}

func parseOllamaArgs(v any) map[string]any {
	switch a := v.(type) {
	case map[string]any:
		return a
	case string:
		var m map[string]any
		if err := json.Unmarshal([]byte(a), &m); err != nil {
			return map[string]any{"_raw": a}
		}
		return m
	default:
		return nil
	}
}
