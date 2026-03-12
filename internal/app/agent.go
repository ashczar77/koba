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
	"koba/internal/provider"
	"koba/internal/term"
)

// BuildAgentSystemPrompt returns the unified system prompt for the single agentic flow.
func BuildAgentSystemPrompt(cwd, repoRoot string) string {
	if repoRoot == "" {
		repoRoot = cwd
	}
	return "You are Koba, a coding companion in the user's terminal. Do what the user asks, then stop.\n\n" +
		"You have tools: read_file, run, grep, write_file. Use them when needed. When you see tool results, reply with one short summary and no further tool calls—that signals you are done.\n\n" +
		"For code edits (suggesting changes to existing files), output a ```diff ... ``` block in your response; Koba will ask the user to apply it.\n\n" +
		"Working directory: " + cwd + "\n" +
		"Repo root: " + repoRoot
}

// RunAgent runs the agentic loop with structured tool calling. Messages must contain system prompt and end with the latest user message.
func RunAgent(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	modelOverride string,
	messages *[]provider.Message,
) error {
	if messages == nil || len(*messages) == 0 {
		return fmt.Errorf("messages required (system + user)")
	}

	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		return err
	}

	repoRoot, _ := contextx.FindRepoRoot(".")
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}

	w := bufio.NewWriter(out)
	defer w.Flush()

	tools := AgentToolDefs()
	opts := provider.ChatOptions{
		Model:       modelOverride,
		Temperature: cfg.Temperature,
		Stream:      true,
		Tools:       tools,
	}

	maxTurns := 12
	for turn := 0; turn < maxTurns; turn++ {
		stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
		streamObj, err := client.Chat(ctx, *messages, opts)
		if err != nil {
			stopSpinner()
			return err
		}

		var resp strings.Builder
		prefixPrinted := false
		for {
			chunk, err := streamObj.Recv(ctx)
			if err != nil {
				if err != io.EOF {
					stopSpinner()
					fmt.Fprintln(errOut, "stream error:", err)
				}
				break
			}
			resp.WriteString(chunk.Text)
			if chunk.Text != "" {
				stopSpinner()
				if !prefixPrinted {
					fmt.Fprint(w, term.AssistantPrefix())
					prefixPrinted = true
				}
				fmt.Fprint(w, chunk.Text)
				w.Flush()
			}
			if chunk.Done {
				break
			}
		}
		stopSpinner()
		toolCalls := streamObj.ToolCalls()
		streamObj.Close()
		if prefixPrinted {
			fmt.Fprintln(w)
		}

		respStr := resp.String()
		assistantMsg := provider.Message{Role: provider.RoleAssistant, Content: respStr}
		if len(toolCalls) > 0 {
			assistantMsg.OptionalToolCalls = toolCalls
		}
		*messages = append(*messages, assistantMsg)

		var hadSuccessfulWrite bool
		if len(toolCalls) > 0 {
			for _, call := range toolCalls {
				result, err := ExecuteProviderTool(repoRoot, call)
				if err != nil {
					result = "Error: " + err.Error()
				} else if call.Name == "write_file" {
					hadSuccessfulWrite = true
				}
				fmt.Fprintf(w, "%s[Tool %s] %s\n%s\n", term.AssistantPrefix(), call.Name, call.Name, result)
				w.Flush()
				*messages = append(*messages, provider.Message{
					Role:       provider.RoleTool,
					Content:    result,
					ToolCallID: call.ID,
					ToolName:   call.Name,
				})
			}
		}

		if blocks := extractDiffBlocks(respStr); len(blocks) > 0 && !hadSuccessfulWrite {
			w.Flush()
			offerApplyDiff(in, out, errOut, repoRoot, blocks)
			return nil
		}

		if len(toolCalls) == 0 {
			return nil
		}
	}

	return nil
}

// offerApplyDiff prompts the user and applies the diff if they confirm.
func offerApplyDiff(in io.Reader, out, errOut io.Writer, repoRoot string, blocks []string) {
	diffContent := strings.Join(blocks, "\n\n")
	fmt.Fprint(out, "\nApply this diff? [y/N] ")
	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer == "y" || answer == "yes" {
			if err := applyPatch(repoRoot, diffContent, out, errOut); err != nil {
				fmt.Fprintln(errOut, "Patch failed:", err)
			} else {
				fmt.Fprintln(out, "Diff applied successfully.")
			}
		}
	}
}
