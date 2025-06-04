// Package: internal/agent/enhanced_agent.go
package agent

import (
	builtinContext "context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/N0tT1m/claude-code-go/internal/config"
	"github.com/N0tT1m/claude-code-go/internal/context"
	"github.com/N0tT1m/claude-code-go/internal/llm"
	"github.com/N0tT1m/claude-code-go/internal/mcp"
	"github.com/N0tT1m/claude-code-go/internal/tools"
)

type EnhancedAgent struct {
	llmClient      *llm.Client
	config         *config.Config
	tools          *tools.Registry
	contextManager *context.ContextManager
	mcpClient      *mcp.Client
	mcpServer      *mcp.Server
	sessionMemory  []llm.Message
	workingDir     string
}

func NewEnhanced(client *llm.Client, cfg *config.Config) *EnhancedAgent {
	workingDir, _ := os.Getwd()

	return &EnhancedAgent{
		llmClient:      client,
		config:         cfg,
		tools:          tools.NewRegistry(),
		contextManager: context.NewContextManager(workingDir, cfg.Agent.MaxTokens),
		sessionMemory:  []llm.Message{},
		workingDir:     workingDir,
	}
}

func (a *EnhancedAgent) StartMCPServer(socketPath string) error {
	a.mcpServer = mcp.NewMCPServer("claude-go", "0.1.0", a.tools)

	// Register project files as MCP resources
	if err := a.registerProjectResources(); err != nil {
		return fmt.Errorf("failed to register resources: %w", err)
	}

	return a.mcpServer.Start(socketPath)
}

func (a *EnhancedAgent) ConnectToMCPServer(socketPath string) error {
	a.mcpClient = mcp.NewMCPClient()
	if err := a.mcpClient.ConnectUnix(socketPath); err != nil {
		return err
	}

	return a.mcpClient.Initialize("claude-go-client", "0.1.0")
}

func (a *EnhancedAgent) ProcessInputStreaming(ctx builtinContext.Context, input string, callback func(string) error) error {
	// Add input to session memory
	a.sessionMemory = append(a.sessionMemory, llm.Message{
		Role:    "user",
		Content: input,
	})

	// Get project context
	projectCtx, err := a.contextManager.GetProjectContext()
	if err != nil {
		return fmt.Errorf("failed to get project context: %w", err)
	}

	// Build enhanced system prompt with context
	systemPrompt := a.buildEnhancedSystemPrompt(projectCtx)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	// Add recent session memory (keep last 10 exchanges)
	if len(a.sessionMemory) > 20 {
		a.sessionMemory = a.sessionMemory[len(a.sessionMemory)-20:]
	}
	messages = append(messages, a.sessionMemory...)

	req := llm.ChatRequest{
		Model:       a.config.LMStudio.Model,
		Messages:    messages,
		Tools:       a.tools.GetAvailable(),
		MaxTokens:   a.config.Agent.MaxTokens,
		Temperature: a.config.Agent.Temperature,
		Stream:      true,
	}

	var fullResponse strings.Builder

	err = a.llmClient.ChatStream(ctx, req, func(response llm.StreamResponse) error {
		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				fullResponse.WriteString(delta)
				return callback(delta)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("streaming request failed: %w", err)
	}

	// Add response to session memory
	a.sessionMemory = append(a.sessionMemory, llm.Message{
		Role:    "assistant",
		Content: fullResponse.String(),
	})

	return nil
}

func (a *EnhancedAgent) buildEnhancedSystemPrompt(projectCtx *context.ProjectContext) string {
	var prompt strings.Builder

	prompt.WriteString(a.config.Agent.SystemPrompt)
	prompt.WriteString("\n\n## Current Project Context\n\n")

	// Add project structure
	prompt.WriteString("### Project Structure:\n```\n")
	prompt.WriteString(projectCtx.Structure)
	prompt.WriteString("\n```\n\n")

	// Add git information
	if projectCtx.GitInfo.Branch != "" {
		prompt.WriteString(fmt.Sprintf("### Git Information:\n"))
		prompt.WriteString(fmt.Sprintf("- Current branch: %s\n", projectCtx.GitInfo.Branch))
		prompt.WriteString(fmt.Sprintf("- Status: %s\n", projectCtx.GitInfo.Status))
		if len(projectCtx.GitInfo.RecentCommits) > 0 {
			prompt.WriteString("- Recent commits:\n")
			for _, commit := range projectCtx.GitInfo.RecentCommits {
				prompt.WriteString(fmt.Sprintf("  - %s\n", commit))
			}
		}
		prompt.WriteString("\n")
	}

	// Add dependencies
	if len(projectCtx.Dependencies) > 0 {
		prompt.WriteString("### Dependencies:\n")
		for _, dep := range projectCtx.Dependencies {
			prompt.WriteString(fmt.Sprintf("- %s\n", dep))
		}
		prompt.WriteString("\n")
	}

	// Add relevant files (sample of recent files)
	if len(projectCtx.Files) > 0 {
		prompt.WriteString("### Key Files (recently modified):\n")
		for i, file := range projectCtx.Files {
			if i >= 5 { // Limit to first 5 files to save tokens
				break
			}
			prompt.WriteString(fmt.Sprintf("- %s (%s, %d tokens)\n", file.Path, file.Language, file.TokenCount))
		}

		if len(projectCtx.Files) > 5 {
			prompt.WriteString(fmt.Sprintf("... and %d more files\n", len(projectCtx.Files)-5))
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString(fmt.Sprintf("Total project context: %d tokens\n\n", projectCtx.TotalTokens))

	prompt.WriteString("Use this context to provide more accurate and relevant assistance. ")
	prompt.WriteString("When referencing files or making changes, consider the project structure and existing code patterns.")

	return prompt.String()
}

func (a *EnhancedAgent) registerProjectResources() error {
	projectCtx, err := a.contextManager.GetProjectContext()
	if err != nil {
		return err
	}

	for _, file := range projectCtx.Files {
		mimeType := "text/plain"
		if strings.Contains(file.Language, "json") {
			mimeType = "application/json"
		}

		metadata := map[string]string{
			"language":      file.Language,
			"size":          fmt.Sprintf("%d", file.Size),
			"last_modified": file.LastModified.Format(time.RFC3339),
			"token_count":   fmt.Sprintf("%d", file.TokenCount),
		}

		a.mcpServer.RegisterResource(
			file.Path,
			file.Path,
			fmt.Sprintf("%s file (%s)", file.Language, file.Path),
			mimeType,
			metadata,
		)
	}

	return nil
}

func (a *EnhancedAgent) GetProjectSummary(ctx builtinContext.Context) (string, error) {
	projectCtx, err := a.contextManager.GetProjectContext()
	if err != nil {
		return "", err
	}

	var summary strings.Builder
	summary.WriteString("# Project Summary\n\n")

	summary.WriteString(fmt.Sprintf("**Working Directory:** %s\n", a.workingDir))
	summary.WriteString(fmt.Sprintf("**Total Files:** %d\n", len(projectCtx.Files)))
	summary.WriteString(fmt.Sprintf("**Total Tokens:** %d\n", projectCtx.TotalTokens))

	if projectCtx.GitInfo.Branch != "" {
		summary.WriteString(fmt.Sprintf("**Git Branch:** %s\n", projectCtx.GitInfo.Branch))
		summary.WriteString(fmt.Sprintf("**Git Status:** %s\n", projectCtx.GitInfo.Status))
	}

	summary.WriteString("\n## Languages Used:\n")
	langCount := make(map[string]int)
	for _, file := range projectCtx.Files {
		langCount[file.Language]++
	}

	for lang, count := range langCount {
		summary.WriteString(fmt.Sprintf("- %s: %d files\n", lang, count))
	}

	summary.WriteString("\n## Dependencies:\n")
	for _, dep := range projectCtx.Dependencies {
		summary.WriteString(fmt.Sprintf("- %s\n", dep))
	}

	return summary.String(), nil
}

func (a *EnhancedAgent) ExecuteCommand(ctx builtinContext.Context, command string) (string, error) {
	// Enhanced command execution with context awareness
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	switch parts[0] {
	case "analyze":
		return a.analyzeCodebase(ctx)
	case "summary":
		return a.GetProjectSummary(ctx)
	case "context":
		return a.showCurrentContext(ctx)
	case "refresh":
		a.contextManager = context.NewContextManager(a.workingDir, a.config.Agent.MaxTokens)
		return "Context refreshed", nil
	default:
		// Delegate to regular tool execution
		return a.tools.Execute("shell_execute", map[string]interface{}{
			"command":     command,
			"working_dir": a.workingDir,
		})
	}
}

func (a *EnhancedAgent) analyzeCodebase(ctx builtinContext.Context) (string, error) {
	projectCtx, err := a.contextManager.GetProjectContext()
	if err != nil {
		return "", err
	}

	// Use LLM to analyze the codebase
	analysisPrompt := fmt.Sprintf(`Analyze this codebase and provide insights:

Project Structure:
%s

Files: %d
Total Tokens: %d

Recent Files:
%s

Please provide:
1. Overall architecture assessment
2. Code quality observations  
3. Potential improvements
4. Technology stack summary
5. Any concerns or recommendations`,
		projectCtx.Structure,
		len(projectCtx.Files),
		projectCtx.TotalTokens,
		a.formatFileList(projectCtx.Files[:min(10, len(projectCtx.Files))]))

	messages := []llm.Message{
		{Role: "system", Content: "You are a senior software architect reviewing a codebase. Provide a comprehensive but concise analysis."},
		{Role: "user", Content: analysisPrompt},
	}

	req := llm.ChatRequest{
		Model:       a.config.LMStudio.Model,
		Messages:    messages,
		MaxTokens:   2048,
		Temperature: 0.3,
	}

	resp, err := a.llmClient.Chat(ctx, req)
	if err != nil {
		return "", fmt.Errorf("analysis failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no analysis generated")
	}

	return resp.Choices[0].Message.Content, nil
}

func (a *EnhancedAgent) showCurrentContext(ctx builtinContext.Context) (string, error) {
	projectCtx, err := a.contextManager.GetProjectContext()
	if err != nil {
		return "", err
	}

	var context strings.Builder
	context.WriteString("# Current Context\n\n")
	context.WriteString(fmt.Sprintf("Working Directory: %s\n", a.workingDir))
	context.WriteString(fmt.Sprintf("Session Messages: %d\n", len(a.sessionMemory)))
	context.WriteString(fmt.Sprintf("Project Files: %d\n", len(projectCtx.Files)))
	context.WriteString(fmt.Sprintf("Context Tokens: %d/%d\n", projectCtx.TotalTokens, a.config.Agent.MaxTokens))

	context.WriteString("\n## Available Tools:\n")
	tools := a.tools.GetAvailable()
	for _, tool := range tools {
		context.WriteString(fmt.Sprintf("- %s: %s\n", tool.Function.Name, tool.Function.Description))
	}

	return context.String(), nil
}

func (a *EnhancedAgent) formatFileList(files []context.FileContext) string {
	var list strings.Builder
	for _, file := range files {
		list.WriteString(fmt.Sprintf("- %s (%s, %d bytes)\n", file.Path, file.Language, file.Size))
	}
	return list.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
