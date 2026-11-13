package services

import (
	"fmt"
	"net/url"
	"strings"

	"puck/internal/store"
)

// validateProviderEndpoint prevents stored API keys from being sent to plain
// HTTP or malformed endpoints. Loopback HTTP is allowed for local runtimes.
func validateProviderEndpoint(provider store.ProviderConfig) error {
	providerType := strings.TrimSpace(provider.Type)
	if providerType == "" {
		providerType = store.ProviderTypeOpenAI
	}

	baseURL := strings.TrimSpace(provider.BaseURL)
	if baseURL == "" {
		if providerType == store.ProviderTypeOpenAI {
			return fmt.Errorf("请填写服务地址")
		}
		return nil
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("服务地址无效")
	}
	if parsed.User != nil {
		return fmt.Errorf("服务地址不能包含用户名或密码")
	}

	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		switch strings.ToLower(parsed.Hostname()) {
		case "localhost", "127.0.0.1", "::1":
			return nil
		}
	}

	return fmt.Errorf("服务地址必须使用 HTTPS（本地地址除外）")
}

func normalizeEndpoint(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func endpointChanged(existing *store.ProviderConfig, incoming store.ProviderConfig) bool {
	if existing == nil {
		return false
	}
	return strings.TrimSpace(existing.Type) != strings.TrimSpace(incoming.Type) ||
		normalizeEndpoint(existing.BaseURL) != normalizeEndpoint(incoming.BaseURL)
}

// resolveProviderRequest keeps a saved secret bound to its saved endpoint.
// A renderer-supplied key is never used with a saved key, and a saved key is
// never paired with a renderer-supplied BaseURL.
func resolveProviderRequest(
	provider store.ProviderConfig,
	saved *store.ProviderConfig,
	storedKey string,
	apiKey string,
) (store.ProviderConfig, string) {
	if key := strings.TrimSpace(apiKey); key != "" {
		return provider, key
	}
	if saved != nil {
		if key := strings.TrimSpace(storedKey); key != "" {
			return *saved, key
		}
	}
	return provider, ""
}
