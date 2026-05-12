# koba

A coding agent in your terminal. Talk to it, and it writes code, edits files, runs commands, and searches your codebase.

Built with Go. Works with Anthropic Claude or locally with Ollama (no API key needed).

## Installation

### Download binary (recommended)

Download the latest release for your platform from [Releases](https://github.com/ashczar77/koba/releases).

```bash
# macOS / Linux
chmod +x koba
sudo mv koba /usr/local/bin/
```

### Build from source

Requires Go 1.21+:

```bash
go install ./cmd/koba
```

## Quick start

```bash
# Local mode (no API key)
export KOBA_PROVIDER=ollama
koba

# Or with Anthropic
export ANTHROPIC_API_KEY=sk-ant-...
koba
```

## Usage

```bash
koba                              # Start a conversation
koba "fix the bug in main.go"     # One-shot request
koba doctor                       # Check provider status
koba history                      # List past sessions
```

In a session, just type naturally. Koba can read files, run commands, search code, propose diffs, and apply them with your confirmation.

## Using Koba with Ollama

Ollama lets you run models locally — no API key, no internet, fully private.

### 1. Install Ollama

```bash
# macOS
brew install ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh
```

Or download from [ollama.com](https://ollama.com).

### 2. Start the Ollama server

```bash
ollama serve
```

Leave this running in the background (or it runs automatically on macOS after install).

### 3. Pull a model

```bash
ollama pull qwen3:1.7b
```

This is a small, fast model that supports tool use. For better quality (but slower, more resource-heavy):

```bash
ollama pull qwen3
```

### 4. Run Koba

```bash
export KOBA_PROVIDER=ollama
koba
```

That's it. Koba will connect to your local Ollama and start chatting.

## Configuration

Global config at `~/.agent/config.yaml`:

```yaml
default_provider: ollama    # or "anthropic"
default_model: qwen3:1.7b
temperature: 0.2
```

Project config at `.koba/config.yaml` (overrides global):

```yaml
default_model: codellama
system_prompt: "You are helping with this specific codebase."
```

Environment variables override everything:

| Variable | Purpose |
|----------|---------|
| `KOBA_PROVIDER` | Provider (`anthropic`, `ollama`) |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OLLAMA_HOST` | Ollama server address |

## Features

- Interactive conversation with readline (arrow keys, history)
- One-shot requests for quick tasks
- Agentic tool use: `read_file`, `write_file`, `run`, `grep`
- Diff-based edits with confirmation before applying
- Repo-aware context (git diff, project files)
- Streamed responses with markdown rendering
- Works fully offline with Ollama

## License

MIT
