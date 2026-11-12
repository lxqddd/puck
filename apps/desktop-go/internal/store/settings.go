package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ProviderTypeOpenAI    = "openai-compatible"
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeGemini    = "gemini"
)

type ProviderConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	BaseURL string `json:"baseURL"`
	Model   string `json:"model"`
	HasKey  bool   `json:"hasKey"`
}

type Settings struct {
	Providers        []*ProviderConfig `json:"providers"`
	ActiveProviderID string            `json:"activeProviderId"`
	MascotScale      float64           `json:"mascotScale"`
}

type SettingsStore struct {
	mu   sync.Mutex
	path string
}

// rawSettings 兼容旧版单 provider 配置格式。
type rawSettings struct {
	Providers        []*ProviderConfig `json:"providers"`
	ActiveProviderID string            `json:"activeProviderId"`
	MascotScale      float64           `json:"mascotScale"`
	Provider         *ProviderConfig   `json:"provider"`
}

func NewSettingsStore() *SettingsStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return &SettingsStore{path: filepath.Join(dir, "puck", "settings.json")}
}

func (s *SettingsStore) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *SettingsStore) loadLocked() (Settings, error) {
	var raw rawSettings
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Settings{}, err
	}

	providers := raw.Providers
	migrated := len(providers) == 0 && raw.Provider != nil
	if migrated {
		providers = []*ProviderConfig{raw.Provider}
	}
	settings := Settings{
		Providers:        providers,
		ActiveProviderID: raw.ActiveProviderID,
		MascotScale:      raw.MascotScale,
	}
	normalizeSettings(&settings)
	if migrated {
		_ = s.saveLocked(settings)
	}
	return settings, nil
}

func (s *SettingsStore) Save(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(settings)
}

func (s *SettingsStore) Update(apply func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.loadLocked()
	if err != nil {
		return err
	}
	apply(&settings)
	normalizeSettings(&settings)
	return s.saveLocked(settings)
}

func (s *SettingsStore) saveLocked(settings Settings) error {
	normalizeSettings(&settings)
	for _, provider := range settings.Providers {
		provider.HasKey = false
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	tempPath := s.path + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, s.path)
}

func normalizeSettings(settings *Settings) {
	seen := make(map[string]bool, len(settings.Providers))
	providers := make([]*ProviderConfig, 0, len(settings.Providers))
	for _, provider := range settings.Providers {
		if provider == nil {
			continue
		}
		provider.ID = strings.TrimSpace(provider.ID)
		if provider.ID == "" {
			provider.ID = newProviderID()
		}
		if seen[provider.ID] {
			continue
		}
		if provider.Type == "" {
			provider.Type = ProviderTypeOpenAI
		}
		provider.Name = strings.TrimSpace(provider.Name)
		if provider.Name == "" {
			provider.Name = provider.ID
		}
		provider.BaseURL = strings.TrimSpace(provider.BaseURL)
		provider.Model = strings.TrimSpace(provider.Model)
		seen[provider.ID] = true
		providers = append(providers, provider)
	}
	settings.Providers = providers

	if !seen[settings.ActiveProviderID] {
		if len(providers) > 0 {
			settings.ActiveProviderID = providers[0].ID
		} else {
			settings.ActiveProviderID = ""
		}
	}
}

func newProviderID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "provider"
	}
	return "p-" + hex.EncodeToString(buffer)
}

func NewProviderID() string {
	return newProviderID()
}
