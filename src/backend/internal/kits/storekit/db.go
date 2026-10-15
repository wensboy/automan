package storekit

import (
	"context"

	"github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
)

// Exec 执行写语句.
func Exec(ctx context.Context, s store.Store, a store.ExecArgs) (store.ExecResult, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdExec, Args: a})
	if err != nil {
		return store.ExecResult{}, err
	}
	res, ok := cast.As[store.ExecResult](v)
	if !ok {
		return store.ExecResult{}, errResult
	}
	return res, nil
}

// Query 执行读语句.
func Query(ctx context.Context, s store.Store, a store.QueryArgs) (store.QueryResult, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdQuery, Args: a})
	if err != nil {
		return store.QueryResult{}, err
	}
	res, ok := cast.As[store.QueryResult](v)
	if !ok {
		return store.QueryResult{}, errResult
	}
	return res, nil
}

// Get 读取单行; 无行返回错误 (区别于空结果集).
func Get(ctx context.Context, s store.Store, a store.QueryArgs) (store.Row, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdGet, Args: a})
	if err != nil {
		return nil, err
	}
	row, ok := cast.As[store.Row](v)
	if !ok {
		return nil, errResult
	}
	return row, nil
}

// Tx 以原子方式执行多语句, 任一步失败整体回滚.
func Tx(ctx context.Context, s store.Store, a store.TxArgs) ([]store.ExecResult, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdTx, Args: a})
	if err != nil {
		return nil, err
	}
	res, ok := cast.As[[]store.ExecResult](v)
	if !ok {
		return nil, errResult
	}
	return res, nil
}
