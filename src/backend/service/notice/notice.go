// Package notice 是"运行时提示(notification)"的独立服务.
//
// 定位(见 docs/design/v23.md §28):
//   - 面向**人**的业务提示, 不持久化; 只保留最近若干条(内存环形);
//   - 前端 toast 与抽屉都来自同一条链路: 先投递到本服务 -> 服务广播事件 -> 前端追加 + toast;
//   - 可被后端服务直接调用(Push), 也可由前端经 api/web 触发(同一条路径)。
//
// 与日志的边界: 日志用于完整记录(落盘/流式), 提示只用于"值得看、值得回看"的业务信息。
package notice

import (
	"fmt"
	"strings"
	"sync"
	"time"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/domain/key"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// defaultCapacity 是队列上限: 超出丢最旧(提示不是审计)。
const defaultCapacity = 200

// Key 是提示服务在 srvs 中的键。
var Key = key.New[*Service]("service.notice")

// 编译期断言: 本服务实现领域契约。
var _ domainnotice.Publisher = (*Service)(nil)

// Entry 复用领域契约(绑定层字段保持一致)。
type Entry = domainnotice.Entry

// Service 持有提示队列与广播能力.
type Service struct {
	mu       sync.Mutex
	items    []Entry
	capacity int
	seq      int64

	ebus   event.Bus
	logger domainlog.Logger
	now    func() time.Time
	// dnd 返回"勿扰模式是否开启"(由设置项驱动); 开启时提示**既不排队也不弹 toast**。
	dnd func() bool
}

// New 创建提示服务(基础初始化; 依赖经 WithXxx 注入)。
func New() *Service {
	return &Service{capacity: defaultCapacity, now: time.Now, items: []Entry{}}
}

// WithEventBus 注入事件总线(广播 notice.appended / notice.cleared)。
func (s *Service) WithEventBus(bus event.Bus) *Service {
	s.ebus = bus
	return s
}

// WithLogger 注入日志出口(记录关键操作, 不记录每条内容)。
func (s *Service) WithLogger(logger domainlog.Logger) *Service {
	s.logger = logger
	return s
}

// WithDND 注入勿扰判定(每次 Push 实时读取设置, 改设置立即生效)。
func (s *Service) WithDND(enabled func() bool) *Service {
	s.dnd = enabled
	return s
}

// WithCapacity 覆盖队列容量(<=0 时保持默认)。
func (s *Service) WithCapacity(capacity int) *Service {
	if capacity > 0 {
		s.capacity = capacity
	}
	return s
}

// Push 投递一条提示, 返回落库形态(带 id/时间)。
//
// 校验: **标题必填** —— 它是给人看的一句话结论, 空标题只会渲染出空壳。
func (s *Service) Push(item domainnotice.Entry) (domainnotice.Entry, error) {
	entry := item
	title := strings.TrimSpace(entry.Title)
	if title == "" {
		return domainnotice.Entry{}, fmt.Errorf("notice: title is required")
	}
	level := strings.ToLower(strings.TrimSpace(entry.Level))
	if level == "" {
		level = "info"
	}

	// 勿扰: 直接丢弃(不入队、不广播) —— 否则"关掉勿扰"后会一次性刷出积压的提示
	if s.dnd != nil && s.dnd() {
		if s.logger != nil {
			s.logger.Deliver(logkit.New(domainlog.LevelInfo, "notice: suppressed (do not disturb)",
				logkit.F("level", level), logkit.F("label", entry.Label)))
		}
		return domainnotice.Entry{}, nil
	}

	s.mu.Lock()
	s.seq++
	now := s.now().UnixMilli()
	created := domainnotice.Entry{
		ID:      fmt.Sprintf("%d-%d", now, s.seq),
		Level:   level,
		Label:   strings.TrimSpace(entry.Label),
		Title:   title,
		Content: strings.TrimSpace(entry.Content),
		At:      now,
	}
	s.items = append([]domainnotice.Entry{created}, s.items...)
	if len(s.items) > s.capacity {
		s.items = s.items[:s.capacity]
	}
	s.mu.Unlock()

	if s.ebus != nil {
		s.ebus.Emit(event.TopicNoticeAppended, event.NoticeAppended{
			ID: created.ID, Label: created.Label, Title: created.Title,
			Content: created.Content, Level: created.Level, At: created.At,
		})
	}
	if s.logger != nil {
		s.logger.Deliver(logkit.New(domainlog.LevelInfo, "notice: pushed",
			logkit.F("level", created.Level), logkit.F("label", created.Label)))
	}
	return created, nil
}

// List 返回当前队列(最新在前)。
func (s *Service) List(_ domaincontext.Context) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domainnotice.Entry, len(s.items))
	copy(out, s.items)
	return out
}

// Clear 清空队列并广播(所有窗口同步清空)。
func (s *Service) Clear(_ domaincontext.Context) {
	s.mu.Lock()
	s.items = nil
	s.mu.Unlock()
	if s.ebus != nil {
		s.ebus.Emit(event.TopicNoticeCleared, event.NoticeCleared{At: s.now().UnixMilli()})
	}
}
