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
	Role      string          `json:"role"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string          `json:"tool_name,omitempty"`
}

type ollamaToolCall struct {
	Type     string `json:"type"`
	Function struct {
		Index      int                    `json:"index,omitempty"`
		Name       string                 `json:"name"`
		Arguments  map[string]interface{} `json:"arguments,omitempty"`
	} `json:"function"`
}

type ollamaToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []ollamaToolDef `json:"tools,omitempty"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaStreamChunk struct {
	Message struct {
		Content   string          `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
		Done      bool            `json:"done"`
	} `json:"message"`
	Done bool `json:"done"`
}

// Chat performs a chat completion against the local Ollama API.
// When opts.Tools is set, uses a non-streaming request to get tool_calls and returns a stream that yields content once.
func (c *OllamaClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error) {
	om, err := c.convertMessages(messages)
	if err != nil {
		return nil, err
	}

	model := opts.Model
	if model == "" {
		model = c.model
	}

	reqBody := ollamaRequest{
		Model:    model,
		Messages: om,
		Stream:   len(opts.Tools) == 0,
	}
	if opts.Temperature > 0 {
		reqBody.Options = map[string]any{"temperature": opts.Temperature}
	}
	if len(opts.Tools) > 0 {
		reqBody.Tools = c.convertTools(opts.Tools)
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

	if len(opts.Tools) > 0 {
		return c.readNonStreamResponse(resp)
	}
	return &ollamaStream{body: bufio.NewReader(resp.Body), resp: resp}, nil
}

func (c *OllamaClient) convertMessages(messages []Message) ([]ollamaMessage, error) {
	var om []ollamaMessage
	for _, m := range messages {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		switch m.Role {
		case RoleTool:
			om = append(om, ollamaMessage{Role: "tool", ToolName: m.ToolName, Content: m.Content})
		case RoleAssistant:
			msg := ollamaMessage{Role: "assistant", Content: m.Content}
			if len(m.OptionalToolCalls) > 0 {
				for i, tc := range m.OptionalToolCalls {
					msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
						Type: "function",
						Function: struct {
							Index      int                    `json:"index,omitempty"`
							Name       string                 `json:"name"`
							Arguments  map[string]interface{} `json:"arguments,omitempty"`
						}{Index: i, Name: tc.Name, Arguments: tc.Arguments},
					})
				}
			}
			om = append(om, msg)
		default:
			om = append(om, ollamaMessage{Role: role, Content: m.Content})
		}
	}
	return om, nil
}

func (c *OllamaClient) convertTools(tools []ToolDef) []ollamaToolDef {
	var out []ollamaToolDef
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		out = append(out, ollamaToolDef{
			Type: "function",
			Function: struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters"`
			}{Name: t.Name, Description: t.Description, Parameters: params},
		})
	}
	return out
}

// ollamaToolResponse is the full message in a non-streaming response.
type ollamaToolResponse struct {
	Message struct {
		Content   string          `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
}

func (c *OllamaClient) readNonStreamResponse(resp *http.Response) (Stream, error) {
	defer resp.Body.Close()
	var full ollamaToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&full); err != nil {
		return nil, fmt.Errorf("ollama response decode: %w", err)
	}
	var toolCalls []ToolCall
	for _, oc := range full.Message.ToolCalls {
		args := oc.Function.Arguments
		if args == nil {
			args = make(map[string]interface{})
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:        fmt.Sprintf("%d", oc.Function.Index),
			Name:      oc.Function.Name,
			Arguments: args,
		})
	}
	return &ollamaToolStream{content: full.Message.Content, toolCalls: toolCalls, done: false}, nil
}

// ollamaToolStream yields content once then Done; ToolCalls() returns the parsed tool calls.
type ollamaToolStream struct {
	content   string
	toolCalls []ToolCall
	done      bool
}

func (s *ollamaToolStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.done {
		return StreamChunk{Done: true}, io.EOF
	}
	s.done = true
	return StreamChunk{Text: s.content, Done: true}, io.EOF
}

func (s *ollamaToolStream) Close() error {
	s.done = true
	return nil
}

func (s *ollamaToolStream) ToolCalls() []ToolCall {
	return s.toolCalls
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

func (s *ollamaStream) ToolCalls() []ToolCall {
	return nil
}
