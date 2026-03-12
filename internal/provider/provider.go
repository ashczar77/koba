package provider

import "context"

// MessageRole represents the role of a message in the conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message is a single turn in a conversation.
// For tool results: Role=RoleTool, Content=result text, ToolCallID and/or ToolName set.
// For assistant messages that included tool calls: OptionalToolCalls holds them when re-sending to the API.
type Message struct {
	Role             MessageRole
	Content          string
	ToolCallID       string    // Anthropic: correlates with tool_use.id
	ToolName         string    // Ollama: tool_name for the result
	OptionalToolCalls []ToolCall `json:"-"` // set when Role=assistant and message had tool_calls (for next request)
}

// ToolDef describes a tool the model can call (JSON Schema style parameters).
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // "type":"object", "properties":{...}, "required":[...]
}

// ToolCall is a single tool invocation from the model.
type ToolCall struct {
	ID        string            `json:"id"`   // Anthropic tool_use id; Ollama may use index
	Name      string            `json:"name"`
	Arguments map[string]interface{} `json:"arguments"` // parsed from model output
}

// ChatOptions contains optional parameters for a chat call.
type ChatOptions struct {
	Temperature float32
	Model       string
	Stream      bool
	Tools       []ToolDef `json:"tools,omitempty"` // when set, model can return tool_calls
}

// StreamChunk represents a piece of streamed output.
type StreamChunk struct {
	Text      string     `json:"text"`
	Done      bool       `json:"done"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // set when stream delivers tool calls (e.g. at end)
}

// Stream is a pull-based streaming interface.
// After Recv returns a chunk with Done=true or EOF, ToolCalls() may return tool calls from the response.
type Stream interface {
	Recv(ctx context.Context) (StreamChunk, error)
	Close() error
	ToolCalls() []ToolCall // tool calls from the completed response (optional)
}

// Provider is an abstract interface over different model providers.
type Provider interface {
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (Stream, error)
}

