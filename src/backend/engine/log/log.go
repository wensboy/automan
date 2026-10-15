// Package log 实现统一日志引擎: 投递式消息 + 多 out 分发 + log.sync 落盘(经 store).
package log

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/fallback"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 缺省参数, 可经 data/conf/log.json 覆盖.
const (
	defaultDir          = "logs"
	defaultFile         = "automan.log"
	defaultBuffer       = 2048
	defaultBatch        = 256
	defaultConsoleLevel = domainlog.LevelInfo
	defaultFileLevel    = domainlog.LevelDebug
)

// Engine 是统一日志引擎, 实现 domain/log 的 Logger/Registrar/Statser.
type Engine struct {
	mu     sync.RWMutex
	outs   []namedOut
	sealed bool

	ebus  event.Bus
	unsub func()

	delivered atomic.Uint64
	dropped   atomic.Uint64
	flushed   atomic.Uint64
}

type namedOut struct {
	name string
	out  domainlog.Out
}

// New 创建日志引擎.
func New() *Engine { return &Engine{} }

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "log" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return []string{"config", "eventbus", "store"} }

// Init 解析参数, 注册内置 out 并订阅落盘事件.
func (e *Engine) Init(ctx context.Context) error {
	provider, ok := enginekit.Of[config.Provider](ctx, "config")
	if !ok {
		return errConfigUnavailable
	}
	params, ok := configkit.Get(provider, config.LogKey)
	if !ok {
		return fmt.Errorf("log: config section %q missing", config.LogKey.Name())
	}
	store, ok := enginekit.Of[domainstore.Store](ctx, "store")
	if !ok {
		return errStoreUnavailable
	}
	ebus, ok := enginekit.Of[event.Bus](ctx, "eventbus")
	if !ok {
		return errEventBusUnavailable
	}

	consoleLevel, err := levelOrDefault(params.ConsoleLevel, defaultConsoleLevel)
	if err != nil {
		return err
	}
	fileLevel, err := levelOrDefault(params.FileLevel, defaultFileLevel)
	if err != nil {
		return err
	}

	if err := e.Register("console", newConsole(os.Stderr, consoleLevel)); err != nil {
		return err
	}
	disk := newDisk(store, diskConfig{
		path:   path.Join(fallback.String(params.Dir, defaultDir), fallback.String(params.File, defaultFile)),
		buffer: fallback.Int(params.Buffer, defaultBuffer),
		batch:  fallback.Int(params.Batch, defaultBatch),
		level:  fileLevel,
		report: e.report,
	})
	if err := e.Register("disk", disk); err != nil {
		return err
	}

	e.ebus = ebus
	e.unsub = ebus.Subscribe(event.TopicLogSync, func(any) { e.flush() })
	return nil
}

// Start 封口: 之后不再接受 out 注册.
func (e *Engine) Start(context.Context) error {
	e.mu.Lock()
	e.sealed = true
	e.mu.Unlock()
	return nil
}

// Stop 注销订阅并做末次落盘: 关机不丢日志 (生命周期动作, 非定时策略).
func (e *Engine) Stop(context.Context) error {
	if e.unsub != nil {
		e.unsub()
	}
	e.flush()
	return nil
}

// Register 实现 domain/log.Registrar, 仅允许 Start 之前注册.
func (e *Engine) Register(name string, out domainlog.Out) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sealed {
		return fmt.Errorf("log: sealed, cannot register %q", name)
	}
	for _, o := range e.outs {
		if o.name == name {
			return fmt.Errorf("log: duplicate out %q", name)
		}
	}
	e.outs = append(e.outs, namedOut{name: name, out: out})
	return nil
}

// Stats 实现 domain/log.Statser.
func (e *Engine) Stats() domainlog.Stats {
	e.mu.RLock()
	outs := e.outs
	e.mu.RUnlock()
	buffered := 0
	for _, o := range outs {
		if b, ok := o.out.(domainlog.Buffer); ok {
			buffered += b.Buffered()
		}
	}
	return domainlog.Stats{
		Delivered: e.delivered.Load(),
		Dropped:   e.dropped.Load(),
		Flushed:   e.flushed.Load(),
		Buffered:  buffered,
		Outs:      len(outs),
	}
}

func (e *Engine) flush() {
	e.mu.RLock()
	outs := e.outs
	e.mu.RUnlock()
	for _, o := range outs {
		f, ok := o.out.(domainlog.Flusher)
		if !ok {
			continue
		}
		if err := f.Flush(); err == nil {
			e.flushed.Add(1)
		}
	}
}

// report 汇总 out 的异常: 只有 overflow 真正丢数据; flush 失败仅上报 (记录仍留在缓冲).
func (e *Engine) report(reason event.LogDropReason, n int) {
	if n <= 0 {
		return
	}
	if reason == event.LogDropOverflow {
		e.dropped.Add(uint64(n))
	}
	if e.ebus != nil {
		e.ebus.Emit(event.TopicLogDropped, event.LogDropped{Reason: reason, Count: uint64(n)})
	}
}

func levelOrDefault(name string, def domainlog.Level) (domainlog.Level, error) {
	if name == "" {
		return def, nil
	}
	level, ok := logkit.ParseLevel(name)
	if !ok {
		return 0, fmt.Errorf("log: invalid level %q", name)
	}
	return level, nil
}

// format 把记录格式化为单行 (不含换行), 供各 out 复用.
func format(r domainlog.Record) string {
	var b strings.Builder
	b.WriteString(r.Time.Format(time.RFC3339Nano))
	b.WriteByte(' ')
	b.WriteString(strings.ToUpper(r.Level.String()))
	b.WriteByte(' ')
	b.WriteString(r.Msg)
	for _, f := range r.Fields {
		b.WriteByte(' ')
		b.WriteString(f.Key)
		b.WriteByte('=')
		fmt.Fprintf(&b, "%v", f.Value)
	}
	return b.String()
}

var (
	errConfigUnavailable   = errors.New(`log: engine "config" unavailable`)
	errStoreUnavailable    = errors.New(`log: engine "store" unavailable`)
	errEventBusUnavailable = errors.New(`log: engine "eventbus" unavailable`)
)
