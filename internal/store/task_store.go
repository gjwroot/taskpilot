package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"taskpilot/internal/model"
)

type TaskStore struct {
	db *DB
}

func NewTaskStore(db *DB) *TaskStore {
	return &TaskStore{db: db}
}

func (s *TaskStore) Create(t model.Task) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	now := time.Now().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	_, err := s.db.Exec(
		`INSERT INTO tasks (id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ProjectID, t.Title, t.Description, t.Status, t.Priority, t.DueDate, t.Tags, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (s *TaskStore) Update(t model.Task) error {
	t.UpdatedAt = time.Now().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE tasks SET project_id=?, title=?, description=?, status=?, priority=?, due_date=?, tags=?, updated_at=? WHERE id=?`,
		t.ProjectID, t.Title, t.Description, t.Status, t.Priority, t.DueDate, t.Tags, t.UpdatedAt, t.ID,
	)
	return err
}

func (s *TaskStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}

func (s *TaskStore) GetByID(id string) (*model.Task, error) {
	row := s.db.QueryRow(
		`SELECT id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at FROM tasks WHERE id=?`, id,
	)
	t, err := scanTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func (s *TaskStore) ListByProject(projectID string) ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at
		 FROM tasks WHERE project_id=? ORDER BY priority ASC, created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *TaskStore) ListByStatus(status string) ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at
		 FROM tasks WHERE status=? ORDER BY priority ASC, created_at ASC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

// ListTodayTasks returns tasks that are due today OR have status "doing".
func (s *TaskStore) ListTodayTasks() ([]model.Task, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := s.db.Query(
		`SELECT id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at
		 FROM tasks
		 WHERE status = 'doing' OR (due_date != '' AND due_date LIKE ?)
		 ORDER BY priority ASC, created_at ASC`,
		today+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (s *TaskStore) ListAll() ([]model.Task, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at
		 FROM tasks ORDER BY priority ASC, created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectTasks(rows)
}

func scanTask(s scanner) (*model.Task, error) {
	var t model.Task
	err := s.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.DueDate, &t.Tags, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func collectTasks(rows *sql.Rows) ([]model.Task, error) {
	var tasks []model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}
