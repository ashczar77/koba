package contextx

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
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
	return filepath.Clean(strings.TrimSpace(out.String())), nil
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

// GitStatusClean returns true if the working tree has no uncommitted changes.
func GitStatusClean(root string) (bool, error) {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return strings.TrimSpace(out.String()) == "", nil
}

