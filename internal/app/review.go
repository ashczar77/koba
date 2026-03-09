package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"koba/internal/config"
	"koba/internal/contextx"
	"koba/internal/errors"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunReview reads the current git diff (or stdin when piped) and asks the model
// for a structured code review. Supports: agent review, git diff | agent review
func RunReview(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	modelOverride string,
) error {
	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	var diff string
	if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		// Stdin is a pipe (e.g. git diff | agent review)
		data, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		diff = strings.TrimSpace(string(data))
	} else {
		repoRoot, err := contextx.FindRepoRoot(".")
	if err != nil || repoRoot == "" {
		return fmt.Errorf("%s", errors.FriendlyGit(fmt.Errorf("not inside a git repository")))
	}
		diff, err = contextx.GitDiff(repoRoot)
		if err != nil {
			return fmt.Errorf("failed to read git diff: %w", err)
		}
	}

	if diff == "" {
		return fmt.Errorf("no diff to review (git diff is empty or stdin was empty)")
	}

	cwd, _ := os.Getwd()
	systemPrompt := fmt.Sprintf(`You are Koba, a senior engineer doing a focused code review.
Working directory: %s

You are given a git diff. Provide specific, actionable feedback on:
- Correctness and potential bugs
- Edge cases and error handling
- Readability and maintainability
- Performance and scalability

Structure your response as:
1) High-level summary
2) Strengths
3) Issues / risks (with file/line references if possible)
4) Concrete suggestions or example patches.`, cwd)

	messages := []provider.Message{
		{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    provider.RoleUser,
			Content: diff,
		},
	}

	stopSpinner := term.StartSpinner(errOut, "Koba is reviewing...")
	streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:       modelOverride,
		Temperature: cfg.Temperature,
		Stream:      true,
	})
	stopSpinner()
	if err != nil {
		return err
	}
	defer streamObj.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	fmt.Fprint(w, term.AssistantPrefix())
	w.Flush()

	for {
		chunk, err := streamObj.Recv(ctx)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(errOut, "stream error:", err)
			}
			break
		}
		if chunk.Text != "" {
			fmt.Fprint(w, chunk.Text)
			w.Flush()
		}
		if chunk.Done {
			break
		}
	}
	fmt.Fprintln(w)

	return nil
}

