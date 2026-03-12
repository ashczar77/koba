package provider

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// MockClient is a simple Provider implementation that does not call any
// external API. It is useful for local testing without API keys or credits.
type MockClient struct{}

// NewMockClient constructs a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// Chat generates a deterministic mock response based on the last user message.
func (m *MockClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error) {
	var lastUser string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			lastUser = messages[i].Content
			break
		}
	}

	if strings.TrimSpace(lastUser) == "" {
		lastUser = "(no user message detected)"
	}

	text := fmt.Sprintf(
		"(mock provider)\n\nYou said:\n%s\n\nThis is a mock response from Koba.\nNo real API calls were made.\n",
		lastUser,
	)

	return newSlowMemoryStream(text, 80*time.Millisecond), nil
}

// slowMemoryStream is like memoryStream but sleeps briefly between chunks to
// better simulate streaming behaviour.
type slowMemoryStream struct {
	text   string
	offset int
	closed bool
	delay  time.Duration
}

func newSlowMemoryStream(text string, delay time.Duration) Stream {
	return &slowMemoryStream{text: text, delay: delay}
}

func (s *slowMemoryStream) Recv(ctx context.Context) (StreamChunk, error) {
	if s.closed {
		return StreamChunk{Done: true}, io.EOF
	}
	if s.offset >= len(s.text) {
		s.closed = true
		return StreamChunk{Done: true}, io.EOF
	}

	select {
	case <-ctx.Done():
		return StreamChunk{}, ctx.Err()
	case <-time.After(s.delay):
	}

	const chunkSize = 64
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

func (s *slowMemoryStream) Close() error {
	s.closed = true
	return nil
}

func (s *slowMemoryStream) ToolCalls() []ToolCall {
	return nil
}

