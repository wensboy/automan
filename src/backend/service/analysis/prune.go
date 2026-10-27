package analysis

// pruneOnce 按行数与时间双阈值清理 trace 表(见 v25 §3)。
func (s *Service) pruneOnce() {
	s.pruneTable("proxy_trace", s.params.ProxyRows)
	s.pruneTable("log_trace", s.params.LogRows)
	s.pruneTable("bridge_trace", s.params.BridgeRows)
}

func (s *Service) pruneTable(table string, maxRows int) {
	if maxRows > 0 {
		// 保留最新 maxRows 行: 删除 id <= 最大id - maxRows
		if _, err := s.exec(`DELETE FROM `+table+` WHERE id <= (SELECT COALESCE(MAX(id), 0) - ? FROM `+table+`)`, maxRows); err != nil {
			s.warn("analysis: prune by rows failed", fieldString("table", table), fieldErr(err))
		}
	}
	if s.params.MaxAge > 0 {
		cutoff := domainNowMillis() - s.params.MaxAge.Milliseconds()
		if _, err := s.exec(`DELETE FROM `+table+` WHERE created_at < ?`, cutoff); err != nil {
			s.warn("analysis: prune by age failed", fieldString("table", table), fieldErr(err))
		}
	}
}
