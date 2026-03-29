package core

import (
	"fmt"
	"os"
	"path/filepath"

	"taskpilot/internal/logger"
	"taskpilot/internal/store"
)

// AppCore holds shared infrastructure used by all services.
type AppCore struct {
	DB           *store.DB
	ProjectStore *store.ProjectStore
	TaskStore    *store.TaskStore
	ConfigStore  *store.ConfigStore
	ChatStore    *store.ChatStore
	DataDir      string
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

	if err := logger.Init(dataDir); err != nil {
		return nil, fmt.Errorf("could not init logger: %w", err)
	}
	logger.Log.Info("TaskPilot starting", "dataDir", dataDir)

	db, err := store.NewDB(filepath.Join(dataDir, "data.db"))
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	logger.Log.Info("database opened")

	return &AppCore{
		DB:           db,
		ProjectStore: store.NewProjectStore(db),
		TaskStore:    store.NewTaskStore(db),
		ConfigStore:  store.NewConfigStore(db),
		ChatStore:    store.NewChatStore(db),
		DataDir:      dataDir,
	}, nil
}
