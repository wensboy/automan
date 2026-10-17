package database

import (
	stdctx "context"
	"testing"

	domainstore "github.com/wensboy/automan/backend/domain/store"
)

type contractRow struct {
	ID   string `db:"id"`
	Name string `db:"name"`
}

// 记录处理器契约: **query 会填充 Into, get 不会**。
// 消费侧要拿强类型结果就必须用 CmdQuery —— 这条差异曾导致代理列表空而无报错。
func TestQueryFillsIntoAndGetDoesNot(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/contract.db"
	if err := MigrateDataForTest(path); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	db := openTestDB(t, path)
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO proxies (id,name,target,status,rules,description,created_at,updated_at,deleted_at)
        VALUES ('p1','alpha','127.0.0.1:1','stopped','{}','',1,1,0)`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	engine := &Engine{db: newSQLX(t, path)}
	args := domainstore.QueryArgs{
		SQL:  `SELECT id, name FROM proxies WHERE deleted_at = 0`,
		Into: &[]contractRow{},
	}

	value, err := engine.query(stdctx.Background(), args)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	filled, ok := args.Into.(*[]contractRow)
	if !ok || len(*filled) != 1 || (*filled)[0].Name != "alpha" {
		t.Fatalf("query 应填充 Into, got %+v (result=%+v)", args.Into, value)
	}

	// get 只返回"单行 map"(不填充 Into): 消费侧要强类型结果就必须用 CmdQuery
	other := domainstore.QueryArgs{SQL: `SELECT id, name FROM proxies WHERE deleted_at = 0`, Into: &[]contractRow{}}
	got, err := engine.get(stdctx.Background(), other)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	row, ok := got.(domainstore.Row)
	if !ok || row["name"] != "alpha" {
		t.Fatalf("get 应返回单行 map, got %#v", got)
	}
	if into, ok := other.Into.(*[]contractRow); ok && len(*into) != 0 {
		t.Fatalf("get 不应填充 Into, got %+v", *into)
	}
}
