// Package analysis 是分析视图的领域契约: trace 记录、只读视图与采样结构。
//
// 设计见 docs/design/v25.md: service/analysis 负责写入与聚合, web 层只做绑定。
package analysis

import "time"

// ProxyAction 是 proxy 操作类型(写入 proxy_trace)。
const (
	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionStart   = "start"
	ActionStop    = "stop"
	ActionRestart = "restart"
)

// ProxyEvent 是一次 proxy 操作记录。
type ProxyEvent struct {
	ProxyID     string `json:"proxyId" db:"proxy_id"`
	Name        string `json:"name" db:"name"`
	Listen      string `json:"listen" db:"listen"`
	Action      string `json:"action" db:"action"`
	WorkspaceID string `json:"workspaceId" db:"workspace_id"`
	At          int64  `json:"at" db:"created_at"`
}

// ActionRow 是操作追踪的一行(含自增 id)。
type ActionRow struct {
	ID        int64  `json:"id" db:"id"`
	ProxyID   string `json:"proxyId" db:"proxy_id"`
	Name      string `json:"name" db:"name"`
	Listen    string `json:"listen" db:"listen"`
	Action    string `json:"action" db:"action"`
	Workspace string `json:"workspace" db:"workspace"`
	At        int64  `json:"at" db:"created_at"`
}

// TopKItem 是启动频次统计项。
type TopKItem struct {
	Name  string `json:"name" db:"name"`
	Count int    `json:"count" db:"count"`
}

// StatusCount 是状态分布项。
type StatusCount struct {
	Status string `json:"status" db:"status"`
	Count  int    `json:"count" db:"count"`
}

// LogEvent 是一条日志追踪记录。
type LogEvent struct {
	Level   string `json:"level" db:"level"`
	Domain  string `json:"domain" db:"domain"`
	Message string `json:"message" db:"message"`
	Extra   string `json:"extra" db:"extra"`
	At      int64  `json:"at" db:"created_at"`
}

// LevelCount 是级别分布项。
type LevelCount struct {
	Level string `json:"level" db:"level"`
	Count int    `json:"count" db:"count"`
}

// DomainRow 是领域统计行。
// Workspace 是可选工作区(状态分布切换用)。
type Workspace struct {
	ID   string `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type DomainRow struct {
	ID     int64  `json:"id" db:"id"`
	Domain string `json:"domain" db:"domain"`
	Total  int    `json:"total" db:"total"`
	Last   int64  `json:"last" db:"last"`
}

// IssueRow 是 warn 及以上的日志行。
type IssueRow struct {
	ID      int64  `json:"id" db:"id"`
	Level   string `json:"level" db:"level"`
	Message string `json:"message" db:"message"`
	Extra   string `json:"extra" db:"extra"`
	At      int64  `json:"at" db:"created_at"`
}

// LogStats 是日志总量与缓冲。
type LogStats struct {
	Total  int64 `json:"total"`
	Buffer int64 `json:"buffer"`
}

// BridgeEvent 是桥接事件(用于连接可用率)。
type BridgeEvent struct {
	Service string `json:"service"`
	OK      bool   `json:"ok"`
	At      int64  `json:"at"`
}

// BridgeService 是可选 bridge 服务。
type BridgeService struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SystemItem 是系统信息的一行。
type SystemItem struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SeriesDef 描述一个指标系列。
type SeriesDef struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Unit   string `json:"unit"`
	Digits int    `json:"digits"`
}

// Sample 是一个采样点。
type Sample struct {
	TS     int64              `json:"ts"`
	Values map[string]float64 `json:"values"`
}

// ResourceMetric 是单个资源的系列与窗口。
type ResourceMetric struct {
	Series  []SeriesDef `json:"series"`
	Samples []Sample    `json:"samples"`
}

// BridgeSnapshot 是 bridge 资源块。
type BridgeSnapshot struct {
	Services []BridgeService `json:"services"`
	Current  string          `json:"current"`
	Series   []SeriesDef     `json:"series"`
	Samples  []Sample        `json:"samples"`
}

// ServiceSnapshot 是 service 视图的一次快照。
type ServiceSnapshot struct {
	IntervalMs int64                     `json:"intervalMs"`
	PID        int                       `json:"pid"`
	Resources  map[string]ResourceMetric `json:"resources"`
	Bridge     BridgeSnapshot            `json:"bridge"`
}

// Recorder 由 analysis 服务实现, 供其它服务写入追踪记录(单向依赖)。
type Recorder interface {
	RecordProxy(event ProxyEvent)
}

// NowMillis 便于统一时间口径(测试可替换)。
var NowMillis = func() int64 { return time.Now().UnixMilli() }
