package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
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
	Provider    *ProviderConfig `json:"provider"`
	MascotScale float64         `json:"mascotScale"`
}

type SettingsStore struct {
	mu   sync.Mutex
	path string
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
	var settings Settings
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
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
	return s.saveLocked(settings)
}

func (s *SettingsStore) saveLocked(settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
