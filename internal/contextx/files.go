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
// Tries .zsh_history and .bash_history. Only reads the tail of the file.
func RecentShellHistory(maxLines int) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{".zsh_history", ".bash_history"} {
		path := filepath.Join(home, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		// Read last 8KB to find recent lines without loading the whole file.
		const tailSize = 8 * 1024
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}
		offset := fi.Size() - tailSize
		if offset < 0 {
			offset = 0
		}
		buf := make([]byte, fi.Size()-offset)
		_, err = f.ReadAt(buf, offset)
		f.Close()
		if err != nil && err.Error() != "EOF" {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
		if len(lines) == 0 {
			continue
		}
		// Drop first line (likely partial if we seeked)
		if offset > 0 && len(lines) > 1 {
			lines = lines[1:]
		}
		start := 0
		if len(lines) > maxLines {
			start = len(lines) - maxLines
		}
		return "Recent shell commands:\n" + strings.Join(lines[start:], "\n")
	}
	return ""
}

