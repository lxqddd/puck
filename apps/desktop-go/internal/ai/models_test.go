package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListModelsOpenAICompatible(t *testing.T) {
	var gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-reasoner"},{"id":"deepseek-chat"},{"id":"deepseek-chat"}]}`))
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), "openai-compatible", server.URL+"/v1", " sk-test ")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	want := []string{"deepseek-chat", "deepseek-reasoner"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("path = %q, want /v1/models", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("authorization = %q", gotAuth)
	}
}

func TestListModelsOpenAICompatibleOllamaShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5"},{"name":"llama3.3"}]}`))
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), "openai-compatible", server.URL, "")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	want := []string{"llama3.3", "qwen2.5"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
}

func TestListModelsOpenAICompatibleAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key"}}`))
	}))
	defer server.Close()

	_, err := ListModels(context.Background(), "openai-compatible", server.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "鉴权失败") || !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want friendly auth error", err)
	}
}

func TestListModelsAnthropicPaginates(t *testing.T) {
	var afterIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		after := r.URL.Query().Get("after_id")
		afterIDs = append(afterIDs, after)
		if after == "" {
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet"},{"id":"claude-opus"}],"has_more":true,"last_id":"claude-opus"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-haiku"}],"has_more":false}`))
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), "anthropic", server.URL, "sk-ant")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	want := []string{"claude-haiku", "claude-opus", "claude-sonnet"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	if !reflect.DeepEqual(afterIDs, []string{"", "claude-opus"}) {
		t.Fatalf("afterIDs = %v", afterIDs)
	}
}

func TestListModelsGeminiFiltersAndPaginates(t *testing.T) {
	var pageTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "gm-key" {
			t.Errorf("x-goog-api-key = %q", r.Header.Get("x-goog-api-key"))
		}
		token := r.URL.Query().Get("pageToken")
		pageTokens = append(pageTokens, token)
		if token == "" {
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]},{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]}],"nextPageToken":"page-2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]}]}`))
	}))
	defer server.Close()

	models, err := ListModels(context.Background(), "gemini", server.URL, "gm-key")
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	want := []string{"gemini-2.5-flash", "gemini-2.5-pro"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %v, want %v", models, want)
	}
	if !reflect.DeepEqual(pageTokens, []string{"", "page-2"}) {
		t.Fatalf("pageTokens = %v", pageTokens)
	}
}

func TestListModelsRejectsCrossHostRedirect(t *testing.T) {
	targetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
		if r.Header.Get("x-api-key") != "" {
			t.Error("x-api-key leaked to redirect target")
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()

	_, err := ListModels(context.Background(), "anthropic", redirector.URL, "sk-ant-secret")
	if err == nil {
		t.Fatal("expected error for unfollowed cross-host redirect")
	}
	if targetHit {
		t.Fatal("followed cross-host redirect and leaked credentials")
	}
}

func TestListModelsRequiresKey(t *testing.T) {
	for _, kind := range []string{"anthropic", "gemini"} {
		_, err := ListModels(context.Background(), kind, "", "")
		if err == nil || !strings.Contains(err.Error(), "API Key") {
			t.Fatalf("%s: error = %v, want API Key error", kind, err)
		}
	}
}
