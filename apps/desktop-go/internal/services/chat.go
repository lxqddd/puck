package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"puck/internal/ai"
	"puck/internal/events"
	"puck/internal/history"
	"puck/internal/secure"
	"puck/internal/store"
)

type SendResult struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
}

type ChatService struct {
	store   *store.SettingsStore
	keys    *secure.Keys
	history *history.Store

	personaMu sync.RWMutex
	persona   string

	busy    atomic.Bool
	counter atomic.Uint64
}

func NewChat(settings *store.SettingsStore, keys *secure.Keys, historyStore *history.Store) *ChatService {
	return &ChatService{store: settings, keys: keys, history: historyStore}
}

func (s *ChatService) SetPersona(prompt string) {
	s.personaMu.Lock()
	s.persona = prompt
	s.personaMu.Unlock()
}

func (s *ChatService) ListConversations() ([]history.Conversation, error) {
	if s.history == nil {
		return nil, fmt.Errorf("history store unavailable")
	}
	return s.history.ListConversations(50)
}

func (s *ChatService) GetMessages(conversationID string) ([]history.Message, error) {
	if s.history == nil {
		return nil, fmt.Errorf("history store unavailable")
	}
	return s.history.GetMessages(conversationID, 200)
}

func (s *ChatService) DeleteConversation(conversationID string) error {
	if s.history == nil {
		return fmt.Errorf("history store unavailable")
	}
	return s.history.DeleteConversation(conversationID)
}

// Send 立即返回消息 ID 与会话 ID，流式内容通过事件推送给所有窗口。
func (s *ChatService) Send(text string, conversationID string) (SendResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return SendResult{}, fmt.Errorf("message is empty")
	}
	if s.history == nil {
		return SendResult{}, fmt.Errorf("history store unavailable")
	}
	if !s.busy.CompareAndSwap(false, true) {
		return SendResult{}, fmt.Errorf("上一条消息还在生成中，请稍候")
	}
	release := true
	defer func() {
		if release {
			s.busy.Store(false)
		}
	}()

	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		conversation, err := s.history.CreateConversation(text)
		if err != nil {
			return SendResult{}, err
		}
		conversationID = conversation.ID
	}
	if _, err := s.history.AppendMessage(conversationID, string(ai.RoleUser), text); err != nil {
		return SendResult{}, err
	}

	messageID := fmt.Sprintf("m-%d", s.counter.Add(1))
	go s.run(messageID, conversationID, text)
	release = false
	return SendResult{MessageID: messageID, ConversationID: conversationID}, nil
}

func (s *ChatService) run(messageID, conversationID, text string) {
	defer s.busy.Store(false)

	app := application.Get()
	emitState := func(state string) {
		app.Event.Emit(events.ChatState, events.ChatStateEvent{State: state})
	}
	emitState("thinking")

	settings, _ := s.store.Load()
	provider, model, providerErr := providerFor(settings, s.keys)
	if providerErr != nil {
		app.Event.Emit(events.ChatError, events.ChatErrorEvent{
			MessageID:      messageID,
			ConversationID: conversationID,
			Message:        providerErr.Error(),
		})
		emitState("idle")
		return
	}

	s.personaMu.RLock()
	system := s.persona
	s.personaMu.RUnlock()

	messages := make([]ai.Message, 0, 21)
	if stored, err := s.history.GetMessages(conversationID, 20); err == nil && len(stored) > 0 {
		for _, message := range stored {
			role := ai.RoleUser
			if message.Role == "assistant" {
				role = ai.RoleAssistant
			}
			messages = append(messages, ai.Message{Role: role, Content: message.Content})
		}
		// Anthropic 要求对话从 user 开始，截断历史时丢弃开头的 assistant 消息。
		for len(messages) > 0 && messages[0].Role == ai.RoleAssistant {
			messages = messages[1:]
		}
	} else {
		messages = append(messages, ai.Message{Role: ai.RoleUser, Content: text})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	var full strings.Builder
	speaking := false
	err := provider.Chat(ctx, ai.ChatRequest{Model: model, System: system, Messages: messages}, func(event ai.ChatEvent) error {
		if event.Type != ai.EventDelta || event.Text == "" {
			return nil
		}
		if !speaking {
			speaking = true
			emitState("speaking")
		}
		full.WriteString(event.Text)
		app.Event.Emit(events.ChatDelta, events.ChatDeltaEvent{
			MessageID:      messageID,
			ConversationID: conversationID,
			Text:           event.Text,
		})
		return nil
	})
	if err != nil {
		app.Event.Emit(events.ChatError, events.ChatErrorEvent{
			MessageID:      messageID,
			ConversationID: conversationID,
			Message:        err.Error(),
		})
		emitState("idle")
		return
	}

	reply := full.String()
	if reply == "" {
		app.Event.Emit(events.ChatError, events.ChatErrorEvent{
			MessageID:      messageID,
			ConversationID: conversationID,
			Message:        "模型没有返回内容，可能被安全策略拦截或请求被取消",
		})
		emitState("idle")
		return
	}
	if _, err := s.history.AppendMessage(conversationID, "assistant", reply); err != nil {
		log.Printf("puck: failed to save assistant message: %v", err)
	}
	app.Event.Emit(events.ChatDone, events.ChatDoneEvent{
		MessageID:      messageID,
		ConversationID: conversationID,
		Text:           reply,
	})
	emitState("idle")
}

func providerFor(settings store.Settings, keys *secure.Keys) (ai.Provider, string, error) {
	var config *store.ProviderConfig
	for _, provider := range settings.Providers {
		if provider.ID == settings.ActiveProviderID {
			config = provider
			break
		}
	}
	if config == nil && len(settings.Providers) > 0 {
		config = settings.Providers[0]
	}
	if config == nil {
		return &ai.EchoProvider{}, "", nil
	}

	key, _ := keys.Get(config.ID)
	provider, model, err := buildProvider(*config, key)
	if err != nil {
		return nil, "", fmt.Errorf("「%s」%w", config.Name, err)
	}
	return provider, model, nil
}
