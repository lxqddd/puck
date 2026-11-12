package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"puck/internal/ai"
	"puck/internal/events"
	"puck/internal/secure"
	"puck/internal/store"
)

type ChatService struct {
	store *store.SettingsStore
	keys  *secure.Keys

	personaMu sync.RWMutex
	persona   string

	historyMu sync.Mutex
	history   []ai.Message

	requestMu sync.Mutex
	counter   atomic.Uint64
}

func NewChat(settings *store.SettingsStore, keys *secure.Keys) *ChatService {
	return &ChatService{store: settings, keys: keys}
}

func (s *ChatService) SetPersona(prompt string) {
	s.personaMu.Lock()
	s.persona = prompt
	s.personaMu.Unlock()
}

// Send 立即返回消息 ID，真正的流式输出通过事件推给所有窗口。
func (s *ChatService) Send(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("message is empty")
	}
	messageID := fmt.Sprintf("m-%d", s.counter.Add(1))
	go s.run(messageID, text)
	return messageID, nil
}

func (s *ChatService) run(messageID, text string) {
	app := application.Get()
	emitState := func(state string) {
		app.Event.Emit(events.ChatState, events.ChatStateEvent{State: state})
	}
	emitState("thinking")

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	settings, _ := s.store.Load()
	var provider ai.Provider = &ai.EchoProvider{}
	model := ""
	if settings.Provider != nil && settings.Provider.BaseURL != "" && settings.Provider.Model != "" {
		key, _ := s.keys.Get(settings.Provider.ID)
		provider = &ai.OpenAICompatible{BaseURL: settings.Provider.BaseURL, APIKey: key}
		model = settings.Provider.Model
	}

	s.personaMu.RLock()
	system := s.persona
	s.personaMu.RUnlock()

	s.historyMu.Lock()
	messages := append([]ai.Message{}, s.history...)
	s.historyMu.Unlock()
	messages = append(messages, ai.Message{Role: ai.RoleUser, Content: text})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var full strings.Builder
	speaking := false
	err := provider.Chat(ctx, ai.ChatRequest{Model: model, System: system, Messages: messages}, func(ev ai.ChatEvent) error {
		if ev.Type != ai.EventDelta || ev.Text == "" {
			return nil
		}
		if !speaking {
			speaking = true
			emitState("speaking")
		}
		full.WriteString(ev.Text)
		app.Event.Emit(events.ChatDelta, events.ChatDeltaEvent{MessageID: messageID, Text: ev.Text})
		return nil
	})
	if err != nil {
		app.Event.Emit(events.ChatError, events.ChatErrorEvent{MessageID: messageID, Message: err.Error()})
		emitState("idle")
		return
	}

	reply := full.String()
	s.historyMu.Lock()
	s.history = append(s.history, ai.Message{Role: ai.RoleUser, Content: text})
	s.history = append(s.history, ai.Message{Role: ai.RoleAssistant, Content: reply})
	if len(s.history) > 40 {
		s.history = s.history[len(s.history)-40:]
	}
	s.historyMu.Unlock()

	app.Event.Emit(events.ChatDone, events.ChatDoneEvent{MessageID: messageID, Text: reply})
	emitState("idle")
}
