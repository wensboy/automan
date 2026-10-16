package log

import (
	stdctx "context"
	"errors"
	"sync"
	"testing"

	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// fakeStore 是内存版 store.Store, 只实现 disk out 用到的读/写/追加/建目录/元信息.
type fakeStore struct {
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{files: map[string][]byte{}, dirs: map[string]bool{}}
}

func (s *fakeStore) Submit(domainstore.Task) <-chan domainstore.Result {
	ch := make(chan domainstore.Result, 1)
	ch <- domainstore.Result{}
	close(ch)
	return ch
}

func (s *fakeStore) Do(_ stdctx.Context, task domainstore.Task) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch task.Cmd {
	case domainstore.CmdMkdir:
		args, _ := task.Args.(domainstore.MkdirArgs)
		s.dirs[args.Path] = true
		return nil, nil
	case domainstore.CmdWrite:
		args, _ := task.Args.(domainstore.WriteArgs)
		s.files[args.Path] = append([]byte(nil), args.Data...)
		return nil, nil
	case domainstore.CmdAppend:
		args, _ := task.Args.(domainstore.AppendArgs)
		s.files[args.Path] = append(s.files[args.Path], args.Data...)
		return nil, nil
	case domainstore.CmdStat:
		args, _ := task.Args.(domainstore.StatArgs)
		data, ok := s.files[args.Path]
		if !ok {
			return nil, errors.New("not exist")
		}
		return domainstore.FileInfo{Path: args.Path, Size: int64(len(data))}, nil
	default:
		return nil, errors.New("unsupported cmd")
	}
}

func (s *fakeStore) size(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.files[path])
}

func record(msg string) domainlog.Record { return logkit.New(domainlog.LevelInfo, msg) }

// 启动期 fs 处理: 建目录 + 建文件(缺失时).
func TestDiskPrepareCreatesDirAndFile(t *testing.T) {
	store := newFakeStore()
	d := newDisk(store, diskConfig{path: "logs/automan.log", buffer: 8, batch: 8, level: domainlog.LevelDebug})

	if err := d.prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !store.dirs["logs"] {
		t.Fatalf("应创建日志目录")
	}
	if _, ok := store.files["logs/automan.log"]; !ok {
		t.Fatalf("应创建日志文件")
	}
	if d.size != 0 {
		t.Fatalf("初始尺寸应为 0, got %d", d.size)
	}
}

// 启动时文件已超限: 直接重置为空.
func TestDiskPrepareResetsOversizedFile(t *testing.T) {
	store := newFakeStore()
	store.files["logs/automan.log"] = make([]byte, 2048)
	d := newDisk(store, diskConfig{path: "logs/automan.log", buffer: 8, batch: 8, maxSize: 1024, level: domainlog.LevelDebug})

	if err := d.prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if got := store.size("logs/automan.log"); got != 0 {
		t.Fatalf("启动时超限应重置, got %d", got)
	}
}

// 正常追加: 尺寸累加且内容保留.
func TestDiskAppendAccumulates(t *testing.T) {
	store := newFakeStore()
	d := newDisk(store, diskConfig{path: "logs/automan.log", buffer: 8, batch: 8, maxSize: 4096, level: domainlog.LevelDebug})
	if err := d.prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if err := d.Emit([]domainlog.Record{record("a")}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := d.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	first := store.size("logs/automan.log")
	if first == 0 || first != int(d.size) {
		t.Fatalf("尺寸计数应与文件一致, file=%d mem=%d", first, d.size)
	}

	if err := d.Emit([]domainlog.Record{record("b")}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if err := d.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := store.size("logs/automan.log"); got <= first {
		t.Fatalf("正常追加应增长, before=%d after=%d", first, got)
	}
}

// 循环文件: 追加会超出上限时, 文件被重置为"最新一批", 尺寸不超过上限.
func TestDiskRotatesWhenExceedingMaxSize(t *testing.T) {
	store := newFakeStore()
	// 上限 256 字节: 单条记录约 60~80 字节, 两批必然触发一次重置
	d := newDisk(store, diskConfig{path: "logs/automan.log", buffer: 64, batch: 16, maxSize: 256, level: domainlog.LevelDebug})
	if err := d.prepare(); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	fill := func(tag string) {
		records := make([]domainlog.Record, 0, 4)
		for i := 0; i < 4; i++ {
			records = append(records, record(tag+"-"+string(rune('a'+i))))
		}
		if err := d.Emit(records); err != nil {
			t.Fatalf("Emit: %v", err)
		}
		if err := d.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
	}

	fill("first")
	fill("second")
	fill("third")

	size := store.size("logs/automan.log")
	if int64(size) > 256 {
		t.Fatalf("循环文件不应超过上限, got %d", size)
	}
	content := string(store.files["logs/automan.log"])
	if content == "" {
		t.Fatalf("重置后应保留最新一批内容")
	}
	// 最新一批是 third-*, 之前的批次应已被丢弃
	if want := "third-a"; !contains(content, want) {
		t.Fatalf("应保留最新一批, got %q", content)
	}
	if contains(content, "first-a") {
		t.Fatalf("最旧内容应被丢弃, got %q", content)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
