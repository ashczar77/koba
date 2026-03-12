package app

import (
	"context"
	"io"
	"os"
	"strings"

	"koba/internal/config"
	"koba/internal/contextx"
	"koba/internal/provider"
)

// RunDo is the single entrypoint. It uses one agentic flow: no keyword routing.
// If messages is nil (e.g. CLI one-shot), a fresh conversation is started with the request.
// If messages is non-nil (e.g. session), the request is appended and RunAgent continues the conversation.
func RunDo(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	request string,
	modelOverride string,
	messages *[]provider.Message,
) error {
	request = strings.TrimSpace(request)
	if request == "" && (messages == nil || len(*messages) <= 1) {
		// No request and no ongoing conversation: start interactive chat (legacy chat loop).
		return RunChat(ctx, cfg, in, out, errOut, modelOverride, "", true)
	}

	cwd, _ := os.Getwd()
	repoRoot, _ := contextx.FindRepoRoot(".")
	if repoRoot == "" {
		repoRoot = cwd
	}
	systemPrompt := BuildAgentSystemPrompt(cwd, repoRoot)

	if messages == nil {
		// One-shot: start a new conversation.
		msgs := []provider.Message{
			{Role: provider.RoleSystem, Content: systemPrompt},
			{Role: provider.RoleUser, Content: request},
		}
		return RunAgent(ctx, cfg, in, out, errOut, modelOverride, &msgs)
	}

	// Session: append the new user message and continue.
	*messages = append(*messages, provider.Message{Role: provider.RoleUser, Content: request})
	return RunAgent(ctx, cfg, in, out, errOut, modelOverride, messages)
}
