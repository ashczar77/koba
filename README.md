## koba

`koba` is a Go-based terminal coding agent CLI, inspired by tools like Claude Code, Gemini CLI, Kiro CLI, and Augment CLI.

It runs in your terminal, talks to Anthropic Claude (Haiku by default), and is designed so you can plug in other providers later.

The goal is simple: **give you a smart coding assistant directly in your shell**, with good repo context and a clean, minimal UX.

---

### Features

- **Interactive conversation** – just run `koba` and start talking.
- **One-shot requests** – `koba "fix the bug in main.go"` and done.
- **Agentic tool use** – reads files, runs commands, greps, writes files.
- **Diff-based edits** – proposes diffs and applies them with your confirmation.
- **Repo-aware** – picks up git diff, README, go.mod, and shell history as context.
- **Local-first** – Ollama provider for fully offline use; no API keys required.
- **Project-scoped config** – `.koba/config.yaml` in repo root overrides global settings.

---

### Installation

```bash
go install ./cmd/koba
```

Make sure `$GOBIN` (usually `$HOME/go/bin`) is on your `PATH`.

---

### Configuration

1. **Set your Anthropic API key** (optional if using Ollama or mock):

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

2. **Optional config file** at `~/.agent/config.yaml`:

```yaml
default_provider: anthropic   # or "ollama", "mock"
default_model: claude-3-haiku-20240307
temperature: 0.2
```

3. **Ollama (local, no API key)**:

```bash
export KOBA_PROVIDER=ollama
```

Ensure [Ollama](https://ollama.ai) is running. Default model: `llama3.2`.

4. **Project-scoped config** – create `.koba/config.yaml` in your repo root:

```yaml
default_provider: ollama
default_model: codellama
system_prompt: "You are helping with this specific codebase."
```

---

### Usage

#### Interactive session

```bash
koba
```

Start a conversation. Everything you type is handled by Koba – it can review diffs, edit files, search code, answer questions, and run commands. Just talk to it.

#### One-shot request

```bash
koba "refactor the auth handler"
koba "review my diff"
koba "add error handling to main.go"
koba "find all usages of Foo"
koba "explain how this function works"
```

Koba handles the request, uses tools as needed, and exits.

#### Utility commands

```bash
koba doctor              # Provider diagnostics
koba history             # List past sessions
koba history 3           # Show session #3
```

---

### How it works

- **Single agentic flow** – every request goes through the same agent loop with tool access (read_file, run, grep, write_file).
- **Config & env**: `internal/config` loads `~/.agent/config.yaml`, then merges project `.koba/config.yaml`. Env vars override.
- **Providers**: Anthropic, Ollama (local), and mock. Select via `default_provider` or `KOBA_PROVIDER`.
- **Repo context**: `internal/contextx` gathers git diff, README, go.mod, and recent shell history.
- **Diff apply**: Parses fenced diff blocks from model output and applies with `patch`.

---

### License

MIT. See `LICENSE` for details.
