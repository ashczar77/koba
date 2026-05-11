package contextx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileLimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "hello world, this is a test file with some content"
	os.WriteFile(path, []byte(content), 0644)

	// Read full
	got, err := ReadFileLimited(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Errorf("expected full content, got %q", got)
	}

	// Read truncated
	got, err = ReadFileLimited(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestReadFileLimited_NotExist(t *testing.T) {
	_, err := ReadFileLimited("/nonexistent/path", 100)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFindRepoRoot(t *testing.T) {
	// This test runs inside the koba repo, so it should find a root
	root, err := FindRepoRoot(".")
	if err != nil {
		t.Skip("not in a git repo")
	}
	if root == "" {
		t.Error("expected non-empty repo root")
	}
	// Verify it contains go.mod
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Errorf("repo root %s doesn't contain go.mod", root)
	}
}
