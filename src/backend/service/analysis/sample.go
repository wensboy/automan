package analysis

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	domainanalysis "github.com/wensboy/automan/backend/domain/analysis"
)

// 资源键(与前端一致)。
const (
	resourceCPU       = "cpu"
	resourceRAM       = "ram"
	resourceIO        = "io"
	resourceGC        = "gc"
	resourceHeap      = "heap"
	resourceGoroutine = "goroutine"
)

var baseSeries = map[string][]domainanalysis.SeriesDef{
	resourceCPU: {{Key: "avg", Label: "CPU average", Unit: "%"}},
	resourceRAM: {{Key: "rss", Label: "RSS", Unit: "MB"}},
	resourceIO: {
		{Key: "readTotal", Label: "Read total", Unit: "MB", Digits: 2},
		{Key: "writeTotal", Label: "Write total", Unit: "MB", Digits: 2},
		{Key: "read", Label: "Read rate", Unit: "MB/s", Digits: 2},
		{Key: "write", Label: "Write rate", Unit: "MB/s", Digits: 2},
		{Key: "readOpsTotal", Label: "Read ops total", Unit: "count"},
		{Key: "writeOpsTotal", Label: "Write ops total", Unit: "count"},
	},
	resourceGC:        {{Key: "pause", Label: "GC pause", Unit: "ms"}, {Key: "count", Label: "GC count", Unit: "count"}},
	resourceHeap:      {{Key: "alloc", Label: "Heap alloc", Unit: "MB"}, {Key: "sys", Label: "Heap sys", Unit: "MB"}},
	resourceGoroutine: {{Key: "count", Label: "Goroutines", Unit: "count"}},
}

// sampler 保存两次采样之间需要的基线。
type sampler struct {
	proc         *process.Process
	lastIOAt     time.Time
	lastRead     uint64
	lastWrite    uint64
	lastReadOps  uint64
	lastWriteOps uint64
	lastGC       uint32
	lastPause    uint64
	ready        bool
	ioWarned     bool
	mu           sync.Mutex
}

func (s *Service) ensureSampler() *sampler {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sampler == nil {
		proc, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {
			return nil
		}
		s.sampler = &sampler{proc: proc, lastIOAt: time.Now()}
	}
	return s.sampler
}

// sampleOnce 采集一次并推进每个资源的环形窗口。
func (s *Service) sampleOnce() {
	sampler := s.ensureSampler()
	if sampler == nil {
		return
	}
	now := time.Now()
	values := map[string]map[string]float64{}

	// cpu: 第一次调用是基线, 返回 0
	if percent, err := sampler.proc.Percent(0); err == nil {
		values[resourceCPU] = map[string]float64{"avg": percent}
	}

	if info, err := sampler.proc.MemoryInfo(); err == nil {
		values[resourceRAM] = map[string]float64{"rss": float64(info.RSS) / (1024 * 1024)}
	}

	if counters, err := sampler.proc.IOCounters(); err == nil {
		elapsed := now.Sub(sampler.lastIOAt).Seconds()
		if elapsed <= 0 {
			elapsed = 1
		}
		ioValues := map[string]float64{
			"readTotal":     float64(counters.ReadBytes) / (1024 * 1024),
			"writeTotal":    float64(counters.WriteBytes) / (1024 * 1024),
			"readOpsTotal":  float64(counters.ReadCount),
			"writeOpsTotal": float64(counters.WriteCount),
		}
		if sampler.ready {
			ioValues["read"] = float64(counters.ReadBytes-sampler.lastRead) / (1024 * 1024) / elapsed
			ioValues["write"] = float64(counters.WriteBytes-sampler.lastWrite) / (1024 * 1024) / elapsed
		}
		values[resourceIO] = ioValues
		sampler.lastRead, sampler.lastWrite = counters.ReadBytes, counters.WriteBytes
		sampler.lastReadOps, sampler.lastWriteOps = counters.ReadCount, counters.WriteCount
		sampler.lastIOAt = now
	} else {
		// /proc/<pid>/io 不可用(内核未开 TASK_IO_ACCOUNTING 等)时也要出指标,
		// 否则前端会一直显示 "on the way"; 值保持 0, 并只告警一次。
		values[resourceIO] = map[string]float64{
			"readTotal": 0, "writeTotal": 0, "read": 0, "write": 0,
			"readOpsTotal": 0, "writeOpsTotal": 0,
		}
		if !sampler.ioWarned && s.logger != nil {
			sampler.ioWarned = true
			s.warn("analysis: process io counters unavailable", fieldErr(err))
		}
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	values[resourceGC] = map[string]float64{"pause": 0, "count": 0}
	if sampler.ready {
		pauseDelta := stats.PauseTotalNs - sampler.lastPause
		gcDelta := stats.NumGC - sampler.lastGC
		values[resourceGC] = map[string]float64{
			"pause": float64(pauseDelta) / float64(time.Millisecond),
			"count": float64(gcDelta),
		}
	}
	sampler.lastPause, sampler.lastGC = stats.PauseTotalNs, stats.NumGC
	values[resourceHeap] = map[string]float64{
		"alloc": float64(stats.HeapAlloc) / (1024 * 1024),
		"sys":   float64(stats.HeapSys) / (1024 * 1024),
	}
	values[resourceGoroutine] = map[string]float64{"count": float64(runtime.NumGoroutine())}
	sampler.ready = true

	s.mu.Lock()
	for resource, sampleValues := range values {
		metric := s.samples[resource]
		metric.Series = baseSeries[resource]
		metric.Samples = append(metric.Samples, domainanalysis.Sample{TS: now.UnixMilli(), Values: sampleValues})
		if len(metric.Samples) > s.params.WindowPoints {
			metric.Samples = metric.Samples[len(metric.Samples)-s.params.WindowPoints:]
		}
		s.samples[resource] = metric
	}
	// bridge: 每个已知服务按窗口计算可用率/连接数/错误数
	for name, events := range s.bridgeEvents {
		samples := s.bridgeSeries[name]
		samples = append(samples, domainanalysis.Sample{TS: now.UnixMilli(), Values: aggregateBridge(events)})
		if len(samples) > s.params.WindowPoints {
			samples = samples[len(samples)-s.params.WindowPoints:]
		}
		s.bridgeSeries[name] = samples
	}
	s.mu.Unlock()
}

// bridgeWindow 是可用率统计窗口。
const bridgeWindow = 5 * time.Minute

// aggregateBridge 从事件窗口计算 rate/peers/errors。
func aggregateBridge(events []domainanalysis.BridgeEvent) map[string]float64 {
	since := time.Now().Add(-bridgeWindow).UnixMilli()
	total, okCount, errors := 0, 0, 0
	peers := 0.0
	last := int64(0)
	for _, event := range events {
		if event.At < since {
			continue
		}
		total++
		if event.OK {
			okCount++
		} else {
			errors++
		}
		if event.At >= last {
			last = event.At
			if event.OK {
				peers = 1
			} else {
				peers = 0
			}
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(okCount) / float64(total) * 100
	}
	return map[string]float64{"rate": rate, "peers": peers, "errors": float64(errors)}
}
