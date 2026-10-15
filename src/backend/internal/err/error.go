// Package error 提供基于 code 的统一错误注册与构造.
// 定义来源为 data/conf/error.json, code 唯一性由 Register 保证.
package err

import (
	"fmt"
	"maps"
	"sync"
)

// 内置 code, 与 data/conf/error.json 对应.
const (
	CodeNotFound    = 10000002
	CodeServiceFail = 20000001
)

// Entry 是一条错误定义.
type Entry struct {
	Code int
	Msg  string
}

// Error 是带 code 的错误.
type Error struct {
	Code int
	Msg  string
}

// Error 实现 error 接口.
func (e *Error) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }

// Register 注册错误定义, 构建并缓存 code -> *Error, code 必须唯一且为正.
func Register(entries []Entry) error {
	mu.Lock()
	defer mu.Unlock()
	pending := make(map[int]*Error, len(entries))
	for _, e := range entries {
		if e.Code <= 0 {
			return fmt.Errorf("err: invalid code %d", e.Code)
		}
		if _, dup := pending[e.Code]; dup {
			return fmt.Errorf("err: duplicate code %d", e.Code)
		}
		if _, dup := errs[e.Code]; dup {
			return fmt.Errorf("err: duplicate code %d", e.Code)
		}
		pending[e.Code] = &Error{Code: e.Code, Msg: e.Msg}
	}
	maps.Copy(errs, pending)
	return nil
}

// New 返回 code 对应的错误实例, 复用注册时构建的对象.
func New(code int) error {
	mu.RLock()
	err, ok := errs[code]
	mu.RUnlock()
	if !ok {
		return fmt.Errorf("err: unknown code %d", code)
	}
	return err
}

var (
	mu   sync.RWMutex
	errs = map[int]*Error{}
)
