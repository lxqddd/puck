package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTavilyKeylessSearch(t *testing.T) {
	var gotKeyless string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKeyless = r.Header.Get("X-Tavily-Access-Mode")
		var payload struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotQuery = payload.Query
		_, _ = w.Write([]byte(`{"results":[{"title":"结果一","url":"https://example.com/a","content":"内容一","published_date":"2026-11-12"},{"title":"结果二","url":"https://example.com/b","content":"内容二"}]}`))
	}))
	defer server.Close()

	client := &Client{Endpoint: server.URL, HTTP: server.Client()}
	results, err := client.Search(context.Background(), "杭州天气", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotKeyless != "keyless" {
		t.Fatalf("keyless header = %q", gotKeyless)
	}
	if gotQuery != "杭州天气" {
		t.Fatalf("query = %q", gotQuery)
	}
	if len(results) != 2 || results[0].URL != "https://example.com/a" {
		t.Fatalf("results = %#v", results)
	}
}

func TestTavilyUsesBearerKey(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	client := &Client{APIKey: "tvly-test", Endpoint: server.URL, HTTP: server.Client()}
	if _, err := client.Search(context.Background(), "test", 3); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if auth != "Bearer tvly-test" {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestTavilyRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
	}))
	defer server.Close()

	client := &Client{Endpoint: server.URL, HTTP: server.Client()}
	_, err := client.Search(context.Background(), "test", 3)
	if err == nil || !strings.Contains(err.Error(), "限流") {
		t.Fatalf("error = %v", err)
	}
}

func TestTavilyCooldownAfterRateLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
	}))
	defer server.Close()

	client := &Client{Endpoint: server.URL, HTTP: server.Client()}
	if _, err := client.Search(context.Background(), "test", 3); err == nil {
		t.Fatal("expected rate limit error")
	}
	if _, err := client.Search(context.Background(), "test", 3); err == nil || !strings.Contains(err.Error(), "冷却") {
		t.Fatalf("second error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (cooldown should skip network)", requests)
	}
}
