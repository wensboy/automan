package database

import (
	"path/filepath"
	"testing"
)

// 000005 的分析 trace 表必须随启动链一起建出来(漏登记会让 UI 报错)。
func TestMigrateCreatesAnalysisTraces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "analysis.db")
	if err := MigrateDataForTest(path); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db := newSQLX(t, path)
	defer func() { _ = db.Close() }()

	for _, table := range []string{"proxy_trace", "log_trace", "bridge_trace"} {
		var name string
		err := db.Get(&name, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table)
		if err != nil || name != table {
			t.Fatalf("缺少表 %s: err=%v name=%q", table, err, name)
		}
	}

	// 列名以 v25 §3 为准
	rows, err := db.Queryx(`SELECT * FROM proxy_trace LIMIT 0`)
	if err != nil {
		t.Fatalf("查询 proxy_trace: %v", err)
	}
	columns, err := rows.Columns()
	_ = rows.Close()
	if err != nil {
		t.Fatalf("读取列名: %v", err)
	}
	want := map[string]bool{"id": false, "proxy_id": false, "name": false, "listen": false, "action": false, "created_at": false}
	for _, column := range columns {
		if _, ok := want[column]; ok {
			want[column] = true
		}
	}
	for column, found := range want {
		if !found {
			t.Fatalf("proxy_trace 缺少列 %s(实际: %v)", column, columns)
		}
	}

	// 幂等: 再次执行不应报错
	if err := MigrateDataForTest(path); err != nil {
		t.Fatalf("二次 migrate 应幂等: %v", err)
	}
}
