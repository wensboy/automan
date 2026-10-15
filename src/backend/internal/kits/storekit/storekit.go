// Package storekit 提供 store 领域的消费侧辅助: 把指令包装为便捷调用.
package storekit

import (
	"context"
	"errors"

	"github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
)

// Read 读取整个文件.
func Read(ctx context.Context, s store.Store, path string) ([]byte, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdRead, Args: store.ReadArgs{Path: path}})
	if err != nil {
		return nil, err
	}
	data, ok := cast.As[[]byte](v)
	if !ok {
		return nil, errResult
	}
	return data, nil
}

// Write 以原子方式写入整个文件.
func Write(ctx context.Context, s store.Store, path string, data []byte) error {
	_, err := s.Do(ctx, store.Task{Cmd: store.CmdWrite, Args: store.WriteArgs{Path: path, Data: data}})
	return err
}

// Append 追加写入.
func Append(ctx context.Context, s store.Store, path string, data []byte) error {
	_, err := s.Do(ctx, store.Task{Cmd: store.CmdAppend, Args: store.AppendArgs{Path: path, Data: data}})
	return err
}

// Remove 删除文件或空目录.
func Remove(ctx context.Context, s store.Store, path string) error {
	_, err := s.Do(ctx, store.Task{Cmd: store.CmdRemove, Args: store.RemoveArgs{Path: path}})
	return err
}

// Mkdir 创建目录及其父级.
func Mkdir(ctx context.Context, s store.Store, path string) error {
	_, err := s.Do(ctx, store.Task{Cmd: store.CmdMkdir, Args: store.MkdirArgs{Path: path}})
	return err
}

// Stat 获取文件元信息.
func Stat(ctx context.Context, s store.Store, path string) (store.FileInfo, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdStat, Args: store.StatArgs{Path: path}})
	if err != nil {
		return store.FileInfo{}, err
	}
	info, ok := cast.As[store.FileInfo](v)
	if !ok {
		return store.FileInfo{}, errResult
	}
	return info, nil
}

// List 列出目录下的条目名.
func List(ctx context.Context, s store.Store, dir string) ([]string, error) {
	v, err := s.Do(ctx, store.Task{Cmd: store.CmdList, Args: store.ListArgs{Dir: dir}})
	if err != nil {
		return nil, err
	}
	names, ok := cast.As[[]string](v)
	if !ok {
		return nil, errResult
	}
	return names, nil
}

var errResult = errors.New("store: unexpected handler result")
