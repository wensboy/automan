package store

// 数据库指令 (当前仅 SQL / SQLite).
const (
	CmdExec  Cmd = "db.exec"
	CmdQuery Cmd = "db.query"
	CmdGet   Cmd = "db.get"
	CmdTx    Cmd = "db.tx"
)

// 分区语义键: 写走同一车道串行, 读走读车道 (读并发车道为后续 step).
const (
	shardWrite = "db.write"
	shardRead  = "db.read"
)

// Page 是分页游标, 仅对 SELECT 生效.
type Page struct {
	Limit  int
	Offset int
}

// ExecArgs 是 db.exec 的参数, 面向 INSERT/UPDATE/DELETE/DDL.
type ExecArgs struct {
	SQL       string
	Args      []any
	Partition string // 可选: 显式分区键 (缺省为写语义)
}

// Key 实现 Keyed.
func (a ExecArgs) Key() string {
	if a.Partition != "" {
		return a.Partition
	}
	return shardWrite
}

// QueryArgs 是 db.query / db.get 的参数.
// Into 非空时由 sqlx 直接填充到调用方结构 (强类型); 为空时返回通用行集.
type QueryArgs struct {
	SQL       string
	Args      []any
	Into      any
	Page      *Page
	Partition string
}

// Key 实现 Keyed.
func (a QueryArgs) Key() string {
	if a.Partition != "" {
		return a.Partition
	}
	return shardRead
}

// TxArgs 是 db.tx 的参数: 单任务内原子执行多语句.
type TxArgs struct {
	Steps     []Step
	Partition string
}

// Key 实现 Keyed.
func (a TxArgs) Key() string {
	if a.Partition != "" {
		return a.Partition
	}
	return shardWrite
}

// Step 是事务中的一条语句.
type Step struct {
	SQL  string
	Args []any
}

// ExecResult 是写操作结果.
type ExecResult struct {
	RowsAffected int64
	LastInsertID int64
}

// QueryResult 是读操作结果: Into 为空时给出通用行集, 否则只给出填充条数.
type QueryResult struct {
	Count int
	Rows  []Row
}

// Row 是一行结果 (列名 -> 值).
type Row = map[string]any
