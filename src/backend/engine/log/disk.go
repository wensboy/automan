package log

import (
	stdctx "context"
	"strings"
	"sync"

	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// disk 是落盘 out: 先写入内存环形缓冲, 收到 log.sync 时经 store 批量追加.
type disk struct {
	store  domainstore.Store
	path   string
	level  domainlog.Level
	batch  int
	report func(event.LogDropReason, int)

	mu   sync.Mutex
	ring ring
}

type diskConfig struct {
	path   string
	buffer int
	batch  int
	level  domainlog.Level
	report func(event.LogDropReason, int)
}

func newDisk(store domainstore.Store, cfg diskConfig) *disk {
	return &disk{
		store:  store,
		path:   cfg.path,
		level:  cfg.level,
		batch:  cfg.batch,
		report: cfg.report,
		ring:   newRing(cfg.buffer),
	}
}

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

// Flush 实现 domain/log.Flusher: 批量追加落盘, 成功后才从缓冲移除.
func (d *disk) Flush() error {
	d.mu.Lock()
	batch := d.ring.peek(d.batch)
	d.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := storekit.Append(stdctx.Background(), d.store, d.path, encode(batch)); err != nil {
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
