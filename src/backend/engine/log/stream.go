package log

import (
	"fmt"
	"sync"
	"time"

	"github.com/wensboy/automan/backend/domain/event"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// stream 是"送往外部消费者(前端终端)"的 out: 有界 FIFO + 批量窗口 + 溢出自持.
//
// 设计取舍见 docs/design/v16.md 第 3 节:
//   - 投递路径 Emit 只入队, 不做 IO/序列化, 不阻塞投递方;
//   - 冲刷在自己的 goroutine: 条数达 batch 立即发, 否则按 interval 定时发(空转不发);
//   - 溢出丢最旧; 丢弃数按"日志形式"随下一批下发(批首插一条 warn 记录), 并计入引擎 dropped;
//   - 引擎 Start 之前入队的记录直接丢弃 —— 只转发"程序运行启动后"的日志.
type stream struct {
	interval time.Duration
	batch    int
	capacity int
	report   func(event.LogDropReason, int)

	mu      sync.Mutex
	queue   []domainlog.Record
	dropped int
	subs    map[int]func([]domainlog.Record)
	nextID  int
	started bool

	notify chan struct{}
	stop   chan struct{}
	done   chan struct{}
}

type streamConfig struct {
	interval time.Duration
	batch    int
	queue    int
	report   func(event.LogDropReason, int)
}

func newStream(cfg streamConfig) *stream {
	if cfg.report == nil {
		cfg.report = func(event.LogDropReason, int) {}
	}
	return &stream{
		interval: cfg.interval,
		batch:    cfg.batch,
		capacity: cfg.queue,
		report:   cfg.report,
		subs:     map[int]func([]domainlog.Record){},
		notify:   make(chan struct{}, 1),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Name 实现 domain/log.Out.
func (s *stream) Name() string { return domainlog.OutStream }

// Emit 实现 domain/log.Out: 只入队; 达到批量阈值时唤醒冲刷.
func (s *stream) Emit(records []domainlog.Record) error {
	s.mu.Lock()
	if !s.started || s.capacity <= 0 {
		s.mu.Unlock()
		return nil
	}
	for _, record := range records {
		if len(s.queue) >= s.capacity {
			s.queue = s.queue[1:] // 丢最旧
			s.dropped++
		}
		s.queue = append(s.queue, record)
	}
	ready := len(s.queue) >= s.batch
	s.mu.Unlock()
	if ready {
		s.wake()
	}
	return nil
}

// Buffered 实现 domain/log.Buffer.
func (s *stream) Buffered() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// Subscribe 注册订阅者(即引擎内的"二级总线"); 返回取消函数.
// 订阅者由冲刷 goroutine 同步调用, 必须快速返回: 阻塞只会让队列积压(投递路径不受影响).
func (s *stream) Subscribe(fn func([]domainlog.Record)) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subs[id] = fn
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}
}

// start 开始接收与冲刷: 清空启动前的积压(只转发启动后的日志).
func (s *stream) start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.queue = nil
	s.dropped = 0
	s.mu.Unlock()
	go s.run()
}

// shutdown 停止冲刷 goroutine(幂等).
func (s *stream) shutdown() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()
	close(s.stop)
	<-s.done
}

func (s *stream) run() {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-s.notify:
			s.flush()
		case <-ticker.C:
			s.flush()
		}
	}
}

// wake 非阻塞唤醒冲刷(已有待处理唤醒时直接返回).
func (s *stream) wake() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// flush 取出一批(至多 batch 条)并扇出; 若期间有丢弃, 在批首插入一条 warn 记录.
func (s *stream) flush() {
	s.mu.Lock()
	if len(s.queue) == 0 {
		s.mu.Unlock()
		return
	}
	n := s.batch
	if n > len(s.queue) {
		n = len(s.queue)
	}
	batch := make([]domainlog.Record, n)
	copy(batch, s.queue[:n])
	s.queue = s.queue[n:]
	dropped := s.dropped
	s.dropped = 0
	s.mu.Unlock()

	if dropped > 0 {
		batch = append([]domainlog.Record{
			logkit.New(domainlog.LevelWarn, fmt.Sprintf("log: dropped %d records (overflow)", dropped)),
		}, batch...)
		s.report(event.LogDropOverflow, dropped)
	}
	s.fanout(batch)
}

func (s *stream) fanout(batch []domainlog.Record) {
	s.mu.Lock()
	subs := make([]func([]domainlog.Record), 0, len(s.subs))
	for _, fn := range s.subs {
		subs = append(subs, fn)
	}
	s.mu.Unlock()
	for _, fn := range subs {
		func() {
			defer func() { _ = recover() }() // 单个订阅者异常不影响其他订阅者与后续批次
			fn(batch)
		}()
	}
}
