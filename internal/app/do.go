package app

import (
	"context"
	"io"
	"strings"

	"koba/internal/config"
)

// Intent is the classified action for a user request.
type Intent string

const (
	IntentAsk    Intent = "ask"
	IntentCode   Intent = "code"
	IntentReview Intent = "review"
	IntentApply  Intent = "apply"
	IntentRun    Intent = "run"
)

// ClassifyIntent routes a request to the appropriate handler using heuristics.
func ClassifyIntent(request string) Intent {
	r := strings.ToLower(strings.TrimSpace(request))
	if r == "" {
		return IntentCode
	}

	// Question-like: explain, how, what, why
	if strings.HasPrefix(r, "explain ") || strings.HasPrefix(r, "how ") ||
		strings.HasPrefix(r, "what ") || strings.HasPrefix(r, "why ") ||
		strings.HasSuffix(r, "?") {
		return IntentAsk
	}

	// Review: explicit review request
	if strings.Contains(r, "review") {
		return IntentReview
	}

	// Apply: refactor, fix, add, change, modify, update, implement
	applyWords := []string{"refactor", "fix ", "add ", "change ", "modify", "update ", "implement"}
	for _, w := range applyWords {
		if strings.Contains(r, w) {
			return IntentApply
		}
	}

	// Run: find, list, search, grep, run, execute
	runWords := []string{"find ", "list ", "search ", "grep", " run ", "execute"}
	for _, w := range runWords {
		if strings.Contains(r, w) {
			return IntentRun
		}
	}

	// Default: code (repo-aware suggestions)
	return IntentCode
}

// RunDo is the single entrypoint: it classifies the request and dispatches
// to the appropriate handler (ask, code, review, apply, run).
func RunDo(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	request string,
	modelOverride string,
) error {
	request = strings.TrimSpace(request)
	if request == "" {
		// No request: start interactive chat
		return RunChat(ctx, cfg, in, out, errOut, modelOverride, "", true)
	}

	intent := ClassifyIntent(request)

	switch intent {
	case IntentAsk:
		return RunAsk(ctx, cfg, in, out, errOut, []string{request}, modelOverride, "")
	case IntentReview:
		return RunReview(ctx, cfg, in, out, errOut, modelOverride)
	case IntentApply:
		return RunApply(ctx, cfg, in, out, errOut, []string{request}, modelOverride, false, false, false)
	case IntentRun:
		return RunRun(ctx, cfg, in, out, errOut, []string{request}, modelOverride)
	case IntentCode:
		fallthrough
	default:
		return RunCode(ctx, cfg, in, out, errOut, []string{request}, modelOverride)
	}
}
