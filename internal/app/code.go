package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"koba/internal/config"
	"koba/internal/contextx"
	"koba/internal/errors"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunCode is a coding-focused helper that gathers repo context and sends it
// along with the user's request to the model.
func RunCode(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	args []string,
	modelOverride string,
) error {
	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	request := strings.TrimSpace(strings.Join(args, " "))
	if request == "" {
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		request = strings.TrimSpace(string(data))
	}
	if request == "" {
		return fmt.Errorf("no request provided")
	}

	// Collect basic repo context.
	repoRoot, _ := contextx.FindRepoRoot(".")
	var ctxLines []string
	if repoRoot != "" {
		diff, _ := contextx.GitDiff(repoRoot)
		if diff != "" {
			ctxLines = append(ctxLines, "Git diff:", "```", diff, "```")
		}

		readmePath := filepath.Join(repoRoot, "README.md")
		if content, err := contextx.ReadFileLimited(readmePath, 8*1024); err == nil && content != "" {
			ctxLines = append(ctxLines, "README.md (truncated):", "```markdown", content, "```")
		}

		goModPath := filepath.Join(repoRoot, "go.mod")
		if content, err := contextx.ReadFileLimited(goModPath, 4*1024); err == nil && content != "" {
			ctxLines = append(ctxLines, "go.mod (truncated):", "```go", content, "```")
		}
	}
	if hist := contextx.RecentShellHistory(15); hist != "" {
		ctxLines = append(ctxLines, hist)
	}

	cwd, _ := os.Getwd()
	systemPrompt := fmt.Sprintf(`You are Koba, a senior software engineer helping with coding tasks.
Working directory: %s

You are running inside the user's terminal and can see their git diff and key project files.
Use the provided context to understand the codebase and give precise, implementation-level answers.
Prefer concrete suggestions, diffs, or code snippets over high-level ideas.`, cwd)

	var messages []provider.Message
	messages = append(messages, provider.Message{
		Role:    provider.RoleSystem,
		Content: systemPrompt,
	})

	if len(ctxLines) > 0 {
		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: strings.Join(ctxLines, "\n"),
		})
	}

	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: request,
	})

	stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
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

