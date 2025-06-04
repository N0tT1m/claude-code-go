package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/N0tT1m/claude-code-go/internal/agent"
	"github.com/N0tT1m/claude-code-go/internal/config"
	"github.com/N0tT1m/claude-code-go/internal/llm"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "claude-go",
		Short: "AI-powered coding assistant using LM Studio",
		Long:  "A Go implementation of Claude Code that uses LM Studio for local AI assistance",
		Run:   runInteractiveMode,
	}

	// Add flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "Config file path")
	rootCmd.PersistentFlags().StringP("model", "m", "", "LM Studio model to use")
	rootCmd.PersistentFlags().BoolP("headless", "p", false, "Run in headless mode")
	rootCmd.PersistentFlags().String("output-format", "text", "Output format (text, json)")

	// Add subcommands
	rootCmd.AddCommand(
		newCommitCommand(),
		newConfigCommand(),
		newChatCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runInteractiveMode(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize LM Studio client
	client := llm.NewLMStudioClient(cfg.LMStudio.BaseURL)

	// Initialize agent
	a := agent.New(client, cfg)

	fmt.Println("Claude Go - AI Coding Assistant")
	fmt.Println("Type 'exit' to quit, '/help' for commands")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("claude> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "exit" {
			break
		}

		if strings.HasPrefix(input, "/") {
			handleSlashCommand(input, a)
			continue
		}

		// Process natural language input
		ctx := context.Background()
		response, err := a.ProcessInput(ctx, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Println(response)
		fmt.Println()
	}
}

func handleSlashCommand(input string, a *agent.Agent) {
	parts := strings.Fields(input)
	command := parts[0][1:] // Remove the '/'

	switch command {
	case "help":
		showHelp()
	case "commit":
		handleCommit(a)
	case "config":
		showConfig()
	case "models":
		showAvailableModels(a)
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}
}

func showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  /help     - Show this help")
	fmt.Println("  /commit   - Create a git commit")
	fmt.Println("  /config   - Show current configuration")
	fmt.Println("  /models   - List available models")
	fmt.Println("  exit      - Exit the program")
}

func newCommitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "commit",
		Short: "Create an AI-generated git commit",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _ := config.Load()
			client := llm.NewLMStudioClient(cfg.LMStudio.BaseURL)
			a := agent.New(client, cfg)

			handleCommit(a)
		},
	}
}

func newConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Run: func(cmd *cobra.Command, args []string) {
			showConfig()
		},
	}
}

func newChatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat mode",
		Run:   runInteractiveMode,
	}
}

func handleCommit(a *agent.Agent) {
	ctx := context.Background()

	// Get git status
	status, err := a.GetGitStatus(ctx)
	if err != nil {
		fmt.Printf("Error getting git status: %v\n", err)
		return
	}

	if len(status.Changes) == 0 {
		fmt.Println("No changes to commit")
		return
	}

	// Generate commit message
	commitMsg, err := a.GenerateCommitMessage(ctx, status)
	if err != nil {
		fmt.Printf("Error generating commit message: %v\n", err)
		return
	}

	fmt.Printf("Generated commit message: %s\n", commitMsg)
	fmt.Print("Proceed with commit? (y/N): ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() && strings.ToLower(scanner.Text()) == "y" {
		err := a.CreateCommit(ctx, commitMsg)
		if err != nil {
			fmt.Printf("Error creating commit: %v\n", err)
			return
		}
		fmt.Println("Commit created successfully!")
	}
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	configJSON, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println(string(configJSON))
}

func showAvailableModels(a *agent.Agent) {
	ctx := context.Background()
	models, err := a.GetAvailableModels(ctx)
	if err != nil {
		fmt.Printf("Error getting models: %v\n", err)
		return
	}

	fmt.Println("Available models:")
	for _, model := range models {
		fmt.Printf("  - %s\n", model)
	}
}
