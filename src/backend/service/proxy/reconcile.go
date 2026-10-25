package proxy

import (
	domaincontext "github.com/wensboy/automan/backend/domain/context"
	"github.com/wensboy/automan/backend/domain/event"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// 状态对账(见 docs/design/v23.md §50):
//
//	代理是**子进程**, 进程消失后数据库里的 running 不会自动变; 若不处理会出现
//	"应用里显示 running, 实际端口没监听"的假状态。策略:
//	  · 应用启动时: 把历史遗留的 running 全部落回 stopped(此时不可能有本进程拉起的实例);
//	  · 桥接断开(node 子进程退出)时: 同样全部落回 stopped(实例随子进程一起消失)。

// WithEventBus 注入事件总线: 订阅 bridge.disconnected 做状态对账; 并立即对账一次(启动时).
func (s *Service) WithEventBus(bus event.Bus) *Service {
	if bus == nil {
		return s
	}
	bus.Subscribe(event.TopicBridgeDisconnected, func(any) { s.ResetRunning(nil) })
	// 启动对账: 上个进程残留的 running 一律视为已停止
	s.ResetRunning(nil)
	return s
}

// ResetRunning 把所有未删除的 running 记录落回 stopped, 返回受影响条数.
//
// 这是"进程级事实"的对账: 不区分用户/工作区(进程已经不存在, 状态没有保留意义)。
func (s *Service) ResetRunning(_ domaincontext.Context) int {
	now := s.now().UnixMilli()
	result, err := s.exec(`UPDATE proxies SET status = ?, updated_at = ? WHERE status = ? AND deleted_at = 0`,
		StatusStopped, now, StatusRunning)
	if err != nil {
		s.fail("reconcile", "", err)
		return 0
	}
	if result.RowsAffected > 0 {
		s.info("proxy: reconciled running -> stopped", logkit.F("rows", result.RowsAffected))
	}
	return int(result.RowsAffected)
}
