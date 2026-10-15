// Package config 实现配置引擎: 加载静态配置, 经 ereg 提供取值能力.
package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/internal/cast"
	"github.com/wensboy/automan/backend/internal/flat"
)

// Engine 加载静态配置, 实现 domain/config.Provider.
type Engine struct {
	fsys   fs.FS
	dir    string
	values map[string]any
}

// Option 设置配置引擎的必要字段.
type Option func(*Engine)

// WithFS 指定配置文件的文件系统.
func WithFS(fsys fs.FS) Option { return func(e *Engine) { e.fsys = fsys } }

// WithDir 指定配置目录.
func WithDir(dir string) Option { return func(e *Engine) { e.dir = dir } }

// New 创建配置引擎.
func New(opts ...Option) *Engine {
	e := &Engine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name 实现 engine.Engine.
func (e *Engine) Name() string { return "config" }

// Deps 实现 engine.Engine.
func (e *Engine) Deps() []string { return nil }

// Init 实现 engine.Engine.
func (e *Engine) Init(context.Context) error { return nil }

// Start 加载目录下全部 json, 按 meta.namespace 平铺为 namespace.<path> 键.
func (e *Engine) Start(context.Context) error {
	files, err := fs.Glob(e.fsys, path.Join(e.dir, "*.json"))
	if err != nil {
		return fmt.Errorf("config: glob %s: %w", e.dir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("config: no json found under %s", e.dir)
	}
	values := make(map[string]any, len(files))
	for _, file := range files {
		raw, err := fs.ReadFile(e.fsys, file)
		if err != nil {
			return fmt.Errorf("config: read %s: %w", file, err)
		}
		var root map[string]any
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("config: decode %s: %w", file, err)
		}
		meta, ok := cast.As[domainconfig.Meta](root["meta"])
		if !ok || meta.Namespace == "" {
			return fmt.Errorf("config: %s missing meta.namespace", file)
		}
		delete(root, "meta")
		values[meta.Namespace] = root
		flat.Paths(values, meta.Namespace, root)
	}
	e.values = values
	return nil
}

// Stop 实现 engine.Engine.
func (e *Engine) Stop(context.Context) error {
	e.values = nil
	return nil
}

// Lookup 实现 domain/config.Provider, key 格式为 namespace.<path>.
func (e *Engine) Lookup(key string) (any, bool) {
	v, ok := e.values[key]
	return v, ok
}
