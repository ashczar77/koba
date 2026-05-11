package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestFriendlyProvider(t *testing.T) {
	tests := []struct {
		err    error
		expect string
	}{
		{nil, ""},
		{fmt.Errorf("ANTHROPIC_API_KEY is not set"), "Anthropic API key is not set"},
		{fmt.Errorf("connection refused"), "Ollama is not running"},
		{fmt.Errorf("404 not found"), "Model not found"},
		{fmt.Errorf("credit balance too low"), "no credits"},
		{fmt.Errorf("something else"), "something else"},
	}
	for _, tt := range tests {
		got := FriendlyProvider(tt.err)
		if !strings.Contains(got, tt.expect) {
			t.Errorf("FriendlyProvider(%v) = %q, want contains %q", tt.err, got, tt.expect)
		}
	}
}

func TestFriendlyGit(t *testing.T) {
	tests := []struct {
		err    error
		expect string
	}{
		{nil, ""},
		{fmt.Errorf("not inside a git repository"), "Not inside a git repository"},
		{fmt.Errorf("uncommitted changes"), "uncommitted changes"},
		{fmt.Errorf("other error"), "other error"},
	}
	for _, tt := range tests {
		got := FriendlyGit(tt.err)
		if !strings.Contains(got, tt.expect) {
			t.Errorf("FriendlyGit(%v) = %q, want contains %q", tt.err, got, tt.expect)
		}
	}
}
