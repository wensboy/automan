package dispatch

import (
	"errors"
	"fmt"

	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// Register 注册指令 handler, 仅允许 Seal 之前注册.
func (q *Queue) Register(cmd domainstore.Cmd, h domainstore.Handler) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.sealed {
		return fmt.Errorf("dispatch: sealed, cannot register %q", cmd)
	}
	if _, dup := q.handlers[cmd]; dup {
		return fmt.Errorf("dispatch: duplicate cmd %q", cmd)
	}
	q.handlers[cmd] = h
	return nil
}

var (
	errNotStarted = errors.New("dispatch: not started")
	errClosed     = errors.New("dispatch: closed")
)
