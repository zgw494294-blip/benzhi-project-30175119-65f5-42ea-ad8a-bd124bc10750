package repository

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	dsn := path
	if path == ":memory:" {
		dsn = "file:dialectrelease?mode=memory&cache=shared"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	var result string
	if err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil || result != "ok" {
		db.Close()
		return nil, fmt.Errorf("SQLite 完整性检查失败: %s: %w", result, err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
