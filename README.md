## koba

`koba` is a Go-based terminal coding agent CLI, inspired by tools like Claude Code, Gemini CLI, Kiro CLI, and Augment CLI.

It runs in your terminal, talks to Anthropic Claude (Haiku by default), and is designed so you can plug in other providers later.

The goal is simple: **give you a smart coding assistant directly in your shell**, with good repo context and a clean, minimal UX.

---

### Features

- **Interactive chat** in your terminal (`agent chat`).
- **One-off questions** for quick answers (`agent ask`).
- **Repo-aware coding help** that reads your `git diff`, `README`, and `go.mod` (`agent code`).
- **Pluggable providers** via a small interface layer (Anthropic implemented first).
- **Lightweight UI** with simple colored prompts (`you>` and `koba>`).

---

### Installation

From the project root:

```bash
go install ./cmd/agent
```

This builds an `agent` binary into your `$GOBIN` (usually `$HOME/go/bin`).

Make sure that directory is on your `PATH`.

---

### Configuration

1. **Set your Anthropic API key**:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

2. **Optional: config file**

You can create a config file at `~/.agent/config.yaml` to set defaults:

```yaml
default_provider: anthropic
default_model: claude-3-haiku-20240307
temperature: 0.2
# anthropic_api_key: sk-ant-...  # optional; ANTHROPIC_API_KEY env takes precedence
```

If both are present, the environment variable `ANTHROPIC_API_KEY` wins over the config field.

---

### Usage

#### `agent chat`

Interactive multi-turn chat session.

```bash
agent chat
```

Flags:

- `--model` – override the default model.
- `--no-stream` – disable streaming output.
- `--system` – custom system prompt.

#### `agent ask`

Single-turn question, then exit.

From CLI args:

```bash
agent ask "How do I write a Go HTTP server?"
```

Or from stdin:

```bash
echo "Explain this function" | agent ask
```

Flags:

- `--model` – override the default model.
- `--system` – custom system prompt.

#### `agent code`

Coding-focused helper that uses basic repo context (run this inside a git repo).

```bash
agent code "Refactor the handler in handlers/user.go for clarity"
```

`agent code` will:

- Look for your git repo root.
- Read `git diff` output.
- Read `README.md` and `go.mod` (with size limits).
- Send that context, plus your request, to the model with a coding-focused system prompt.

---

### How it works (high level)

- **Config & env**: `internal/config` loads `~/.agent/config.yaml` and overlays env vars like `ANTHROPIC_API_KEY`.
- **Provider abstraction**: `internal/provider` defines a generic `Provider` interface, plus an Anthropic implementation that calls the Claude Messages API and exposes a streaming-like interface.
- **Repo context**: `internal/contextx` gathers git diff and key files such as `README.md` and `go.mod` (truncated to avoid huge prompts).
- **Terminal UX**: `internal/term` provides simple ANSI-colored prefixes (`you>` and `koba>`).
- **CLI wiring**: `cmd/agent/main.go` wires subcommands to the app layer in `internal/app`.

---

### Roadmap

- Add additional providers (Gemini, OpenAI, local models).
- Richer repo context and tools (e.g., running tests, applying diffs).
- More subcommands (`review`, `commit-msg`, etc.).
- Smarter, configurable system prompts per command.

---

### License

This project is licensed under the **MIT License**. See `LICENSE` for details.


