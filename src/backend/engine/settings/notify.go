package settings

import (
	"sort"
	"sync"
	"time"
)

// notifier 合并高频变更, 按窗口批量广播; 窗口非正时退化为即时广播.
type notifier struct {
	window time.Duration
	emit   func(name string, value any)

	mu      sync.Mutex
	pending map[string]any

	signal chan struct{}
	done   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

func newNotifier(window time.Duration, emit func(name string, value any)) *notifier {
	return &notifier{
		window:  window,
		emit:    emit,
		pending: map[string]any{},
		signal:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
}

// start 启动合并协程.
func (n *notifier) start() {
	if n.window <= 0 {
		return
	}
	n.wg.Add(1)
	go n.loop()
}

// push 记录一次变更; 无窗口或已停止时立即广播, 避免静默丢失.
func (n *notifier) push(name string, value any) {
	if n.window <= 0 || n.stopped() {
		n.emit(name, value)
		return
	}
	n.mu.Lock()
	n.pending[name] = value
	n.mu.Unlock()
	select {
	case n.signal <- struct{}{}:
	default:
	}
}

// stop 停止合并并广播剩余变更.
func (n *notifier) stop() {
	n.once.Do(func() { close(n.done) })
	n.wg.Wait()
	n.flush()
}

func (n *notifier) stopped() bool {
	select {
	case <-n.done:
		return true
	default:
		return false
	}
}

func (n *notifier) loop() {
	defer n.wg.Done()
	for {
		select {
		case <-n.done:
			return
		case <-n.signal:
			timer := time.NewTimer(n.window)
			select {
			case <-timer.C:
			case <-n.done:
				timer.Stop()
				return
			}
			n.flush()
		}
	}
}

// flush 广播并清空待发变更 (按键排序, 保证确定性).
func (n *notifier) flush() {
	n.mu.Lock()
	pending := n.pending
	n.pending = map[string]any{}
	n.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	keys := make([]string, 0, len(pending))
	for name := range pending {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		n.emit(name, pending[name])
	}
}
