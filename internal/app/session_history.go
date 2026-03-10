package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const sessionsDirName = ".koba/sessions"

// SessionsDir returns the directory for session logs (~/.koba/sessions).
func SessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, sessionsDirName), nil
}

// EnsureSessionsDir creates ~/.koba/sessions if it does not exist.
func EnsureSessionsDir() (string, error) {
	dir, err := SessionsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create sessions dir: %w", err)
	}
	return dir, nil
}

// StartSessionLog creates a new timestamped session log file and returns its path
// and the file. Caller must close the file when done.
func StartSessionLog() (path string, f *os.File, err error) {
	dir, err := EnsureSessionsDir()
	if err != nil {
		return "", nil, err
	}
	name := time.Now().UTC().Format("2006-01-02T15-04-05") + ".log"
	path = filepath.Join(dir, name)
	f, err = os.Create(path)
	if err != nil {
		return "", nil, fmt.Errorf("create session log: %w", err)
	}
	return path, f, nil
}

// ListSessions returns paths of session log files, newest first. Limit 0 means no limit.
func ListSessions(limit int) ([]string, error) {
	dir, err := SessionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".log" {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	sort.Slice(paths, func(i, j int) bool {
		infoI, _ := os.Stat(paths[i])
		infoJ, _ := os.Stat(paths[j])
		if infoI == nil || infoJ == nil {
			return paths[i] > paths[j]
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	return paths, nil
}

// ShowSession writes the contents of the session file at path to w.
func ShowSession(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// RunHistory lists recent session logs or shows one by index (0 = most recent).
// listLimit used when showIndex < 0; pass 0 for no limit.
func RunHistory(out, errOut io.Writer, listLimit int, showIndex int) error {
	if showIndex >= 0 {
		sessions, err := ListSessions(0)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return err
		}
		if showIndex >= len(sessions) {
			fmt.Fprintf(errOut, "no session at index %d (have %d)\n", showIndex, len(sessions))
			return nil
		}
		return ShowSession(sessions[showIndex], out)
	}

	sessions, err := ListSessions(listLimit)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return err
	}
	if len(sessions) == 0 {
		fmt.Fprintln(out, "No session history yet. Run `koba` and chat to create one.")
		return nil
	}
	for i, path := range sessions {
		info, _ := os.Stat(path)
		mtime := ""
		if info != nil {
			mtime = info.ModTime().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(out, "%d  %s  %s\n", i, mtime, filepath.Base(path))
	}
	fmt.Fprintf(out, "\nShow one: koba history <index>\n")
	return nil
}
