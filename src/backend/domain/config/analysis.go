package config

import "github.com/wensboy/automan/backend/domain/key"

// Analysis 映射 data/conf/analysis.json。
type Analysis struct {
	SampleIntervalMs int `json:"sampleIntervalMs"`
	WindowPoints     int `json:"windowPoints"`
	IssueLimit       int `json:"issueLimit"`
	TraceQueue       int `json:"traceQueue"`
	Prune            struct {
		ProxyRows   int `json:"proxyRows"`
		LogRows     int `json:"logRows"`
		BridgeRows  int `json:"bridgeRows"`
		MaxAgeHours int `json:"maxAgeHours"`
	} `json:"prune"`
}

// AnalysisKey 取 conf/analysis 命名空间。
var AnalysisKey = key.New[Analysis]("conf/analysis")
