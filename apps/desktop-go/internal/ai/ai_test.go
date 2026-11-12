package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEchoProviderStreamsReply(t *testing.T) {
	provider := &EchoProvider{}
	var chunks []string
	err := provider.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "你好"}},
	}, func(event ChatEvent) error {
		chunks = append(chunks, event.Text)
		return nil
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple streamed chunks, got %d", len(chunks))
	}
	reply := strings.Join(chunks, "")
	if !strings.Contains(reply, "你好") {
		t.Fatalf("reply does not reference the user message: %q", reply)
	}
}

func TestEchoProviderHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (&EchoProvider{}).Chat(ctx, ChatRequest{
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	}, func(ChatEvent) error { return nil })
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestOpenAICompatibleStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer token")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider := &OpenAICompatible{BaseURL: server.URL, APIKey: "test-key"}
	var reply strings.Builder
	err := provider.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	}, func(event ChatEvent) error {
		reply.WriteString(event.Text)
		return nil
	})
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if reply.String() != "你好" {
		t.Fatalf("unexpected reply: %q", reply.String())
	}
}

func TestOpenAICompatibleHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := &OpenAICompatible{BaseURL: server.URL}
	err := provider.Chat(context.Background(), ChatRequest{}, func(ChatEvent) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}
