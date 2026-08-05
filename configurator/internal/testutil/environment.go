package testutil

import (
	"fmt"
	"runtime"
)

// EnvironmentInfo содержит данные о текущем оборудовании и среде исполнения.
type EnvironmentInfo struct {
	GoVersion string
	OS        string
	Arch      string
	NumCPU    int
	MaxProcs  int
	Compiler  string
}

// GetEnvironmentInfo собирает системные метрики.
func GetEnvironmentInfo() EnvironmentInfo {
	return EnvironmentInfo{
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		MaxProcs:  runtime.GOMAXPROCS(0),
		Compiler:  runtime.Compiler,
	}
}

// SummaryString возвращает форматированное представление.
func (e EnvironmentInfo) SummaryString() string {
	return fmt.Sprintf("Go: %s, OS/Arch: %s/%s, CPUs: %d, GOMAXPROCS: %d",
		e.GoVersion, e.OS, e.Arch, e.NumCPU, e.MaxProcs)
}
