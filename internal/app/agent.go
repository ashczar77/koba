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
func BuildAgentSystemPrompt(cwd, repoRoot, customPrompt string) string {
	if repoRoot == "" {
		repoRoot = cwd
	}
	base := "You are Koba, a coding companion in the user's terminal. Do what the user asks, then stop.\n\n" +
		"You have tools: read_file, edit_file, write_file, run, grep. Use them when needed.\n" +
		"- Use edit_file for targeted changes to existing files (search and replace).\n" +
		"- Use write_file only for creating new files.\n" +
		"- read_file and grep run silently. run will ask the user for confirmation.\n\n" +
		"Working directory: " + cwd + "\n" +
		"Repo root: " + repoRoot
	if customPrompt != "" {
		base += "\n\nProject instructions: " + customPrompt
	}
	return base
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
		MaxTokens:   cfg.MaxTokens,
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
		var usage *provider.Usage
		for {
			chunk, err := streamObj.Recv(ctx)
			if err != nil {
				if err != io.EOF {
					stopSpinner()
					fmt.Fprintln(errOut, "stream error:", err)
				}
				break
			}
			if chunk.Text != "" {
				stopSpinner()
			}
			resp.WriteString(chunk.Text)
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			if chunk.Done {
				break
			}
		}
		stopSpinner()
		toolCalls := streamObj.ToolCalls()
		streamObj.Close()

		respStr := resp.String()
		if strings.TrimSpace(respStr) != "" {
			fmt.Fprintf(w, "\n%s\n", term.AssistantPrefix())
			fmt.Fprint(w, term.FormatResponse(respStr))
			if usage != nil {
				fmt.Fprintf(w, "\n%s  tokens: %d in · %d out%s\n", term.ColorDim(), usage.InputTokens, usage.OutputTokens, term.ColorReset())
			}
			w.Flush()
		}

		assistantMsg := provider.Message{Role: provider.RoleAssistant, Content: respStr}
		if len(toolCalls) > 0 {
			assistantMsg.OptionalToolCalls = toolCalls
		}
		*messages = append(*messages, assistantMsg)

		var hadSuccessfulWrite bool
		if len(toolCalls) > 0 {
			for _, call := range toolCalls {
				// Show subtle tool indicator
				detail := ""
				if v, ok := call.Arguments["path"]; ok {
					detail = fmt.Sprint(v)
				} else if v, ok := call.Arguments["cmd"]; ok {
					detail = fmt.Sprint(v)
				} else if v, ok := call.Arguments["pattern"]; ok {
					detail = fmt.Sprint(v)
				}
				fmt.Fprint(w, term.ToolPrefix(call.Name, detail))
				w.Flush()

				result, err := ExecuteProviderTool(repoRoot, call)
				if err != nil {
					result = "Error: " + err.Error()
				} else if call.Name == "write_file" || call.Name == "edit_file" {
					hadSuccessfulWrite = true
				}
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
