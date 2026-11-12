package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/zalando/go-keyring"

	"puck/internal/ai"
	"puck/internal/events"
	"puck/internal/secure"
	"puck/internal/store"
)

type ProviderTestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	LatencyMs int64  `json:"latencyMs"`
}

var errTestComplete = errors.New("provider test complete")

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
		key, err := s.keys.Get(provider.ID)
		if err == nil && strings.TrimSpace(key) != "" {
			provider.HasKey = true
			provider.KeyMask = maskKey(key)
			continue
		}
		provider.HasKey = false
		provider.KeyMask = ""
	}
	return settings, nil
}

// maskKey 只回传首尾少量字符，完整 Key 始终留在钥匙串里。
func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:3] + "••••••••" + key[len(key)-4:]
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
	if key, err := s.keys.Get(saved.ID); err == nil && strings.TrimSpace(key) != "" {
		saved.HasKey = true
		saved.KeyMask = maskKey(key)
	} else {
		saved.HasKey = false
		saved.KeyMask = ""
	}
	return saved, nil
}

// SetTheme 保存主题偏好并广播给所有窗口。
func (s *SettingsService) SetTheme(theme string) error {
	theme = strings.TrimSpace(theme)
	switch theme {
	case "system", "light", "dark":
	default:
		return fmt.Errorf("未知主题: %s", theme)
	}
	if err := s.store.Update(func(settings *store.Settings) {
		settings.Theme = theme
	}); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.Emit(events.ThemeChanged, events.ThemeChangedEvent{Theme: theme})
	}
	return nil
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

// ListModels 拉取供应商的模型列表；apiKey 为空时使用已保存的 Key。
func (s *SettingsService) ListModels(provider store.ProviderConfig, apiKey string) ([]string, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" && strings.TrimSpace(provider.ID) != "" {
		stored, err := s.keys.Get(strings.TrimSpace(provider.ID))
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("读取系统钥匙串失败：%w", err)
		}
		key = stored
	}

	providerType := strings.TrimSpace(provider.Type)
	if providerType == "" {
		providerType = store.ProviderTypeOpenAI
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, err := ai.ListModels(ctx, providerType, provider.BaseURL, key)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("获取模型列表超时（15 秒）")
		}
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("没有获取到模型，请手动填写模型名")
	}
	return models, nil
}

// TestProvider 用当前表单内容发一条最小请求，验证地址、模型与 Key 是否可用。
func (s *SettingsService) TestProvider(provider store.ProviderConfig, apiKey string) ProviderTestResult {
	key := strings.TrimSpace(apiKey)
	if key == "" && strings.TrimSpace(provider.ID) != "" {
		stored, err := s.keys.Get(strings.TrimSpace(provider.ID))
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return ProviderTestResult{OK: false, Message: fmt.Sprintf("读取系统钥匙串失败：%v", err)}
		}
		key = stored
	}

	chatProvider, model, err := buildProvider(provider, key)
	if err != nil {
		return ProviderTestResult{OK: false, Message: err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	received := false
	err = chatProvider.Chat(ctx, ai.ChatRequest{
		Model:    model,
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "只回复 OK"}},
	}, func(event ai.ChatEvent) error {
		if event.Type == ai.EventDelta && strings.TrimSpace(event.Text) != "" {
			received = true
			return errTestComplete
		}
		return nil
	})
	latency := time.Since(start).Milliseconds()
	if received {
		return ProviderTestResult{OK: true, Message: "连接成功", LatencyMs: latency}
	}
	if err != nil && !errors.Is(err, errTestComplete) {
		if errors.Is(err, context.DeadlineExceeded) {
			return ProviderTestResult{OK: false, Message: "连接超时（20 秒）"}
		}
		return ProviderTestResult{OK: false, Message: err.Error()}
	}
	return ProviderTestResult{OK: false, Message: "模型没有返回内容"}
}
