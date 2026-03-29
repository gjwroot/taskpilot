package core

import (
	"fmt"
	"os"
	"path/filepath"

	"taskpilot/internal/store"
)

// AppCore holds shared infrastructure used by all services.
type AppCore struct {
	DB           *store.DB
	ProjectStore *store.ProjectStore
	TaskStore    *store.TaskStore
	ConfigStore  *store.ConfigStore
}

// NewAppCore initializes the database and all stores.
func NewAppCore() (*AppCore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home dir: %w", err)
	}
	dataDir := filepath.Join(home, ".taskpilot")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create data dir: %w", err)
	}

	db, err := store.NewDB(filepath.Join(dataDir, "data.db"))
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	return &AppCore{
		DB:           db,
		ProjectStore: store.NewProjectStore(db),
		TaskStore:    store.NewTaskStore(db),
		ConfigStore:  store.NewConfigStore(db),
	}, nil
}
