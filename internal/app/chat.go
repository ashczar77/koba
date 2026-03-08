package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"koba/internal/config"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunChat implements an interactive chat loop with the configured provider.
func RunChat(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	modelOverride string,
	systemPrompt string,
	stream bool,
) error {
	client, err := provider.NewAnthropicClient(cfg.AnthropicAPIKey, chooseModel(cfg, modelOverride))
	if err != nil {
		fmt.Fprintln(errOut, "provider error:", err)
		return err
	}

	w := bufio.NewWriter(out)
	defer w.Flush()

	fmt.Fprintln(w, "koba chat - interactive mode (Ctrl+D to exit)")

	var messages []provider.Message
	if systemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		})
	}

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(w, term.UserPrefix())
		w.Flush()

		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: line,
		})

		fmt.Fprint(w, term.AssistantPrefix())
		w.Flush()

		streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
			Model:       modelOverride,
			Temperature: cfg.Temperature,
			Stream:      stream,
		})
		if err != nil {
			fmt.Fprintln(errOut, "chat error:", err)
			continue
		}

		var respText strings.Builder
		for {
			chunk, err := streamObj.Recv(ctx)
			if err != nil {
				if err != io.EOF {
					fmt.Fprintln(errOut, "stream error:", err)
				}
				break
			}
			if chunk.Text != "" {
				respText.WriteString(chunk.Text)
				fmt.Fprint(w, chunk.Text)
				w.Flush()
			}
			if chunk.Done {
				break
			}
		}
		_ = streamObj.Close()
		fmt.Fprintln(w)

		messages = append(messages, provider.Message{
			Role:    provider.RoleAssistant,
			Content: respText.String(),
		})
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintln(errOut, "input error:", err)
	}

	return nil
}

// RunAsk performs a single-turn question and prints the answer, then exits.
func RunAsk(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	args []string,
	modelOverride string,
	systemPrompt string,
) error {
	question := strings.TrimSpace(strings.Join(args, " "))
	if question == "" {
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		question = strings.TrimSpace(string(data))
	}
	if question == "" {
		return fmt.Errorf("no question provided")
	}

	client, err := provider.NewAnthropicClient(cfg.AnthropicAPIKey, chooseModel(cfg, modelOverride))
	if err != nil {
		fmt.Fprintln(errOut, "provider error:", err)
		return err
	}

	var messages []provider.Message
	if systemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		})
	}
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: question,
	})

	streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:       modelOverride,
		Temperature: cfg.Temperature,
		Stream:      true,
	})
	if err != nil {
		return err
	}
	defer streamObj.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	for {
		chunk, err := streamObj.Recv(ctx)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(errOut, "stream error:", err)
			}
			break
		}
		if chunk.Text != "" {
			fmt.Fprint(w, chunk.Text)
			w.Flush()
		}
		if chunk.Done {
			break
		}
	}
	fmt.Fprintln(w)

	return nil
}

func chooseModel(cfg config.Config, override string) string {
	if override != "" {
		return override
	}
	if cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return "claude-3-haiku-20240307"
}


