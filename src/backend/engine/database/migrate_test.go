package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 清单过滤: 只有 migrations 里登记的文件可见(避免把目录里的 SQL 无差别加载)。
func TestMigrationFSFiltersToWhitelist(t *testing.T) {
	root := os.DirFS("../../../data/migrate")
	filtered := newMigrationFS(root, []string{"000001_create_proxies.up.sql"})

	entries, err := filtered.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "000001_create_proxies.up.sql" {
		t.Fatalf("应只暴露清单内的文件, got %d 项", len(entries))
	}
	if _, err := filtered.Open("000001_create_proxies.down.sql"); err == nil {
		t.Fatalf("清单外的文件不应可打开")
	}
}

// 迁移实跑: 建表 -> schema_migrations 记录版本 -> 再次执行幂等; 并验证软删除用的部分唯一索引存在。
func TestMigrateUpCreatesSchemaAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dsn := "sqlite3://" + filepath.Join(dir, "test.db")
	dataFS := os.DirFS("../../../data")

	if err := migrateUp(dataFS, "migrate", dsn); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := migrateUp(dataFS, "migrate", dsn); err != nil {
		t.Fatalf("重复迁移应幂等(ErrNoChange 被吞掉), got %v", err)
	}

	// 直接用标准驱动校验 schema(sqlx 与 migrate 各用各的连接)
	db := openTestDB(t, filepath.Join(dir, "test.db"))
	defer db.Close()

	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='proxies'`).Scan(&name); err != nil {
		t.Fatalf("proxies 表未创建: %v", err)
	}

	// 部分唯一索引: 只约束未删除行
	var indexSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_proxies_name_active'`).Scan(&indexSQL); err != nil {
		t.Fatalf("唯一索引缺失: %v", err)
	}
	if !contains(indexSQL, "deleted_at = 0") {
		t.Fatalf("唯一索引应为部分索引(仅未删除行), got %q", indexSQL)
	}

	// 逻辑删除后同名可重建: 两条未删除同名 -> 违反唯一约束
	now := time.Now().UnixMilli()
	insert := `INSERT INTO proxies (id, name, target, status, rules, description, created_at, updated_at, deleted_at)
	           VALUES (?, ?, '127.0.0.1:8899', 'stopped', '{}', '', ?, ?, 0)`
	if _, err := db.Exec(insert, "p1", "dup", now, now); err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	if _, err := db.Exec(insert, "p2", "dup", now, now); err == nil {
		t.Fatalf("未删除的同名应被唯一索引拒绝")
	}
	if _, err := db.Exec(`UPDATE proxies SET deleted_at = ? WHERE id = 'p1'`, now); err != nil {
		t.Fatalf("逻辑删除失败: %v", err)
	}
	if _, err := db.Exec(insert, "p3", "dup", now, now); err != nil {
		t.Fatalf("逻辑删除后同名应可重建: %v", err)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
