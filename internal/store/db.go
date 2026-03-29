package store

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS projects (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			color       TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS tasks (
			id          TEXT PRIMARY KEY,
			project_id  TEXT NOT NULL DEFAULT '',
			title       TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT 'todo',
			priority    INTEGER NOT NULL DEFAULT 2,
			due_date    TEXT NOT NULL DEFAULT '',
			tags        TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);

		CREATE TABLE IF NOT EXISTS chat_messages (
			id          TEXT PRIMARY KEY,
			project_id  TEXT NOT NULL DEFAULT '',
			role        TEXT NOT NULL,
			content     TEXT NOT NULL,
			tool_results TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_chat_messages_project ON chat_messages(project_id);
		CREATE INDEX IF NOT EXISTS idx_chat_messages_created ON chat_messages(created_at);
	`)
	if err != nil {
		return err
	}

	// 为已有的 tasks 表添加 tags 列（忽略"列已存在"错误）
	db.Exec(`ALTER TABLE tasks ADD COLUMN tags TEXT NOT NULL DEFAULT ''`)

	return nil
}
