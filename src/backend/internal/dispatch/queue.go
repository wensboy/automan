package dispatch

import (
	stdctx "context"
	"fmt"

	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// job 是内部任务: 在指令之上附加执行上下文与结果回传.
type job struct {
	ctx   stdctx.Context
	cmd   domainstore.Cmd
	args  any
	reply chan domainstore.Result
}

func (q *Queue) launch() {
	for id := range q.shards {
		q.wg.Add(1)
		go q.work(id)
	}
}

// work 串行消费分区队列, 保证同键有序.
func (q *Queue) work(id int) {
	defer q.wg.Done()
	for j := range q.shards[id] {
		q.run(j)
	}
}

func (q *Queue) run(j job) {
	q.inflight <- struct{}{}
	defer func() { <-q.inflight }()

	var r domainstore.Result
	if h := q.handlers[j.cmd]; h == nil {
		r.Err = fmt.Errorf("dispatch: no handler for %q", j.cmd)
	} else {
		r.Value, r.Err = h(j.ctx, j.args)
	}
	if r.Err != nil {
		q.failed.Add(1)
	} else {
		q.completed.Add(1)
	}
	j.reply <- r
	close(j.reply)
}

// enqueue 计算分区并投递; 持读锁发送, 与 Close 的关队列互斥, 避免 send on closed channel.
func (q *Queue) enqueue(ctx stdctx.Context, t domainstore.Task, reply chan domainstore.Result) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if !q.sealed {
		return errNotStarted
	}
	if q.closed {
		return errClosed
	}
	select {
	case q.shards[q.shard(t.Args)] <- job{ctx: ctx, cmd: t.Cmd, args: t.Args, reply: reply}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
