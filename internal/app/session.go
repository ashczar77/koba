package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"

	"koba/internal/config"
	"koba/internal/contextx"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunSession starts an interactive session with readline support and Ctrl+C handling.
func RunSession(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	modelOverride string,
) error {
	providerName := providerNameFromEnv(cfg)
	mode := "LIVE"
	switch providerName {
	case "mock":
		mode = "MOCK"
	case "ollama":
		mode = "LOCAL"
	}
	banner := term.Banner(strings.ToUpper(providerName), modelForDisplay(providerName, cfg, modelOverride), mode)

	// Session log.
	var sessionFile *os.File
	var combinedOut io.Writer = out
	if _, err := EnsureSessionsDir(); err == nil {
		_, f, err := StartSessionLog()
		if err == nil {
			sessionFile = f
			defer func() { _ = sessionFile.Close() }()
			fmt.Fprint(sessionFile, banner)
			combinedOut = io.MultiWriter(out, sessionFile)
		}
	}

	fmt.Fprint(out, banner)

	cwd, _ := os.Getwd()
	repoRoot, _ := contextx.FindRepoRoot(".")
	if repoRoot == "" {
		repoRoot = cwd
	}
	messages := []provider.Message{
		{Role: provider.RoleSystem, Content: BuildAgentSystemPrompt(cwd, repoRoot, cfg.SystemPrompt)},
	}

	// Setup readline.
	historyFile := ""
	if home, err := os.UserHomeDir(); err == nil {
		dir := home + "/.koba"
		_ = os.MkdirAll(dir, 0755)
		historyFile = dir + "/history"
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          term.UserPrefix(),
		HistoryFile:     historyFile,
		InterruptPrompt: fmt.Sprintf("\n  %sPress Ctrl+C again to exit.%s\n  %sPress Ctrl+D to close Koba.%s", term.ColorDim(), term.ColorReset(), term.ColorDim(), term.ColorReset()),
		EOFPrompt:       "",
	})
	if err != nil {
		return fmt.Errorf("readline init: %w", err)
	}
	defer rl.Close()

	// Ctrl+C handling: first press shows warning, second within 2s exits.
	var lastInterrupt time.Time
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)
	defer signal.Stop(sigCh)

	go func() {
		for range sigCh {
			now := time.Now()
			if now.Sub(lastInterrupt) < 2*time.Second {
				fmt.Fprint(out, term.ExitMessage())
				os.Exit(0)
			}
			lastInterrupt = now
			// The readline InterruptPrompt handles the display.
		}
	}()

	var lastErr error
	var lastUser string
	for {
		rl.SetPrompt(term.UserPrefix())
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				lastInterrupt = time.Now()
				continue
			}
			// EOF (Ctrl+D)
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if sessionFile != nil {
			fmt.Fprintf(sessionFile, "%s%s\n", term.UserPrefix(), line)
		}

		request := line
		if lastErr != nil && lastUser != "" {
			request = "Context: The user's previous message was: \"" + lastUser + "\". Koba returned an error: " + lastErr.Error() + "\n\nCurrent message: " + line
		}
		lastUser = line
		lastErr = nil

		if err := RunDo(ctx, cfg, in, combinedOut, errOut, request, modelOverride, &messages); err != nil {
			lastErr = err
			fmt.Fprintln(errOut, err)
		}
		fmt.Fprintln(combinedOut)
	}

	fmt.Fprint(out, term.ExitMessage())
	return nil
}
