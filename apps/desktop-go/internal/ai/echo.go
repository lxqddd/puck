package ai

import (
	"context"
	"fmt"
	"time"
)

// EchoProvider 在没有配置真实模型时跑通完整链路。
type EchoProvider struct{}

func (p *EchoProvider) Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error {
	last := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == RoleUser {
			last = req.Messages[i].Content
			break
		}
	}

	reply := fmt.Sprintf(
		"我听见啦。你刚才说的是：「%s」。\n\n我现在还没有接上真实的大模型——去「设置」里填一个 OpenAI 兼容的地址、模型和 API Key，我就能正经回答你了。",
		last,
	)

	for _, chunk := range splitRunes(reply, 4) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(35 * time.Millisecond):
		}
		if err := onEvent(ChatEvent{Type: EventDelta, Text: chunk}); err != nil {
			return err
		}
	}
	return nil
}

func splitRunes(text string, size int) []string {
	runes := []rune(text)
	chunks := make([]string, 0, len(runes)/size+1)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
