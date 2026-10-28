// Package analysis 提供 Home > Analysis 的数据来源: trace 写入 + 统计聚合 + 采样。
//
// 设计见 docs/design/v25.md。约定:
//   - 只读聚合全部走 SQL(trace 表);
//   - service 指标来自 gopsutil 与 runtime;
//   - 采样与清理由本服务内部协程负责, 启动失败不阻断主链(由 bootstrap 记录)。
package analysis

import (
	stdctx "context"
	"strings"
	"sync"
	"time"

	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
	domainconfig "github.com/wensboy/automan/backend/domain/config"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 缺省参数(配置文件可覆盖)。
const (
	defaultSampleMs   = 10000
	defaultWindow     = 30
	defaultIssueLimit = 200
	defaultQueue      = 2048
	defaultProxyRows  = 50000
	defaultLogRows    = 200000
	defaultBridgeRows = 50000
	defaultMaxAgeHour = 168
)

// Key 是服务注册键。
var Key = key.New[*Service]("analysis")

// Params 是运行参数(来自 conf/analysis, 缺省兜底)。
type Params struct {
	SampleInterval time.Duration
	WindowPoints   int
	IssueLimit     int
	ProxyRows      int
	LogRows        int
	BridgeRows     int
	MaxAge         time.Duration
}

// Service 是分析服务。
type Service struct {
	db      domainstore.Store
	logger  domainlog.Logger
	params  Params
	started time.Time

	// proxy 事件队列(有界; 满则丢弃并计数)
	proxyQueue chan domainanalysis.ProxyEvent

	mu           sync.RWMutex
	logTotal     int64
	staticBuffer int64
	sampler      *sampler
	samples      map[string]domainanalysis.ResourceMetric
	bridgeEvents map[string][]domainanalysis.BridgeEvent
	bridgeSeries map[string][]domainanalysis.Sample

	stop chan struct{}
	once sync.Once
}

// New 创建分析服务。
func New() *Service {
	return &Service{
		proxyQueue:   make(chan domainanalysis.ProxyEvent, defaultQueue),
		samples:      map[string]domainanalysis.ResourceMetric{},
		bridgeEvents: map[string][]domainanalysis.BridgeEvent{},
		bridgeSeries: map[string][]domainanalysis.Sample{},
		stop:         make(chan struct{}),
	}
}

// WithStore 注入数据库。
func (s *Service) WithStore(db domainstore.Store) *Service { s.db = db; return s }

// WithLogger 注入日志。
func (s *Service) WithLogger(logger domainlog.Logger) *Service { s.logger = logger; return s }

// WithStaticBuffer 设置日志缓冲容量(来自 conf/log.buffer, 启动时固定)。
func (s *Service) WithStaticBuffer(capacity int) *Service {
	if capacity > 0 {
		s.staticBuffer = int64(capacity)
	}
	return s
}

// WithParams 应用配置(非法值回落缺省)。
func (s *Service) WithParams(cfg domainconfig.Analysis) *Service {
	s.params = Params{
		SampleInterval: time.Duration(positive(cfg.SampleIntervalMs, defaultSampleMs)) * time.Millisecond,
		WindowPoints:   positive(cfg.WindowPoints, defaultWindow),
		IssueLimit:     positive(cfg.IssueLimit, defaultIssueLimit),
		ProxyRows:      positive(cfg.Prune.ProxyRows, defaultProxyRows),
		LogRows:        positive(cfg.Prune.LogRows, defaultLogRows),
		BridgeRows:     positive(cfg.Prune.BridgeRows, defaultBridgeRows),
		MaxAge:         time.Duration(positive(cfg.Prune.MaxAgeHours, defaultMaxAgeHour)) * time.Hour,
	}
	return s
}

// Init 启动后台协程: proxy 写入、采样、清理。
func (s *Service) Init() error {
	if s.db == nil {
		return errNoDB
	}
	s.started = time.Now()
	go s.consumeProxyEvents()
	go s.loopSample()
	go s.loopPrune()
	return nil
}

// Stop 停止后台协程(幂等)。
func (s *Service) Stop() {
	s.once.Do(func() { close(s.stop) })
}

// RecordProxy 实现 domain/analysis.Recorder: 非阻塞投递(满则丢弃)。
func (s *Service) RecordProxy(event domainanalysis.ProxyEvent) {
	if event.At == 0 {
		event.At = domainanalysis.NowMillis()
	}
	select {
	case s.proxyQueue <- event:
	default:
		// 队列满: 直接丢弃(不保留只写不读的计数)
	}
}

func (s *Service) consumeProxyEvents() {
	for {
		select {
		case <-s.stop:
			return
		case event := <-s.proxyQueue:
			if _, err := s.exec(`INSERT INTO proxy_trace (proxy_id, name, listen, action, workspace_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				event.ProxyID, event.Name, event.Listen, event.Action, event.WorkspaceID, event.At); err != nil {
				s.warn("analysis: write proxy trace failed", logkit.F("err", err))
			}
		}
	}
}

func (s *Service) loopSample() {
	ticker := time.NewTicker(s.params.SampleInterval)
	defer ticker.Stop()
	s.sampleOnce()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.sampleOnce()
		}
	}
}

func (s *Service) loopPrune() {
	s.pruneOnce()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.pruneOnce()
		}
	}
}

// --- 基础设施 ---

var errNoDB = errorString("analysis: store unavailable")

type errorString string

func (e errorString) Error() string { return string(e) }

func (s *Service) warn(msg string, fields ...domainlog.Field) {
	if s.logger == nil {
		return
	}
	s.logger.Deliver(logkit.New(domainlog.LevelWarn, msg, fields...))
}

func (s *Service) exec(sqlText string, args ...any) (domainstore.ExecResult, error) {
	value, err := s.db.Do(stdctx.Background(), domainstore.Task{
		Cmd:  domainstore.CmdExec,
		Args: domainstore.ExecArgs{SQL: sqlText, Args: args},
	})
	if err != nil {
		return domainstore.ExecResult{}, err
	}
	result, _ := value.(domainstore.ExecResult)
	return result, nil
}

func (s *Service) query(into any, sqlText string, args ...any) error {
	_, err := s.db.Do(stdctx.Background(), domainstore.Task{
		Cmd:  domainstore.CmdQuery,
		Args: domainstore.QueryArgs{SQL: sqlText, Args: args, Into: into},
	})
	return err
}

// DomainOf 取日志消息里第一个 ':' 之前的标识(无冒号则为 app)。
func DomainOf(message string) string {
	index := strings.Index(message, ":")
	if index <= 0 {
		return "app"
	}
	domain := strings.TrimSpace(message[:index])
	if domain == "" {
		return "app"
	}
	return domain
}

func positive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// WithConfig 从 conf 读取参数(读不到用缺省)并注入静态日志缓冲容量。
func (s *Service) WithConfig(provider domainconfig.Provider) *Service {
	cfg, _ := configkit.Get[domainconfig.Analysis](provider, domainconfig.AnalysisKey)
	s.WithParams(cfg)
	if logCfg, ok := configkit.Get[domainconfig.Log](provider, domainconfig.LogKey); ok {
		s.WithStaticBuffer(logCfg.Buffer)
	}
	return s
}
