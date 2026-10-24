package settings

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// 字体来源(见 docs/design/v23.md §42):
//   - builtin: 随包内置(默认 font.json 提供), 每项只需 id + path;
//   - system : 预留分组, 由用户/环境补充(同样只用 id + path), 不解析字体目录;
//   - options 把两组聚合后交给前端, 前端按 id -> path 应用为 CSS 字体栈。

// targetFont 是用户配置目录下的字体配置(与 theme.json 同一套"用户可改"思路)。
const targetFont = "font.json"

// targetFontFallback 是内置缺省(嵌入资源里随包分发)。
const targetFontFallback = "data/default/font.json"

// FontEntry 是一个可选字体(id 为设置值, path 为 CSS 字体栈)。
type FontEntry struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Group string `json:"group"` // builtin | system
}

// fontFile 是 font.json 的结构: 两个分组, 每组是 [{id, path}] 数组。
type fontFile struct {
	Builtin []fontItem `json:"builtin"`
	System  []fontItem `json:"system"`
}

type fontItem struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// WithData 注入内嵌资源(用于把缺省的 font.json 同步到用户配置目录)。
func (s *Service) WithData(data fs.FS) *Service {
	s.data = data
	return s
}

// Fonts 返回聚合后的字体候选(builtin 在前, system 在后, 同 id 去重)。
func (s *Service) Fonts(_ domaincontext.Context) ([]FontEntry, error) {
	payload, err := s.fontPayload()
	if err != nil {
		return nil, err
	}
	var parsed fontFile
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("settings: parse font file: %w", err)
	}

	entries := make([]FontEntry, 0, len(parsed.Builtin)+len(parsed.System))
	seen := map[string]bool{}
	appendGroup := func(group string, items []fontItem) {
		for _, item := range items {
			id := strings.TrimSpace(item.ID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			entries = append(entries, FontEntry{ID: id, Path: strings.TrimSpace(item.Path), Group: group})
		}
	}
	appendGroup("builtin", parsed.Builtin)
	appendGroup("system", parsed.System)
	return entries, nil
}

// fontPayload 读取配置目录的 font.json(由 bootstrap 与 theme.json 同样方式同步)。
func (s *Service) fontPayload() ([]byte, error) {
	if s.files != nil {
		if payload, err := storekit.Read(stdctx.Background(), s.files, targetFont); err == nil {
			return payload, nil
		}
	}
	// 用户文件缺失/不可读时仍可用(回落到内置缺省, 不写盘)
	if s.data != nil {
		if payload, err := fs.ReadFile(s.data, targetFontFallback); err == nil {
			return payload, nil
		}
	}
	return nil, fmt.Errorf("settings: font file %q not found", targetFont)
}

// Font 按 id 返回单个字体(前端针对"当前选中项"取 path 做热替换)。
func (s *Service) Font(_ domaincontext.Context, id string) (FontEntry, error) {
	fonts, err := s.Fonts(nil)
	if err != nil {
		return FontEntry{}, err
	}
	target := strings.TrimSpace(id)
	for _, font := range fonts {
		if font.ID == target {
			return font, nil
		}
	}
	return FontEntry{}, fmt.Errorf("settings: font %q not found", target)
}
