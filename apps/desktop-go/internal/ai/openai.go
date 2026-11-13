package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
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

	messages := make([]map[string]any, 0, len(req.Messages)+1)
	if req.System != "" {
		messages = append(messages, map[string]any{"role": string(RoleSystem), "content": req.System})
	}
	for _, message := range req.Messages {
		messages = append(messages, map[string]any{"role": string(message.Role), "content": message.Content})
	}

	rounds := 1
	if len(req.Tools) > 0 && req.ToolExecutor != nil {
		rounds = 4
	}
	for round := 0; round < rounds; round++ {
		content, calls, err := p.streamOnce(ctx, client, req, messages, onEvent)
		if err != nil {
			return err
		}
		if len(calls) == 0 {
			return nil
		}
		if req.ToolExecutor == nil {
			return fmt.Errorf("模型请求调用工具，但当前供应商未启用工具")
		}

		toolCalls := make([]map[string]any, 0, len(calls))
		for _, call := range calls {
			toolCalls = append(toolCalls, map[string]any{
				"id":   call.ID,
				"type": "function",
				"function": map[string]any{
					"name":      call.Name,
					"arguments": string(call.Arguments),
				},
			})
		}
		assistantMessage := map[string]any{
			"role":       string(RoleAssistant),
			"tool_calls": toolCalls,
		}
		if content != "" {
			assistantMessage["content"] = content
		} else {
			assistantMessage["content"] = nil
		}
		messages = append(messages, assistantMessage)
		for _, call := range calls {
			result, err := req.ToolExecutor(ctx, call)
			if err != nil {
				result = ToolResult{CallID: call.ID, Name: call.Name, Content: err.Error(), IsError: true}
			}
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": call.ID,
				"content":      result.Content,
			})
		}
	}
	return fmt.Errorf("工具调用轮次超过上限")
}

func (p *OpenAICompatible) streamOnce(
	ctx context.Context,
	client *http.Client,
	req ChatRequest,
	messages []map[string]any,
	onEvent func(ChatEvent) error,
) (string, []ToolCall, error) {
	payload := map[string]any{
		"model":    req.Model,
		"messages": messages,
		"stream":   true,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = toolPayload(req.Tools)
		payload["tool_choice"] = "auto"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}

	endpoint := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, providerHTTPError(resp.StatusCode, string(responseBody))
	}

	type toolCallState struct {
		call ToolCall
		args strings.Builder
	}
	states := map[int]*toolCallState{}
	var content strings.Builder

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		if data == "" {
			continue
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string          `json:"content"`
					ReasoningContent string          `json:"reasoning_content"`
					Reasoning        json.RawMessage `json:"reasoning"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if reasoning := reasoningDelta(choice.Delta.ReasoningContent, choice.Delta.Reasoning); reasoning != "" {
				if err := onEvent(ChatEvent{Type: EventReasoning, Text: reasoning}); err != nil {
					return "", nil, err
				}
			}
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if err := onEvent(ChatEvent{Type: EventDelta, Text: choice.Delta.Content}); err != nil {
					return "", nil, err
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				state, ok := states[delta.Index]
				if !ok {
					state = &toolCallState{}
					states[delta.Index] = state
				}
				if delta.ID != "" {
					state.call.ID = delta.ID
				}
				if delta.Function.Name != "" {
					state.call.Name = delta.Function.Name
				}
				if delta.Function.Arguments != "" {
					state.args.WriteString(delta.Function.Arguments)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}

	indexes := make([]int, 0, len(states))
	for index := range states {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	calls := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		state := states[index]
		state.call.Arguments = json.RawMessage(state.args.String())
		if state.call.ID == "" {
			state.call.ID = fmt.Sprintf("call_%d", index)
		}
		if state.call.Name == "" {
			continue
		}
		calls = append(calls, state.call)
	}
	return content.String(), calls, nil
}

// reasoningDelta 归一化不同兼容端点的推理字段（DeepSeek 的 reasoning_content、
// OpenRouter 的 reasoning），返回本次增量文本。
func reasoningDelta(reasoningContent string, raw json.RawMessage) string {
	if reasoningContent != "" {
		return reasoningContent
	}
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return ""
	}
	return text
}

func toolPayload(tools []ToolDefinition) []map[string]any {
	payload := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		payload = append(payload, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  parameters,
			},
		})
	}
	return payload
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
