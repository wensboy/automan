package log

import (
	stdctx "context"
	"path"
	"strings"
	"sync"

	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// disk 是落盘 out: 先写入内存环形缓冲, 收到 log.sync 时经 store 批量追加.
// 日志文件按 maxSize 循环: 追加会超出上限时, 整段重置为"最新一批"
// (即丢弃文件中最旧的内容), 见 docs/design/v16.md 第 7 节.
type disk struct {
	store   domainstore.Store
	path    string
	level   domainlog.Level
	batch   int
	maxSize int64 // 字节; <= 0 表示不限制
	report  func(event.LogDropReason, int)

	mu   sync.Mutex
	ring ring
	size int64 // 当前文件大小(内存计数, flush 时以 Stat 校正)
}

type diskConfig struct {
	path    string
	buffer  int
	batch   int
	level   domainlog.Level
	maxSize int64
	report  func(event.LogDropReason, int)
}

func newDisk(store domainstore.Store, cfg diskConfig) *disk {
	if cfg.report == nil {
		cfg.report = func(event.LogDropReason, int) {}
	}
	return &disk{
		store:   store,
		path:    cfg.path,
		level:   cfg.level,
		batch:   cfg.batch,
		maxSize: cfg.maxSize,
		report:  cfg.report,
		ring:    newRing(cfg.buffer),
	}
}

// Name 实现 domain/log.Out.
func (d *disk) Name() string { return domainlog.OutDisk }

// Emit 实现 domain/log.Out: 只入内存, 投递路径上不做任何 IO.
func (d *disk) Emit(records []domainlog.Record) error {
	d.mu.Lock()
	overflowed := 0
	for _, r := range records {
		if r.Level < d.level {
			continue
		}
		if d.ring.push(r) {
			overflowed++
		}
	}
	d.mu.Unlock()
	if overflowed > 0 {
		d.report(event.LogDropOverflow, overflowed)
	}
	return nil
}

// prepare 启动期 fs 处理: 建目录 -> 确保文件存在 -> 检查体积(见 v16 第 8 节).
func (d *disk) prepare() error {
	ctx := stdctx.Background()
	if dir := path.Dir(d.path); dir != "." && dir != "/" {
		if err := storekit.Mkdir(ctx, d.store, dir); err != nil {
			return err
		}
	}
	info, err := storekit.Stat(ctx, d.store, d.path)
	if err != nil {
		if err := storekit.Write(ctx, d.store, d.path, nil); err != nil {
			return err
		}
		d.mu.Lock()
		d.size = 0
		d.mu.Unlock()
		return nil
	}
	// 启动即超限: 直接重置(此刻没有"最新内容"可保留)
	size := info.Size
	if d.maxSize > 0 && size >= d.maxSize {
		if err := storekit.Write(ctx, d.store, d.path, nil); err != nil {
			return err
		}
		size = 0
	}
	d.mu.Lock()
	d.size = size
	d.mu.Unlock()
	return nil
}

// Flush 实现 domain/log.Flusher: 批量追加落盘, 成功后才从缓冲移除.
func (d *disk) Flush() error {
	d.mu.Lock()
	batch := d.ring.peek(d.batch)
	d.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := d.append(stdctx.Background(), encode(batch)); err != nil {
		d.report(event.LogDropFlush, len(batch))
		return err
	}
	d.mu.Lock()
	d.ring.pop(len(batch))
	d.mu.Unlock()
	return nil
}

// Buffered 实现 domain/log.Buffer.
func (d *disk) Buffered() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ring.size
}

// append 追加写入; 超出上限时重置为最新一段(循环文件), 并维护内存尺寸计数.
func (d *disk) append(ctx stdctx.Context, payload []byte) error {
	if d.maxSize <= 0 {
		return storekit.Append(ctx, d.store, d.path, payload)
	}
	// 外部可能改动/删除过文件: 每次 flush 用 Stat 校正一次(flush 频率低, 代价可接受)
	if info, err := storekit.Stat(ctx, d.store, d.path); err == nil {
		d.mu.Lock()
		d.size = info.Size
		d.mu.Unlock()
	}
	d.mu.Lock()
	size := d.size
	d.mu.Unlock()
	if size+int64(len(payload)) > d.maxSize {
		if err := storekit.Write(ctx, d.store, d.path, payload); err != nil {
			return err
		}
		d.mu.Lock()
		d.size = int64(len(payload))
		d.mu.Unlock()
		return nil
	}
	if err := storekit.Append(ctx, d.store, d.path, payload); err != nil {
		return err
	}
	d.mu.Lock()
	d.size += int64(len(payload))
	d.mu.Unlock()
	return nil
}

func encode(records []domainlog.Record) []byte {
	var b strings.Builder
	for _, r := range records {
		b.WriteString(format(r))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// ring 是有界环形缓冲: 满时覆盖最旧一条.
type ring struct {
	buf  []domainlog.Record
	head int // 最旧元素下标
	size int
}

func newRing(capacity int) ring { return ring{buf: make([]domainlog.Record, capacity)} }

// push 写入一条; 缓冲已满时覆盖最旧并返回 true.
func (r *ring) push(rec domainlog.Record) bool {
	if len(r.buf) == 0 {
		return false
	}
	if r.size == len(r.buf) {
		r.buf[r.head] = rec
		r.head = (r.head + 1) % len(r.buf)
		return true
	}
	r.buf[(r.head+r.size)%len(r.buf)] = rec
	r.size++
	return false
}

// peek 返回最旧的至多 n 条, 不改变缓冲.
func (r *ring) peek(n int) []domainlog.Record {
	if n > r.size {
		n = r.size
	}
	out := make([]domainlog.Record, n)
	for i := range out {
		out[i] = r.buf[(r.head+i)%len(r.buf)]
	}
	return out
}

// pop 移除最旧的 n 条.
func (r *ring) pop(n int) {
	if n > r.size {
		n = r.size
	}
	r.head = (r.head + n) % len(r.buf)
	r.size -= n
}
