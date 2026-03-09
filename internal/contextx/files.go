package contextx

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadFileLimited reads up to maxBytes from the specified file path.
func ReadFileLimited(path string, maxBytes int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data), nil
}

// RecentShellHistory returns the last maxLines of shell history for context.
// Tries .zsh_history and .bash_history. Returns empty string if unavailable.
func RecentShellHistory(maxLines int) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{".zsh_history", ".bash_history"} {
		path := filepath.Join(home, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) == 0 {
			continue
		}
		start := 0
		if len(lines) > maxLines {
			start = len(lines) - maxLines
		}
		return "Recent shell commands:\n" + strings.Join(lines[start:], "\n")
	}
	return ""
}

