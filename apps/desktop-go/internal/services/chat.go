package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"puck/internal/ai"
	"puck/internal/events"
	"puck/internal/history"
	"puck/internal/search"
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
	search  *search.Client

	personaMu sync.RWMutex
	persona   string

	busy    atomic.Bool
	counter atomic.Uint64
}

func NewChat(settings *store.SettingsStore, keys *secure.Keys, historyStore *history.Store) *ChatService {
	return &ChatService{store: settings, keys: keys, history: historyStore, search: search.NewClient()}
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

func (s *ChatService) ListHistory(limit int) ([]history.Conversation, error) {
	if s.history == nil {
		return nil, fmt.Errorf("history store unavailable")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 500 {
		limit = 500
	}
	return s.history.ListConversations(limit)
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
func (s *ChatService) Send(text string, conversationID string, providerID string) (SendResult, error) {
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

	settings, _ := s.store.Load()
	requestedProviderID := strings.TrimSpace(providerID)

	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		providerID = resolveProviderID(settings, requestedProviderID)
		conversation, err := s.history.CreateConversationForProvider(text, providerID)
		if err != nil {
			return SendResult{}, err
		}
		conversationID = conversation.ID
	} else if requestedProviderID != "" && providerExists(settings, requestedProviderID) {
		providerID = requestedProviderID
		if err := s.history.SetConversationProvider(conversationID, providerID); err != nil {
			return SendResult{}, err
		}
	} else {
		providerID, _ = s.history.ConversationProvider(conversationID)
		if !providerExists(settings, providerID) {
			providerID = resolveProviderID(settings, "")
			if err := s.history.SetConversationProvider(conversationID, providerID); err != nil {
				return SendResult{}, err
			}
		}
	}
	if _, err := s.history.AppendMessage(conversationID, string(ai.RoleUser), text); err != nil {
		return SendResult{}, err
	}

	messageID := fmt.Sprintf("m-%d", s.counter.Add(1))
	go s.run(messageID, conversationID, text, providerID)
	release = false
	return SendResult{MessageID: messageID, ConversationID: conversationID}, nil
}

func (s *ChatService) run(messageID, conversationID, text, providerID string) {
	defer s.busy.Store(false)

	app := application.Get()
	emitState := func(state string) {
		app.Event.Emit(events.ChatState, events.ChatStateEvent{State: state})
	}
	emitState("thinking")

	settings, _ := s.store.Load()
	provider, model, providerConfig, providerErr := providerFor(settings, s.keys, providerID)
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

	tools, toolExecutor := s.webSearchTools(providerConfig)
	if len(tools) > 0 {
		system = strings.TrimSpace(system + "\n\n当用户询问实时信息、新闻、天气、价格，或你不确定最新情况时，先调用 web_search，并在回答中引用结果里的链接。")
		if len(tools) > 1 {
			system += "如果摘要不够，可以再调用 web_fetch 抓取具体网页。"
		}
	}

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
	err := provider.Chat(ctx, ai.ChatRequest{
		Model:        model,
		System:       system,
		Messages:     messages,
		Tools:        tools,
		ToolExecutor: toolExecutor,
	}, func(event ai.ChatEvent) error {
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

func providerFor(settings store.Settings, keys *secure.Keys, providerID string) (ai.Provider, string, *store.ProviderConfig, error) {
	var config *store.ProviderConfig
	if providerID != "" {
		for _, provider := range settings.Providers {
			if provider.ID == providerID {
				config = provider
				break
			}
		}
	}
	if config == nil {
		for _, provider := range settings.Providers {
			if provider.ID == settings.ActiveProviderID {
				config = provider
				break
			}
		}
	}
	if config == nil && len(settings.Providers) > 0 {
		config = settings.Providers[0]
	}
	if config == nil {
		return &ai.EchoProvider{}, "", nil, nil
	}

	key, _ := keys.Get(config.ID)
	provider, model, err := buildProvider(*config, key)
	if err != nil {
		return nil, "", config, fmt.Errorf("「%s」%w", config.Name, err)
	}
	return provider, model, config, nil
}

func resolveProviderID(settings store.Settings, requested string) string {
	requested = strings.TrimSpace(requested)
	if providerExists(settings, requested) {
		return requested
	}
	if providerExists(settings, settings.ActiveProviderID) {
		return settings.ActiveProviderID
	}
	if len(settings.Providers) > 0 {
		return settings.Providers[0].ID
	}
	return ""
}

func providerExists(settings store.Settings, providerID string) bool {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return false
	}
	for _, provider := range settings.Providers {
		if provider.ID == providerID {
			return true
		}
	}
	return false
}

var webSearchTool = ai.ToolDefinition{
	Name:        "web_search",
	Description: "Search the web for current information such as news, weather, prices, releases, or facts you are unsure about. Returns result titles, URLs, and summaries. Use it before answering when up-to-date information matters, and cite the URLs you used.",
	Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Web search query"}},"required":["query"]}`),
}

var webFetchTool = ai.ToolDefinition{
	Name:        "web_fetch",
	Description: "Fetch a specific web page as Markdown. Use it after web_search when the summary is not enough and you need the full content of a result.",
	Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"HTTP or HTTPS URL to fetch"}},"required":["url"]}`),
}

type webSearcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]search.Result, error)
}

func (s *ChatService) webSearchTools(config *store.ProviderConfig) ([]ai.ToolDefinition, ai.ToolExecutor) {
	if config == nil || config.Type != store.ProviderTypeOpenAI || s.search == nil {
		return nil, nil
	}
	if config.WebSearch != nil && !*config.WebSearch {
		return nil, nil
	}

	primary, deepSeek := s.searchBackend(config)
	tools := []ai.ToolDefinition{webSearchTool}
	if deepSeek != nil {
		tools = append(tools, webFetchTool)
	}
	searchUnavailable := false

	executor := func(ctx context.Context, call ai.ToolCall) (ai.ToolResult, error) {
		if call.Name == "web_fetch" {
			return s.runWebFetch(ctx, call, deepSeek)
		}
		if searchUnavailable {
			return ai.ToolResult{
				CallID: call.ID, Name: call.Name,
				Content: "联网搜索当前不可用，不要再次调用 web_search；请直接告诉用户无法获取实时信息，建议稍后再试。",
				IsError: true,
			}, nil
		}
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil || strings.TrimSpace(args.Query) == "" {
			return ai.ToolResult{
				CallID: call.ID, Name: call.Name, Content: "搜索参数无效：需要非空的 query。", IsError: true,
			}, nil
		}
		results, err := primary.Search(ctx, args.Query, 5)
		if err != nil && deepSeek != nil && s.search != nil {
			fallback, fallbackErr := s.search.Search(ctx, args.Query, 5)
			if fallbackErr == nil {
				results, err = fallback, nil
			} else {
				err = fmt.Errorf("%v；备用搜索也失败：%v", err, fallbackErr)
			}
		}
		if err != nil {
			searchUnavailable = true
			return ai.ToolResult{
				CallID: call.ID, Name: call.Name,
				Content: "联网搜索失败：" + err.Error() + "。不要再次调用 web_search，请直接向用户说明情况。",
				IsError: true,
			}, nil
		}
		if len(results) == 0 {
			return ai.ToolResult{
				CallID: call.ID, Name: call.Name, Content: "没有找到相关结果，请尝试更具体的搜索词。",
			}, nil
		}
		var output strings.Builder
		for index, result := range results {
			title := result.Title
			if title == "" {
				title = result.URL
			}
			fmt.Fprintf(&output, "%d. %s\nURL: %s\n", index+1, title, result.URL)
			if result.Published != "" {
				fmt.Fprintf(&output, "Published: %s\n", result.Published)
			}
			if result.Content != "" {
				fmt.Fprintf(&output, "%s\n", truncateRunes(result.Content, 1200))
			}
			output.WriteString("\n")
		}
		return ai.ToolResult{CallID: call.ID, Name: call.Name, Content: output.String()}, nil
	}
	return tools, executor
}

func (s *ChatService) searchBackend(config *store.ProviderConfig) (webSearcher, *search.DeepSeekClient) {
	if isDeepSeekProvider(config) {
		key := ""
		if s.keys != nil {
			key, _ = s.keys.Get(config.ID)
		}
		client := search.NewDeepSeekClient(config.BaseURL, key)
		return client, client
	}
	return s.search, nil
}

func isDeepSeekProvider(config *store.ProviderConfig) bool {
	if config == nil || config.Type != store.ProviderTypeOpenAI {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.deepseek.com" || strings.HasSuffix(host, ".deepseek.com")
}

func (s *ChatService) runWebFetch(ctx context.Context, call ai.ToolCall, client *search.DeepSeekClient) (ai.ToolResult, error) {
	if client == nil {
		return ai.ToolResult{
			CallID: call.ID, Name: call.Name, Content: "web_fetch 仅在 DeepSeek 搜索后端可用。", IsError: true,
		}, nil
	}
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil || strings.TrimSpace(args.URL) == "" {
		return ai.ToolResult{
			CallID: call.ID, Name: call.Name, Content: "抓取参数无效：需要非空的 url。", IsError: true,
		}, nil
	}
	title, content, err := client.Fetch(ctx, args.URL)
	if err != nil {
		return ai.ToolResult{
			CallID: call.ID, Name: call.Name, Content: "网页抓取失败：" + err.Error(), IsError: true,
		}, nil
	}
	if strings.TrimSpace(content) == "" {
		content = "（没有可提取的正文）"
	}
	var output strings.Builder
	if title != "" {
		fmt.Fprintf(&output, "Title: %s\n", title)
	}
	fmt.Fprintf(&output, "URL: %s\n\n%s", args.URL, truncateRunes(content, 8000))
	return ai.ToolResult{CallID: call.ID, Name: call.Name, Content: output.String()}, nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
