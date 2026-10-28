package analysis

import (
	"fmt"

	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// Workspaces 返回当前用户可选的工作区(状态分布切换)。
func (s *Service) Workspaces(userID string) ([]domainanalysis.Workspace, error) {
	rows := []domainanalysis.Workspace{}
	err := s.query(&rows, `SELECT id, name FROM workspaces WHERE user_id = ? AND deleted_at = 0 ORDER BY name ASC`, userID)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ProxyStatuses 返回 proxy 状态分布: workspaceID 为空时统计全部。
//
// 说明: 历史行的 workspace_id 可能为空(未归属), 因此用 LEFT JOIN 且放宽条件,
// 保证"未选择 workspace"或"选中的 workspace 没有 proxy"时仍能看到真实分布。
func (s *Service) ProxyStatuses(userID, workspaceID string) ([]domainanalysis.StatusCount, error) {
	rows := []domainanalysis.StatusCount{}
	sqlText := `SELECT p.status AS status, COUNT(*) AS count
		FROM proxies p LEFT JOIN workspaces w ON w.id = p.workspace_id
		WHERE p.deleted_at = 0`
	args := []any{}
	if workspaceID != "" {
		sqlText += ` AND p.workspace_id = ?`
		args = append(args, workspaceID)
	} else if userID != "" {
		sqlText += ` AND (w.user_id = ? OR p.workspace_id = '' OR w.id IS NULL)`
		args = append(args, userID)
	}
	sqlText += ` GROUP BY p.status`
	if err := s.query(&rows, sqlText, args...); err != nil {
		return nil, fmt.Errorf("analysis.statuses: %w", err)
	}
	return rows, nil
}

// ProxyTotal 返回该用户所有工作区的代理总数(footer badge)。
func (s *Service) ProxyTotal(userID string) (int, error) {
	rows := []struct {
		Total int `db:"total"`
	}{}
	sqlText := `SELECT COUNT(*) AS total FROM proxies p JOIN workspaces w ON w.id = p.workspace_id
		WHERE p.deleted_at = 0 AND p.workspace_id <> '' AND w.user_id <> ''`
	args := []any{}
	if userID != "" {
		sqlText += ` AND (w.user_id = ? OR p.workspace_id = '' OR w.id IS NULL)`
		args = append(args, userID)
	}
	err := s.query(&rows, sqlText, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Total, nil
}

// ProxyActions 返回最近的操作追踪(归属工作区名)。
func (s *Service) ProxyActions(userID string, limit int) ([]domainanalysis.ActionRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows := []domainanalysis.ActionRow{}
	sqlText := `SELECT t.id, t.proxy_id, t.name, COALESCE(w.name, '') AS workspace,
		t.listen, t.action, t.created_at
		FROM proxy_trace t
		LEFT JOIN proxies p ON p.id = t.proxy_id
		LEFT JOIN workspaces w ON w.id = COALESCE(p.workspace_id, t.workspace_id)
		WHERE 1 = 1`
	args := []any{}
	if userID != "" {
		sqlText += ` AND (w.user_id = ? OR t.workspace_id = '' OR p.id IS NULL)`
		args = append(args, userID)
	}
	sqlText += ` ORDER BY t.id DESC LIMIT ?`
	args = append(args, limit)
	err := s.query(&rows, sqlText, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ProxyTopK 返回启动频次最高的 proxy(全局, 不受 workspace 限制)。
func (s *Service) ProxyTopK(k int) ([]domainanalysis.TopKItem, error) {
	if k <= 0 || k > 100 {
		k = 5
	}
	rows := []domainanalysis.TopKItem{}
	err := s.query(&rows, `SELECT name, COUNT(*) AS count FROM proxy_trace WHERE action = ?
		GROUP BY name ORDER BY count DESC, name ASC LIMIT ?`, domainanalysis.ActionStart, k)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// LogLevels 返回级别分布。
func (s *Service) LogLevels() ([]domainanalysis.LevelCount, error) {
	rows := []domainanalysis.LevelCount{}
	err := s.query(&rows, `SELECT level, COUNT(*) AS count FROM log_trace GROUP BY level`)
	if err != nil {
		return nil, fmt.Errorf("analysis.levels: %w", err)
	}
	return rows, nil
}

// LogDomains 返回领域统计(总数 + 最近时间)。
func (s *Service) LogDomains(limit int) ([]domainanalysis.DomainRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows := []domainanalysis.DomainRow{}
	err := s.query(&rows, `SELECT ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) AS id,
		domain, COUNT(*) AS total, MAX(created_at) AS last
		FROM log_trace GROUP BY domain ORDER BY total DESC, domain ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// LogIssues 返回时间范围内的 warn/error 记录。
func (s *Service) LogIssues(rangeKey string, limit int) ([]domainanalysis.IssueRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = s.params.IssueLimit
	}
	since := domainanalysis.NowMillis() - rangeMillis(rangeKey)
	rows := []domainanalysis.IssueRow{}
	err := s.query(&rows, `SELECT id, level, message, extra, created_at FROM log_trace
		WHERE level IN (?, ?) AND created_at >= ? ORDER BY id DESC LIMIT ?`,
		domainlog.LevelWarn.String(), domainlog.LevelError.String(), since, limit)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// LogStats 返回日志累计总数(自启动)与当前缓冲大小。
func (s *Service) LogStats() domainanalysis.LogStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domainanalysis.LogStats{Total: s.logTotal, Buffer: s.staticBuffer}
}

// AttachLogStream 订阅日志二级总线: 批量落 log_trace 并累计总数。
func (s *Service) AttachLogStream(streamer domainlog.Streamer) error {
	if streamer == nil {
		return nil
	}
	_, err := streamer.SubscribeStream(func(records []domainlog.Record) {
		if len(records) == 0 {
			return
		}
		events := make([]domainanalysis.LogEvent, 0, len(records))
		for _, record := range records {
			events = append(events, domainanalysis.LogEvent{
				Level:   record.Level.String(),
				Domain:  DomainOf(record.Msg),
				Message: record.Msg,
				Extra:   joinFields(record.Fields),
				At:      record.Time.UnixMilli(),
			})
		}
		s.writeLogEvents(events)
	})
	return err
}

func (s *Service) writeLogEvents(events []domainanalysis.LogEvent) {
	if len(events) == 0 {
		return
	}
	// 单条多值 INSERT, 一个事务由 db.exec 保证
	sqlText := `INSERT INTO log_trace (level, domain, message, extra, created_at) VALUES ` +
		placeholders(len(events), 5)
	args := make([]any, 0, len(events)*5)
	for _, event := range events {
		args = append(args, event.Level, event.Domain, event.Message, event.Extra, event.At)
	}
	if _, err := s.exec(sqlText, args...); err != nil {
		s.warn("analysis: write log trace failed", logkit.F("err", err), logkit.F("rows", len(events)))
		return
	}
	s.mu.Lock()
	s.logTotal += int64(len(events))
	s.mu.Unlock()
}

func placeholders(rows, cols int) string {
	buf := ""
	for row := 0; row < rows; row++ {
		if row > 0 {
			buf += ","
		}
		buf += "("
		for col := 0; col < cols; col++ {
			if col > 0 {
				buf += ","
			}
			buf += "?"
		}
		buf += ")"
	}
	return buf
}

func joinFields(fields []domainlog.Field) string {
	if len(fields) == 0 {
		return ""
	}
	out := ""
	for _, field := range fields {
		if out != "" {
			out += " "
		}
		text := ""
		if value, ok := field.Value.(string); ok {
			text = value
		}
		out += field.Key + "=" + text
	}
	return out
}

// rangeMillis 把前端时间范围映射为毫秒(非法回落 1h)。
func rangeMillis(key string) int64 {
	switch key {
	case "0.5h":
		return 30 * 60 * 1000
	case "4h":
		return 4 * 60 * 60 * 1000
	case "8h":
		return 8 * 60 * 60 * 1000
	case "24h":
		return 24 * 60 * 60 * 1000
	default:
		return 60 * 60 * 1000
	}
}
