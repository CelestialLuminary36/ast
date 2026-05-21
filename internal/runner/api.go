package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/CelestialLuminary36/agent-skill-test/internal/config"
	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

const (
	defaultMaxRounds    = 30
	defaultMaxTokens    = 4096
	defaultAPIEndpoint  = "https://api.anthropic.com/v1/messages"
	systemPromptHeader  = "You are an AI agent operating in an isolated, sandboxed workspace."
	systemPromptFooter  = "Use the provided tools to inspect and modify the workspace. " +
		"When the task is complete, stop calling tools and reply with a short summary of what you did."
)

type APIRunner struct {
	cfg *config.APIConfig
}

func NewAPIRunner(cfg *config.APIConfig) *APIRunner {
	return &APIRunner{cfg: cfg}
}

func (r *APIRunner) Run(ctx context.Context, sk skill.Skill, sc scenario.Scenario, ws string) (*RunResult, error) {
	start := time.Now()

	// 1. Prepare workspace + git baseline
	if err := r.prepareWorkspace(ctx, sc, ws); err != nil {
		return nil, err
	}

	// 2. Build client
	client, err := r.buildClient()
	if err != nil {
		return nil, err
	}

	// 3. Apply scenario timeout if set, else use config default
	if r.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(r.cfg.Timeout)*time.Second)
		defer cancel()
	}

	// 4. Build system + tools
	systemBlocks := []anthropic.TextBlockParam{
		{Text: buildSystemPrompt(sk)},
	}
	tools, err := buildToolDefs(sk)
	if err != nil {
		return nil, fmt.Errorf("build tool defs: %w", err)
	}

	// 5. Conversation loop
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(sc.Input.UserPrompt)),
	}
	executor := NewToolExecutor(ws)

	model := r.cfg.Model
	if model == "" {
		model = anthropic.ModelClaudeSonnet4_6
	}

	var finalOutput string
	maxRounds := defaultMaxRounds

	for round := range maxRounds {
		resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: int64(defaultMaxTokens),
			System:    systemBlocks,
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return nil, fmt.Errorf("api call round %d: %w", round+1, err)
		}

		// Parse response blocks into text + tool calls
		var textParts []string
		var toolCalls []anthropic.ToolUseBlock
		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.TextBlock:
				if strings.TrimSpace(v.Text) != "" {
					textParts = append(textParts, v.Text)
				}
			case anthropic.ToolUseBlock:
				toolCalls = append(toolCalls, v)
			}
		}

		// Terminal: model didn't request any tool use
		if len(toolCalls) == 0 || resp.StopReason == anthropic.StopReasonEndTurn {
			if len(textParts) > 0 {
				finalOutput = strings.Join(textParts, "\n")
			}
			if len(toolCalls) == 0 {
				break
			}
		}

		// Append assistant turn (text + tool_use blocks) to history
		var assistantBlocks []anthropic.ContentBlockParamUnion
		for _, t := range textParts {
			assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(t))
		}
		for _, tc := range toolCalls {
			var input any
			if len(tc.Input) > 0 {
				_ = json.Unmarshal(tc.Input, &input)
			}
			assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Execute tools, then append the results as the next user turn
		var resultBlocks []anthropic.ContentBlockParamUnion
		for _, tc := range toolCalls {
			var args map[string]any
			_ = json.Unmarshal(tc.Input, &args)
			result := executor.Execute(tc.Name, args)
			payload, _ := json.Marshal(result)
			resultBlocks = append(resultBlocks, anthropic.NewToolResultBlock(tc.ID, string(payload), result.Type == "error"))
		}
		messages = append(messages, anthropic.NewUserMessage(resultBlocks...))
	}

	// 6. Capture mutations via git diff
	mutatedFiles, _ := captureMutations(ctx, ws)

	return &RunResult{
		ScenarioID:   sc.ID,
		Output:       finalOutput,
		ExecutedCmds: executor.ExecutedCmds,
		MutatedFiles: mutatedFiles,
		Duration:     time.Since(start),
	}, nil
}

func (r *APIRunner) prepareWorkspace(ctx context.Context, sc scenario.Scenario, ws string) error {
	if sc.Environment.FixtureDir != "" {
		if err := copyDir(sc.Environment.FixtureDir, ws); err != nil {
			return fmt.Errorf("copy fixture: %w", err)
		}
	}
	if sc.Environment.InitScript != "" {
		cmd := exec.CommandContext(ctx, "sh", "-c", sc.Environment.InitScript)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("init script: %w\n%s", err, out)
		}
	}
	_ = runGit(ctx, ws, "init")
	_ = runGit(ctx, ws, "add", ".")
	_ = runGit(ctx, ws, "commit", "-m", "init", "--allow-empty")
	return nil
}

func (r *APIRunner) buildClient() (anthropic.Client, error) {
	apiKey := r.cfg.Key
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return anthropic.Client{}, fmt.Errorf("missing API key: set api.key in ast.yaml or ANTHROPIC_API_KEY env var")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if r.cfg.Endpoint != "" && r.cfg.Endpoint != defaultAPIEndpoint {
		// Strip trailing "/messages" if present — SDK appends it.
		base := strings.TrimSuffix(strings.TrimSuffix(r.cfg.Endpoint, "/messages"), "/")
		opts = append(opts, option.WithBaseURL(base+"/"))
	}
	return anthropic.NewClient(opts...), nil
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
