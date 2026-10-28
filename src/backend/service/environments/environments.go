// Package environments 提供统一的环境变量读取与挂载点管理。
//
// 设计见 docs/design/v26.md:
//   - 文件即挂载点, 全部读写走 **store 引擎**(不直接操作 os 文件系统);
//   - 目录名/模板路径/条目上限/是否允许递归删除来自 **conf/environments**;
//   - 读多写少: RWMutex + 按路径缓存失效; 写入原子性由 store 引擎保证。
package environments

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
	"github.com/wensboy/automan/backend/domain/key"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
)

// Key 是服务注册键。
var Key = key.New[*Service]("environments")

const (
	defaultDir      = "environments"
	defaultTemplate = "data/default/env.tpl.json"
)

var errNoStore = errors.New("environments: store unavailable")

// Service 是环境变量服务。
type Service struct {
	store     domainstore.Store
	data      fs.FS
	dir       string
	template  string
	maxEntry  int
	recursive bool
	notifier  domainnotice.Publisher

	mu    sync.RWMutex
	cache map[string]domainenvironments.Document
}

// New 创建服务(依赖通过 With* 注入)。
func New() *Service {
	return &Service{
		dir:      defaultDir,
		template: defaultTemplate,
		cache:    map[string]domainenvironments.Document{},
	}
}

// WithStore 注入 store 引擎(文件读写统一入口)。
func (s *Service) WithStore(store domainstore.Store) *Service {
	s.store = store
	return s
}

// WithData 注入内嵌 data 目录(读取模板)。
func (s *Service) WithData(data fs.FS) *Service {
	s.data = data
	return s
}

// WithConfig 读取 conf/environments(缺省值兜底)。
func (s *Service) WithConfig(provider domainconfig.Provider) *Service {
	cfg, ok := configkit.Get[domainconfig.Environments](provider, domainconfig.EnvironmentsKey)
	if !ok {
		return s
	}
	if strings.TrimSpace(cfg.Dir) != "" {
		s.dir = strings.Trim(strings.TrimSpace(cfg.Dir), "/")
	}
	if strings.TrimSpace(cfg.Template) != "" {
		s.template = strings.TrimSpace(cfg.Template)
	}
	if cfg.MaxEntries > 0 {
		s.maxEntry = cfg.MaxEntries
	}
	s.recursive = cfg.AllowRecursiveDelete
	return s
}

// WithNotice 注入业务提示。
func (s *Service) WithNotice(publisher domainnotice.Publisher) *Service {
	s.notifier = publisher
	return s
}

// Init 确认 store 可用并创建根目录。
func (s *Service) Init() error {
	if s.store == nil {
		return errNoStore
	}
	return s.mkdir("")
}

// Tree 返回树形路径(目录优先, 名称升序)。
func (s *Service) Tree() ([]domainenvironments.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.treeLocked("")
}

// Read 读取单个挂载点(原文 + 解析值)。
func (s *Service) Read(rel string) (domainenvironments.Document, error) {
	normalized, err := normalizeFile(rel)
	if err != nil {
		return domainenvironments.Document{}, err
	}

	s.mu.RLock()
	if doc, ok := s.cache[normalized]; ok {
		s.mu.RUnlock()
		return doc, nil
	}
	s.mu.RUnlock()

	payload, err := s.readFile(normalized)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	doc, err := s.parseDocument(normalized, payload)
	if err != nil {
		return domainenvironments.Document{}, err
	}

	s.mu.Lock()
	s.cache[normalized] = doc
	s.mu.Unlock()
	return doc, nil
}

// CreateDir 创建分组目录(允许嵌套, 已存在报错)。
func (s *Service) CreateDir(rel string) (domainenvironments.Node, error) {
	normalized, err := normalizeDir(rel)
	if err != nil {
		return domainenvironments.Node{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := s.stat(normalized); statErr == nil {
		return domainenvironments.Node{}, domainenvironments.ErrExists
	}
	if err := s.mkdir(normalized); err != nil {
		return domainenvironments.Node{}, err
	}
	s.notify("Environment group created", normalized)
	return domainenvironments.Node{Name: path.Base(normalized), Path: normalized, Type: domainenvironments.NodeDir}, nil
}

// CreateFile 通过模板创建挂载点; entries 非空时覆盖模板条目。
func (s *Service) CreateFile(rel string, entries []domainenvironments.Entry) (domainenvironments.Document, error) {
	normalized, err := normalizeFile(rel)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	if s.maxEntry > 0 && len(entries) > s.maxEntry {
		return domainenvironments.Document{}, fmt.Errorf("%w: max %d entries", domainenvironments.ErrInvalidFile, s.maxEntry)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := s.stat(normalized); statErr == nil {
		return domainenvironments.Document{}, domainenvironments.ErrExists
	}
	if parent := path.Dir(normalized); parent != "." {
		if info, statErr := s.stat(parent); statErr != nil || !info.IsDir {
			return domainenvironments.Document{}, domainenvironments.ErrNotFound
		}
	}

	content, err := s.renderTemplate(normalized, entries)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	compact := compactJSON([]byte(content))
	doc, err := s.parseDocument(normalized, compact)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	if err := s.writeFile(normalized, compact); err != nil {
		return domainenvironments.Document{}, err
	}

	s.cache[normalized] = doc
	s.notify("Environment file created", normalized)
	return doc, nil
}

// DeleteDir 删除分组; recursive=false 时要求目录为空(递归受配置开关限制)。
func (s *Service) DeleteDir(rel string, recursive bool) error {
	normalized, err := normalizeDir(rel)
	if err != nil {
		return err
	}
	if recursive && !s.recursive {
		return domainenvironments.ErrRecursive
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	info, statErr := s.stat(normalized)
	if statErr != nil {
		return domainenvironments.ErrNotFound
	}
	if !info.IsDir {
		return domainenvironments.ErrInvalidPath
	}

	names, err := s.list(normalized)
	if err != nil {
		return err
	}
	if len(names) > 0 && !recursive {
		return domainenvironments.ErrDirNotEmpty
	}
	if recursive {
		if err := s.removeAll(normalized); err != nil {
			return err
		}
	} else if err := s.remove(normalized); err != nil {
		return err
	}

	s.invalidateLocked(normalized)
	s.notify("Environment group deleted", normalized)
	return nil
}

// DeleteFile 删除挂载点。
func (s *Service) DeleteFile(rel string) error {
	normalized, err := normalizeFile(rel)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, statErr := s.stat(normalized); statErr != nil {
		return domainenvironments.ErrNotFound
	}
	if err := s.remove(normalized); err != nil {
		return err
	}
	s.invalidateLocked(normalized)
	s.notify("Environment file deleted", normalized)
	return nil
}

// UpdateFile 保存编辑内容: 校验 -> 原子写 -> 缓存更新。
func (s *Service) UpdateFile(rel string, content string) (domainenvironments.Document, error) {
	normalized, err := normalizeFile(rel)
	if err != nil {
		return domainenvironments.Document{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := s.stat(normalized); statErr != nil {
		return domainenvironments.Document{}, domainenvironments.ErrNotFound
	}
	compact := compactJSON([]byte(content))
	doc, err := s.parseDocument(normalized, compact)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	if err := s.writeFile(normalized, compact); err != nil {
		return domainenvironments.Document{}, err
	}
	s.cache[normalized] = doc
	s.notify("Environment file saved", normalized)
	return doc, nil
}

// Rename 重命名文件或目录(store 不支持 rename, 采用 copy + remove)。
func (s *Service) Rename(from, to string) error {
	fromFile := strings.HasSuffix(strings.ToLower(from), ".json")
	var normalizedFrom, normalizedTo string
	var err error
	if fromFile {
		normalizedFrom, err = normalizeFile(from)
	} else {
		normalizedFrom, err = normalizeDir(from)
	}
	if err != nil {
		return err
	}
	if fromFile {
		normalizedTo, err = normalizeFile(to)
	} else {
		normalizedTo, err = normalizeDir(to)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, statErr := s.stat(normalizedFrom); statErr != nil {
		return domainenvironments.ErrNotFound
	}
	if _, statErr := s.stat(normalizedTo); statErr == nil {
		return domainenvironments.ErrExists
	}
	if err := s.copyTree(normalizedFrom, normalizedTo); err != nil {
		return err
	}
	if err := s.removeAll(normalizedFrom); err != nil {
		return err
	}
	s.invalidateLocked(normalizedFrom)
	s.invalidateLocked(normalizedTo)
	s.notify("Environment renamed", normalizedFrom+" -> "+normalizedTo)
	return nil
}

// Refresh 失效缓存并重新读取。
func (s *Service) Refresh(rel string) (domainenvironments.Document, error) {
	normalized, err := normalizeFile(rel)
	if err != nil {
		return domainenvironments.Document{}, err
	}
	s.mu.Lock()
	delete(s.cache, normalized)
	s.mu.Unlock()
	return s.Read(normalized)
}

// Values 汇总指定挂载点(为空则全部文件)的扁平值; 跨文件 key 冲突报错。
func (s *Service) Values(paths ...string) (map[string]string, error) {
	files := paths
	if len(files) == 0 {
		all, err := s.listAllFiles()
		if err != nil {
			return nil, err
		}
		files = all
	}

	merged := map[string]string{}
	origins := map[string]string{}
	for _, rel := range files {
		doc, err := s.Read(rel)
		if err != nil {
			return nil, err
		}
		for key, value := range doc.Values {
			if previous, ok := origins[key]; ok && previous != doc.Path {
				return nil, fmt.Errorf("%w: key %q in %q and %q", domainenvironments.ErrConflict, key, previous, doc.Path)
			}
			origins[key] = doc.Path
			merged[key] = value
		}
	}
	return merged, nil
}

func (s *Service) invalidateLocked(rel string) {
	delete(s.cache, rel)
	prefix := rel + "/"
	for key := range s.cache {
		if strings.HasPrefix(key, prefix) {
			delete(s.cache, key)
		}
	}
}

func (s *Service) notify(title, content string) {
	if s.notifier == nil {
		return
	}
	_, _ = s.notifier.Push(domainnotice.Entry{Level: string(domainnotice.LevelSuccess), Label: "environments", Title: title, Content: content})
}

var _ domainenvironments.Provider = (*Service)(nil)
