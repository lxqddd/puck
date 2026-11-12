package history

import "testing"

func TestConversationProviderPersistence(t *testing.T) {
	store := newTestStore(t)

	conversation, err := store.CreateConversationForProvider("hi", "p-a")
	if err != nil {
		t.Fatalf("CreateConversationForProvider: %v", err)
	}
	if conversation.ProviderID != "p-a" {
		t.Fatalf("providerId = %q", conversation.ProviderID)
	}

	list, err := store.ListConversations(10)
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(list) != 1 || list[0].ProviderID != "p-a" {
		t.Fatalf("list = %#v", list)
	}

	if err := store.SetConversationProvider(conversation.ID, "p-b"); err != nil {
		t.Fatalf("SetConversationProvider: %v", err)
	}
	got, err := store.ConversationProvider(conversation.ID)
	if err != nil {
		t.Fatalf("ConversationProvider: %v", err)
	}
	if got != "p-b" {
		t.Fatalf("provider = %q, want p-b", got)
	}
}
