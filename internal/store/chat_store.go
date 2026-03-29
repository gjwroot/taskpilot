package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ChatMsg represents a persisted chat message.
type ChatMsg struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Role        string `json:"role"`
	Content     string `json:"content"`
	ToolResults string `json:"toolResults"`
	CreatedAt   string `json:"createdAt"`
}

type ChatStore struct {
	db *DB
}

func NewChatStore(db *DB) *ChatStore {
	return &ChatStore{db: db}
}

func (s *ChatStore) Save(projectID, role, content, toolResults string) error {
	id := uuid.NewString()
	now := time.Now().Format(time.RFC3339)
	if toolResults == "" {
		toolResults = "[]"
	}
	_, err := s.db.Exec(
		`INSERT INTO chat_messages (id, project_id, role, content, tool_results, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, projectID, role, content, toolResults, now,
	)
	return err
}

func (s *ChatStore) GetMessages(projectID string, limit, offset int) ([]ChatMsg, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, role, content, tool_results, created_at
		 FROM chat_messages
		 WHERE project_id = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ChatMsg
	for rows.Next() {
		var m ChatMsg
		if err := rows.Scan(&m.ID, &m.ProjectID, &m.Role, &m.Content, &m.ToolResults, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	// Reverse to chronological order (query is DESC for LIMIT/OFFSET)
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}

func (s *ChatStore) DeleteByProject(projectID string) error {
	_, err := s.db.Exec(`DELETE FROM chat_messages WHERE project_id = ?`, projectID)
	return err
}

func (s *ChatStore) DeleteAll() error {
	_, err := s.db.Exec(`DELETE FROM chat_messages`)
	return err
}

// SaveToolResultsJSON marshals tool results to JSON string for storage.
func SaveToolResultsJSON(results interface{}) string {
	if results == nil {
		return "[]"
	}
	b, err := json.Marshal(results)
	if err != nil {
		return "[]"
	}
	return string(b)
}
