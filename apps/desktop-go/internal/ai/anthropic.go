package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

type AnthropicProvider struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error {
	model := strings.TrimSpace(p.Model)
	if model == "" {
		return fmt.Errorf("anthropic model is empty")
	}

	options := []option.RequestOption{}
	if p.APIKey != "" {
		options = append(options, option.WithAPIKey(p.APIKey))
	}
	if baseURL := strings.TrimSpace(p.BaseURL); baseURL != "" {
		options = append(options, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(options...)

	messages := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, message := range req.Messages {
		if message.Role == RoleAssistant {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(message.Content)))
			continue
		}
		messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(message.Content)))
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		Messages:  messages,
	}
	if system := strings.TrimSpace(req.System); system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	stream := client.Messages.NewStreaming(ctx, params)
	defer func() { _ = stream.Close() }()
	for stream.Next() {
		event := stream.Current()
		blockDelta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			continue
		}
		textDelta, ok := blockDelta.Delta.AsAny().(anthropic.TextDelta)
		if !ok || textDelta.Text == "" {
			continue
		}
		if err := onEvent(ChatEvent{Type: EventDelta, Text: textDelta.Text}); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic stream: %w", err)
	}
	return nil
}
