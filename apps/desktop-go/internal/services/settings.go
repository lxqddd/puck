package services

import (
	"strings"

	"puck/internal/secure"
	"puck/internal/shell"
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
	if settings.Provider != nil {
		settings.Provider.HasKey = s.keys.Has(settings.Provider.ID)
	}
	return settings, nil
}

func (s *SettingsService) Save(settings store.Settings, apiKey string) error {
	// mascotScale 以运行时值为准，避免设置页用打开时的旧值覆盖实时缩放。
	if current, err := s.store.Load(); err == nil {
		settings.MascotScale = current.MascotScale
	}
	if settings.MascotScale <= 0 {
		settings.MascotScale = shell.MascotDefaultScale
	}
	if settings.Provider != nil {
		p := settings.Provider
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" {
			p.ID = "default"
		}
		if p.Type == "" {
			p.Type = "openai-compatible"
		}
		p.BaseURL = strings.TrimSpace(p.BaseURL)
		p.Model = strings.TrimSpace(p.Model)
		if apiKey != "" {
			if err := s.keys.Set(p.ID, apiKey); err != nil {
				return err
			}
		}
		p.HasKey = s.keys.Has(p.ID)
	}
	return s.store.Save(settings)
}
