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
	Role    string                 `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type       string                 `json:"type"`
	Text       string                 `json:"text,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Input      map[string]interface{} `json:"input,omitempty"`
	ToolUseID  string                 `json:"tool_use_id,omitempty"`
	Content    string                 `json:"content,omitempty"`
}

type anthropicToolDef struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
}

type anthropicRequest struct {
	Model       string               `json:"model"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature float32              `json:"temperature,omitempty"`
	Messages    []anthropicMessage   `json:"messages"`
	Tools       []anthropicToolDef   `json:"tools,omitempty"`
	Stream      bool                 `json:"stream"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
}

// Chat performs a streaming chat completion call. When opts.Tools is set, uses non-streaming to get tool_calls.
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
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("anthropic API error: %s: %s", resp.Status, string(b))
	}

	if len(opts.Tools) > 0 {
		return c.readNonStreamResponse(resp)
	}
	return newAnthropicStream(resp), nil
}

func (c *AnthropicClient) buildRequestBody(messages []Message, opts ChatOptions) ([]byte, error) {
	var am []anthropicMessage
	for _, m := range messages {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		switch m.Role {
		case RoleTool:
			am = append(am, anthropicMessage{
				Role: "user",
				Content: []anthropicContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
		case RoleAssistant:
			var blocks []anthropicContentBlock
			if m.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.OptionalToolCalls {
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Arguments,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: ""})
			}
			am = append(am, anthropicMessage{Role: "assistant", Content: blocks})
		default:
			am = append(am, anthropicMessage{
				Role: role,
				Content: []anthropicContentBlock{{Type: "text", Text: m.Content}},
			})
		}
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
		Stream:      len(opts.Tools) == 0,
	}
	if len(opts.Tools) > 0 {
		for _, t := range opts.Tools {
			schema := t.Parameters
			if schema == nil {
				schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
			}
			req.Tools = append(req.Tools, anthropicToolDef{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: schema,
			})
		}
	}

	return json.Marshal(&req)
}

func (c *AnthropicClient) readNonStreamResponse(resp *http.Response) (Stream, error) {
	defer resp.Body.Close()
	var full anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&full); err != nil {
		return nil, fmt.Errorf("anthropic response decode: %w", err)
	}
	var text strings.Builder
	var toolCalls []ToolCall
	for _, block := range full.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			args := block.Input
			if args == nil {
				args = make(map[string]interface{})
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}
	return &anthropicToolStream{content: text.String(), toolCalls: toolCalls, done: false}, nil
}

type anthropicToolStream struct {
	content   string
	toolCalls []ToolCall
	done      bool
}

func (s *anthropicToolStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.done {
		return StreamChunk{Done: true}, io.EOF
	}
	s.done = true
	return StreamChunk{Text: s.content, Done: true}, io.EOF
}

func (s *anthropicToolStream) Close() error {
	s.done = true
	return nil
}

func (s *anthropicToolStream) ToolCalls() []ToolCall {
	return s.toolCalls
}

// anthropicStream parses SSE from the messages API and yields text_delta chunks.
type anthropicStream struct {
	body *bufio.Reader
	resp *http.Response
	done bool
}

// contentBlockDeltaEvent matches Anthropic SSE content_block_delta data.
type contentBlockDeltaEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func newAnthropicStream(resp *http.Response) Stream {
	return &anthropicStream{body: bufio.NewReader(resp.Body), resp: resp}
}

func (s *anthropicStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.done {
		return StreamChunk{Done: true}, io.EOF
	}
	var eventType string
	for {
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
			continue
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = strings.TrimSpace(string(line[6:]))
			continue
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(line[5:])
			if len(data) == 0 {
				continue
			}
			if eventType == "message_stop" {
				s.done = true
				return StreamChunk{Done: true}, io.EOF
			}
			if eventType == "error" {
				var errData struct {
					Type  string `json:"type"`
					Error struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					} `json:"error"`
				}
				_ = json.Unmarshal(data, &errData)
				s.done = true
				return StreamChunk{}, fmt.Errorf("anthropic stream error: %s", errData.Error.Message)
			}
			if eventType == "content_block_delta" {
				var ev contentBlockDeltaEvent
				if err := json.Unmarshal(data, &ev); err != nil {
					continue
				}
				if ev.Delta.Type == "text_delta" && ev.Delta.Text != "" {
					return StreamChunk{Text: ev.Delta.Text, Done: false}, nil
				}
			}
		}
	}
}

func (s *anthropicStream) Close() error {
	s.done = true
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}

func (s *anthropicStream) ToolCalls() []ToolCall {
	return nil
}

