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
		client = &http.Client{}
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
		return providerHTTPError(resp.StatusCode, string(body))
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

func providerHTTPError(status int, body string) error {
	body = strings.TrimSpace(body)
	switch status {
	case 401, 403:
		return fmt.Errorf("鉴权失败（HTTP %d）：API Key 无效或无权限，请检查后重新填写。服务返回：%s", status, body)
	case 404:
		return fmt.Errorf("接口或模型不存在（HTTP 404）：请检查服务地址与模型名。服务返回：%s", body)
	case 429:
		return fmt.Errorf("请求过于频繁或额度不足（HTTP 429）。服务返回：%s", body)
	default:
		return fmt.Errorf("服务返回 HTTP %d：%s", status, body)
	}
}
