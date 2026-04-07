package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// Store wraps a bun.DB.
type Store struct {
	db *bun.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	sqldb, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	sqldb.SetMaxOpenConns(1)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	if err := migrate(context.Background(), db); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func migrate(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewCreateTable().Model(&Chatroom{}).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewCreateTable().Model(&Member{}).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewCreateTable().Model(&Message{}).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewCreateTable().Model(&Event{}).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewCreateTable().Model(&Subscriber{}).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_messages_chatroom ON messages (chatroom_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type       ON events (event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts         ON events (timestamp)`,
	}
	for _, idx := range indexes {
		if _, err := db.ExecContext(ctx, idx); err != nil {
			return err
		}
	}
	return nil
}
