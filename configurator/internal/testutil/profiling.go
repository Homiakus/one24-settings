package testutil

import (
	"fmt"
	"os"
	"runtime/pprof"
)

// StartCPUProfile начинает сбор CPU профиля в указанный файл.
func StartCPUProfile(filepath string) (stop func(), err error) {
	f, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("create cpu profile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("start cpu profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}, nil
}

// WriteHeapProfile записывает текущий heap профиль в указанный файл.
func WriteHeapProfile(filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create heap profile: %w", err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("write heap profile: %w", err)
	}
	return nil
}
