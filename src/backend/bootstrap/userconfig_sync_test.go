package bootstrap

import (
	stdctx "context"
	"errors"
	"testing"
	"testing/fstest"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// fakeStore 是内存版 store.Store: 只实现同步测试需要的读/写/建目录.
type fakeStore struct {
	files       map[string][]byte
	mkdirCalled bool
}

func (s *fakeStore) Submit(domainstore.Task) <-chan domainstore.Result {
	ch := make(chan domainstore.Result, 1)
	ch <- domainstore.Result{}
	close(ch)
	return ch
}

func (s *fakeStore) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	switch task.Cmd {
	case domainstore.CmdRead:
		args, ok := task.Args.(domainstore.ReadArgs)
		if !ok {
			return nil, errors.New("bad read args")
		}
		data, ok := s.files[args.Path]
		if !ok {
			return nil, errors.New("not exist")
		}
		return data, nil
	case domainstore.CmdWrite:
		args, ok := task.Args.(domainstore.WriteArgs)
		if !ok {
			return nil, errors.New("bad write args")
		}
		s.files[args.Path] = args.Data
		return nil, nil
	case domainstore.CmdMkdir:
		s.mkdirCalled = true
		return nil, nil
	default:
		return nil, errors.New("unsupported cmd")
	}
}

type fakeContext struct{ files domainstore.Store }

func (c *fakeContext) Done() <-chan struct{}      { return nil }
func (c *fakeContext) Err() error                 { return nil }
func (c *fakeContext) Cancel()                    {}
func (c *fakeContext) Mode() domaincontext.Mode   { return domaincontext.ModeDesktop }
func (c *fakeContext) Register(string, any) error { return nil }
func (c *fakeContext) Srv(string) (any, bool)     { return nil, false }

func (c *fakeContext) Engine(name string) (any, bool) {
	if name != "store" {
		return nil, false
	}
	return c.files, true
}

// 首次运行: 用户文件由模板创建; schema 每次启动覆盖(随应用更新同步).
func TestSyncUserFilesCreatesThenOverwritesSchema(t *testing.T) {
	data := fstest.MapFS{
		templateSrc: &fstest.MapFile{Data: []byte(`{"appearence":{"mode":"auto"}}`)},
		schemaSrc:   &fstest.MapFile{Data: []byte(`{"title":"schema-v2"}`)},
		themeSrc:    &fstest.MapFile{Data: []byte(`[{"id":"default"}]`)},
	}
	files := &fakeStore{files: map[string][]byte{
		userSchemaFile: []byte("schema-v1"),
	}}
	ctx := &fakeContext{files: files}

	if err := syncUserFiles(ctx, data); err != nil {
		t.Fatalf("syncUserFiles: %v", err)
	}
	if got := string(files.files[userSettingsFile]); got != `{"appearence":{"mode":"auto"}}` {
		t.Fatalf("用户文件应由模板创建, got %q", got)
	}
	if got := string(files.files[userSchemaFile]); got != `{"title":"schema-v2"}` {
		t.Fatalf("schema 应被每次启动覆盖, got %q", got)
	}
	if got := string(files.files[themeFile]); got != `[{"id":"default"}]` {
		t.Fatalf("主题文件应被同步到用户目录, got %q", got)
	}
	if !files.mkdirCalled {
		t.Fatalf("应先确保用户目录存在")
	}
}

// 二次运行: 用户文件保持用户改动, schema 继续覆盖.
func TestSyncUserFilesKeepsUserFile(t *testing.T) {
	data := fstest.MapFS{
		templateSrc: &fstest.MapFile{Data: []byte(`{"appearence":{"mode":"auto"}}`)},
		schemaSrc:   &fstest.MapFile{Data: []byte(`{"title":"schema"}`)},
		themeSrc:    &fstest.MapFile{Data: []byte(`[{"id":"default"}]`)},
	}
	files := &fakeStore{files: map[string][]byte{
		userSettingsFile: []byte(`{"appearence":{"mode":"dark"}}`),
		userSchemaFile:   []byte("old"),
		themeFile:        []byte(`[{"id":"custom"}]`),
	}}
	if err := syncUserFiles(&fakeContext{files: files}, data); err != nil {
		t.Fatalf("syncUserFiles: %v", err)
	}
	if got := string(files.files[userSettingsFile]); got != `{"appearence":{"mode":"dark"}}` {
		t.Fatalf("用户文件不应被覆盖, got %q", got)
	}
	if got := string(files.files[userSchemaFile]); got != `{"title":"schema"}` {
		t.Fatalf("schema 应被覆盖, got %q", got)
	}
	if got := string(files.files[themeFile]); got != `[{"id":"custom"}]` {
		t.Fatalf("已有主题文件不应被覆盖, got %q", got)
	}
}
