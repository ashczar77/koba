package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const ollamaDefaultURL = "http://localhost:11434"

// OllamaClient implements Provider for local Ollama models.
type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	model      string
}

// NewOllamaClient constructs a new OllamaClient.
func NewOllamaClient(baseURL, model string) (*OllamaClient, error) {
	if baseURL == "" {
		baseURL = ollamaDefaultURL
	}
	if model == "" {
		model = "llama3.2"
	}
	return &OllamaClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
	}, nil
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string         `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

type ollamaStreamChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Chat performs a chat completion against the local Ollama API.
func (c *OllamaClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error) {
	var om []ollamaMessage
	for _, m := range messages {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		om = append(om, ollamaMessage{Role: role, Content: m.Content})
	}

	model := opts.Model
	if model == "" {
		model = c.model
	}

	reqBody := ollamaRequest{
		Model:    model,
		Messages: om,
		Stream:   false,
	}
	if opts.Temperature > 0 {
		reqBody.Options = map[string]any{"temperature": opts.Temperature}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %s: %s", resp.Status, string(b))
	}

	var ar ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}

	return newMemoryStream(ar.Message.Content), nil
}
