package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dir string) (*sql.DB, error) {
	return OpenWithLanguage(ctx, dir, "en")
}

// OpenWithLanguage 仅在新库初始化时使用指定语言，已有分类保持原样。
func OpenWithLanguage(ctx context.Context, dir, language string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "library.db"))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)
	for _, query := range []string{"PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL"} {
		if _, err = db.ExecContext(ctx, query); err != nil {
			db.Close()
			return nil, err
		}
	}

	var version int
	if err = db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}

	if version > 1 {
		db.Close()
		return nil, fmt.Errorf("Library version %d is newer than this app supports and cannot be opened", version)
	}

	if err = MigrateWithLanguage(ctx, db, language); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
