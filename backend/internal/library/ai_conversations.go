package library

import (
	"database/sql"
	"time"
)

// AiConversation is one saved AI chat thread — a scoped conversation the
// user can revisit, distinct from the single global playback queue/history.
type AiConversation struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type AiMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

// CreateAiConversation starts a new, empty conversation and returns its id.
func (s *Store) CreateAiConversation() (string, error) {
	id := randomID()
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO ai_conversations (id, title, created_at, updated_at) VALUES (?, '', ?, ?)`, id, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListAiConversations returns saved conversations, most recently active first.
func (s *Store) ListAiConversations() ([]AiConversation, error) {
	rows, err := s.db.Query(`SELECT id, title, created_at, updated_at FROM ai_conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AiConversation
	for rows.Next() {
		var c AiConversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// DeleteAiConversation removes a conversation and its messages.
func (s *Store) DeleteAiConversation(id string) error {
	if _, err := s.db.Exec(`DELETE FROM ai_messages WHERE conversation_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM ai_conversations WHERE id = ?`, id)
	return err
}

// AiConversationExists reports whether a conversation id is valid, so the
// API layer can 404 on a stale/unknown id instead of silently creating an
// orphaned message trail.
func (s *Store) AiConversationExists(id string) (bool, error) {
	var found int
	err := s.db.QueryRow(`SELECT 1 FROM ai_conversations WHERE id = ?`, id).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// ListAiMessages returns a conversation's transcript in order.
func (s *Store) ListAiMessages(conversationID string) ([]AiMessage, error) {
	rows, err := s.db.Query(`SELECT role, content, created_at FROM ai_messages WHERE conversation_id = ? ORDER BY id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AiMessage
	for rows.Next() {
		var m AiMessage
		if err := rows.Scan(&m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// SaveAiMessage appends one turn to a conversation's transcript, bumps its
// updated_at, and — the first time a user message lands — derives a title
// from it so the conversation list has something readable to show.
func (s *Store) SaveAiMessage(conversationID, role, content string) error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(
		`INSERT INTO ai_messages (conversation_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		conversationID, role, content, now,
	); err != nil {
		return err
	}

	if role == "user" {
		title := content
		if len(title) > 60 {
			title = title[:60] + "…"
		}
		if _, err := s.db.Exec(
			`UPDATE ai_conversations SET updated_at = ?, title = CASE WHEN title = '' THEN ? ELSE title END WHERE id = ?`,
			now, title, conversationID,
		); err != nil {
			return err
		}
		return nil
	}

	_, err := s.db.Exec(`UPDATE ai_conversations SET updated_at = ? WHERE id = ?`, now, conversationID)
	return err
}
