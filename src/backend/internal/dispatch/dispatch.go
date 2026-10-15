// Package dispatch 提供指令式执行核心: 有界分区队列 + 串行 worker + 处理器注册表.
// store 与 database 共用本核心, 各自只贡献指令 handler.
package dispatch

import (
	stdctx "context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wensboy/automan/backend/domain/context"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
)

// Config 是执行器参数.
type Config struct {
	Shards   int
	Inflight int
	Queue    int
	Timeout  time.Duration
}

// Queue 实现 domain/store 的 Store / Registrar / Statser.
type Queue struct {
	mu       sync.RWMutex
	handlers map[domainstore.Cmd]domainstore.Handler
	sealed   bool
	closed   bool

	shards   []chan job
	inflight chan struct{}
	timeout  time.Duration

	submitted atomic.Uint64
	completed atomic.Uint64
	failed    atomic.Uint64
	dropped   atomic.Uint64

	wg sync.WaitGroup
}

// New 创建执行器.
func New(cfg Config) *Queue {
	shards := cfg.Shards
	if shards < 1 {
		shards = 1
	}
	inflight := cfg.Inflight
	if inflight < 1 {
		inflight = 1
	}
	buffer := cfg.Queue / shards
	if buffer < 1 {
		buffer = 1
	}
	q := &Queue{
		handlers: map[domainstore.Cmd]domainstore.Handler{},
		shards:   make([]chan job, shards),
		inflight: make(chan struct{}, inflight),
		timeout:  cfg.Timeout,
	}
	for i := range q.shards {
		q.shards[i] = make(chan job, buffer)
	}
	return q
}

// Submit 实现 domain/store.Store: 异步入队并返回结果句柄.
func (q *Queue) Submit(t domainstore.Task) <-chan domainstore.Result {
	return q.submit(stdctx.Background(), t)
}

// Do 实现 domain/store.Store: 投递并阻塞等待结果.
func (q *Queue) Do(ctx stdctx.Context, t domainstore.Task) (any, error) {
	if q.timeout > 0 {
		var cancel stdctx.CancelFunc
		ctx, cancel = stdctx.WithTimeout(ctx, q.timeout)
		defer cancel()
	}
	select {
	case r := <-q.submit(ctx, t):
		return r.Value, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Stats 实现 domain/store.Statser.
func (q *Queue) Stats() domainstore.Stats {
	q.mu.RLock()
	depth := 0
	for _, ch := range q.shards {
		depth += len(ch)
	}
	shards := len(q.shards)
	q.mu.RUnlock()
	return domainstore.Stats{
		Shards:    shards,
		Depth:     depth,
		Inflight:  len(q.inflight),
		Submitted: q.submitted.Load(),
		Completed: q.completed.Load(),
		Failed:    q.failed.Load(),
		Dropped:   q.dropped.Load(),
	}
}

// Seal 关闭注册: 之后 Register 一律报错.
func (q *Queue) Seal() {
	q.mu.Lock()
	q.sealed = true
	q.mu.Unlock()
}

// Launch 启动各分区 worker.
func (q *Queue) Launch() { q.launch() }

// Close 停止接收并排空队列 (worker 退出即排空完成).
func (q *Queue) Close(ctx context.Context) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return nil
	}
	q.closed = true
	for _, ch := range q.shards {
		close(ch)
	}
	q.mu.Unlock()

	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) submit(ctx stdctx.Context, t domainstore.Task) chan domainstore.Result {
	q.submitted.Add(1)
	reply := make(chan domainstore.Result, 1)
	if err := q.enqueue(ctx, t, reply); err != nil {
		q.dropped.Add(1)
		reply <- domainstore.Result{Err: err}
		close(reply)
	}
	return reply
}

func (q *Queue) shard(args any) int {
	key, _ := cast.As[domainstore.Keyed](args)
	h := fnv.New32a()
	if key != nil {
		_, _ = h.Write([]byte(key.Key()))
	}
	return int(h.Sum32() % uint32(len(q.shards)))
}
