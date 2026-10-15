package store

import (
	stdctx "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/cast"
)

func (e *Engine) registerFS() error {
	handlers := map[domainstore.Cmd]domainstore.Handler{
		domainstore.CmdRead:   e.fsRead,
		domainstore.CmdWrite:  e.fsWrite,
		domainstore.CmdAppend: e.fsAppend,
		domainstore.CmdRemove: e.fsRemove,
		domainstore.CmdMkdir:  e.fsMkdir,
		domainstore.CmdStat:   e.fsStat,
		domainstore.CmdList:   e.fsList,
	}
	for cmd, h := range handlers {
		if err := e.Register(cmd, h); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) fsRead(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.ReadArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func (e *Engine) fsWrite(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.WriteArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	if err := e.checkSize(a.Data); err != nil {
		return nil, err
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	return nil, writeAtomic(p, a.Data)
}

func (e *Engine) fsAppend(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.AppendArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	if err := e.checkSize(a.Data); err != nil {
		return nil, err
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(a.Data); err != nil {
		return nil, err
	}
	return nil, nil
}

func (e *Engine) fsRemove(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.RemoveArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	return nil, os.Remove(p)
}

func (e *Engine) fsMkdir(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.MkdirArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	return nil, os.MkdirAll(p, 0o755)
}

func (e *Engine) fsStat(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.StatArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	p, err := e.path(a.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	return domainstore.FileInfo{
		Path:    a.Path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

func (e *Engine) fsList(_ stdctx.Context, args any) (any, error) {
	a, ok := cast.As[domainstore.ListArgs](args)
	if !ok {
		return nil, errBadArgs
	}
	p, err := e.path(a.Dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// path 把相对路径解析到存储根内, 拒绝绝对路径与任何向上逃逸.
func (e *Engine) path(rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", errEscaped
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errEscaped
	}
	return filepath.Join(e.root, clean), nil
}

func (e *Engine) checkSize(data []byte) error {
	if int64(len(data)) > e.maxSize {
		return fmt.Errorf("store: payload %d exceeds max %d", len(data), e.maxSize)
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

var (
	errBadArgs = errors.New("store: invalid task args")
	errEscaped = errors.New("store: path escapes root")
)
