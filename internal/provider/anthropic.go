package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicClient implements Provider for Anthropic Claude models.
type AnthropicClient struct {
	httpClient *http.Client
	apiKey     string
	model      string
}

// NewAnthropicClient constructs a new AnthropicClient.
func NewAnthropicClient(apiKey, model string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}
	if model == "" {
		model = "claude-3-haiku-20240307"
	}
	return &AnthropicClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		apiKey: apiKey,
		model:  model,
	}, nil
}

// anthropicMessage mirrors the Anthropic messages API.
type anthropicMessage struct {
	Role    string                   `json:"role"`
	Content []anthropicContentBlock  `json:"content"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float32             `json:"temperature,omitempty"`
	Messages    []anthropicMessage  `json:"messages"`
	Stream      bool                `json:"stream"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// Chat performs a chat completion call. For simplicity, this implementation
// uses Anthropic's non-streaming API and then exposes the result via the
// Stream interface in chunks.
func (c *AnthropicClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error) {
	reqBody, err := c.buildRequestBody(messages, opts)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	// Version string chosen to match current Claude messages API.
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error: %s: %s", resp.Status, string(b))
	}

	var ar anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}

	var fullText string
	for _, block := range ar.Content {
		if block.Type == "text" {
			fullText += block.Text
		}
	}

	return newMemoryStream(fullText), nil
}

func (c *AnthropicClient) buildRequestBody(messages []Message, opts ChatOptions) ([]byte, error) {
	var am []anthropicMessage
	for _, m := range messages {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		am = append(am, anthropicMessage{
			Role: role,
			Content: []anthropicContentBlock{
				{
					Type: "text",
					Text: m.Content,
				},
			},
		})
	}

	temp := opts.Temperature
	if temp == 0 {
		temp = 0.2
	}

	model := opts.Model
	if model == "" {
		model = c.model
	}

	req := anthropicRequest{
		Model:       model,
		MaxTokens:   2048,
		Temperature: temp,
		Messages:    am,
		Stream:      false,
	}

	return json.Marshal(&req)
}

// memoryStream is a simple in-memory implementation of Stream that
// yields the response text in fixed-size chunks.
type memoryStream struct {
	text   string
	offset int
	closed bool
}

func newMemoryStream(text string) Stream {
	return &memoryStream{text: text}
}

func (s *memoryStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.closed {
		return StreamChunk{Done: true}, io.EOF
	}
	if s.offset >= len(s.text) {
		s.closed = true
		return StreamChunk{Done: true}, io.EOF
	}

	const chunkSize = 512
	end := s.offset + chunkSize
	if end > len(s.text) {
		end = len(s.text)
	}
	chunk := s.text[s.offset:end]
	s.offset = end

	return StreamChunk{
		Text: chunk,
		Done: s.offset >= len(s.text),
	}, nil
}

func (s *memoryStream) Close() error {
	s.closed = true
	return nil
}

