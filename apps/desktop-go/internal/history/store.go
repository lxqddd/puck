package history

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Conversation struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	ProviderID string `json:"providerId"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type Message struct {
	ID             int64  `json:"id"`
	ConversationID string `json:"conversationId"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Reasoning      string `json:"reasoning"`
	CreatedAt      int64  `json:"createdAt"`
}

type Store struct {
	db *sql.DB
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "puck", "history.db")
}

func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`); err != nil {
		return err
	}
	if err := s.ensureMessagesSchema(); err != nil {
		return err
	}
	if err := s.ensureMessagesReasoningColumn(); err != nil {
		return err
	}
	if err := s.ensureConversationProviderColumn(); err != nil {
		return err
	}
	_, err := s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, id);
`)
	return err
}

func (s *Store) ensureConversationProviderColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(conversations)`)
	if err != nil {
		return err
	}
	exists := false
	for rows.Next() {
		var (
			columnID   int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "provider_id" {
			exists = true
		}
	}
	rows.Close()
	if exists {
		return nil
	}
	_, err = s.db.Exec(`ALTER TABLE conversations ADD COLUMN provider_id TEXT NOT NULL DEFAULT ''`)
	return err
}

// ensureMessagesSchema 创建带外键与角色约束的 messages 表；旧库缺少约束时就地重建。
func (s *Store) ensureMessagesSchema() error {
	var exists int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'messages'`,
	).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return s.createMessagesTable()
	}

	rows, err := s.db.Query(`PRAGMA foreign_key_list(messages)`)
	if err != nil {
		return err
	}
	hasForeignKey := rows.Next()
	rows.Close()
	if hasForeignKey {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`ALTER TABLE messages RENAME TO messages_legacy`); err != nil {
		return err
	}
	if _, err := tx.Exec(createMessagesTableSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO messages(id, conversation_id, role, content, created_at)
SELECT id, conversation_id, role, content, created_at FROM messages_legacy
WHERE conversation_id IN (SELECT id FROM conversations)
  AND role IN ('user', 'assistant')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE messages_legacy`); err != nil {
		return err
	}
	return tx.Commit()
}

const createMessagesTableSQL = `
CREATE TABLE messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
  content TEXT NOT NULL,
  reasoning TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);`

// ensureMessagesReasoningColumn 为旧库补上推理内容列，已有则跳过。
func (s *Store) ensureMessagesReasoningColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(messages)`)
	if err != nil {
		return err
	}
	exists := false
	for rows.Next() {
		var (
			columnID   int
			name       string
			columnType string
			notNull    int
			defaultVal any
			primaryKey int
		)
		if err := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultVal, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == "reasoning" {
			exists = true
		}
	}
	rows.Close()
	if exists {
		return nil
	}
	_, err = s.db.Exec(`ALTER TABLE messages ADD COLUMN reasoning TEXT NOT NULL DEFAULT ''`)
	return err
}

func (s *Store) createMessagesTable() error {
	_, err := s.db.Exec(createMessagesTableSQL)
	return err
}

func (s *Store) ListConversations(limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, title, provider_id, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	conversations := make([]Conversation, 0, limit)
	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(&conversation.ID, &conversation.Title, &conversation.ProviderID, &conversation.CreatedAt, &conversation.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, conversation)
	}
	return conversations, rows.Err()
}

func (s *Store) CreateConversation(title string) (Conversation, error) {
	return s.CreateConversationForProvider(title, "")
}

func (s *Store) CreateConversationForProvider(title, providerID string) (Conversation, error) {
	now := time.Now().UnixMilli()
	conversation := Conversation{
		ID:         newID(),
		Title:      TitleFromText(title),
		ProviderID: strings.TrimSpace(providerID),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_, err := s.db.Exec(
		`INSERT INTO conversations(id, title, provider_id, created_at, updated_at) VALUES(?,?,?,?,?)`,
		conversation.ID, conversation.Title, conversation.ProviderID, conversation.CreatedAt, conversation.UpdatedAt,
	)
	if err != nil {
		return Conversation{}, err
	}
	return conversation, nil
}

func (s *Store) SetConversationProvider(conversationID, providerID string) error {
	_, err := s.db.Exec(
		`UPDATE conversations SET provider_id = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(providerID), time.Now().UnixMilli(), conversationID,
	)
	return err
}

func (s *Store) ConversationProvider(conversationID string) (string, error) {
	var providerID string
	err := s.db.QueryRow(
		`SELECT provider_id FROM conversations WHERE id = ?`, conversationID,
	).Scan(&providerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return providerID, err
}

// GetMessages 返回最近 limit 条消息，按时间正序排列。
func (s *Store) GetMessages(conversationID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT id, conversation_id, role, content, reasoning, created_at
		 FROM messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?`,
		conversationID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.Role, &message.Content, &message.Reasoning, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (s *Store) AppendMessage(conversationID, role, content, reasoning string) (Message, error) {
	now := time.Now().UnixMilli()
	result, err := s.db.Exec(
		`INSERT INTO messages(conversation_id, role, content, reasoning, created_at) VALUES(?,?,?,?,?)`,
		conversationID, role, content, reasoning, now,
	)
	if err != nil {
		return Message{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	if _, err := s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, conversationID); err != nil {
		return Message{}, err
	}
	return Message{
		ID:             id,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Reasoning:      reasoning,
		CreatedAt:      now,
	}, nil
}

func (s *Store) DeleteConversation(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conversations WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// TitleFromText 用首条用户消息生成会话标题。
func TitleFromText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	runes := []rune(text)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	return text
}

func newID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("c%d", time.Now().UnixNano())
	}
	return "c" + hex.EncodeToString(buffer)
}
