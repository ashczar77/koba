package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"koba/internal/config"
	"koba/internal/term"
)

// RunSession starts an interactive session: everything the user types is
// routed and handled (review, apply, ask, code, run). Like Kiro/Gemini CLI.
// Each session is logged under ~/.koba/sessions/<timestamp>.log for koba history.
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

	// Optional session log: tee response output so we can list/replay later.
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

	w := bufio.NewWriter(out)
	combined := bufio.NewWriter(combinedOut)
	defer w.Flush()
	defer combined.Flush()

	fmt.Fprint(out, banner)

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(w, term.UserPrefix())
		w.Flush()

		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if sessionFile != nil {
			fmt.Fprintf(sessionFile, "%s%s\n", term.UserPrefix(), line)
		}

		if err := RunDo(ctx, cfg, in, combined, errOut, line, modelOverride); err != nil {
			fmt.Fprintln(errOut, err)
		}
		fmt.Fprintln(combined)
		combined.Flush()
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintln(errOut, "input error:", err)
	}
	return nil
}

