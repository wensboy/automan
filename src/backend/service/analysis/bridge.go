package analysis

import (
	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
)

// RecordBridge 记录一次桥接事件(连接成功/断开), 并落 bridge_trace。
func (s *Service) RecordBridge(event domainanalysis.BridgeEvent) {
	if event.Service == "" {
		return
	}
	if event.At == 0 {
		event.At = domainanalysis.NowMillis()
	}
	s.mu.Lock()
	events := append(s.bridgeEvents[event.Service], event)
	// 只保留窗口内事件, 避免无限增长
	cutoff := event.At - bridgeWindow.Milliseconds()
	trimmed := events[:0]
	for _, item := range events {
		if item.At >= cutoff {
			trimmed = append(trimmed, item)
		}
	}
	s.bridgeEvents[event.Service] = trimmed
	s.mu.Unlock()

	ok := 0
	if event.OK {
		ok = 1
	}
	if _, err := s.exec(`INSERT INTO bridge_trace (service, ok, created_at) VALUES (?, ?, ?)`,
		event.Service, ok, event.At); err != nil {
		s.warn("analysis: write bridge trace failed", fieldErr(err))
	}
}

// BridgeServices 返回可选服务(来自 bridge_trace + 当前事件)。
func (s *Service) BridgeServices() []domainanalysis.BridgeService {
	names := []string{}
	rows := []struct {
		Service string `db:"service"`
	}{}
	if err := s.query(&rows, `SELECT service FROM bridge_trace GROUP BY service ORDER BY service ASC`); err == nil {
		for _, row := range rows {
			names = append(names, row.Service)
		}
	}
	s.mu.RLock()
	for name := range s.bridgeEvents {
		found := false
		for _, existing := range names {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			names = append(names, name)
		}
	}
	s.mu.RUnlock()

	if len(names) == 0 {
		names = []string{"node"}
	}
	services := make([]domainanalysis.BridgeService, 0, len(names))
	for _, name := range names {
		services = append(services, domainanalysis.BridgeService{Value: name, Label: name})
	}
	return services
}

// bridgeMetric 返回某服务的 bridge 资源块。
func (s *Service) bridgeMetric(service string) domainanalysis.BridgeSnapshot {
	s.mu.RLock()
	samples := append([]domainanalysis.Sample{}, s.bridgeSeries[service]...)
	s.mu.RUnlock()

	return domainanalysis.BridgeSnapshot{
		Services: s.BridgeServices(),
		Current:  service,
		Series: []domainanalysis.SeriesDef{
			{Key: "rate", Label: "Normal rate", Unit: "%"},
			{Key: "peers", Label: "Peers", Unit: "count"},
			{Key: "errors", Label: "Errors", Unit: "count"},
		},
		Samples: samples,
	}
}
