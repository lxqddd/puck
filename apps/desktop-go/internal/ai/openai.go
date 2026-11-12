package ai

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

// OpenAICompatible 覆盖 OpenAI、DeepSeek、通义、Kimi、智谱、Ollama 等兼容端点。
type OpenAICompatible struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (p *OpenAICompatible) Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error {
	if strings.TrimSpace(p.BaseURL) == "" {
		return fmt.Errorf("provider base URL is empty")
	}
	client := p.HTTP
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}

	messages := make([]Message, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, Message{Role: RoleSystem, Content: req.System})
	}
	messages = append(messages, req.Messages...)

	payload, err := json.Marshal(map[string]any{
		"model":    req.Model,
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return err
	}

	endpoint := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		if data == "" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onEvent(ChatEvent{Type: EventDelta, Text: choice.Delta.Content}); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
