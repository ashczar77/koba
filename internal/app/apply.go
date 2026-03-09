package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"koba/internal/config"
	"koba/internal/contextx"
	"koba/internal/errors"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunApply asks the model for a diff, shows it to the user, and optionally applies it.
func RunApply(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	args []string,
	modelOverride string,
	autoYes bool,
	dryRun bool,
	force bool,
) error {
	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	repoRoot, err := contextx.FindRepoRoot(".")
	if err != nil || repoRoot == "" {
		return fmt.Errorf("%s", errors.FriendlyGit(fmt.Errorf("not inside a git repository")))
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
		return fmt.Errorf("no request provided (e.g. agent apply \"add error handling to main.go\")")
	}

	// Gather context
	var ctxLines []string
	diff, _ := contextx.GitDiff(repoRoot)
	if diff != "" {
		ctxLines = append(ctxLines, "Current git diff:", "```", diff, "```")
	}
	readmePath := filepath.Join(repoRoot, "README.md")
	if content, err := contextx.ReadFileLimited(readmePath, 4*1024); err == nil && content != "" {
		ctxLines = append(ctxLines, "README (excerpt):", "```", content, "```")
	}

	systemPrompt := "You are Koba. The user wants you to produce a unified diff that implements their request.\n" +
		"Rules:\n" +
		"- Output ONLY a valid unified diff. No explanations before or after.\n" +
		"- Wrap the diff in a fenced block: ```diff ... ```\n" +
		"- Use paths relative to the repository root.\n" +
		"- Follow existing code style."

	var messages []provider.Message
	messages = append(messages, provider.Message{Role: provider.RoleSystem, Content: systemPrompt})
	if len(ctxLines) > 0 {
		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: strings.Join(ctxLines, "\n"),
		})
	}
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: "Produce a unified diff for: " + request,
	})

	stopSpinner := term.StartSpinner(errOut, "Koba is generating diff...")
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

	var resp strings.Builder
	for {
		chunk, err := streamObj.Recv(ctx)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(errOut, "stream error:", err)
			}
			break
		}
		resp.WriteString(chunk.Text)
		if chunk.Done {
			break
		}
	}

	diffContent := extractDiffBlock(resp.String())
	if diffContent == "" {
		fmt.Fprintln(errOut, "No diff block found in model response. Raw output:")
		fmt.Fprintln(errOut, resp.String())
		return fmt.Errorf("no diff to apply")
	}

	fmt.Fprintln(out, term.AssistantPrefix()+"Proposed diff:")
	fmt.Fprintln(out, "---")
	fmt.Fprintln(out, diffContent)
	fmt.Fprintln(out, "---")

	if dryRun {
		fmt.Fprintln(out, "(dry-run: diff not applied)")
		return nil
	}

	if !force {
		clean, err := contextx.GitStatusClean(repoRoot)
		if err == nil && !clean {
			fmt.Fprintln(errOut, "Working tree has uncommitted changes. Commit or stash them first, or use --force.")
			fmt.Fprintln(errOut, errors.FriendlyGit(fmt.Errorf("uncommitted changes")))
			return fmt.Errorf("uncommitted changes; use --force to override")
		}
	}

	if !autoYes {
		fmt.Fprint(out, "Apply this diff? [y/N] ")
		scanner := bufio.NewScanner(in)
		if !scanner.Scan() {
			return nil
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	// Write diff to temp file and apply with patch
	tmp, err := os.CreateTemp("", "koba-apply-*.diff")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(diffContent); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	cmd := exec.Command("patch", "-p1", "-d", repoRoot, "--forward", "-i", tmpPath)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(errOut, "Patch failed. Check the output above for conflicting hunks.")
		return fmt.Errorf("patch failed: %w", err)
	}

	fmt.Fprintln(out, "Diff applied successfully.")
	return nil
}

var diffBlockRe = regexp.MustCompile("(?s)```(?:diff)?\\s*\\n(.*?)```")

func extractDiffBlock(s string) string {
	m := diffBlockRe.FindStringSubmatch(s)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
