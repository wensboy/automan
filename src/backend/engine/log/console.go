package log

import (
	"io"
	"sync"

	domainlog "github.com/wensboy/automan/backend/domain/log"
)

// console 是控制台 out: 立即格式化输出, 级别阈值自持.
type console struct {
	mu    sync.Mutex
	out   io.Writer
	level domainlog.Level
}

func newConsole(out io.Writer, level domainlog.Level) *console {
	return &console{out: out, level: level}
}

// Name 实现 domain/log.Out.
func (c *console) Name() string { return domainlog.OutConsole }

// Emit 实现 domain/log.Out.
func (c *console) Emit(records []domainlog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range records {
		if r.Level < c.level {
			continue
		}
		if _, err := io.WriteString(c.out, format(r)+"\n"); err != nil {
			return err
		}
	}
	return nil
}
