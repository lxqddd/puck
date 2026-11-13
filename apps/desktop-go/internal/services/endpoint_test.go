package services

import (
	"testing"

	"puck/internal/store"
)

func TestValidateProviderEndpoint(t *testing.T) {
	cases := []struct {
		name    string
		config  store.ProviderConfig
		wantErr bool
	}{
		{
			name:   "https",
			config: store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "https://api.example.com/v1"},
		},
		{
			name:   "localhost http",
			config: store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://localhost:11434/v1"},
		},
		{
			name:   "loopback ipv4 http",
			config: store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://127.0.0.1:11434/v1"},
		},
		{
			name:   "loopback ipv6 http",
			config: store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://[::1]:11434/v1"},
		},
		{
			name:    "remote plain http",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://example.com/v1"},
			wantErr: true,
		},
		{
			name:    "lookalike localhost host",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "http://localhost.evil.com/v1"},
			wantErr: true,
		},
		{
			name:    "userinfo in endpoint",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "https://user:pass@example.com/v1"},
			wantErr: true,
		},
		{
			name:    "invalid url",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "://bad"},
			wantErr: true,
		},
		{
			name:    "openai empty url",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI},
			wantErr: true,
		},
		{
			name:   "anthropic optional empty url",
			config: store.ProviderConfig{Type: store.ProviderTypeAnthropic},
		},
		{
			name:    "unsupported scheme",
			config:  store.ProviderConfig{Type: store.ProviderTypeOpenAI, BaseURL: "ftp://example.com/v1"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateProviderEndpoint(tc.config)
			if tc.wantErr && err == nil {
				t.Fatal("expected endpoint validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected endpoint validation error: %v", err)
			}
		})
	}
}

func TestEndpointChanged(t *testing.T) {
	existing := &store.ProviderConfig{
		Type:    store.ProviderTypeOpenAI,
		BaseURL: "https://api.example.com/v1",
	}

	same := *existing
	if endpointChanged(existing, same) {
		t.Fatal("same endpoint should not require a new key")
	}

	trailingSlash := *existing
	trailingSlash.BaseURL += "/"
	if endpointChanged(existing, trailingSlash) {
		t.Fatal("trailing slash should not count as an endpoint change")
	}

	changedBaseURL := *existing
	changedBaseURL.BaseURL = "https://evil.example.com/v1"
	if !endpointChanged(existing, changedBaseURL) {
		t.Fatal("base URL change must require a new key")
	}

	changedType := *existing
	changedType.Type = store.ProviderTypeAnthropic
	if !endpointChanged(existing, changedType) {
		t.Fatal("provider type change must require a new key")
	}

	if endpointChanged(nil, same) {
		t.Fatal("new provider cannot have a stored key")
	}
}

func TestResolveProviderRequest(t *testing.T) {
	incoming := store.ProviderConfig{
		ID:      "p1",
		Type:    store.ProviderTypeOpenAI,
		BaseURL: "https://renderer.example.com/v1",
		Model:   "renderer-model",
	}
	saved := &store.ProviderConfig{
		ID:      "p1",
		Type:    store.ProviderTypeOpenAI,
		BaseURL: "https://saved.example.com/v1",
		Model:   "saved-model",
	}

	t.Run("explicit key uses renderer endpoint", func(t *testing.T) {
		got, key := resolveProviderRequest(incoming, saved, "stored-key", "new-key")
		if key != "new-key" || got.BaseURL != incoming.BaseURL {
			t.Fatalf("got provider=%+v key=%q", got, key)
		}
	})

	t.Run("stored key binds to saved endpoint", func(t *testing.T) {
		got, key := resolveProviderRequest(incoming, saved, "stored-key", "")
		if key != "stored-key" || got.BaseURL != saved.BaseURL {
			t.Fatalf("got provider=%+v key=%q", got, key)
		}
	})

	t.Run("saved provider without stored key keeps renderer endpoint", func(t *testing.T) {
		got, key := resolveProviderRequest(incoming, saved, "", "")
		if key != "" || got.BaseURL != incoming.BaseURL {
			t.Fatalf("got provider=%+v key=%q", got, key)
		}
	})

	t.Run("missing saved provider keeps renderer endpoint", func(t *testing.T) {
		got, key := resolveProviderRequest(incoming, nil, "", "")
		if key != "" || got.BaseURL != incoming.BaseURL {
			t.Fatalf("got provider=%+v key=%q", got, key)
		}
	})
}
