package contextx

import (
	"bytes"
	"os/exec"
	"path/filepath"
)

// FindRepoRoot returns the git repository root for the given directory,
// or an empty string if the directory is not inside a git repository.
func FindRepoRoot(startDir string) (string, error) {
	cmd := exec.Command("git", "-C", startDir, "rev-parse", "--show-toplevel")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return filepath.Clean(out.String()), nil
}

// GitDiff returns the output of `git diff` for the repository at root.
func GitDiff(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "diff")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

