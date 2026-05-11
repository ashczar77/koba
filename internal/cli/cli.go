package cli

import (
	"context"
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
		// No args: interactive session.
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
	case "doctor":
		if err := app.RunDoctor(cfg, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	case "history":
		if includeHistory {
			if err := runHistory(args); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Everything else is a one-shot request.
	request := strings.Join(os.Args[1:], " ")
	if err := app.RunDo(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, request, "", nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
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
  koba <request>           One-shot request (e.g. "refactor auth handler")
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

Examples:
  koba                              Start a conversation
  koba "fix the bug in main.go"     One-shot request
  koba "review my diff"             Review current changes
  koba "find all usages of Foo"     Search with tools
`)
}
