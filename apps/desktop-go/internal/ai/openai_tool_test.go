package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleToolLoop(t *testing.T) {
	requests := 0
	var secondBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		if requests == 1 {
			if !strings.Contains(string(body), `"tools"`) {
				t.Errorf("first request missing tools: %s", truncateForTest(string(body), 300))
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"web_search\",\"arguments\":\"{\\\"query\\\":\\\"杭州天气\\\"}\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		secondBody = string(body)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"今天杭州晴\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &OpenAICompatible{BaseURL: server.URL, APIKey: "test-key"}
	var reply strings.Builder
	var gotCall ToolCall
	err := provider.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "杭州天气"}},
		Tools: []ToolDefinition{{
			Name:        "web_search",
			Description: "search",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		}},
		ToolExecutor: func(ctx context.Context, call ToolCall) (ToolResult, error) {
			gotCall = call
			return ToolResult{CallID: call.ID, Name: call.Name, Content: "晴，25 摄氏度"}, nil
		},
	}, func(event ChatEvent) error {
		reply.WriteString(event.Text)
		return nil
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if gotCall.Name != "web_search" {
		t.Fatalf("tool name = %q", gotCall.Name)
	}
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(gotCall.Arguments, &args); err != nil || args.Query != "杭州天气" {
		t.Fatalf("tool args = %s, err = %v", gotCall.Arguments, err)
	}
	if reply.String() != "今天杭州晴" {
		t.Fatalf("reply = %q", reply.String())
	}
	if !strings.Contains(secondBody, `"role":"tool"`) || !strings.Contains(secondBody, "晴，25 摄氏度") {
		t.Fatalf("second request missing tool result: %s", truncateForTest(secondBody, 500))
	}
}

func truncateForTest(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
