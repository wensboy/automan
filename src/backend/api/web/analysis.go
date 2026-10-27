// Analysis 服务绑定: Home > Analysis 各视图的数据入口。
package web

import (
	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceanalysis "github.com/wensboy/automan/backend/service/analysis"
	servicesession "github.com/wensboy/automan/backend/service/session"
)

// AnalysisService 绑定到前端的分析服务。
type AnalysisService struct {
	ctx context.Context
}

// NewAnalysisService 创建绑定。
func NewAnalysisService(ctx context.Context) *AnalysisService {
	return &AnalysisService{ctx: ctx}
}

func (s *AnalysisService) service() (*serviceanalysis.Service, error) {
	svc, ok := srvkit.Of(s.ctx, serviceanalysis.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc, nil
}

// userID 取当前登录用户(分析数据按用户隔离)。
func (s *AnalysisService) userID() string {
	if session, ok := srvkit.Of(s.ctx, servicesession.Key); ok {
		return session.CurrentUserID()
	}
	return ""
}

// Workspaces 返回当前用户可选工作区(状态分布切换)。
func (s *AnalysisService) Workspaces() ([]domainanalysis.Workspace, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.Workspaces(s.userID())
}

// ProxyStatuses 返回 proxy 运行状态分布(workspaceId 为空 = 全部工作区)。
func (s *AnalysisService) ProxyStatuses(workspaceID string) ([]domainanalysis.StatusCount, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.ProxyStatuses(s.userID(), workspaceID)
}

// ProxyTotal 返回所有工作区的代理总数。
func (s *AnalysisService) ProxyTotal() (int, error) {
	svc, err := s.service()
	if err != nil {
		return 0, err
	}
	return svc.ProxyTotal(s.userID())
}

// ProxyActions 返回最近的 proxy 操作追踪。
func (s *AnalysisService) ProxyActions(limit int) ([]domainanalysis.ActionRow, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.ProxyActions(s.userID(), limit)
}

// ProxyTopK 返回启动频次 Top-K。
func (s *AnalysisService) ProxyTopK(k int) ([]domainanalysis.TopKItem, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.ProxyTopK(k)
}

// LogLevels 返回日志级别分布。
func (s *AnalysisService) LogLevels() ([]domainanalysis.LevelCount, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.LogLevels()
}

// LogDomains 返回日志领域统计。
func (s *AnalysisService) LogDomains(limit int) ([]domainanalysis.DomainRow, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.LogDomains(limit)
}

// LogIssues 返回时间范围内的 warn/error 追踪。
func (s *AnalysisService) LogIssues(rangeKey string, limit int) ([]domainanalysis.IssueRow, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.LogIssues(rangeKey, limit)
}

// LogStats 返回日志总数与缓冲。
func (s *AnalysisService) LogStats() (domainanalysis.LogStats, error) {
	svc, err := s.service()
	if err != nil {
		return domainanalysis.LogStats{}, err
	}
	return svc.LogStats(), nil
}

// SystemInfo 返回平台信息。
func (s *AnalysisService) SystemInfo() ([]domainanalysis.SystemItem, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.SystemInfo(), nil
}

// ServiceSnapshot 返回 service 指标快照(含所选 bridge 服务)。
func (s *AnalysisService) ServiceSnapshot(bridgeService string) (domainanalysis.ServiceSnapshot, error) {
	svc, err := s.service()
	if err != nil {
		return domainanalysis.ServiceSnapshot{}, err
	}
	return svc.Snapshot(bridgeService), nil
}
