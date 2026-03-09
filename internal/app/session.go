package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"koba/internal/config"
	"koba/internal/term"
)

// RunSession starts an interactive session: everything the user types is
// routed and handled (review, apply, ask, code, run). Like Kiro/Gemini CLI.
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
	banner := term.Banner(strings.ToUpper(providerName), chooseModel(cfg, modelOverride), mode)
	fmt.Fprint(out, banner)

	w := bufio.NewWriter(out)
	defer w.Flush()

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

		if err := RunDo(ctx, cfg, in, w, errOut, line, modelOverride); err != nil {
			fmt.Fprintln(errOut, err)
		}
		// Ensure newline after each response
		fmt.Fprintln(w)
		w.Flush()
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintln(errOut, "input error:", err)
	}
	return nil
}
