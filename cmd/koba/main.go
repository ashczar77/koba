package main

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

func main() {
	ctx := context.Background()

	cwd, _ := os.Getwd()
	cfg, err := config.LoadForDir(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		// No args: start interactive session (everything you type goes to Koba)
		if err := app.RunSession(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, ""); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	knownCommands := map[string]bool{
		"chat": true, "ask": true, "code": true, "review": true,
		"apply": true, "run": true, "doctor": true, "history": true,
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
	case "review":
		reviewFlags := flag.NewFlagSet("review", flag.ExitOnError)
		model := reviewFlags.String("model", "", "override default model")
		_ = reviewFlags.Parse(args)

		if err := app.RunReview(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, *model); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "apply":
		applyFlags := flag.NewFlagSet("apply", flag.ExitOnError)
		model := applyFlags.String("model", "", "override default model")
		yes := applyFlags.Bool("yes", false, "apply without prompting")
		dryRun := applyFlags.Bool("dry-run", false, "show diff only, do not apply")
		force := applyFlags.Bool("force", false, "apply even with uncommitted changes")
		_ = applyFlags.Parse(args)

		if err := app.RunApply(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, applyFlags.Args(), *model, *yes, *dryRun, *force); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "run":
		runFlags := flag.NewFlagSet("run", flag.ExitOnError)
		model := runFlags.String("model", "", "override default model")
		_ = runFlags.Parse(args)

		if err := app.RunRun(ctx, cfg, os.Stdin, os.Stdout, os.Stderr, runFlags.Args(), *model); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "doctor":
		if err := app.RunDoctor(cfg, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "history":
		listLimit := 10
		showIndex := -1
		for i := 0; i < len(args); i++ {
			if args[i] == "-n" {
				if i+1 >= len(args) {
					fmt.Fprintf(os.Stderr, "history: -n requires a number\n")
					os.Exit(1)
				}
				n, err := strconv.Atoi(args[i+1])
				if err != nil || n < 0 {
					fmt.Fprintf(os.Stderr, "history: -n must be a non-negative number\n")
					os.Exit(1)
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
		if err := app.RunHistory(os.Stdout, os.Stderr, listLimit, showIndex); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
