package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hhy/ast/internal/config"
	"github.com/hhy/ast/internal/provider"
	"github.com/hhy/ast/internal/scenario"
	"github.com/hhy/ast/internal/skill"
)

const (
	defaultMaxRounds = 30
	defaultMaxTokens = 4096

	systemPromptHeader = "You are an AI agent operating in an isolated, sandboxed workspace."
	systemPromptFooter = "Use the provided tools to inspect and modify the workspace. " +
		"When the task is complete, stop calling tools and reply with a short summary of what you did."
)

// LLMRunner drives a skill test via any Provider backend.
type LLMRunner struct {
	p   provider.Provider
	cfg config.ProviderConfig
}

// NewLLMRunner creates a runner backed by the given provider.
func NewLLMRunner(p provider.Provider, cfg config.ProviderConfig) *LLMRunner {
	return &LLMRunner{p: p, cfg: cfg}
}

func (r *LLMRunner) Run(ctx context.Context, sk skill.Skill, sc scenario.Scenario, ws string) (*RunResult, error) {
	start := time.Now()

	if err := prepareWorkspace(ctx, sc, ws); err != nil {
		return nil, err
	}

	tools, err := provider.BuildToolList(sk)
	if err != nil {
		return nil, fmt.Errorf("build tool list: %w", err)
	}

	systemPrompt := buildSystemPrompt(sk)

	if r.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(r.cfg.Timeout)*time.Second)
		defer cancel()
	}

	executor := NewToolExecutor(ws)

	var messages []provider.Message
	messages = append(messages, provider.Message{
		Role: "user",
		Content: []provider.ContentBlock{
			{Type: "text", Text: sc.Input.UserPrompt},
		},
	})

	var finalOutput string

	for round := range defaultMaxRounds {
		resp, err := r.p.Send(ctx, provider.Request{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
			MaxTokens:    defaultMaxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("api call round %d: %w", round+1, err)
		}

		if len(resp.ToolCalls) == 0 {
			finalOutput = strings.Join(resp.TextBlocks, "\n")
			break
		}
		if resp.StopReason == "end_turn" || resp.StopReason == "stop" {
			if len(resp.TextBlocks) > 0 {
				finalOutput = strings.Join(resp.TextBlocks, "\n")
			}
			if len(resp.ToolCalls) == 0 {
				break
			}
		}

		// Build assistant message
		var assistantBlocks []provider.ContentBlock
		for _, t := range resp.TextBlocks {
			assistantBlocks = append(assistantBlocks, provider.ContentBlock{Type: "text", Text: t})
		}
		for _, tc := range resp.ToolCalls {
			assistantBlocks = append(assistantBlocks, provider.ContentBlock{
				Type:      "tool_use",
				ToolID:    tc.ID,
				ToolName:  tc.Name,
				ToolInput: tc.Input,
			})
		}
		messages = append(messages, provider.Message{
				Role:             "assistant",
				Content:          assistantBlocks,
				ReasoningContent: resp.ReasoningContent,
			})

		// Execute tools and build user message with results
		var resultBlocks []provider.ContentBlock
		for _, tc := range resp.ToolCalls {
			result := executor.Execute(tc.Name, tc.Input)
			resultBlocks = append(resultBlocks, provider.ContentBlock{
				Type:       "tool_result",
				ToolID:     tc.ID,
				ToolOutput: result.Content,
				IsError:    result.Type == "error",
			})
		}
		messages = append(messages, provider.Message{Role: "user", Content: resultBlocks})
	}

	// Capture mutations via git diff
	mutatedFiles, _ := captureMutations(ctx, ws)

	return &RunResult{
		ScenarioID:   sc.ID,
		Output:       finalOutput,
		ExecutedCmds: executor.ExecutedCmds,
		MutatedFiles: mutatedFiles,
		Duration:     time.Since(start),
	}, nil
}

func buildSystemPrompt(sk skill.Skill) string {
	var b strings.Builder
	b.WriteString(systemPromptHeader)
	b.WriteString("\n\n--- Skill Instructions ---\n")
	b.WriteString(sk.Instructions)
	b.WriteString("\n--- End of Instructions ---\n\n")
	b.WriteString(systemPromptFooter)
	return b.String()
}
