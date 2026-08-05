package testutil

import (
	"math"
	"sort"
	"time"
)

// LatencyStats представляет статистику распределения задержек.
type LatencyStats struct {
	Count     int
	TotalTime time.Duration
	OpsPerSec float64
	Mean      time.Duration
	P50       time.Duration
	P90       time.Duration
	P95       time.Duration
	P98       time.Duration
	P99       time.Duration
	Max       time.Duration
	Min       time.Duration
	StdDev    time.Duration
}

// CalculateStats рассчитывает процентили и сводные метрики по массиву длительностей.
func CalculateStats(durations []time.Duration) LatencyStats {
	n := len(durations)
	if n == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, n)
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total int64
	for _, d := range sorted {
		total += int64(d)
	}

	meanNanos := float64(total) / float64(n)
	mean := time.Duration(meanNanos)

	var varianceSum float64
	for _, d := range sorted {
		diff := float64(int64(d)) - meanNanos
		varianceSum += diff * diff
	}
	stdDevNanos := math.Sqrt(varianceSum / float64(n))
	stdDev := time.Duration(stdDevNanos)

	totalDur := time.Duration(total)
	var opsSec float64
	if totalDur > 0 {
		opsSec = float64(n) / totalDur.Seconds()
	}

	percentile := func(p float64) time.Duration {
		if p <= 0 {
			return sorted[0]
		}
		if p >= 100 {
			return sorted[n-1]
		}
		idx := int(math.Ceil((p/100.0)*float64(n))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return sorted[idx]
	}

	return LatencyStats{
		Count:     n,
		TotalTime: totalDur,
		OpsPerSec: opsSec,
		Mean:      mean,
		P50:       percentile(50),
		P90:       percentile(90),
		P95:       percentile(95),
		P98:       percentile(98),
		P99:       percentile(99),
		Min:       sorted[0],
		Max:       sorted[n-1],
		StdDev:    stdDev,
	}
}
