package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"puck/internal/ai"
	"puck/internal/search"
	"puck/internal/store"
)

func TestWebSearchToolsRespectToggle(t *testing.T) {
	enabled := true
	disabled := false
	service := &ChatService{search: search.NewClient()}

	tools, executor := service.webSearchTools(&store.ProviderConfig{
		Type:      store.ProviderTypeOpenAI,
		WebSearch: &enabled,
	})
	if len(tools) != 1 || executor == nil {
		t.Fatalf("expected web search tool, got %d tools", len(tools))
	}
	if tools[0].Name != "web_search" {
		t.Fatalf("tool name = %q", tools[0].Name)
	}

	tools, _ = service.webSearchTools(&store.ProviderConfig{
		Type:      store.ProviderTypeOpenAI,
		WebSearch: &disabled,
	})
	if len(tools) != 0 {
		t.Fatalf("expected disabled search, got %d tools", len(tools))
	}

	tools, _ = service.webSearchTools(&store.ProviderConfig{Type: store.ProviderTypeAnthropic})
	if len(tools) != 0 {
		t.Fatalf("expected no tools for anthropic, got %d", len(tools))
	}
}

func TestWebSearchToolsUseDeepSeekBackend(t *testing.T) {
	service := &ChatService{search: search.NewClient()}
	tools, _ := service.webSearchTools(&store.ProviderConfig{
		ID:      "p-deepseek",
		Type:    store.ProviderTypeOpenAI,
		BaseURL: "https://api.deepseek.com/v1",
	})
	if len(tools) != 2 {
		t.Fatalf("deepseek tools = %d, want 2 (search + fetch)", len(tools))
	}
	if tools[0].Name != "web_search" || tools[1].Name != "web_fetch" {
		t.Fatalf("tools = %#v", tools)
	}
}

func TestIsDeepSeekProvider(t *testing.T) {
	cases := map[string]bool{
		"https://api.deepseek.com/v1":  true,
		"https://api.deepseek.com":     true,
		"https://api.deepseek.com/v1/": true,
		"https://proxy.example.com/v1": false,
		"https://api.openai.com/v1":    false,
	}
	for baseURL, want := range cases {
		config := &store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: baseURL}
		if got := isDeepSeekProvider(config); got != want {
			t.Fatalf("isDeepSeekProvider(%q) = %v, want %v", baseURL, got, want)
		}
	}
}

func TestWebSearchExecutorFormatsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"title":"杭州天气","url":"https://example.com/weather","content":"晴，25 度","published_date":"2026-11-12"}]}`))
	}))
	defer server.Close()

	enabled := true
	service := &ChatService{search: &search.Client{Endpoint: server.URL, HTTP: server.Client()}}
	_, executor := service.webSearchTools(&store.ProviderConfig{
		Type:      store.ProviderTypeOpenAI,
		WebSearch: &enabled,
	})
	result, err := executor(context.Background(), ai.ToolCall{
		ID:        "call-1",
		Name:      "web_search",
		Arguments: json.RawMessage(`{"query":"杭州天气"}`),
	})
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	for _, want := range []string{"杭州天气", "https://example.com/weather", "晴，25 度"} {
		if !strings.Contains(result.Content, want) {
			t.Fatalf("result missing %q: %s", want, result.Content)
		}
	}
}
