package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"koba/internal/config"
	"koba/internal/errors"
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
	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	w := bufio.NewWriter(out)
	defer w.Flush()

	providerName := providerNameFromEnv(cfg)
	mode := "LIVE"
	switch providerName {
	case "mock":
		mode = "MOCK"
	case "ollama":
		mode = "LOCAL"
	}
	banner := term.Banner(strings.ToUpper(providerName), chooseModel(cfg, modelOverride), mode)
	fmt.Fprintln(w, banner)

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

		stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
		streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
			Model:       modelOverride,
			Temperature: cfg.Temperature,
			Stream:      stream,
		})
		stopSpinner()
		if err != nil {
			fmt.Fprintln(errOut, errors.FriendlyProvider(err))
			continue
		}

		fmt.Fprint(w, term.AssistantPrefix())
		w.Flush()

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

	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
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

	stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
	streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:       modelOverride,
		Temperature: cfg.Temperature,
		Stream:      true,
	})
	stopSpinner()
	if err != nil {
		return err
	}
	defer streamObj.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	var resp strings.Builder
	for {
		chunk, err := streamObj.Recv(ctx)
		if err != nil {
			if err != io.EOF {
				fmt.Fprintln(errOut, "stream error:", err)
			}
			break
		}
		if chunk.Text != "" {
			resp.WriteString(chunk.Text)
		}
		if chunk.Done {
			break
		}
	}

	fmt.Fprint(w, term.AssistantPrefix())
	fmt.Fprint(w, term.FormatResponse(resp.String()))
	w.Flush()

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

// newProviderClient chooses which provider to use based on environment and
// config. Env var KOBA_PROVIDER takes precedence over config.DefaultProvider.
// Supported values: "anthropic", "ollama", "mock".
func newProviderClient(cfg config.Config, modelOverride string) (provider.Provider, error) {
	name := providerNameFromEnv(cfg)
	switch name {
	case "mock":
		return provider.NewMockClient(), nil
	case "ollama":
		baseURL := cfg.OllamaBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := chooseModel(cfg, modelOverride)
		if strings.Contains(model, "claude") {
			model = "codellama"
		}
		return provider.NewOllamaClient(baseURL, model)
	case "anthropic", "":
		fallthrough
	default:
		return provider.NewAnthropicClient(cfg.AnthropicAPIKey, chooseModel(cfg, modelOverride))
	}
}

func providerNameFromEnv(cfg config.Config) string {
	name := os.Getenv("KOBA_PROVIDER")
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(cfg.DefaultProvider))
	}
	if name == "" {
		name = "anthropic"
	}
	return name
}




