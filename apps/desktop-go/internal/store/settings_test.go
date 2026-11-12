package store

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *SettingsStore {
	t.Helper()
	return &SettingsStore{path: filepath.Join(t.TempDir(), "settings.json")}
}

func TestLoadMigratesLegacySingleProvider(t *testing.T) {
	store := newTestStore(t)
	legacy := `{
  "provider": {
    "id": "default",
    "name": "DeepSeek",
    "type": "openai-compatible",
    "baseURL": "https://api.deepseek.com/v1",
    "model": "deepseek-chat"
  },
  "mascotScale": 1.2
}`
	if err := os.WriteFile(store.path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy settings: %v", err)
	}

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(settings.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(settings.Providers))
	}
	provider := settings.Providers[0]
	if provider.ID != "default" || provider.Model != "deepseek-chat" {
		t.Fatalf("unexpected provider: %+v", provider)
	}
	if settings.ActiveProviderID != "default" {
		t.Fatalf("active provider = %q, want default", settings.ActiveProviderID)
	}
	if settings.MascotScale != 1.2 {
		t.Fatalf("mascot scale = %v, want 1.2", settings.MascotScale)
	}
}

func TestNormalizeSettingsAssignsMissingFields(t *testing.T) {
	settings := Settings{
		Providers: []*ProviderConfig{
			{Name: "One"},
			{ID: "one", Name: "Duplicate"},
			nil,
		},
		ActiveProviderID: "missing",
	}
	normalizeSettings(&settings)

	if len(settings.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(settings.Providers))
	}
	if settings.Providers[0].ID == "" || settings.Providers[0].Type != ProviderTypeOpenAI {
		t.Fatalf("unexpected first provider: %+v", settings.Providers[0])
	}
	if settings.ActiveProviderID != settings.Providers[0].ID {
		t.Fatalf("active provider = %q, want %q", settings.ActiveProviderID, settings.Providers[0].ID)
	}
}

func TestUpdatePreservesOtherFields(t *testing.T) {
	store := newTestStore(t)
	if err := store.Save(Settings{
		Providers: []*ProviderConfig{
			{ID: "a", Name: "A", Type: ProviderTypeAnthropic, Model: "claude-opus-5-5"},
			{ID: "b", Name: "B", Type: ProviderTypeGemini, Model: "gemini-2.5-flash"},
		},
		ActiveProviderID: "a",
		MascotScale:      0.6,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Update(func(settings *Settings) {
		settings.ActiveProviderID = "b"
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(settings.Providers) != 2 || settings.ActiveProviderID != "b" || settings.MascotScale != 0.6 {
		t.Fatalf("unexpected settings after update: %+v", settings)
	}
}
