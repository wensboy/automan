package analysis

import (
	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

func domainNowMillis() int64 { return domainanalysis.NowMillis() }

func fieldString(key, value string) domainlog.Field { return logkit.F(key, value) }

func fieldErr(err error) domainlog.Field {
	if err == nil {
		return logkit.F("err", "")
	}
	return logkit.F("err", err)
}

// Snapshot 返回 service 视图的一次快照(含所选 bridge 服务)。
func (s *Service) Snapshot(bridgeService string) domainanalysis.ServiceSnapshot {
	if bridgeService == "" {
		bridgeService = "node"
	}
	s.mu.RLock()
	resources := map[string]domainanalysis.ResourceMetric{}
	for key, metric := range s.samples {
		resources[key] = domainanalysis.ResourceMetric{
			Series:  metric.Series,
			Samples: append([]domainanalysis.Sample{}, metric.Samples...),
		}
	}
	s.mu.RUnlock()

	return domainanalysis.ServiceSnapshot{
		IntervalMs: s.params.SampleInterval.Milliseconds(),
		PID:        pidOf(),
		Resources:  resources,
		Bridge:     s.bridgeMetric(bridgeService),
	}
}
