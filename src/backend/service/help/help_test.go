package help

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func newTestService() *Service {
	data := fstest.MapFS{
		"data/docs/meta.json": &fstest.MapFile{Data: []byte(`{
            "version": "v0.1.0-outline",
            "entry": "outline",
            "toc": [
                {"id": "1", "label": "Quick Start", "sub": []},
                {"id": "2", "label": "Download", "sub": [{"id": "2-1", "label": "Release"}]}
            ]
        }`)},
		"data/docs/outline/1.md": &fstest.MapFile{Data: []byte("# Quick Start")},
		"data/docs/outline/2.md": &fstest.MapFile{Data: []byte("# Download")},
	}
	return New().WithData(data)
}

// meta 能解析出目录与版本(结构以 data/docs/meta.json 为准).
func TestMetaParsesToc(t *testing.T) {
	meta, err := newTestService().Meta(nil)
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if meta.Version != "v0.1.0-outline" || meta.Entry != "outline" || len(meta.Toc) != 2 {
		t.Fatalf("meta 解析异常: %+v", meta)
	}
	if meta.Toc[1].Sub[0].ID != "2-1" {
		t.Fatalf("二级目录异常: %+v", meta.Toc[1])
	}
}

// 一级 id 直接命中文件; 二级 id 回落到一级文件; 标题取自目录.
func TestDocReadsMarkdownWithFallback(t *testing.T) {
	svc := newTestService()

	doc, err := svc.Doc(nil, "1")
	if err != nil {
		t.Fatalf("Doc(1): %v", err)
	}
	if doc.Label != "Quick Start" || !strings.Contains(doc.Content, "Quick Start") {
		t.Fatalf("doc 内容/标题异常: %+v", doc)
	}

	sub, err := svc.Doc(nil, "2-1")
	if err != nil {
		t.Fatalf("二级 id 应回落到 2.md: %v", err)
	}
	if sub.Label != "Release" || !strings.Contains(sub.Content, "Download") {
		t.Fatalf("回落结果异常: %+v", sub)
	}
}

// 不存在的文档: 明确 not found; 非法 id: 拒绝(不允许路径穿越).
func TestDocRejectsMissingAndInvalid(t *testing.T) {
	svc := newTestService()
	if _, err := svc.Doc(nil, "9"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("缺失文档应报 not found, got %v", err)
	}
	if _, err := svc.Doc(nil, "../meta"); err == nil {
		t.Fatal("非法 id 应被拒绝")
	}
}

// 用仓库里**真实的** data/docs 校验: meta 能解析, 目录首项能读出 markdown。
func TestRealDocsAreReadable(t *testing.T) {
	svc := New().WithData(os.DirFS("../../..")) // ../../.. = src/
	meta, err := svc.Meta(nil)
	if err != nil {
		t.Fatalf("真实 meta 解析失败: %v", err)
	}
	if len(meta.Toc) < 5 || meta.Entry == "" {
		t.Fatalf("真实目录异常: entry=%q toc=%d", meta.Entry, len(meta.Toc))
	}
	doc, err := svc.Doc(nil, meta.Toc[0].ID)
	if err != nil {
		t.Fatalf("读取首篇文档失败: %v", err)
	}
	if strings.TrimSpace(doc.Content) == "" {
		t.Fatalf("首篇文档内容为空: %+v", doc)
	}
	// 二级目录 id 应回落到一级文件(样例: 2-1 -> 2.md)
	if len(meta.Toc[1].Sub) > 0 {
		sub, err := svc.Doc(nil, meta.Toc[1].Sub[0].ID)
		if err != nil || strings.TrimSpace(sub.Content) == "" {
			t.Fatalf("二级 id 回落失败: %+v err=%v", sub, err)
		}
	}
}
