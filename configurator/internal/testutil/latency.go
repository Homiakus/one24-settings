package testutil

import (
	"sync"
	"time"
)

// LatencyRecorder замеряет и накапливает задержки выполнения операций.
type LatencyRecorder struct {
	mu        sync.Mutex
	durations []time.Duration
}

// NewLatencyRecorder инициализирует накопитель с заданной емкостью.
func NewLatencyRecorder(capacity int) *LatencyRecorder {
	return &LatencyRecorder{
		durations: make([]time.Duration, 0, capacity),
	}
}

// Measure фиксирует время исполнения функции.
func (lr *LatencyRecorder) Measure(fn func()) time.Duration {
	start := time.Now()
	fn()
	elapsed := time.Since(start)

	lr.mu.Lock()
	lr.durations = append(lr.durations, elapsed)
	lr.mu.Unlock()

	return elapsed
}

// Durations возвращает копию накопленных измерений.
func (lr *LatencyRecorder) Durations() []time.Duration {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	res := make([]time.Duration, len(lr.durations))
	copy(res, lr.durations)
	return res
}

// Reset очищает накопленные данные.
func (lr *LatencyRecorder) Reset() {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.durations = lr.durations[:0]
}
