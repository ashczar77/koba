package provider

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestMockClient_Chat(t *testing.T) {
	client := NewMockClient()
	msgs := []Message{
		{Role: RoleUser, Content: "hello world"},
	}
	stream, err := client.Chat(context.Background(), msgs, ChatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var result strings.Builder
	for {
		chunk, err := stream.Recv(context.Background())
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		result.WriteString(chunk.Text)
		if chunk.Done {
			break
		}
	}

	if !strings.Contains(result.String(), "hello world") {
		t.Errorf("expected response to contain user message, got: %s", result.String())
	}
	if !strings.Contains(result.String(), "mock") {
		t.Errorf("expected response to mention mock, got: %s", result.String())
	}
}

func TestMockClient_ToolCallsNil(t *testing.T) {
	client := NewMockClient()
	stream, _ := client.Chat(context.Background(), []Message{{Role: RoleUser, Content: "test"}}, ChatOptions{})
	if tc := stream.ToolCalls(); tc != nil {
		t.Errorf("expected nil tool calls, got %v", tc)
	}
}
