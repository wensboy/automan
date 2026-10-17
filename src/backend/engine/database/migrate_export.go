package database

import "io/fs"

// Migrate 对指定库执行迁移(与启动链同一条路径: 同一个清单、同一个 iofs 源、同一个 migrate 驱动)。
// dir 是迁移 SQL 在 fsys 中的目录(例 "data/migrate")。
func Migrate(fsys fs.FS, dir, path string) error {
	return migrateUp(fsys, dir, "sqlite3://"+path)
}
