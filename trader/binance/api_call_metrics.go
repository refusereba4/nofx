package binance

import (
	"sort"
	"sync"
	"time"
)

const (
	metricsMinuteLayout = "2006-01-02 15:04"
	metricsKeepMinutes  = 720
)

// APICallMinuteStats stores per-minute API call counters by category.
type APICallMinuteStats struct {
	Minute string         `json:"minute"`
	Counts map[string]int `json:"counts"`
	Total  int            `json:"total"`
}

var (
	apiCallMetricsMu sync.Mutex
	apiCallMetrics   = map[string]map[string]int{}
)

// RecordRiskAPICall records one API request for risk-control related categories.
func RecordRiskAPICall(category string) {
	nowMinute := time.Now().Format(metricsMinuteLayout)

	apiCallMetricsMu.Lock()
	defer apiCallMetricsMu.Unlock()

	minuteCounts, ok := apiCallMetrics[nowMinute]
	if !ok {
		minuteCounts = make(map[string]int)
		apiCallMetrics[nowMinute] = minuteCounts
	}
	minuteCounts[category]++

	// Keep only recent windows to avoid unbounded growth.
	cutoff := time.Now().Add(-metricsKeepMinutes * time.Minute)
	for minuteKey := range apiCallMetrics {
		t, err := time.ParseInLocation(metricsMinuteLayout, minuteKey, time.Local)
		if err != nil || t.Before(cutoff) {
			delete(apiCallMetrics, minuteKey)
		}
	}
}

// GetRiskAPICallStats returns latest per-minute counters.
func GetRiskAPICallStats(limit int) []APICallMinuteStats {
	if limit <= 0 {
		limit = 60
	}

	apiCallMetricsMu.Lock()
	defer apiCallMetricsMu.Unlock()

	minutes := make([]string, 0, len(apiCallMetrics))
	for minute := range apiCallMetrics {
		minutes = append(minutes, minute)
	}
	sort.Strings(minutes)
	if len(minutes) > limit {
		minutes = minutes[len(minutes)-limit:]
	}

	out := make([]APICallMinuteStats, 0, len(minutes))
	for _, minute := range minutes {
		src := apiCallMetrics[minute]
		counts := make(map[string]int, len(src))
		total := 0
		for k, v := range src {
			counts[k] = v
			total += v
		}
		out = append(out, APICallMinuteStats{
			Minute: minute,
			Counts: counts,
			Total:  total,
		})
	}
	return out
}

