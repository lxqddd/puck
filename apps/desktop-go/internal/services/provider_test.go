package services

import (
	"testing"

	"puck/internal/store"
)

func TestBuildProviderValidation(t *testing.T) {
	cases := []struct {
		name    string
		config  store.ProviderConfig
		key     string
		wantErr bool
	}{
		{
			name:    "missing model",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "https://example.com/v1"},
			wantErr: true,
		},
		{
			name:    "openai compatible missing base url",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, Model: "gpt"},
			wantErr: true,
		},
		{
			name:   "openai compatible with base url and no key",
			config: store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://localhost:11434/v1", Model: "llama3"},
		},
		{
			name:    "openai compatible rejects remote plain http",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://example.com/v1", Model: "gpt"},
			wantErr: true,
		},
		{
			name:    "anthropic missing key",
			config:  store.ProviderConfig{Type: store.ProviderTypeAnthropic, Model: "claude-opus-5-5"},
			wantErr: true,
		},
		{
			name:   "anthropic with key",
			config: store.ProviderConfig{Type: store.ProviderTypeAnthropic, Model: "claude-opus-5-5"},
			key:    "test-key",
		},
		{
			name:    "gemini missing key",
			config:  store.ProviderConfig{Type: store.ProviderTypeGemini, Model: "gemini-3.8-flash"},
			wantErr: true,
		},
		{
			name:   "gemini with key",
			config: store.ProviderConfig{Type: store.ProviderTypeGemini, Model: "gemini-3.8-flash"},
			key:    "test-key",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider, model, err := buildProvider(tc.config, tc.key)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got provider=%v model=%q", provider, model)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider == nil || model != tc.config.Model {
				t.Fatalf("unexpected provider=%v model=%q", provider, model)
			}
		})
	}
}
