// Package: internal/agent/agent.go
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

func (a *Agent) ProcessInput(ctx context.Context, input string) (string, error) {
	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Read relevant files in the project
	projectContext, err := a.getProjectContext(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to get project context: %w", err)
	}

	// Build enhanced system prompt with project context
	systemPrompt := fmt.Sprintf(`%s

## Current Project Context

### Working Directory: %s

### Project Files:
%s

### Recent Git Status:
%s

Use this context to provide accurate assistance with the codebase.`,
		a.config.Agent.SystemPrompt,
		workingDir,
		projectContext,
		a.getGitStatusString(ctx))

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
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

func (a *Agent) isSourceFile(path string) bool {
	sourceExts := []string{
		".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".c", ".cpp", ".h",
		".cs", ".php", ".rb", ".rs", ".swift", ".kt", ".scala", ".clj",
		".yaml", ".yml", ".json", ".toml", ".md", ".txt", ".sql",
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, sourceExt := range sourceExts {
		if ext == sourceExt {
			return true
		}
	}

	// Check for specific filenames
	base := strings.ToLower(filepath.Base(path))
	specialFiles := []string{"dockerfile", "makefile", "readme"}
	for _, special := range specialFiles {
		if strings.Contains(base, special) {
			return true
		}
	}

	return false
}

func (a *Agent) getGitStatusString(ctx context.Context) string {
	status, err := a.GetGitStatus(ctx)
	if err != nil {
		return "Not a git repository or git not available"
	}

	var statusStr strings.Builder
	statusStr.WriteString(fmt.Sprintf("Branch: %s\n", status.Branch))

	if len(status.Changes) > 0 {
		statusStr.WriteString("Changes:\n")
		for _, change := range status.Changes {
			statusStr.WriteString(fmt.Sprintf("  %s: %s\n", change.Type, change.File))
		}
	} else {
		statusStr.WriteString("No changes")
	}

	return statusStr.String()
}

func (a *Agent) getProjectContext(workingDir string) (string, error) {
	var context strings.Builder
	var totalTokens int
	const maxTokens = 2000 // Reserve tokens for context

	// Get list of relevant files, prioritizing by importance
	files, err := a.getRelevantFiles(workingDir)
	if err != nil {
		return "", err
	}

	context.WriteString("## Project Structure:\n")
	structure, _ := a.getProjectStructure(workingDir)
	context.WriteString(structure)
	context.WriteString("\n## Key Files:\n")

	for _, fileInfo := range files {
		if totalTokens > maxTokens {
			context.WriteString(fmt.Sprintf("\n... and %d more files (truncated due to context limit)\n", len(files)-len(context.String())))
			break
		}

		content, err := os.ReadFile(fileInfo.Path)
		if err != nil {
			continue
		}

		// Estimate tokens (rough: 4 chars per token)
		estimatedTokens := len(content) / 4
		if totalTokens+estimatedTokens > maxTokens {
			// Include just the file header/imports for context
			lines := strings.Split(string(content), "\n")
			preview := strings.Join(lines[:min(10, len(lines))], "\n")
			context.WriteString(fmt.Sprintf("\n--- %s (preview) ---\n%s\n... (truncated)\n", fileInfo.RelPath, preview))
			totalTokens += len(preview) / 4
		} else {
			context.WriteString(fmt.Sprintf("\n--- %s ---\n%s\n", fileInfo.RelPath, string(content)))
			totalTokens += estimatedTokens
		}
	}

	return context.String(), nil
}

type FileInfo struct {
	Path     string
	RelPath  string
	Size     int64
	ModTime  time.Time
	Priority int // Higher = more important
}

func (a *Agent) getRelevantFiles(workingDir string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden and build directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			skipDirs := []string{"node_modules", "vendor", "target", "build", "dist", ".git"}
			for _, skip := range skipDirs {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !a.isSourceFile(path) || info.Size() > 20000 {
			return nil
		}

		relPath, _ := filepath.Rel(workingDir, path)
		priority := a.getFilePriority(relPath)

		files = append(files, FileInfo{
			Path:     path,
			RelPath:  relPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			Priority: priority,
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by priority (high to low), then by modification time (recent first)
	sort.Slice(files, func(i, j int) bool {
		if files[i].Priority != files[j].Priority {
			return files[i].Priority > files[j].Priority
		}
		return files[i].ModTime.After(files[j].ModTime)
	})

	// Limit to most important files
	if len(files) > 10 {
		files = files[:10]
	}

	return files, nil
}

func (a *Agent) getFilePriority(relPath string) int {
	// Higher priority for more important files
	switch {
	case strings.Contains(relPath, "main.go"):
		return 100
	case strings.HasSuffix(relPath, ".go"):
		return 80
	case strings.Contains(relPath, "config"):
		return 70
	case strings.HasSuffix(relPath, ".md"):
		return 60
	case strings.HasSuffix(relPath, ".json") || strings.HasSuffix(relPath, ".yaml"):
		return 50
	default:
		return 30
	}
}

func (a *Agent) getProjectStructure(workingDir string) (string, error) {
	var structure strings.Builder

	err := filepath.Walk(workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			skipDirs := []string{"node_modules", "vendor", "target", "build", "dist", ".git"}
			for _, skip := range skipDirs {
				if info.Name() == skip {
					return filepath.SkipDir
				}
			}
		}

		relPath, _ := filepath.Rel(workingDir, path)
		depth := strings.Count(relPath, string(filepath.Separator))

		// Limit depth to avoid too much structure
		if depth > 3 {
			return nil
		}

		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			structure.WriteString(fmt.Sprintf("%s%s/\n", indent, info.Name()))
		} else if a.isSourceFile(path) {
			structure.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
		}

		return nil
	})

	return structure.String(), err
}
