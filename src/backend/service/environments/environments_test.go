package environments

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

const template = `{
  "meta": { "path": "{{dir}}/{{filename}}", "entry": "{{entry}}" },
  "environments": [
    { "type": "string", "key": "API_URL", "value": "http://127.0.0.1:8080" }
  ]
}
`

// fakeStore 按 store 引擎的语义在临时目录上实现所需指令(测试用)。
type fakeStore struct {
	root string
	mu   sync.Mutex
}

func (f *fakeStore) full(rel string) string { return filepath.Join(f.root, filepath.FromSlash(rel)) }

// Submit 同步执行并返回单值结果通道(实现 store.Store)。
func (f *fakeStore) Submit(task domainstore.Task) <-chan domainstore.Result {
	channel := make(chan domainstore.Result, 1)
	value, err := f.Do(context.Background(), task)
	channel <- domainstore.Result{Value: value, Err: err}
	close(channel)
	return channel
}

func (f *fakeStore) Do(_ context.Context, task domainstore.Task) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch task.Cmd {
	case domainstore.CmdRead:
		args := task.Args.(domainstore.ReadArgs)
		return os.ReadFile(f.full(args.Path))
	case domainstore.CmdWrite:
		args := task.Args.(domainstore.WriteArgs)
		if err := os.MkdirAll(filepath.Dir(f.full(args.Path)), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(f.full(args.Path), args.Data, 0o644)
	case domainstore.CmdMkdir:
		args := task.Args.(domainstore.MkdirArgs)
		return nil, os.MkdirAll(f.full(args.Path), 0o755)
	case domainstore.CmdRemove:
		args := task.Args.(domainstore.RemoveArgs)
		return nil, os.Remove(f.full(args.Path))
	case domainstore.CmdStat:
		args := task.Args.(domainstore.StatArgs)
		info, err := os.Stat(f.full(args.Path))
		if err != nil {
			return nil, err
		}
		return domainstore.FileInfo{Path: args.Path, Size: info.Size(), ModTime: info.ModTime(), IsDir: info.IsDir()}, nil
	case domainstore.CmdList:
		args := task.Args.(domainstore.ListArgs)
		entries, err := os.ReadDir(f.full(args.Dir))
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return names, nil
	default:
		return nil, errors.New("fakeStore: unsupported command")
	}
}

func newService(t *testing.T, tpl string) *Service {
	t.Helper()
	svc := New().
		WithStore(&fakeStore{root: t.TempDir()}).
		WithData(fstest.MapFS{"data/default/env.tpl.json": &fstest.MapFile{Data: []byte(tpl)}})
	if err := svc.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return svc
}

func TestCreateFileFromTemplate(t *testing.T) {
	svc := newService(t, template)
	if _, err := svc.CreateDir("default"); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	doc, err := svc.CreateFile("default/api.json", nil)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if doc.Values["API_URL"] == "" {
		t.Fatalf("模板未生效: %+v", doc)
	}
	if doc.Meta["path"] != "default/api.json" {
		t.Fatalf("meta.path = %q", doc.Meta["path"])
	}

	read, err := svc.Read("default/api.json")
	if err != nil || read.Values["API_URL"] != doc.Values["API_URL"] {
		t.Fatalf("read: %v %+v", err, read)
	}

	if _, err := svc.CreateFile("default/api.json", nil); !errors.Is(err, domainenvironments.ErrExists) {
		t.Fatalf("重名应报 ErrExists, got %v", err)
	}
}

func TestUnknownPlaceholder(t *testing.T) {
	svc := newService(t, `{"meta":{"path":"{{unknown}}","entry":"environments"},"environments":[]}`)
	if _, err := svc.CreateFile("api.json", nil); !errors.Is(err, domainenvironments.ErrUnknownPlace) {
		t.Fatalf("未知占位符应报错, got %v", err)
	}
}

func TestDeleteDirRequiresEmpty(t *testing.T) {
	svc := newService(t, template)
	if _, err := svc.CreateDir("default"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateFile("default/api.json", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteDir("default", false); !errors.Is(err, domainenvironments.ErrDirNotEmpty) {
		t.Fatalf("非空目录应拒绝, got %v", err)
	}
	if err := svc.DeleteDir("default", true); !errors.Is(err, domainenvironments.ErrRecursive) {
		t.Fatalf("默认配置应禁止递归, got %v", err)
	}
}

func TestRenameFile(t *testing.T) {
	svc := newService(t, template)
	if _, err := svc.CreateFile("a.json", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Rename("a.json", "b.json"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := svc.Read("b.json"); err != nil {
		t.Fatalf("rename 后应可读: %v", err)
	}
	if _, err := svc.Read("a.json"); !errors.Is(err, domainenvironments.ErrNotFound) {
		t.Fatalf("旧文件应消失, got %v", err)
	}
}

func TestValuesConflict(t *testing.T) {
	svc := newService(t, template)
	if _, err := svc.CreateFile("a.json", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateFile("b.json", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Values("a.json", "b.json"); !errors.Is(err, domainenvironments.ErrConflict) {
		t.Fatalf("同名 key 应冲突, got %v", err)
	}
}

func TestConcurrentRead(t *testing.T) {
	svc := newService(t, template)
	if _, err := svc.CreateFile("a.json", nil); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for index := 0; index < 50; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Read("a.json"); err != nil {
				t.Errorf("read: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestUpdateFile(t *testing.T) {
	svc := newService(t, template)
	doc, err := svc.CreateFile("a.json", nil)
	if err != nil {
		t.Fatal(err)
	}

	updated := strings.Replace(doc.Content, "http://127.0.0.1:8080", "http://10.0.0.1:80", 1)
	saved, err := svc.UpdateFile("a.json", updated)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if saved.Values["API_URL"] != "http://10.0.0.1:80" {
		t.Fatalf("保存后值未更新: %+v", saved.Values)
	}
	if strings.Contains(saved.Content, "\n") || strings.Contains(saved.Content, "\t") {
		t.Fatalf("存储/缓存应为压缩内容: %q", saved.Content)
	}

	// 缓存应同步更新
	cached, err := svc.Read("a.json")
	if err != nil || cached.Values["API_URL"] != "http://10.0.0.1:80" {
		t.Fatalf("缓存未更新: %v %+v", err, cached.Values)
	}

	if _, err := svc.UpdateFile("a.json", `{"meta":{}}`); !errors.Is(err, domainenvironments.ErrInvalidFile) {
		t.Fatalf("非法内容应拒绝, got %v", err)
	}
}
