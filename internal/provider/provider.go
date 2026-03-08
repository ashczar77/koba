package provider

import "context"

// MessageRole represents the role of a message in the conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    MessageRole
	Content string
}

// ChatOptions contains optional parameters for a chat call.
type ChatOptions struct {
	Temperature float32
	Model       string
	Stream      bool
}

// StreamChunk represents a piece of streamed output.
type StreamChunk struct {
	Text string
	Done bool
}

// Stream is a pull-based streaming interface.
type Stream interface {
	Recv(ctx context.Context) (StreamChunk, error)
	Close() error
}

// Provider is an abstract interface over different model providers.
type Provider interface {
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error)
}

