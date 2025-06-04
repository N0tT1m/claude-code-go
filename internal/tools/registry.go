// Package: internal/tools/registry.go
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/N0tT1m/claude-code-go/internal/llm"
)

type Registry struct {
	tools map[string]Tool
}

type Tool interface {
	Name() string
	Description() string
	Parameters() interface{}
	Execute(args map[string]interface{}) (string, error)
}

func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}

	// Register built-in tools
	r.Register(&FileTool{})
	r.Register(&GitTool{})
	r.Register(&ShellTool{})
	r.Register(&SearchTool{})

	return r
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) GetAvailable() []llm.Tool {
	tools := make([]llm.Tool, 0, len(r.tools))

	for _, tool := range r.tools {
		tools = append(tools, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}

	return tools
}

func (r *Registry) Execute(name string, args map[string]interface{}) (string, error) {
	tool, exists := r.tools[name]
	if !exists {
		return "", fmt.Errorf("tool %s not found", name)
	}

	return tool.Execute(args)
}

// FileTool - File operations
type FileTool struct{}

func (t *FileTool) Name() string { return "file_operations" }

func (t *FileTool) Description() string {
	return "Read, write, and manage files in the codebase"
}

func (t *FileTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"read", "write", "list", "delete"},
				"description": "The file operation to perform",
			},
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File or directory path",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content to write (for write operation)",
			},
		},
		"required": []string{"operation", "path"},
	}
}

func (t *FileTool) Execute(args map[string]interface{}) (string, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return "", fmt.Errorf("operation is required")
	}

	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	switch operation {
	case "read":
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(content), nil

	case "write":
		content, ok := args["content"].(string)
		if !ok {
			return "", fmt.Errorf("content is required for write operation")
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}

		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("File written to %s", path), nil

	case "list":
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}

		var files []string
		for _, entry := range entries {
			files = append(files, entry.Name())
		}
		return strings.Join(files, "\n"), nil

	case "delete":
		err := os.Remove(path)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("File %s deleted", path), nil

	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}
}

// GitTool - Git operations
type GitTool struct{}

func (t *GitTool) Name() string { return "git_operations" }

func (t *GitTool) Description() string {
	return "Perform git operations like status, diff, add, commit, etc."
}

func (t *GitTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"status", "diff", "add", "commit", "log", "branch"},
				"description": "Git command to execute",
			},
			"args": map[string]interface{}{
				"type":        "array",
				"items":       map[string]string{"type": "string"},
				"description": "Additional arguments for the git command",
			},
		},
		"required": []string{"command"},
	}
}

func (t *GitTool) Execute(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	gitArgs := []string{command}

	if argsInterface, exists := args["args"]; exists {
		if argsList, ok := argsInterface.([]interface{}); ok {
			for _, arg := range argsList {
				if argStr, ok := arg.(string); ok {
					gitArgs = append(gitArgs, argStr)
				}
			}
		}
	}

	cmd := exec.Command("git", gitArgs...)
	output, err := cmd.CombinedOutput()

	return string(output), err
}

// ShellTool - Execute shell commands
type ShellTool struct{}

func (t *ShellTool) Name() string { return "shell_execute" }

func (t *ShellTool) Description() string {
	return "Execute shell commands for building, testing, and other operations"
}

func (t *ShellTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "Shell command to execute",
			},
			"working_dir": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for the command",
			},
		},
		"required": []string{"command"},
	}
}

func (t *ShellTool) Execute(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command is required")
	}

	cmd := exec.Command("sh", "-c", command)

	if workingDir, exists := args["working_dir"]; exists {
		if dir, ok := workingDir.(string); ok {
			cmd.Dir = dir
		}
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// SearchTool - Search through codebase
type SearchTool struct{}

func (t *SearchTool) Name() string { return "code_search" }

func (t *SearchTool) Description() string {
	return "Search for text patterns, function definitions, or file names in the codebase"
}

func (t *SearchTool) Parameters() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "Search pattern or text to find",
			},
			"file_pattern": map[string]interface{}{
				"type":        "string",
				"description": "File pattern to limit search (e.g., '*.go', '*.py')",
			},
			"case_sensitive": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether search should be case sensitive",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *SearchTool) Execute(args map[string]interface{}) (string, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return "", fmt.Errorf("pattern is required")
	}

	grepArgs := []string{"-r", "-n"}

	if caseSensitive, exists := args["case_sensitive"]; exists {
		if !caseSensitive.(bool) {
			grepArgs = append(grepArgs, "-i")
		}
	}

	grepArgs = append(grepArgs, pattern)

	if filePattern, exists := args["file_pattern"]; exists {
		if pattern, ok := filePattern.(string); ok {
			grepArgs = append(grepArgs, "--include="+pattern)
		}
	}

	grepArgs = append(grepArgs, ".")

	cmd := exec.Command("grep", grepArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil && len(output) == 0 {
		return "No matches found", nil
	}

	return string(output), nil
}
