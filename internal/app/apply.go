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
		repoRoot, err = os.Getwd()
		if err != nil || repoRoot == "" {
			return fmt.Errorf("%s", errors.FriendlyGit(fmt.Errorf("not inside a git repository")))
		}
		// Use current directory so apply still works (e.g. create hello.sh) when not in git.
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
		"- Use a fenced block with the label 'diff': write exactly ```diff on one line, then the diff, then ``` on its own line.\n" +
		"- Example format:\n  ```diff\n  --- a/main.go\n  +++ b/main.go\n  @@ -0,0 +1,10 @@\n  +package main\n  +...\n  ```\n" +
		"- You may output multiple ```diff blocks (e.g. one per file); Koba will apply them all.\n" +
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
	if err != nil {
		stopSpinner()
		return err
	}
	defer stopSpinner()
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

	respStr := resp.String()
	blocks := extractDiffBlocks(respStr)
	var diffContent string
	if len(blocks) > 0 {
		diffContent = strings.Join(blocks, "\n\n")
	} else {
		// Fallback: model often outputs ```go or ```bash instead of ```diff. If we find one code block
		// and the request mentions a single file path, write that file.
		content, path := extractCodeBlockAndPath(respStr, request)
		if content != "" && path != "" {
			diffContent = ""
			if !dryRun {
				if !force {
					if _, inRepo := contextx.FindRepoRoot("."); inRepo == nil {
						clean, err := contextx.GitStatusClean(repoRoot)
						if err == nil && !clean {
							msg := errors.FriendlyGit(fmt.Errorf("uncommitted changes"))
							fmt.Fprintln(errOut, msg)
							return fmt.Errorf("%s", msg)
						}
					}
				}
				if !autoYes {
					fmt.Fprintf(out, "Create %s with this content? [y/N] ", path)
					scanner := bufio.NewScanner(in)
					if !scanner.Scan() {
						return nil
					}
					if a := strings.TrimSpace(strings.ToLower(scanner.Text())); a != "y" && a != "yes" {
						fmt.Fprintln(out, "Aborted.")
						return nil
					}
				}
				fullPath := filepath.Join(repoRoot, path)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					return fmt.Errorf("create directory: %w", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					return fmt.Errorf("write file: %w", err)
				}
				fmt.Fprintln(out, "Wrote", path)
			} else {
				fmt.Fprint(out, term.AssistantPrefix())
				fmt.Fprintf(out, "Would create %s:\n%s\n", path, content)
			}
			return nil
		}
		fmt.Fprintln(errOut, "No diff block found in model response. Raw output:")
		fmt.Fprintln(errOut, respStr)
		return fmt.Errorf("no diff to apply")
	}

	fmt.Fprint(out, term.AssistantPrefix())
	fmt.Fprint(out, term.FormatDiffBlock(diffContent, dryRun))

	if dryRun {
		return nil
	}

	// Only require clean git status when we're in a repo and applying a real diff (not creating a new file).
	if !force {
		if _, err := contextx.FindRepoRoot("."); err == nil {
			clean, err := contextx.GitStatusClean(repoRoot)
			if err == nil && !clean {
				msg := errors.FriendlyGit(fmt.Errorf("uncommitted changes"))
				fmt.Fprintln(errOut, msg)
				return fmt.Errorf("%s", msg)
			}
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

	if err := applyPatch(repoRoot, diffContent, out, errOut); err != nil {
		fmt.Fprintln(errOut, "Patch failed. Check the output above for conflicting hunks.")
		return err
	}
	fmt.Fprintln(out, "Diff applied successfully.")
	return nil
}

// diffBlockRe matches only ```diff ... ``` blocks (allows optional space after backticks).
var diffBlockRe = regexp.MustCompile("(?s)```\\s*diff\\s*\\n(.*?)```")

// anyCodeBlockRe matches any fenced block (e.g. ```go, ```bash, ```).
var anyCodeBlockRe = regexp.MustCompile("(?s)```(?:\\w+)?\\s*\\n(.*?)```")

// pathInRequestRe matches a likely file path in the user request (e.g. main.go, src/foo.py).
var pathInRequestRe = regexp.MustCompile("\\b([\\w./_-]+\\.(go|sh|py|js|ts|html|css|json|mod|txt|yaml|yml|md|rb|java|kt)\\b)")

// extractDiffBlock returns the first ```diff block content, or empty.
func extractDiffBlock(s string) string {
	blocks := extractDiffBlocks(s)
	if len(blocks) == 0 {
		return ""
	}
	return blocks[0]
}

// extractDiffBlocks returns all ```diff blocks' content in order (for multi-file patches).
func extractDiffBlocks(s string) []string {
	var out []string
	for _, m := range diffBlockRe.FindAllStringSubmatch(s, -1) {
		if len(m) >= 2 {
			block := strings.TrimSpace(m[1])
			if block != "" {
				out = append(out, block)
			}
		}
	}
	return out
}

// extractCodeBlockAndPath returns the first fenced code block's content and a file path
// inferred from the request (e.g. "main.go" from "scaffold main.go"). Used when the
// model outputs ```go instead of ```diff so we can still create the file.
func extractCodeBlockAndPath(respStr, request string) (content, path string) {
	m := anyCodeBlockRe.FindStringSubmatch(respStr)
	if len(m) < 2 {
		return "", ""
	}
	content = m[1]
	// Strip optional language tag on first line
	if nl := strings.Index(content, "\n"); nl >= 0 {
		first := strings.TrimSpace(content[:nl])
		if first != "" && !strings.Contains(first, " ") {
			content = content[nl+1:]
		}
	}
	content = strings.TrimSuffix(content, "\n")

	pm := pathInRequestRe.FindStringSubmatch(request)
	if len(pm) < 2 {
		return content, ""
	}
	return content, pm[1]
}

// applyPatch writes diffContent to a temp file and runs patch -p1 in repoRoot.
// Caller is responsible for confirmations and dry-run; this only performs the patch.
func applyPatch(repoRoot, diffContent string, out, errOut io.Writer) error {
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
		return fmt.Errorf("patch failed: %w", err)
	}
	return nil
}
