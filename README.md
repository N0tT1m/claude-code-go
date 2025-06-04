# Claude Code Go

A Go implementation of Claude Code that uses LM Studio for local AI assistance instead of Anthropic's API.

## Features

- 🤖 **Local AI**: Uses LM Studio for completely local AI processing
- 🔧 **Tool Integration**: File operations, git commands, shell execution
- 💬 **Interactive Mode**: Chat-based interface for natural language commands
- 📁 **Codebase Understanding**: Analyzes and works with your entire project
- 🎯 **Command Line**: Both interactive and command-based usage
- 🔒 **Privacy**: All processing happens locally, no data sent to external APIs

## Prerequisites

1. **LM Studio**: Download and install from [lmstudio.ai](https://lmstudio.ai/)
2. **Go 1.21+**: For building the application
3. **Git**: For repository operations

## Installation

### Option 1: Build from Source

```bash
git clone https://github.com/your-org/claude-code-go.git
cd claude-code-go
make build
make install
```

### Option 2: Go Install

```bash
go install github.com/your-org/claude-code-go@latest
```

## LM Studio Setup

1. Download and install LM Studio
2. Download a coding-focused model (recommended: Qwen2.5-Coder, CodeLlama, or similar)
3. Start LM Studio's local server:
- Open LM Studio
- Go to "Developer" tab
- Click "Start Server"
- Note the server URL (usually `http://localhost:1234/v1`)

## Configuration

On first run, Claude Go creates a config file at `~/.claude-go/config.json`:

```json
{
  "lm_studio": {
    "base_url": "http://localhost:1234/v1",
    "model": "qwen2.5-coder:14b",
    "timeout": 30
  },
  "agent": {
    "max_tokens": 4096,
    "temperature": 0.7,
    "system_prompt": "You are a helpful AI coding assistant..."
  },
  "git": {
    "auto_stage": true,
    "sign_off": false
  }
}
```

## Usage

### Interactive Mode

```bash
claude-go
```

This starts an interactive session where you can chat with the AI:

```
Claude Go - AI Coding Assistant
Type 'exit' to quit, '/help' for commands

claude> help me fix the bug in main.go
claude> create a new function to handle user authentication
claude> /commit
```

### Direct Commands

```bash
# Generate and create a git commit
claude-go commit

# One-shot command
claude-go chat "explain this error message"

# Show configuration
claude-go config
```

### Slash Commands

Within interactive mode, use these commands:

- `/help` - Show available commands
- `/commit` - Generate and create a git commit
- `/config` - Show current configuration
- `/models` - List available LM Studio models
- `exit` - Exit the program

## Key Differences from Original Claude Code

### Architecture Changes

1. **Language**: Transpiled from Node.js/TypeScript to Go
2. **LLM Backend**: Uses LM Studio instead of Anthropic's Claude API
3. **Local Processing**: Everything runs locally, no external API calls
4. **Configuration**: JSON-based config instead of OAuth

### API Compatibility

The tool maintains similar functionality to the original:

- Natural language command processing
- File and git operations
- Code understanding and generation
- Interactive chat interface
- Tool calling/function execution

### Performance Considerations

- **Startup**: Faster startup time due to Go's compiled nature
- **Memory**: Lower memory footprint compared to Node.js
- **Concurrency**: Better handling of concurrent operations
- **Dependencies**: Fewer runtime dependencies

## Available Tools

Claude Go includes these built-in tools:

### File Operations
- Read files
- Write files
- List directories
- Delete files

### Git Operations
- Status checking
- Diff viewing
- Adding files
- Creating commits
- Branch management

### Shell Execution
- Run build commands
- Execute tests
- Custom scripts

### Code Search
- Text pattern matching
- Function finding
- File name searching

## Development

### Building

```bash
make build          # Build binary
make install        # Install to /usr/local/bin
make test          # Run tests
make clean         # Clean build artifacts
```

### Code Style

```bash
make fmt           # Format code
make lint          # Run linter
```

### Docker

```bash
make docker-build  # Build Docker image
make docker-run    # Run in container
```

## Troubleshooting

### LM Studio Connection Issues

1. Ensure LM Studio is running with server enabled
2. Check the base URL in config matches LM Studio's server
3. Verify the model name exists in LM Studio
4. Check firewall settings for localhost connections

### Model Performance

1. Use coding-specific models for better results
2. Adjust temperature in config (lower = more focused)
3. Increase max_tokens for longer responses
4. Ensure sufficient RAM for your chosen model

### Git Integration

1. Ensure you're in a git repository
2. Check git is installed and in PATH
3. Verify proper git configuration (user.name, user.email)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Acknowledgments

- Original Claude Code by Anthropic
- LM Studio for local LLM infrastructure
- Go community for excellent tooling