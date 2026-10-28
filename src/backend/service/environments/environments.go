// Package environments 提供统一的环境变量读取与挂载点管理。
//
// 设计见 docs/design/v26.md:
//   - 文件即挂载点, 全部读写走 **store 引擎**(不直接操作 os 文件系统);
//   - 目录名/模板路径/条目上限/是否允许递归删除来自 **conf/environments**;
//   - 读多写少: RWMutex + 按路径缓存失效; 写入原子性由 store 引擎保证。
package environments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是服务注册键。
var Key = key.New[*Service]("environments")

const (
	defaultDir      = "environments"
	defaultTemplate = "data/default/env.tpl.json"
)

var (
	pathPattern     = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	placeholderExpr = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)
	errNoStore      = errors.New("environments: store unavailable")
)

// Service 是环境变量服务。
type Service struct {
	store     domainstore.Store
	data      fs.FS
	dir       string
	template  string
	maxEntry  int
	recursive bool
	logger    domainlog.Logger
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

// WithLogger 注入日志。
func (s *Service) WithLogger(logger domainlog.Logger) *Service {
	s.logger = logger
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
	doc, err := s.parseDocument(normalized, []byte(content))
	if err != nil {
		return domainenvironments.Document{}, err
	}
	compact := compactJSON([]byte(content))
	doc, err = s.parseDocument(normalized, compact)
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

// --- store 适配 ---

func (s *Service) do(cmd domainstore.Cmd, args any) (any, error) {
	if s.store == nil {
		return nil, errNoStore
	}
	return s.store.Do(context.Background(), domainstore.Task{Cmd: cmd, Args: args})
}

func (s *Service) fullPath(rel string) string {
	if rel == "" {
		return s.dir
	}
	return path.Join(s.dir, rel)
}

func (s *Service) readFile(rel string) ([]byte, error) {
	value, err := s.do(domainstore.CmdRead, domainstore.ReadArgs{Path: s.fullPath(rel)})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domainenvironments.ErrNotFound
		}
		return nil, err
	}
	payload, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("environments: unexpected read result")
	}
	return payload, nil
}

func (s *Service) writeFile(rel string, payload []byte) error {
	_, err := s.do(domainstore.CmdWrite, domainstore.WriteArgs{Path: s.fullPath(rel), Data: payload})
	return err
}

func (s *Service) mkdir(rel string) error {
	_, err := s.do(domainstore.CmdMkdir, domainstore.MkdirArgs{Path: s.fullPath(rel)})
	return err
}

func (s *Service) remove(rel string) error {
	_, err := s.do(domainstore.CmdRemove, domainstore.RemoveArgs{Path: s.fullPath(rel)})
	return err
}

func (s *Service) stat(rel string) (domainstore.FileInfo, error) {
	value, err := s.do(domainstore.CmdStat, domainstore.StatArgs{Path: s.fullPath(rel)})
	if err != nil {
		return domainstore.FileInfo{}, err
	}
	info, ok := value.(domainstore.FileInfo)
	if !ok {
		return domainstore.FileInfo{}, fmt.Errorf("environments: unexpected stat result")
	}
	return info, nil
}

func (s *Service) list(rel string) ([]string, error) {
	value, err := s.do(domainstore.CmdList, domainstore.ListArgs{Dir: s.fullPath(rel)})
	if err != nil {
		return nil, err
	}
	names, ok := value.([]string)
	if !ok {
		return nil, fmt.Errorf("environments: unexpected list result")
	}
	return names, nil
}

func (s *Service) treeLocked(rel string) ([]domainenvironments.Node, error) {
	names, err := s.list(rel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domainenvironments.Node{}, nil
		}
		return nil, err
	}

	nodes := make([]domainenvironments.Node, 0, len(names))
	for _, name := range names {
		childRel := path.Join(rel, name)
		info, statErr := s.stat(childRel)
		if statErr != nil {
			continue
		}
		if info.IsDir {
			children, childErr := s.treeLocked(childRel)
			if childErr != nil {
				return nil, childErr
			}
			nodes = append(nodes, domainenvironments.Node{Name: name, Path: childRel, Type: domainenvironments.NodeDir, Children: children})
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		nodes = append(nodes, domainenvironments.Node{Name: name, Path: childRel, Type: domainenvironments.NodeFile})
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == domainenvironments.NodeDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
	return nodes, nil
}

func (s *Service) listAllFiles() ([]string, error) {
	result := []string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		names, err := s.list(rel)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		for _, name := range names {
			childRel := path.Join(rel, name)
			info, statErr := s.stat(childRel)
			if statErr != nil {
				continue
			}
			if info.IsDir {
				if err := walk(childRel); err != nil {
					return err
				}
				continue
			}
			if strings.HasSuffix(strings.ToLower(name), ".json") {
				result = append(result, childRel)
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func (s *Service) copyTree(from, to string) error {
	info, err := s.stat(from)
	if err != nil {
		return err
	}
	if !info.IsDir {
		payload, readErr := s.readFile(from)
		if readErr != nil {
			return readErr
		}
		return s.writeFile(to, payload)
	}
	if err := s.mkdir(to); err != nil {
		return err
	}
	names, err := s.list(from)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.copyTree(path.Join(from, name), path.Join(to, name)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) removeAll(rel string) error {
	info, err := s.stat(rel)
	if err != nil {
		return err
	}
	if !info.IsDir {
		return s.remove(rel)
	}
	names, err := s.list(rel)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.removeAll(path.Join(rel, name)); err != nil {
			return err
		}
	}
	return s.remove(rel)
}

// compactJSON 去除空白(存储/缓存用), 失败时返回原始内容。
func compactJSON(payload []byte) []byte {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, payload); err != nil {
		return payload
	}
	return buffer.Bytes()
}

// --- 校验/模板/解析 ---

func normalizeDir(rel string) (string, error) {
	cleaned := strings.Trim(strings.TrimSpace(rel), "/")
	if cleaned == "" || strings.Contains(cleaned, "..") || !pathPattern.MatchString(cleaned) {
		return "", domainenvironments.ErrInvalidPath
	}
	if strings.HasSuffix(strings.ToLower(cleaned), ".json") {
		return "", domainenvironments.ErrInvalidPath
	}
	return path.Clean(cleaned), nil
}

func normalizeFile(rel string) (string, error) {
	cleaned := strings.Trim(strings.TrimSpace(rel), "/")
	if cleaned == "" || strings.Contains(cleaned, "..") || !pathPattern.MatchString(cleaned) {
		return "", domainenvironments.ErrInvalidPath
	}
	if !strings.HasSuffix(strings.ToLower(cleaned), ".json") || strings.HasSuffix(cleaned, "/") {
		return "", domainenvironments.ErrInvalidPath
	}
	return path.Clean(cleaned), nil
}

func (s *Service) renderTemplate(rel string, entries []domainenvironments.Entry) (string, error) {
	if s.data == nil {
		return "", fmt.Errorf("environments: template unavailable")
	}
	payload, err := fs.ReadFile(s.data, s.template)
	if err != nil {
		return "", fmt.Errorf("environments: read template: %w", err)
	}

	directory := path.Dir(rel)
	if directory == "." {
		directory = ""
	}
	values := map[string]string{
		"dir":      directory,
		"filename": path.Base(rel),
		"entry":    "environments",
	}
	entryName := values["entry"]

	var unknown string
	rendered := placeholderExpr.ReplaceAllStringFunc(string(payload), func(match string) string {
		name := placeholderExpr.FindStringSubmatch(match)[1]
		value, ok := values[name]
		if !ok {
			unknown = name
			return match
		}
		return value
	})
	if unknown != "" {
		return "", fmt.Errorf("%w: {{%s}}", domainenvironments.ErrUnknownPlace, unknown)
	}
	if len(entries) == 0 {
		return rendered, nil
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(rendered), &document); err != nil {
		return "", fmt.Errorf("%w: %v", domainenvironments.ErrInvalidFile, err)
	}
	document[entryName] = entries
	formatted, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return "", err
	}
	return string(formatted) + "\n", nil
}

func (s *Service) parseDocument(rel string, payload []byte) (domainenvironments.Document, error) {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return domainenvironments.Document{}, fmt.Errorf("%w: %v", domainenvironments.ErrInvalidFile, err)
	}
	metaRaw, ok := root["meta"].(map[string]any)
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: meta is missing", domainenvironments.ErrInvalidFile)
	}
	entry := strings.TrimSpace(fmt.Sprint(metaRaw["entry"]))
	if entry == "" || entry == "<nil>" {
		return domainenvironments.Document{}, fmt.Errorf("%w: meta.entry is missing", domainenvironments.ErrInvalidFile)
	}
	listRaw, ok := root[entry]
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q not found", domainenvironments.ErrInvalidFile, entry)
	}
	list, ok := listRaw.([]any)
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q must be an array", domainenvironments.ErrInvalidFile, entry)
	}
	if s.maxEntry > 0 && len(list) > s.maxEntry {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q exceeds %d items", domainenvironments.ErrInvalidFile, entry, s.maxEntry)
	}

	values := map[string]string{}
	entries := make([]domainenvironments.Entry, 0, len(list))
	for index, item := range list {
		record, ok := item.(map[string]any)
		if !ok {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d is not an object", domainenvironments.ErrInvalidFile, index+1)
		}
		key := strings.TrimSpace(fmt.Sprint(record["key"]))
		valueType := strings.TrimSpace(fmt.Sprint(record["type"]))
		if key == "" || key == "<nil>" {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d key is required", domainenvironments.ErrInvalidFile, index+1)
		}
		if !containsType(valueType) {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d unknown type %q", domainenvironments.ErrInvalidFile, index+1, valueType)
		}
		rendered, renderErr := renderValue(valueType, record["value"])
		if renderErr != nil {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d (%s): %v", domainenvironments.ErrInvalidFile, index+1, key, renderErr)
		}
		values[key] = rendered
		entries = append(entries, domainenvironments.Entry{Type: valueType, Key: key, Value: rendered})
	}

	meta := map[string]string{}
	for name, value := range metaRaw {
		meta[name] = fmt.Sprint(value)
	}

	return domainenvironments.Document{
		Path:    rel,
		Name:    path.Base(rel),
		Content: string(payload),
		Entry:   entry,
		Entries: entries,
		Values:  values,
		Meta:    meta,
	}, nil
}

func renderValue(valueType string, raw any) (string, error) {
	switch valueType {
	case "string":
		return fmt.Sprint(raw), nil
	case "int":
		number, err := toFloat(raw)
		if err != nil || number != float64(int64(number)) {
			return "", fmt.Errorf("value is not an integer")
		}
		return strconv.FormatInt(int64(number), 10), nil
	case "float":
		number, err := toFloat(raw)
		if err != nil {
			return "", fmt.Errorf("value is not a number")
		}
		return strconv.FormatFloat(number, 'f', -1, 64), nil
	case "bool":
		boolean, err := toBool(raw)
		if err != nil {
			return "", err
		}
		return strconv.FormatBool(boolean), nil
	case "array":
		value, err := toJSONValue(raw)
		if err != nil {
			return "", err
		}
		if _, ok := value.([]any); !ok {
			return "", fmt.Errorf("value is not an array")
		}
		payload, _ := json.Marshal(value)
		return string(payload), nil
	default:
		value, err := toJSONValue(raw)
		if err != nil {
			return "", err
		}
		if _, ok := value.(map[string]any); !ok {
			return "", fmt.Errorf("value is not an object")
		}
		payload, _ := json.Marshal(value)
		return string(payload), nil
	}
}

func toFloat(raw any) (float64, error) {
	switch value := raw.(type) {
	case float64:
		return value, nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(value), 64)
	default:
		return 0, fmt.Errorf("value is not numeric")
	}
}

func toBool(raw any) (bool, error) {
	switch value := raw.(type) {
	case bool:
		return value, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(value))
	default:
		return false, fmt.Errorf("value is not a boolean")
	}
}

func toJSONValue(raw any) (any, error) {
	if text, ok := raw.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("value is not valid JSON: %v", err)
		}
		return decoded, nil
	}
	return raw, nil
}

func containsType(value string) bool {
	for _, candidate := range domainenvironments.Types {
		if candidate == value {
			return true
		}
	}
	return false
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

func (s *Service) warn(message string, err error) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(domainlog.LevelWarn, message, logkit.F("err", err)))
}

var _ domainenvironments.Provider = (*Service)(nil)
