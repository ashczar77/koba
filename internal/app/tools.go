package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"koba/internal/config"
	"koba/internal/provider"
	"koba/internal/term"
)

// AgentToolDefs returns the structured tool definitions for the agent (read_file, run, grep, write_file).
func AgentToolDefs() []provider.ToolDef {
	return []provider.ToolDef{
		{
			Name:        "read_file",
			Description: "Read the contents of a file. Use when you need to see the current content of a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Path to the file relative to repo root"},
				},
				"required": []interface{}{"path"},
			},
		},
		{
			Name:        "run",
			Description: "Run a shell command in the project directory. Use to run scripts, build, test, or any shell command.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cmd": map[string]interface{}{"type": "string", "description": "The shell command to run"},
				},
				"required": []interface{}{"cmd"},
			},
		},
		{
			Name:        "grep",
			Description: "Search for a pattern in files. Use to find occurrences of text or code.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string", "description": "Search pattern (regex-friendly)"},
					"path":    map[string]interface{}{"type": "string", "description": "Path or directory to search (default: .)"},
				},
				"required": []interface{}{"pattern"},
			},
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a file with the given content. Use when the user asks to create or replace a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to the file relative to repo root"},
					"content": map[string]interface{}{"type": "string", "description": "Full file content"},
				},
				"required": []interface{}{"path", "content"},
			},
		},
	}
}

// ExecuteProviderTool runs a provider.ToolCall (structured) and returns the result string.
func ExecuteProviderTool(repoRoot string, call provider.ToolCall) (string, error) {
	appCall := providerToolCallToApp(call)
	return executeTool(repoRoot, appCall)
}

// providerToolCallToApp converts a provider.ToolCall to the app's ToolCall (Args map[string]string, Content for write_file).
func providerToolCallToApp(p provider.ToolCall) ToolCall {
	args := make(map[string]string)
	for k, v := range p.Arguments {
		if s, ok := v.(string); ok {
			args[k] = s
		} else {
			args[k] = fmt.Sprint(v)
		}
	}
	content := args["content"]
	delete(args, "content")
	return ToolCall{Name: p.Name, Args: args, Content: content, Raw: p.Name}
}

// ToolCall represents a parsed tool invocation from the model.
type ToolCall struct {
	Name    string
	Args    map[string]string
	Raw     string
	Content string // used by write_file: content from the following fenced block
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
		if name == "diff" && rest != "" {
			args["path"] = rest
		}
		calls = append(calls, ToolCall{Name: name, Args: args, Raw: m[0]})
	}
	return calls
}

// parseWriteFileContent finds the first ```...``` block after the TOOL: write_file line.
// Rely on the prompt so the model uses ```go/``` for file body and ```diff only for patches.
// (Proper approach: provider-native tool calling with structured args, not text parsing.)
func parseWriteFileContent(fullText string, call ToolCall) string {
	idx := strings.Index(fullText, call.Raw)
	if idx < 0 {
		return ""
	}
	rest := fullText[idx+len(call.Raw):]
	open := strings.Index(rest, "```")
	if open < 0 {
		return ""
	}
	rest = rest[open+3:]
	end := strings.Index(rest, "```")
	if end < 0 {
		return ""
	}
	content := rest[:end]
	if nl := strings.Index(content, "\n"); nl >= 0 {
		first := strings.TrimSpace(content[:nl])
		if first != "" && !strings.Contains(first, " ") {
			content = content[nl+1:]
		}
	}
	return content
}

func executeTool(cwd string, call ToolCall) (string, error) {
	switch call.Name {
	case "read_file", "read":
		path := call.Args["path"]
		if path == "" {
			return "", fmt.Errorf("missing path")
		}
		full, err := safePath(cwd, path)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return truncateOutput(string(data)), nil
	case "run", "exec":
		cmdStr := call.Args["cmd"]
		if cmdStr == "" {
			return "", fmt.Errorf("missing cmd")
		}
		if !confirmRun(cmdStr) {
			return "(command declined by user)", nil
		}
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		result := truncateOutput(string(out))
		if err != nil {
			return result + "\nError: " + err.Error(), nil
		}
		return result, nil
	case "grep":
		pattern := call.Args["pattern"]
		path := call.Args["path"]
		if pattern == "" {
			return "", fmt.Errorf("missing pattern")
		}
		if path == "" {
			path = "."
		}
		searchDir, err := safePath(cwd, path)
		if err != nil {
			return "", err
		}
		cmd := exec.Command("grep", "-rn", pattern, searchDir)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return "(no matches)", nil
		}
		return truncateOutput(string(out)), nil
	case "write_file", "write":
		path := call.Args["path"]
		if path == "" {
			return "", fmt.Errorf("missing path")
		}
		full, err := safePath(cwd, path)
		if err != nil {
			return "", err
		}
		if call.Content == "" {
			return "", fmt.Errorf("write_file requires a fenced block with content after the TOOL line")
		}
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, []byte(call.Content), 0644); err != nil {
			return "", err
		}
		return fmt.Sprintf("Wrote %d bytes to %s", len(call.Content), path), nil
	case "diff":
		path := call.Args["path"]
		cmd := exec.Command("git", "diff", "--", path)
		if path == "" {
			cmd = exec.Command("git", "diff")
		}
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			return "(no diff or not a git repo)", nil
		}
		return truncateOutput(string(out)), nil
	case "apply":
		return "Apply is not a tool. To suggest changes, output a ```diff ... ``` block in your response; Koba will then ask the user to apply it. Do not output more TOOL lines for apply.", nil
	default:
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
}

// safePath resolves path relative to root and ensures it doesn't escape.
func safePath(root, path string) (string, error) {
	full := filepath.Join(root, path)
	abs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	absRoot, _ := filepath.Abs(root)
	if !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) && abs != absRoot {
		return "", fmt.Errorf("path %q escapes project root", path)
	}
	return abs, nil
}

const maxToolOutput = 32 * 1024 // 32KB

// truncateOutput limits tool output to maxToolOutput bytes.
func truncateOutput(s string) string {
	if len(s) <= maxToolOutput {
		return s
	}
	return s[:maxToolOutput] + "\n... (output truncated at 32KB)"
}

// confirmRun prompts the user before executing a shell command.
func confirmRun(cmd string) bool {
	fmt.Fprintf(os.Stderr, "\n%s⚠ Run command:%s %s\n", term.ColorYellow(), term.ColorReset(), cmd)
	fmt.Fprintf(os.Stderr, "Allow? [y/N] ")
	var input [64]byte
	n, err := os.Stdin.Read(input[:])
	if err != nil || n == 0 {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(string(input[:n])))
	return answer == "y" || answer == "yes"
}

// RunRun is the CLI entrypoint for `koba run`. It delegates to RunDo (structured tool calling).
func RunRun(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	args []string,
	modelOverride string,
) error {
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
	return RunDo(ctx, cfg, in, out, errOut, request, modelOverride, nil)
}
