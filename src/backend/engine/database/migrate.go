package database

import (
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	// migrate 使用它自己的 sqlite3 驱动(与 sqlx 的标准驱动互不耦合, 见 docs/design/v17.md §4/I)。
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
)

// migrations 是**手动维护**的迁移清单: 只有列在这里的 SQL 会被加载与执行。
// 这样便于按阶段/环境调整(例如临时回退某一步), 也避免把目录里的所有 SQL 无差别加载。
// 新增迁移时: 在 data/migrate/ 放好 up/down 文件后, 把两个文件名都登记到这里。
var migrations = []string{
	"000001_create_proxies.up.sql",
	"000001_create_proxies.down.sql",
	"000002_create_users_and_login_logs.up.sql",
	"000002_create_users_and_login_logs.down.sql",
	"000003_add_proxy_user_id.up.sql",
	"000003_add_proxy_user_id.down.sql",
	"000004_add_workspaces.up.sql",
	"000004_add_workspaces.down.sql",
	"000005_create_analysis_traces.up.sql",
	"000005_create_analysis_traces.down.sql",
	"000006_add_proxy_trace_workspace.up.sql",
	"000006_add_proxy_trace_workspace.down.sql",
}

// migrationFS 只暴露清单内的文件 —— golang-migrate 的 iofs 会遍历整个目录,
// 这里把可见范围收窄到 migrations 清单。
type migrationFS struct {
	root  fs.FS
	allow map[string]struct{}
}

func newMigrationFS(root fs.FS, files []string) migrationFS {
	allow := make(map[string]struct{}, len(files))
	for _, name := range files {
		allow[name] = struct{}{}
	}
	return migrationFS{root: root, allow: allow}
}

func (m migrationFS) Open(name string) (fs.File, error) {
	if _, ok := m.allow[path.Clean(name)]; !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return m.root.Open(name)
}

func (m migrationFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(m.root, name)
	if err != nil {
		return nil, err
	}
	kept := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if _, ok := m.allow[entry.Name()]; ok {
			kept = append(kept, entry)
		}
	}
	return kept, nil
}

// migrateUp 把清单内的迁移应用到 dsn 指向的库。
// dsn 形如 sqlite3://<绝对路径>; 使用 migrate 自带的连接与驱动, 与 sqlx 那条标准驱动连接相互独立。
func migrateUp(fsys fs.FS, dir, dsn string) error {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return fmt.Errorf("database: migrate dir %q: %w", dir, err)
	}
	src, err := iofs.New(newMigrationFS(sub, migrations), ".")
	if err != nil {
		return fmt.Errorf("database: migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("database: migrate open: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// dirty / 版本冲突等都在这里暴露, 交给启动链中断(不静默继续)
		return fmt.Errorf("database: migrate up: %w", err)
	}
	return nil
}
