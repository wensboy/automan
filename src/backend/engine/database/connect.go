package database

import (
	stdctx "context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"

	// 驱动注册: 与 driver 常量成对修改.
	// 切换纯 Go 驱动时改为 _ "modernc.org/sqlite" 并把 driver 改成 "sqlite".
	_ "github.com/mattn/go-sqlite3"
)

// driver 是 database/sql 的驱动注册名 (mattn -> "sqlite3", modernc -> "sqlite").
const driver = "sqlite3"

var journals = map[string]bool{
	"wal":      true,
	"delete":   true,
	"truncate": true,
	"memory":   true,
	"off":      true,
}

func openSQLite(path string, busyTimeoutMs int, journal string) (*sqlx.DB, error) {
	if !journals[journal] {
		return nil, fmt.Errorf("database: invalid journal %q", journal)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("database: create dir: %w", err)
	}
	db, err := sqlx.Connect(driver, path)
	if err != nil {
		return nil, fmt.Errorf("database: connect %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	pragmas := []string{
		"PRAGMA journal_mode = " + journal,
		"PRAGMA foreign_keys = ON",
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMs),
		"PRAGMA synchronous = NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(stdctx.Background(), pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("database: %s: %w", pragma, err)
		}
	}
	return db, nil
}
