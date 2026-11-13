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

	// Claude 模型默认开启扩展思考，旧模型不支持时回退到普通对话。
	withThinking := strings.HasPrefix(strings.ToLower(model), "claude")
	if withThinking {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 2048,
				Display:      anthropic.ThinkingConfigEnabledDisplaySummarized,
			},
		}
	}
	err := p.stream(ctx, params, onEvent)
	if err != nil && withThinking && isThinkingUnsupported(err) {
		params.Thinking = anthropic.ThinkingConfigParamUnion{}
		return p.stream(ctx, params, onEvent)
	}
	return err
}

func (p *AnthropicProvider) stream(
	ctx context.Context,
	params anthropic.MessageNewParams,
	onEvent func(ChatEvent) error,
) error {
	options := []option.RequestOption{}
	if p.APIKey != "" {
		options = append(options, option.WithAPIKey(p.APIKey))
	}
	if baseURL := strings.TrimSpace(p.BaseURL); baseURL != "" {
		options = append(options, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(options...)

	stream := client.Messages.NewStreaming(ctx, params)
	defer func() { _ = stream.Close() }()
	for stream.Next() {
		event := stream.Current()
		blockDelta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			continue
		}
		if thinkingDelta, ok := blockDelta.Delta.AsAny().(anthropic.ThinkingDelta); ok {
			if thinkingDelta.Thinking != "" {
				if err := onEvent(ChatEvent{Type: EventReasoning, Text: thinkingDelta.Thinking}); err != nil {
					return err
				}
			}
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

func isThinkingUnsupported(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "thinking") &&
		(strings.Contains(message, "not supported") ||
			strings.Contains(message, "unsupported") ||
			strings.Contains(message, "invalid"))
}
