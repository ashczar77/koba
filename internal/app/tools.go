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

// ToolCall represents a parsed tool invocation from the model.
type ToolCall struct {
	Name   string
	Args   map[string]string
	Raw    string
}

var toolRe = regexp.MustCompile(`(?m)^TOOL:\s*(\w+)\s*(.*)$`)

func parseToolCalls(text string) []ToolCall {
	var calls []ToolCall
	for _, m := range toolRe.FindAllStringSubmatch(text, -1) {
		if len(m) < 3 {
			continue
		}
		name := strings.TrimSpace(m[1])
		rest := strings.TrimSpace(m[2])
		args := make(map[string]string)
		for _, part := range strings.Fields(rest) {
			if idx := strings.Index(part, "="); idx > 0 {
				args[part[:idx]] = strings.Trim(part[idx+1:], "\"")
			}
		}
		if args["path"] == "" && rest != "" && !strings.Contains(rest, "=") {
			args["path"] = rest
		}
		if args["cmd"] == "" && name == "run" && rest != "" {
			args["cmd"] = rest
		}
		if args["pattern"] == "" && name == "grep" {
			parts := strings.SplitN(rest, " ", 2)
			if len(parts) >= 1 {
				args["pattern"] = strings.Trim(parts[0], "\"")
			}
			if len(parts) >= 2 {
				args["path"] = parts[1]
			}
		}
		calls = append(calls, ToolCall{Name: name, Args: args, Raw: m[0]})
	}
	return calls
}

func executeTool(cwd string, call ToolCall) (string, error) {
	switch call.Name {
	case "read_file", "read":
		path := call.Args["path"]
		if path == "" {
			return "", fmt.Errorf("missing path")
		}
		full := filepath.Join(cwd, path)
		data, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "run", "exec":
		cmdStr := call.Args["cmd"]
		if cmdStr == "" {
			return "", fmt.Errorf("missing cmd")
		}
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out) + "\nError: " + err.Error(), nil
		}
		return string(out), nil
	case "grep":
		pattern := call.Args["pattern"]
		path := call.Args["path"]
		if pattern == "" {
			return "", fmt.Errorf("missing pattern")
		}
		if path == "" {
			path = "."
		}
		cmd := exec.Command("grep", "-rn", pattern, path)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return "(no matches)", nil
		}
		return string(out), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
}

// RunRun executes an agentic loop with tool use. The model can output
// TOOL: read_file path, TOOL: run cmd, TOOL: grep pattern path.
func RunRun(
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

	cwd, _ := os.Getwd()
	repoRoot, _ := contextx.FindRepoRoot(".")
	if repoRoot == "" {
		repoRoot = cwd
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

	toolPrompt := `You are Koba, an agentic coding assistant. You can use tools by outputting lines like:
TOOL: read_file path/to/file
TOOL: run <shell command>
TOOL: grep "pattern" path

Output one TOOL line at a time. After each tool result, you can use more tools or give your final answer.
Working directory: ` + cwd + `
Repo root: ` + repoRoot

	var messages []provider.Message
	messages = append(messages, provider.Message{Role: provider.RoleSystem, Content: toolPrompt})
	messages = append(messages, provider.Message{Role: provider.RoleUser, Content: request})

	w := bufio.NewWriter(out)
	defer w.Flush()

	maxTurns := 10
	for turn := 0; turn < maxTurns; turn++ {
		stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
		streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
			Model:       modelOverride,
			Temperature: cfg.Temperature,
			Stream:      true,
		})
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
		streamObj.Close()
		if prefixPrinted {
			fmt.Fprintln(w)
		}

		calls := parseToolCalls(resp.String())
		if len(calls) == 0 {
			break
		}

		messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.String()})

		for _, call := range calls {
			result, err := executeTool(repoRoot, call)
			if err != nil {
				result = "Error: " + err.Error()
			}
			fmt.Fprintf(w, "%s[Tool %s] %s\n%s\n", term.AssistantPrefix(), call.Name, call.Raw, result)
			w.Flush()
			messages = append(messages, provider.Message{
				Role:    provider.RoleUser,
				Content: fmt.Sprintf("Tool result for %s:\n%s", call.Raw, result),
			})
		}
	}

	return nil
}
