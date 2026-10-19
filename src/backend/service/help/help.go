// Package help 提供离线帮助文档: 只读嵌入的 data/docs(meta.json + markdown), 不依赖数据库.
//
// 结构参照 data/docs/meta.json(**唯一标准, 不允许改结构**):
//   - meta.toc 定义目录; id 用于定位 markdown 文件(data/docs/<entry>/<id>.md)
//   - meta.remote 描述"网络获取"的三个 uri(当前**不实现**, 仅保留结构样例)
//
// 依赖: log(关键处理记日志) 与 ebus(预留, 当前无发布).
package help

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是帮助服务在 srvs 中的键.
var Key = key.New[*Service]("service.help")

// Meta 是 data/docs/meta.json 的解析形态(字段与文件一一对应).
type Meta struct {
	Version     string    `json:"version"`
	Entry       string    `json:"entry"`
	Remote      Remote    `json:"remote"`
	Author      string    `json:"author"`
	ReleaseDate string    `json:"releaseDate"`
	ModifyDate  string    `json:"modifyDate"`
	Toc         []TocItem `json:"toc"`
}

// Remote 是远程文档源的结构样例(当前不实现网络获取).
type Remote struct {
	BaseURL string  `json:"baseUrl"`
	URI     URIStub `json:"uri"`
}

// URIStub 是三个远程接口的路径样例.
type URIStub struct {
	MetaList string `json:"metaList"`
	Meta     string `json:"meta"`
	Docs     string `json:"docs"`
}

// TocItem 是目录的一级项.
type TocItem struct {
	ID    string    `json:"id"`
	Label string    `json:"label"`
	Sub   []TocLeaf `json:"sub"`
}

// TocLeaf 是目录的二级项.
type TocLeaf struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Doc 是一篇文档(标题来自目录, 内容为 markdown 原文).
type Doc struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
}

// Service 是帮助服务: 只读访问嵌入的文档树.
type Service struct {
	data   fs.FS
	logger domainlog.Logger
	bus    event.Bus // 预留: 当前不发事件(见包注释)
	now    func() time.Time
}

// New 创建帮助服务. data 为应用数据源: 内部固定读取 data/docs.
func New(data fs.FS, logger domainlog.Logger, bus event.Bus) *Service {
	return &Service{data: data, logger: logger, bus: bus, now: time.Now}
}

const (
	metaPath  = "data/docs/meta.json"
	docsRoot  = "data/docs"
	maxDocLen = 4 << 20 // 单篇 markdown 上限(防误放超大文件)
)

// Meta 返回文档元数据(目录 + 版本 + 远程结构样例).
func (s *Service) Meta(_ domaincontext.Context) (Meta, error) {
	payload, err := fs.ReadFile(s.data, metaPath)
	if err != nil {
		s.fail("meta", err)
		return Meta{}, errNotFound(metaPath)
	}
	var meta Meta
	if err := json.Unmarshal(payload, &meta); err != nil {
		s.fail("meta.parse", err)
		return Meta{}, errBadMeta(err)
	}
	if meta.Entry == "" {
		meta.Entry = "outline"
	}
	s.info("help: meta loaded", logkit.F("version", meta.Version), logkit.F("toc", len(meta.Toc)))
	return meta, nil
}

// Doc 读取一篇 markdown: id 对应 data/docs/<entry>/<id>.md;
// 二级 id(如 2-1)没有独立文件时回落到一级(2.md) —— 目录可先细分, 不必逐篇铺文件。
func (s *Service) Doc(_ domaincontext.Context, id string) (Doc, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" || strings.ContainsAny(trimmed, `/\`) {
		return Doc{}, errInvalidID(trimmed)
	}

	meta, err := s.Meta(nil)
	if err != nil {
		return Doc{}, err
	}
	label := findLabel(meta, trimmed)

	for _, candidate := range docCandidates(trimmed) {
		rel := path.Join(docsRoot, meta.Entry, candidate+".md")
		payload, readErr := fs.ReadFile(s.data, rel)
		if readErr != nil {
			continue
		}
		if len(payload) > maxDocLen {
			return Doc{}, errTooLarge(rel)
		}
		s.info("help: doc loaded", logkit.F("id", trimmed), logkit.F("path", rel), logkit.F("bytes", len(payload)))
		return Doc{ID: trimmed, Label: label, Content: string(payload)}, nil
	}

	s.fail("doc", fmt.Errorf("document %q not found", trimmed))
	return Doc{}, errNotFound(trimmed)
}

// docCandidates 返回候选文件名(不含扩展名): 自身, 以及去掉最后一级子编号后的父项.
func docCandidates(id string) []string {
	candidates := []string{id}
	if index := strings.LastIndex(id, "-"); index > 0 {
		candidates = append(candidates, id[:index])
	}
	return candidates
}

// findLabel 在目录里查 id 对应的标题(找不到时返回 id 本身).
func findLabel(meta Meta, id string) string {
	for _, item := range meta.Toc {
		if item.ID == id {
			return item.Label
		}
		for _, leaf := range item.Sub {
			if leaf.ID == id {
				return leaf.Label
			}
		}
	}
	return id
}

func (s *Service) info(msg string, fields ...domainlog.Field) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{Time: s.now(), Level: domainlog.LevelInfo, Msg: msg, Fields: fields})
	}
}

func (s *Service) fail(op string, err error) {
	if s.logger != nil {
		s.logger.Deliver(domainlog.Record{
			Time: s.now(), Level: domainlog.LevelError, Msg: "help: " + op + " failed",
			Fields: []domainlog.Field{logkit.F("err", err)},
		})
	}
}

func errNotFound(name string) error {
	return fmt.Errorf("%w: help document %q not found", apperr.New(apperr.CodeNotFound), name)
}

func errBadMeta(err error) error {
	return fmt.Errorf("%w: help meta invalid: %v", apperr.New(apperr.CodeServiceFail), err)
}

func errTooLarge(name string) error {
	return fmt.Errorf("%w: help document %q exceeds %d bytes", apperr.New(apperr.CodeServiceFail), name, maxDocLen)
}

func errInvalidID(id string) error {
	return fmt.Errorf("%w: invalid help document id %q", apperr.New(apperr.CodeServiceFail), id)
}
