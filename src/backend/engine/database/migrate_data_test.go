package database

import (
	"os"

	"github.com/jmoiron/sqlx"

	_ "github.com/mattn/go-sqlite3"
)

// MigrateDataForTest 用仓库里的 data/migrate 建表(与启动链同源)。
func MigrateDataForTest(path string) error {
	return Migrate(os.DirFS("../../../data"), "migrate", path)
}

func newSQLX(t interface{ Fatalf(string, ...any) }, path string) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", path)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return db
}
