## koba

`koba` is a Go-based terminal coding agent CLI, inspired by tools like Claude Code, Gemini CLI, Kiro CLI, and Augment CLI.

It runs in your terminal, talks to Anthropic Claude (Haiku by default), and is designed so you can plug in other providers later.

The goal is simple: **give you a smart coding assistant directly in your shell**, with good repo context and a clean, minimal UX.

---

### Features

- **Interactive chat** in your terminal (`agent chat`).
- **One-off questions** for quick answers (`agent ask`).
- **Repo-aware coding help** that reads your `git diff`, `README`, and `go.mod` (`agent code`).
- **Diff-based review** of your current `git diff` (`agent review`), including pipe support: `git diff | agent review`.
- **Apply changes directly** – propose a diff, confirm, and write to disk (`agent apply`).
- **Agentic tool use** – model can read files, run commands, and grep (`agent run`).
- **Local-first** – Ollama provider for fully offline use; no API keys required.
- **Project-scoped config** – `.koba/config.yaml` in repo root overrides global settings.
- **Shell-native** – cwd and recent shell history included in context.

---

### Installation

From the project root:

```bash
go install ./cmd/koba
```

This builds a `koba` binary into your `$GOBIN` (usually `$HOME/go/bin`).  
(You can also build `./cmd/agent` to get an `agent` binary with the same behavior.)

Make sure that directory is on your `PATH`.

---

### Configuration

1. **Set your Anthropic API key** (for real calls, optional if you use the mock provider):

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

2. **Optional: config file**

You can create a config file at `~/.agent/config.yaml` to set defaults:

```yaml
default_provider: anthropic   # or "mock" for local testing without real API calls
default_model: claude-3-haiku-20240307
temperature: 0.2
# anthropic_api_key: sk-ant-...  # optional; ANTHROPIC_API_KEY env takes precedence
```

If both are present, the environment variable `ANTHROPIC_API_KEY` wins over the config field.

To force the mock provider regardless of config, you can also set:

```bash
export KOBA_PROVIDER=mock
```

3. **Ollama (local, no API key)**

For fully offline use, set:

```bash
export KOBA_PROVIDER=ollama
```

Ensure [Ollama](https://ollama.ai) is running. Default model: `llama3.2` (or set `default_model` in config).

4. **Project-scoped config**

Create `.koba/config.yaml` in your repo root to override defaults per project:

```yaml
default_provider: ollama
default_model: codellama
system_prompt: "You are helping with this specific codebase."
```

---

### Usage

#### Start Koba (recommended)

Run `koba` with no arguments to start an interactive session. Everything you type is sent to Koba and routed automatically (review, apply, ask, code, run):

```bash
koba
```

Then type requests like:
- `review my diff`
- `add a comment to main.go`
- `explain how goroutines work`
- `find all usages of Foo`

#### Single-shot request

Or run a one-off request:

```bash
koba "refactor the auth handler"
koba "review my diff"
koba "add error handling to main.go"
koba "find all usages of Foo"
koba "explain how this function works"
```

Routing:
- **review** → reviews your git diff
- **refactor/fix/add/change/modify** → proposes and applies a diff
- **find/list/search/grep** → agentic with tools
- **explain/how/what/why** → single-turn Q&A
- **default** → repo-aware code suggestions

#### `koba chat`

Interactive multi-turn chat (with conversation history).

```bash
koba chat
```

Flags:

- `--model` – override the default model.
- `--no-stream` – disable streaming output.
- `--system` – custom system prompt.

#### `koba ask`

Single-turn question, then exit.

From CLI args:

```bash
koba ask "How do I write a Go HTTP server?"
```

Or from stdin:

```bash
echo "Explain this function" | koba ask
```

Flags:

- `--model` – override the default model.
- `--system` – custom system prompt.

#### `koba code`

Coding-focused helper that uses basic repo context (run this inside a git repo).

```bash
koba code "Refactor the handler in handlers/user.go for clarity"
```

`agent code` will:

- Look for your git repo root.
- Read `git diff` output.
- Read `README.md` and `go.mod` (with size limits).
- Send that context, plus your request, to the model with a coding-focused system prompt.

#### `koba review`

Review your current `git diff` and get structured feedback.

```bash
koba review
# Or pipe a diff:
git diff | koba review
```

Flags:

- `--model` – override the default model.

#### `koba apply`

Propose a unified diff for your request, show it, and optionally apply it.

```bash
koba apply "add error handling to main.go"
# Or with auto-confirm:
koba apply --yes "fix the typo in README"
# Preview only (no apply):
koba apply --dry-run "refactor handler"
```

Flags:

- `--model` – override the default model.
- `--yes` – apply without prompting.
- `--dry-run` – show diff only, do not apply.
- `--force` – apply even with uncommitted changes (default: refuse).

#### `koba doctor`

Run provider diagnostics: check Anthropic key, Ollama reachability, and list pulled models.

```bash
koba doctor
```

#### `koba run`

Agentic mode: the model can use tools (read file, run command, grep) to accomplish tasks.

```bash
koba run "Find all usages of Foo and summarize them"
```

The model outputs `TOOL: read_file path`, `TOOL: run cmd`, or `TOOL: grep pattern path`. Koba executes them and continues the conversation.

---

### How it works (high level)

- **Config & env**: `internal/config` loads `~/.agent/config.yaml`, then merges project `.koba/config.yaml` from the current directory upward. Env vars override.
- **Providers**: Anthropic, Ollama (local), and mock. Select via `default_provider` or `KOBA_PROVIDER`.
- **Repo context**: `internal/contextx` gathers git diff, `README.md`, `go.mod`, and recent shell history.
- **Apply**: Parses a fenced diff block from model output and applies with `patch`.
- **Tool use**: `agent run` parses `TOOL:` lines, executes read_file/run/grep, and feeds results back to the model.

---

### License

This project is licensed under the **MIT License**. See `LICENSE` for details.


