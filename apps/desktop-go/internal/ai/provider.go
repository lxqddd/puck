package ai

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string
	System   string
	Messages []Message
}

type EventType string

const (
	EventDelta EventType = "delta"
)

type ChatEvent struct {
	Type EventType
	Text string
}

// Provider 是各家模型的最小统一接口：只有流式对话。
type Provider interface {
	Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error
}
