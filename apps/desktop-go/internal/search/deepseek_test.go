package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekSearch(t *testing.T) {
	var gotPath, gotAuth string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var payload struct {
			Query string `json:"query"`
			Count int    `json:"count"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &payload)
		gotQuery = payload.Query
		_, _ = w.Write([]byte(`{"results":[{"rerank_score":2.4,"title":"DeepSeek V4","url":"https://example.com/v4","snippet":"短摘要","summary":"长摘要内容","date":"2026-06-01T00:00:00+00:00","authority":3}]}`))
	}))
	defer server.Close()

	client := NewDeepSeekClient(server.URL, "sk-test")
	client.HTTP = server.Client()
	results, err := client.Search(context.Background(), "DeepSeek V4", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/v1/tools/search" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization = %q", gotAuth)
	}
	if gotQuery != "DeepSeek V4" {
		t.Fatalf("query = %q", gotQuery)
	}
	if len(results) != 1 || results[0].Content != "长摘要内容" || results[0].Published == "" {
		t.Fatalf("results = %#v", results)
	}
}

func TestDeepSeekEndpointWithV1(t *testing.T) {
	if got := deepSeekEndpoint("https://api.deepseek.com/v1", "/tools/search"); got != "https://api.deepseek.com/v1/tools/search" {
		t.Fatalf("endpoint = %q", got)
	}
	if got := deepSeekEndpoint("https://api.deepseek.com", "/tools/fetch"); got != "https://api.deepseek.com/v1/tools/fetch" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestDeepSeekFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/fetch" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"title":"文章标题","content":"# 正文\n\n内容"}`))
	}))
	defer server.Close()

	client := NewDeepSeekClient(server.URL, "sk-test")
	client.HTTP = server.Client()
	title, content, err := client.Fetch(context.Background(), "https://example.com/article")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if title != "文章标题" || !strings.Contains(content, "正文") {
		t.Fatalf("title=%q content=%q", title, content)
	}
}

func TestDeepSeekRejectsBadFetchURL(t *testing.T) {
	client := NewDeepSeekClient("https://api.deepseek.com", "sk-test")
	if _, _, err := client.Fetch(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("expected invalid URL error")
	}
	if _, _, err := client.Fetch(context.Background(), "https://user:pass@example.com"); err == nil {
		t.Fatal("expected credentials error")
	}
}
