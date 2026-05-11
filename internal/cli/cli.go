package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"koba/internal/app"
	"koba/internal/config"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Run is the shared entrypoint for both koba and agent binaries.
func Run(ctx context.Context, includeHistory bool) {
	cwd, _ := os.Getwd()
	cfg, err := config.LoadForDir(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		if err := app.RunSession(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, ""); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "--help", "-h":
		printUsage()
		return
	case "--version", "-v":
		fmt.Println("koba " + Version)
		return
	}

	knownCommands := map[string]bool{
		"chat": true, "ask": true, "code": true, "review": true,
		"apply": true, "run": true, "doctor": true,
	}
	if includeHistory {
		knownCommands["history"] = true
	}

	if !knownCommands[cmd] {
		request := cmd
		if len(args) > 0 {
			request = cmd + " " + strings.Join(args, " ")
		}
		if err := app.RunDo(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, request, "", nil); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := dispatch(ctx, cfg, cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, cfg config.Config, cmd string, args []string) error {
	switch cmd {
	case "chat":
		fs := flag.NewFlagSet("chat", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		noStream := fs.Bool("no-stream", false, "disable streaming output")
		system := fs.String("system", "", "custom system prompt")
		_ = fs.Parse(args)
		return app.RunChat(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, *model, *system, !*noStream)
	case "ask":
		fs := flag.NewFlagSet("ask", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		system := fs.String("system", "", "custom system prompt")
		_ = fs.Parse(args)
		return app.RunAsk(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, fs.Args(), *model, *system)
	case "code":
		fs := flag.NewFlagSet("code", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		_ = fs.Parse(args)
		return app.RunCode(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, fs.Args(), *model)
	case "review":
		fs := flag.NewFlagSet("review", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		_ = fs.Parse(args)
		return app.RunReview(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, *model)
	case "apply":
		fs := flag.NewFlagSet("apply", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		yes := fs.Bool("yes", false, "apply without prompting")
		dryRun := fs.Bool("dry-run", false, "show diff only, do not apply")
		force := fs.Bool("force", false, "apply even with uncommitted changes")
		_ = fs.Parse(args)
		return app.RunApply(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, fs.Args(), *model, *yes, *dryRun, *force)
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		model := fs.String("model", "", "override default model")
		_ = fs.Parse(args)
		return app.RunRun(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, fs.Args(), *model)
	case "doctor":
		return app.RunDoctor(cfg, os.Stdout, os.Stderr)
	case "history":
		return runHistory(args)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runHistory(args []string) error {
	listLimit := 10
	showIndex := -1
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" {
			if i+1 >= len(args) {
				return fmt.Errorf("history: -n requires a number")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 {
				return fmt.Errorf("history: -n must be a non-negative number")
			}
			listLimit = n
			i++
			continue
		}
		if n, err := strconv.Atoi(args[i]); err == nil && n >= 0 {
			showIndex = n
			break
		}
	}
	return app.RunHistory(os.Stdout, os.Stderr, listLimit, showIndex)
}

func printUsage() {
	fmt.Print(`koba - coding companion in your terminal

Usage:
  koba                     Start interactive session
  koba <request>           One-shot agentic request
  koba chat                Interactive multi-turn chat
  koba ask <question>      Single-turn Q&A
  koba code <request>      Repo-aware coding help
  koba review              Review current git diff
  koba apply <request>     Generate and apply a diff
  koba run <request>       Agentic mode with tools
  koba doctor              Provider diagnostics
  koba history             List session history

Flags:
  --help, -h               Show this help
  --version, -v            Show version

Environment:
  ANTHROPIC_API_KEY        Anthropic API key
  KOBA_PROVIDER            Provider override (anthropic, ollama, mock)
  OLLAMA_HOST              Ollama server address
  NO_COLOR                 Disable colored output
`)
}
