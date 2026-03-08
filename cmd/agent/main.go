package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"koba/internal/app"
	"koba/internal/config"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: agent <command> [args]")
		fmt.Println("Commands: chat, ask, code")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "chat":
		chatFlags := flag.NewFlagSet("chat", flag.ExitOnError)
		model := chatFlags.String("model", "", "override default model")
		noStream := chatFlags.Bool("no-stream", false, "disable streaming output")
		system := chatFlags.String("system", "", "custom system prompt")
		_ = chatFlags.Parse(args)

		if err := app.RunChat(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, *model, *system, !*noStream); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "ask":
		askFlags := flag.NewFlagSet("ask", flag.ExitOnError)
		model := askFlags.String("model", "", "override default model")
		system := askFlags.String("system", "", "custom system prompt")
		_ = askFlags.Parse(args)

		if err := app.RunAsk(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, askFlags.Args(), *model, *system); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "code":
		codeFlags := flag.NewFlagSet("code", flag.ExitOnError)
		model := codeFlags.String("model", "", "override default model")
		_ = codeFlags.Parse(args)

		if err := app.RunCode(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, codeFlags.Args(), *model); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}


