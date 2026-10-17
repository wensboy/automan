// Package database 实现 database 引擎: 与 store 同构的指令式执行器, 底层 SQLite (sqlx).
package database

import (
	stdctx "context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/dispatch"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
	"github.com/wensboy/automan/backend/internal/paths"
)

// 缺省参数, 可经 data/conf/store.json 的 database 子对象覆盖.
const (
	driverSQLite         = "sqlite"
	defaultPath          = "runtime/store/database/automan.db"
	defaultShards        = 1
	defaultInflight      = 1
	defaultQueue         = 128
	defaultTimeoutMs     = 10000
	defaultBusyTimeoutMs = 5000
	defaultJournal       = "wal"
	defaultSlowMs        = 200
	defaultMaxRows       = 10000
)

// Engine 是 database 引擎, 经 dispatch 实现 domain/store 的 Store/Registrar/Statser.
type Engine struct {
	queue  *dispatch.Queue
	db     *sqlx.DB
	logger domainlog.Logger

	path          string
	busyTimeoutMs int
	journal       string
	slowMs        int
	maxRows       int
	logArgs       bool

	// 迁移: migrations 为携带 SQL 的文件系统(data), migrateDir 为其中的目录(例 "data/migrate")。
	// 只加载 migrate.go 里手动维护的清单(见 docs/design/v17.md §4)。
	migrations fs.FS
	migrateDir string
}

// Option 设置 database 引擎的必要字段.
type Option func(*Engine)

// WithPath 覆盖数据库文件路径 (缺省按平台约定解析).
func WithPath(path string) Option { return func(e *Engine) { e.path = path } }

// WithMigrations 指定迁移 SQL 的位置: fsys 为文件系统(组合根传内嵌的 data), dir 为其中目录。
// 未指定时启动不跑迁移(schema 由外部准备)。
func WithMigrations(fsys fs.FS, dir string) Option {
	return func(e *Engine) {
		e.migrations = fsys
		e.migrateDir = dir
	}
}

// New 创建 database 引擎.
func New(opts ...Option) *Engine {
	e := &Engine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "database" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return []string{"config", "eventbus", "log"} }

// Init 解析配置, 取日志并注册 db.* handler.
func (e *Engine) Init(ctx context.Context) error {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	store, ok := configkit.Get(provider, config.StoreKey)
	if !ok {
		return fmt.Errorf("database: config section %q missing", config.StoreKey.Name())
	}
	cfg := store.Database
	if cfg.Driver != driverSQLite {
		return fmt.Errorf("database: unsupported driver %q", cfg.Driver)
	}
	if e.path == "" {
		path, err := paths.Resolve(fallback.String(cfg.Path, defaultPath))
		if err != nil {
			return err
		}
		e.path = path
	}
	e.busyTimeoutMs = fallback.Int(cfg.BusyTimeoutMs, defaultBusyTimeoutMs)
	e.journal = fallback.String(cfg.Journal, defaultJournal)
	e.slowMs = fallback.Int(cfg.SlowMs, defaultSlowMs)
	e.maxRows = fallback.Int(cfg.MaxRows, defaultMaxRows)
	e.logArgs = cfg.LogArgs

	logger, ok := enginekit.Of[domainlog.Logger](ctx, "log")
	if !ok {
		return errLogUnavailable
	}
	e.logger = logger

	e.queue = dispatch.New(dispatch.Config{
		Shards:   fallback.Int(cfg.Shards, defaultShards),
		Inflight: fallback.Int(cfg.Inflight, defaultInflight),
		Queue:    fallback.Int(cfg.Queue, defaultQueue),
		Timeout:  time.Duration(fallback.Int(cfg.TimeoutMs, defaultTimeoutMs)) * time.Millisecond,
	})
	for cmd, h := range e.handlers() {
		if err := e.queue.Register(cmd, h); err != nil {
			return err
		}
	}
	return nil
}

// Start 打开 sqlite (含 pragmas) 并启动 worker.
func (e *Engine) Start(context.Context) error {
	db, err := openSQLite(e.path, e.busyTimeoutMs, e.journal)
	if err != nil {
		e.logger.Deliver(logkit.New(domainlog.LevelError, "database: open failed",
			logkit.F("path", e.path), logkit.F("err", err)))
		return err
	}
	e.db = db
	e.logger.Deliver(logkit.New(domainlog.LevelInfo, "database: opened",
		logkit.F("path", e.path), logkit.F("journal", e.journal), logkit.F("busyTimeoutMs", e.busyTimeoutMs)))

	// 迁移在建队列之前执行: 此刻只有本引擎持有连接, 没有并发写(见 v17 §4)
	if e.migrations != nil {
		if err := Migrate(e.migrations, e.migrateDir, e.path); err != nil {
			e.logger.Deliver(logkit.New(domainlog.LevelError, "database: migrate failed",
				logkit.F("dir", e.migrateDir), logkit.F("err", err)))
			return err
		}
		e.logger.Deliver(logkit.New(domainlog.LevelInfo, "database: migrated", logkit.F("dir", e.migrateDir)))
	}

	e.queue.Seal()
	e.queue.Launch()
	return nil
}

// Stop 停止接收, 排空队列并关闭连接.
func (e *Engine) Stop(ctx context.Context) error {
	var first error
	if err := e.queue.Close(ctx); err != nil {
		first = err
	}
	if e.db != nil {
		if err := e.db.Close(); err != nil && first == nil {
			first = err
		}
		e.logger.Deliver(logkit.New(domainlog.LevelInfo, "database: closed", logkit.F("path", e.path)))
	}
	return first
}

// Submit 实现 domain/store.Store.
func (e *Engine) Submit(t domainstore.Task) <-chan domainstore.Result { return e.queue.Submit(t) }

// Do 实现 domain/store.Store.
func (e *Engine) Do(ctx stdctx.Context, t domainstore.Task) (any, error) {
	return e.queue.Do(ctx, t)
}

// Stats 实现 domain/store.Statser.
func (e *Engine) Stats() domainstore.Stats { return e.queue.Stats() }

// Register 实现 domain/store.Registrar (供后续扩展指令).
func (e *Engine) Register(cmd domainstore.Cmd, h domainstore.Handler) error {
	return e.queue.Register(cmd, h)
}

// logOp 按结果分级记录: 失败 -> Error, 慢 -> Warn, 其余仅 logArgs 打开时 Debug.
func (e *Engine) logOp(op, sql string, args []any, dur time.Duration, err error) {
	if e.logger == nil {
		return
	}
	level := domainlog.LevelDebug
	switch {
	case err != nil:
		level = domainlog.LevelError
	case int(dur.Milliseconds()) >= e.slowMs:
		level = domainlog.LevelWarn
	case !e.logArgs:
		return
	}
	fields := []domainlog.Field{logkit.F("sql", sql), logkit.F("dur", dur)}
	if err != nil {
		fields = append(fields, logkit.F("err", err))
	}
	if e.logArgs {
		fields = append(fields, logkit.F("args", args))
	}
	e.logger.Deliver(logkit.New(level, op, fields...))
}

var (
	errConfigUnavailable = errors.New(`database: engine "config" unavailable`)
	errLogUnavailable    = errors.New(`database: engine "log" unavailable`)
)
