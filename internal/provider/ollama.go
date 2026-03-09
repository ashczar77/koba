package provider

import (
	"bufio"
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
	Stream   bool           `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type ollamaStreamChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Chat performs a chat completion against the local Ollama API with real streaming.
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
		Stream:   true,
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama API error: %s: %s", resp.Status, string(b))
	}

	return &ollamaStream{body: bufio.NewReader(resp.Body), resp: resp}, nil
}

// ollamaStream reads NDJSON from the response and yields content deltas.
type ollamaStream struct {
	body *bufio.Reader
	resp *http.Response
	done bool
}

func (s *ollamaStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.done {
		return StreamChunk{Done: true}, io.EOF
	}
	line, err := s.body.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			s.done = true
			return StreamChunk{Done: true}, io.EOF
		}
		return StreamChunk{}, err
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return s.Recv(ctx)
	}
	var chunk ollamaStreamChunk
	if err := json.Unmarshal(line, &chunk); err != nil {
		return StreamChunk{}, err
	}
	if chunk.Done {
		s.done = true
		return StreamChunk{Text: chunk.Message.Content, Done: true}, io.EOF
	}
	return StreamChunk{Text: chunk.Message.Content, Done: false}, nil
}

func (s *ollamaStream) Close() error {
	s.done = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
