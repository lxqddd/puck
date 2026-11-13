package history

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestConversationLifecycle(t *testing.T) {
	store := newTestStore(t)

	conversation, err := store.CreateConversation("帮我想个周末计划，最好轻松一点")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if conversation.Title == "" {
		t.Fatal("expected a title")
	}

	if _, err := store.AppendMessage(conversation.ID, "user", "你好", ""); err != nil {
		t.Fatalf("AppendMessage user: %v", err)
	}
	if _, err := store.AppendMessage(conversation.ID, "assistant", "在呢", "先想想怎么回应"); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}

	messages, err := store.GetMessages(conversation.ID, 10)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if messages[1].Reasoning != "先想想怎么回应" {
		t.Fatalf("reasoning not persisted: %+v", messages[1])
	}

	conversations, err := store.ListConversations(10)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(conversations) != 1 || conversations[0].ID != conversation.ID {
		t.Fatalf("unexpected conversations: %+v", conversations)
	}

	if err := store.DeleteConversation(conversation.ID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}
	conversations, err = store.ListConversations(10)
	if err != nil {
		t.Fatalf("ListConversations after delete: %v", err)
	}
	if len(conversations) != 0 {
		t.Fatalf("expected empty conversations, got %+v", conversations)
	}
}

func TestGetMessagesOrdersChronologicallyAndLimits(t *testing.T) {
	store := newTestStore(t)
	conversation, err := store.CreateConversation("排序")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	for _, content := range []string{"一", "二", "三", "四"} {
		if _, err := store.AppendMessage(conversation.ID, "user", content, ""); err != nil {
			t.Fatalf("AppendMessage %s: %v", content, err)
		}
	}

	messages, err := store.GetMessages(conversation.ID, 2)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "三" || messages[1].Content != "四" {
		t.Fatalf("unexpected message order: %+v", messages)
	}
}

func TestTitleFromText(t *testing.T) {
	short := TitleFromText("你好")
	if short != "你好" {
		t.Fatalf("short title = %q", short)
	}
	long := TitleFromText("这是一个很长很长很长很长很长很长很长很长很长的会话标题")
	runes := []rune(long)
	if len(runes) != 25 || runes[24] != '…' {
		t.Fatalf("long title should be 24 runes + ellipsis, got %q", long)
	}
}
