// Package certificate 维护 whistle 全局证书目录(certDir)与本地证书记录的一致性(见 docs/design/v27.md)。
//
// 事实链单向: certDir(文件系统) -> 服务记录(内存) -> 事件(UI 重载) -> 记录文件(磁盘)。
// 记录文件在 store root 下, 走 store 引擎; certDir 在 store root 之外, 其读写由 dirfs 直连 os。
package certificate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Key 是服务注册键。
var Key = key.New[*Service]("certificate")

const (
	defaultRoot     = "certs"
	defaultFile     = "certificates.json"
	defaultLimit    = 256
	defaultSyncMs   = 600000
	minSyncMs       = 60000
	defaultTemplate = "data/default/certificate.tpl.json"
)

var errNoStore = errors.New("certificate: store unavailable")

// Service 是证书服务。
type Service struct {
	store    domainstore.Store
	data     fs.FS
	logger   domainlog.Logger
	ebus     event.Bus
	notifier domainnotice.Publisher

	root      string
	file      string
	limit     int
	certDir   string
	syncEvery time.Duration
	template  string
	now       func() time.Time

	mu       sync.RWMutex
	records  []domaincertificate.Record
	nextID   int
	syncedAt string

	syncMu sync.Mutex
	stop   chan struct{}
	done   sync.WaitGroup
	once   sync.Once
}

// New 创建服务(依赖通过 With* 注入)。
func New() *Service {
	return &Service{
		root:      defaultRoot,
		file:      defaultFile,
		limit:     defaultLimit,
		syncEvery: time.Duration(defaultSyncMs) * time.Millisecond,
		template:  defaultTemplate,
		now:       time.Now,
		records:   []domaincertificate.Record{},
		nextID:    1,
		stop:      make(chan struct{}),
	}
}

// WithStore 注入 store 引擎(记录文件读写)。
func (s *Service) WithStore(store domainstore.Store) *Service {
	s.store = store
	return s
}

// WithData 注入内嵌 data 目录(读取模板)。
func (s *Service) WithData(data fs.FS) *Service {
	s.data = data
	return s
}

// WithLogger 注入日志。
func (s *Service) WithLogger(logger domainlog.Logger) *Service {
	s.logger = logger
	return s
}

// WithEventBus 注入事件总线(记录变化通知)。
func (s *Service) WithEventBus(bus event.Bus) *Service {
	s.ebus = bus
	return s
}

// WithNotice 注入业务提示(操作结果由服务层投递, 前端只展示)。
func (s *Service) WithNotice(publisher domainnotice.Publisher) *Service {
	s.notifier = publisher
	return s
}

// WithConfig 读取 conf/certificate(缺省值兜底)。
func (s *Service) WithConfig(provider domainconfig.Provider) *Service {
	cfg, ok := configkit.Get[domainconfig.Certificate](provider, domainconfig.CertificateKey)
	if !ok {
		return s
	}
	if strings.TrimSpace(cfg.Root) != "" {
		s.root = strings.Trim(strings.TrimSpace(cfg.Root), "/")
	}
	if strings.TrimSpace(cfg.File) != "" {
		s.file = strings.TrimSpace(cfg.File)
	}
	if cfg.Limit > 0 {
		s.limit = cfg.Limit
	}
	if cfg.SyncIntervalMs > 0 {
		s.syncEvery = time.Duration(max(cfg.SyncIntervalMs, minSyncMs)) * time.Millisecond
	}
	s.certDir = strings.TrimSpace(cfg.CertDir)
	return s
}

// Init 解析证书目录、载入记录、首次同步并启动后台同步协程。
func (s *Service) Init() error {
	if s.store == nil {
		return errNoStore
	}
	if s.data == nil {
		return errors.New("certificate: template unavailable")
	}
	dir, err := resolveCertDir(s.certDir)
	if err != nil {
		return err
	}
	s.certDir = dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("certificate: prepare cert dir: %w", err)
	}
	if err := s.load(); err != nil {
		return err
	}
	if _, err := s.sync(); err != nil {
		s.log(domainlog.LevelWarn, "certificate: initial sync failed", logkit.F("err", err))
	}
	s.done.Add(1)
	go s.loop()
	return nil
}

// Stop 停止后台协程并做终止同步(幂等)。
func (s *Service) Stop() {
	s.once.Do(func() {
		close(s.stop)
		s.done.Wait()
		if _, err := s.sync(); err != nil {
			s.log(domainlog.LevelWarn, "certificate: final sync failed", logkit.F("err", err))
		}
	})
}

// List 返回记录快照(ID 升序)。
func (s *Service) List() domaincertificate.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

// Read 返回指定记录的详情(记录 + 模板渲染原文)。
func (s *Service) Read(id int) (domaincertificate.Document, error) {
	s.mu.RLock()
	record, ok := s.findLocked(id)
	s.mu.RUnlock()
	if !ok {
		return domaincertificate.Document{}, domaincertificate.ErrNotFound
	}
	return s.document(record)
}

// Sync 立即按证书目录校正记录(前端手动同步与定时同步共用)。
// 定时同步只记日志; 这里代表用户手动触发, 失败时补一条 notice。
func (s *Service) Sync() (domaincertificate.Snapshot, error) {
	if _, err := s.sync(); err != nil {
		s.notice(domainnotice.LevelError, "Certificate sync failed", err.Error())
		return domaincertificate.Snapshot{}, err
	}
	return s.List(), nil
}

// Import 把本地证书文件复制进证书目录并登记记录。
func (s *Service) Import(source string) (document domaincertificate.Document, err error) {
	defer func() { s.report("Certificate imported", "Certificate import failed", document.Record.CertPath, err) }()

	source = strings.TrimSpace(source)
	if source == "" || !filepath.IsAbs(source) {
		return domaincertificate.Document{}, domaincertificate.ErrInvalidPath
	}
	base := filepath.Base(source)
	stem, fileType, ok := splitCertificateName(base)
	if !ok {
		return domaincertificate.Document{}, domaincertificate.ErrInvalidCert
	}
	payload, err := os.ReadFile(source)
	if err != nil {
		return domaincertificate.Document{}, fmt.Errorf("certificate: read source: %w", err)
	}

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	target := filepath.Join(s.certDir, base)
	if _, statErr := os.Stat(target); statErr == nil {
		return domaincertificate.Document{}, domaincertificate.ErrExists
	}
	if fileType == "key" {
		return s.attachKey(base, stem, payload)
	}
	if err := writeAtomic(target, payload); err != nil {
		return domaincertificate.Document{}, err
	}

	// 源目录若有同名 .key 且与本证书配对, 一并复制进来; keyPath 以目录里实际可用的私钥为准
	keyName := stem + ".key"
	if err := s.importSiblingKey(payload, source, keyName); err != nil {
		os.Remove(target)
		return domaincertificate.Document{}, err
	}

	now := s.now().Format(time.RFC3339)
	record := domaincertificate.Record{
		Name:      stem,
		Type:      fileType,
		CreatedAt: now,
		UpdatedAt: now,
		CertPath:  base,
		KeyPath:   s.pairedKey(payload, keyName),
		Valid:     certificateValid(payload, fileType),
		Tags:      []string{},
	}

	s.mu.Lock()
	record.ID = s.nextID
	next := append(append([]domaincertificate.Record(nil), s.records...), record)
	err = s.persist(next, s.syncedAt, s.nextID+1)
	if err == nil {
		s.records = next
		s.nextID++
	}
	s.mu.Unlock()
	if err != nil {
		// 记录未落盘: 撤销刚复制进来的文件, 保持目录与记录一致
		os.Remove(target)
		return domaincertificate.Document{}, err
	}
	s.emit("import", domaincertificate.Stats{})
	return s.document(record)
}

// attachKey 把私钥复制进证书目录并挂到同名证书记录上; 私钥必须与该证书成对。
func (s *Service) attachKey(base, stem string, payload []byte) (domaincertificate.Document, error) {
	paired, found := s.certificateRecord(stem)
	if !found {
		return domaincertificate.Document{}, fmt.Errorf("%w: private key requires a matching certificate", domaincertificate.ErrInvalidCert)
	}
	certificatePayload, err := os.ReadFile(filepath.Join(s.certDir, paired.CertPath))
	if err != nil {
		return domaincertificate.Document{}, fmt.Errorf("certificate: read certificate: %w", err)
	}
	if !privateKeyMatchesCertificate(certificatePayload, payload) {
		return domaincertificate.Document{}, fmt.Errorf("%w: private key does not match %s", domaincertificate.ErrInvalidCert, paired.CertPath)
	}
	target := filepath.Join(s.certDir, base)
	if err := writeAtomic(target, payload); err != nil {
		return domaincertificate.Document{}, err
	}

	s.mu.Lock()
	index := -1
	for position, record := range s.records {
		if record.ID == paired.ID {
			index = position
			break
		}
	}
	if index < 0 {
		s.mu.Unlock()
		os.Remove(target)
		return domaincertificate.Document{}, domaincertificate.ErrNotFound
	}
	next := append([]domaincertificate.Record(nil), s.records...)
	next[index].KeyPath = base
	next[index].UpdatedAt = s.now().Format(time.RFC3339)
	err = s.persist(next, s.syncedAt, s.nextID)
	if err == nil {
		s.records = next
	}
	record := next[index]
	s.mu.Unlock()
	if err != nil {
		os.Remove(target)
		return domaincertificate.Document{}, err
	}
	s.emit("import", domaincertificate.Stats{})
	return s.document(record)
}

// certificateRecord 按文件 stem 找证书记录(私钥配对的前提)。
func (s *Service) certificateRecord(stem string) (domaincertificate.Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, record := range s.records {
		if strings.TrimSuffix(record.CertPath, path.Ext(record.CertPath)) == stem {
			return record, true
		}
	}
	return domaincertificate.Record{}, false
}

// Update 只更新记录元数据(name/tags/valid), 不改动证书文件。
func (s *Service) Update(id int, content string) (document domaincertificate.Document, err error) {
	defer func() { s.report("Certificate saved", "Certificate save failed", document.Record.Name, err) }()

	// 编辑可能把外部私钥复制进证书目录, 与同步/导入串行化
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	s.mu.Lock()
	record, ok := s.findLocked(id)
	if !ok {
		s.mu.Unlock()
		return domaincertificate.Document{}, domaincertificate.ErrNotFound
	}
	updated, err := s.applyDocument(record, content)
	if err != nil {
		s.mu.Unlock()
		return domaincertificate.Document{}, err
	}
	updated.UpdatedAt = s.now().Format(time.RFC3339)
	next := make([]domaincertificate.Record, 0, len(s.records))
	for _, item := range s.records {
		if item.ID == id {
			item = updated
		}
		next = append(next, item)
	}
	err = s.persist(next, s.syncedAt, s.nextID)
	if err == nil {
		s.records = next
	}
	s.mu.Unlock()
	if err != nil {
		return domaincertificate.Document{}, err
	}
	s.emit("update", domaincertificate.Stats{})
	return s.document(updated)
}

// Export 把指定证书复制到 target(绝对路径, 可重命名)。
func (s *Service) Export(id int, target string) (err error) {
	defer func() { s.report("Certificate exported", "Certificate export failed", target, err) }()

	target = strings.TrimSpace(target)
	if target == "" || !filepath.IsAbs(target) {
		return domaincertificate.ErrInvalidPath
	}
	s.mu.RLock()
	record, ok := s.findLocked(id)
	s.mu.RUnlock()
	if !ok {
		return domaincertificate.ErrNotFound
	}
	if _, err := os.Stat(target); err == nil {
		return domaincertificate.ErrExists
	}
	payload, err := os.ReadFile(filepath.Join(s.certDir, record.CertPath))
	if err != nil {
		return fmt.Errorf("certificate: read source: %w", err)
	}
	return writeAtomic(target, payload)
}

// Delete 删除记录; removeFile=true 时连同证书目录里的文件一起删除。
func (s *Service) Delete(id int, removeFile bool) (err error) {
	detail := ""
	defer func() { s.report("Certificate deleted", "Certificate delete failed", detail, err) }()

	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	s.mu.Lock()
	record, ok := s.findLocked(id)
	if !ok {
		s.mu.Unlock()
		return domaincertificate.ErrNotFound
	}
	detail = record.Name
	if removeFile {
		if err := os.Remove(filepath.Join(s.certDir, record.CertPath)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.mu.Unlock()
			return fmt.Errorf("certificate: remove file: %w", err)
		}
	}
	next := make([]domaincertificate.Record, 0, len(s.records)-1)
	for _, item := range s.records {
		if item.ID != id {
			next = append(next, item)
		}
	}
	err = s.persist(next, s.syncedAt, s.nextID)
	if err == nil {
		s.records = next
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	s.emit("delete", domaincertificate.Stats{})
	return nil
}

func (s *Service) loop() {
	defer s.done.Done()
	ticker := time.NewTicker(s.syncEvery)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if _, err := s.sync(); err != nil {
				s.log(domainlog.LevelWarn, "certificate: sync failed", logkit.F("err", err))
			}
		}
	}
}

// sync 扫描证书目录并校正记录; 无变化时不落盘、不发事件。
func (s *Service) sync() (domaincertificate.Stats, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	files, matches, stats, err := scanDir(s.certDir)
	if err != nil {
		return stats, err
	}

	s.mu.Lock()
	next, changed, stats := mergeRecords(s.records, files, matches, s.now(), stats)
	if len(next) > s.limit {
		s.mu.Unlock()
		return stats, fmt.Errorf("%w: %d > %d", domaincertificate.ErrLimit, len(next), s.limit)
	}
	// mergeRecords 已把 id 压缩重排为连续 1..n, nextId 随之同步
	nextID := len(next) + 1
	if changed {
		syncedAt := s.now().Format(time.RFC3339)
		if err := s.persist(next, syncedAt, nextID); err != nil {
			s.mu.Unlock()
			return stats, err
		}
		s.records = next
		s.syncedAt = syncedAt
	}
	s.nextID = nextID
	s.mu.Unlock()

	s.log(domainlog.LevelInfo, "certificate: synced",
		logkit.F("scanned", stats.Scanned), logkit.F("added", stats.Added),
		logkit.F("removed", stats.Removed), logkit.F("updated", stats.Updated),
		logkit.F("skipped", stats.Skipped))
	if changed {
		s.emit("sync", stats)
	}
	return stats, nil
}

func (s *Service) snapshotLocked() domaincertificate.Snapshot {
	records := make([]domaincertificate.Record, len(s.records))
	copy(records, s.records)
	return domaincertificate.Snapshot{SyncedAt: s.syncedAt, Records: records}
}

func (s *Service) findLocked(id int) (domaincertificate.Record, bool) {
	for _, record := range s.records {
		if record.ID == id {
			return record, true
		}
	}
	return domaincertificate.Record{}, false
}

// importSiblingKey 把源文件同目录的同名 .key 一并复制进证书目录:
//   - 源私钥不存在 / 解不出 / 与本证书不配对 -> 忽略(不动目录里已有的同名文件);
//   - 目录里已有同名私钥 -> 不覆盖(是否采用由 pairedKey 判定); 否则原子复制。
func (s *Service) importSiblingKey(certificatePayload []byte, source, keyName string) error {
	payload, err := os.ReadFile(filepath.Join(filepath.Dir(source), keyName))
	if err != nil || !privateKeyMatchesCertificate(certificatePayload, payload) {
		return nil
	}
	target := filepath.Join(s.certDir, keyName)
	if _, statErr := os.Stat(target); statErr == nil {
		return nil
	}
	return writeAtomic(target, payload)
}

// pairedKey 返回证书目录里与该证书配对的私钥文件名(不存在或不配对时为空)。
func (s *Service) pairedKey(certificatePayload []byte, keyName string) string {
	payload, err := os.ReadFile(filepath.Join(s.certDir, keyName))
	if err != nil || !privateKeyMatchesCertificate(certificatePayload, payload) {
		return ""
	}
	return keyName
}

func (s *Service) emit(reason string, stats domaincertificate.Stats) {
	if s.ebus == nil {
		return
	}
	s.ebus.Emit(event.TopicCertificateChanged, event.CertificateChanged{
		Reason:  reason,
		Added:   stats.Added,
		Removed: stats.Removed,
		Updated: stats.Updated,
	})
}

func (s *Service) log(level domainlog.Level, msg string, fields ...domainlog.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(level, msg, fields...))
}

// notice 投递一条运行时提示(业务操作结束由服务层发起, 前端只负责展示)。
func (s *Service) notice(level domainnotice.Level, title, content string) {
	if s.notifier == nil {
		return
	}
	_, _ = s.notifier.Push(domainnotice.Entry{
		Level:   string(level),
		Label:   "certificate",
		Title:   title,
		Content: content,
	})
}

// report 统一收敛"成功/失败"两种结果的通知, 与业务方法的命名返回值配合使用。
func (s *Service) report(successTitle, errorTitle, detail string, err error) {
	if err != nil {
		s.notice(domainnotice.LevelError, errorTitle, err.Error())
		return
	}
	s.notice(domainnotice.LevelSuccess, successTitle, detail)
}

// sortRecords 按 ID 升序排列(记录文件与快照共用同一顺序)。
func sortRecords(records []domaincertificate.Record) {
	sort.Slice(records, func(left, right int) bool { return records[left].ID < records[right].ID })
}
