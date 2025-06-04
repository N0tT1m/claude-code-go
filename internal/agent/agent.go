// Package: internal/agent/agent.go
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/N0tT1m/claude-code-go/internal/config"
	"github.com/N0tT1m/claude-code-go/internal/llm"
	"github.com/N0tT1m/claude-code-go/internal/tools"
)

type Agent struct {
	llmClient *llm.Client
	config    *config.Config
	tools     *tools.Registry
}

type GitStatus struct {
	Changes []GitChange
	Branch  string
}

type GitChange struct {
	Type string
	File string
}

func New(client *llm.Client, cfg *config.Config) *Agent {
	return &Agent{
		llmClient: client,
		config:    cfg,
		tools:     tools.NewRegistry(),
	}
}

func (a *Agent) ProcessInput(ctx context.Context, input string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: a.config.Agent.SystemPrompt},
		{Role: "user", Content: input},
	}

	req := llm.ChatRequest{
		Model:       a.config.LMStudio.Model,
		Messages:    messages,
		Tools:       a.tools.GetAvailable(),
		MaxTokens:   a.config.Agent.MaxTokens,
		Temperature: a.config.Agent.Temperature,
	}

	resp, err := a.llmClient.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}

func (a *Agent) GetGitStatus(ctx context.Context) (*GitStatus, error) {
	// Implementation would use git commands to get status
	// This is a simplified version
	return &GitStatus{
		Changes: []GitChange{
			{Type: "modified", File: "main.go"},
			{Type: "added", File: "config.go"},
		},
		Branch: "main",
	}, nil
}

func (a *Agent) GenerateCommitMessage(ctx context.Context, status *GitStatus) (string, error) {
	var changes []string
	for _, change := range status.Changes {
		changes = append(changes, fmt.Sprintf("%s: %s", change.Type, change.File))
	}

	prompt := fmt.Sprintf(`Generate a concise git commit message for these changes:
%s

Follow conventional commit format and be specific about what was changed.`, strings.Join(changes, "\n"))

	messages := []llm.Message{
		{Role: "system", Content: "You are a git commit message generator. Create clear, concise commit messages following conventional commit format."},
		{Role: "user", Content: prompt},
	}

	req := llm.ChatRequest{
		Model:       a.config.LMStudio.Model,
		Messages:    messages,
		MaxTokens:   100,
		Temperature: 0.3,
	}

	resp, err := a.llmClient.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no commit message generated")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (a *Agent) CreateCommit(ctx context.Context, message string) error {
	// Implementation would execute git commands
	fmt.Printf("Would execute: git commit -m \"%s\"\n", message)
	return nil
}

func (a *Agent) GetAvailableModels(ctx context.Context) ([]string, error) {
	return a.llmClient.GetModels(ctx)
}
