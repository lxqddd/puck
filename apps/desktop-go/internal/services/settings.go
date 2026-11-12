package services

import (
	"fmt"
	"strings"

	"puck/internal/secure"
	"puck/internal/store"
)

type SettingsService struct {
	store *store.SettingsStore
	keys  *secure.Keys
}

func NewSettings(s *store.SettingsStore, k *secure.Keys) *SettingsService {
	return &SettingsService{store: s, keys: k}
}

func (s *SettingsService) Get() (store.Settings, error) {
	settings, err := s.store.Load()
	if err != nil {
		return settings, err
	}
	for _, provider := range settings.Providers {
		provider.HasKey = s.keys.Has(provider.ID)
	}
	return settings, nil
}

func (s *SettingsService) SaveProvider(provider store.ProviderConfig, apiKey string) (store.ProviderConfig, error) {
	provider.ID = strings.TrimSpace(provider.ID)
	if provider.ID == "" {
		provider.ID = store.NewProviderID()
	}
	provider.Type = strings.TrimSpace(provider.Type)
	switch provider.Type {
	case store.ProviderTypeOpenAI, store.ProviderTypeAnthropic, store.ProviderTypeGemini:
	default:
		return store.ProviderConfig{}, fmt.Errorf("不支持的供应商类型: %s", provider.Type)
	}
	if strings.TrimSpace(provider.Model) == "" {
		return store.ProviderConfig{}, fmt.Errorf("请填写模型")
	}
	if provider.Type == store.ProviderTypeOpenAI && strings.TrimSpace(provider.BaseURL) == "" {
		return store.ProviderConfig{}, fmt.Errorf("请填写服务地址")
	}
	if apiKey != "" {
		if err := s.keys.Set(provider.ID, apiKey); err != nil {
			return store.ProviderConfig{}, err
		}
	}

	if err := s.store.Update(func(settings *store.Settings) {
		replaced := false
		for index, existing := range settings.Providers {
			if existing.ID == provider.ID {
				settings.Providers[index] = &provider
				replaced = true
				break
			}
		}
		if !replaced {
			settings.Providers = append(settings.Providers, &provider)
		}
		if settings.ActiveProviderID == "" {
			settings.ActiveProviderID = provider.ID
		}
	}); err != nil {
		return store.ProviderConfig{}, err
	}

	saved := provider
	if settings, err := s.store.Load(); err == nil {
		for _, existing := range settings.Providers {
			if existing.ID == provider.ID {
				saved = *existing
				break
			}
		}
	}
	saved.HasKey = s.keys.Has(saved.ID)
	return saved, nil
}

func (s *SettingsService) SetActiveProvider(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("供应商 ID 为空")
	}
	settings, err := s.store.Load()
	if err != nil {
		return err
	}
	found := false
	for _, provider := range settings.Providers {
		if provider.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("未找到供应商: %s", id)
	}
	return s.store.Update(func(settings *store.Settings) {
		settings.ActiveProviderID = id
	})
}

func (s *SettingsService) DeleteProvider(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if err := s.store.Update(func(settings *store.Settings) {
		providers := make([]*store.ProviderConfig, 0, len(settings.Providers))
		for _, provider := range settings.Providers {
			if provider.ID != id {
				providers = append(providers, provider)
			}
		}
		settings.Providers = providers
	}); err != nil {
		return err
	}
	return s.keys.Delete(id)
}
