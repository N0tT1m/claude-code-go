// Package: internal/config/config.go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LMStudio LMStudioConfig `json:"lm_studio"`
	Agent    AgentConfig    `json:"agent"`
	Git      GitConfig      `json:"git"`
}

type LMStudioConfig struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Timeout int    `json:"timeout"`
}

type AgentConfig struct {
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	SystemPrompt string  `json:"system_prompt"`
}

type GitConfig struct {
	AutoStage bool `json:"auto_stage"`
	SignOff   bool `json:"sign_off"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".claude-go", "config.json")

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &Config{
			LMStudio: LMStudioConfig{
				BaseURL: "http://192.168.1.78:1234/v1",
				Model:   "qwen2.5-coder:14b",
				Timeout: 30,
			},
			Agent: AgentConfig{
				MaxTokens:    4096,
				Temperature:  0.7,
				SystemPrompt: defaultSystemPrompt(),
			},
			Git: GitConfig{
				AutoStage: true,
				SignOff:   false,
			},
		}

		err := os.MkdirAll(filepath.Dir(configPath), 0755)
		if err != nil {
			return nil, err
		}

		return defaultConfig, Save(defaultConfig, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

func Save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func defaultSystemPrompt() string {
	return `You are a helpful AI coding assistant. You have access to various tools to help with software development tasks including:

- Reading and writing files
- Executing shell commands
- Git operations
- Code analysis and refactoring
- Testing and debugging assistance

Always be precise and helpful. When making changes to code, explain what you're doing and why. Ask for clarification if the request is ambiguous.`
}
