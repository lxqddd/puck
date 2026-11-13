package services

import (
	"fmt"
	"strings"

	"puck/internal/ai"
	"puck/internal/store"
)

// buildProvider 按供应商配置构造对应的流式适配器；key 由调用方负责读取。
func buildProvider(config store.ProviderConfig, key string) (ai.Provider, string, error) {
	providerType := strings.TrimSpace(config.Type)
	if providerType == "" {
		providerType = store.ProviderTypeOpenAI
	}
	model := strings.TrimSpace(config.Model)
	if model == "" {
		return nil, "", fmt.Errorf("请填写模型")
	}
	if err := validateProviderEndpoint(config); err != nil {
		return nil, "", err
	}

	key = strings.TrimSpace(key)
	switch providerType {
	case store.ProviderTypeAnthropic:
		if key == "" {
			return nil, "", fmt.Errorf("请配置 Anthropic API Key")
		}
		return &ai.AnthropicProvider{APIKey: key, BaseURL: config.BaseURL, Model: model}, model, nil
	case store.ProviderTypeGemini:
		if key == "" {
			return nil, "", fmt.Errorf("请配置 Gemini API Key")
		}
		return &ai.GeminiProvider{APIKey: key, BaseURL: config.BaseURL, Model: model}, model, nil
	default:
		if strings.TrimSpace(config.BaseURL) == "" {
			return nil, "", fmt.Errorf("请填写服务地址")
		}
		return &ai.OpenAICompatible{BaseURL: config.BaseURL, APIKey: key}, model, nil
	}
}
