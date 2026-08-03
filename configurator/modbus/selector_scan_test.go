package modbus

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type selectorWrite struct {
	addr  uint16
	value uint16
}

type fakeSelectorScanTransport struct {
	waitCalls       int
	writes          []selectorWrite
	readCalls       int
	currentSelector int
	currentHole     int
	failWrite       *selectorWrite
	waitErr         error
}

func (f *fakeSelectorScanTransport) waitReady(time.Duration) error {
	f.waitCalls++
	return f.waitErr
}

func (f *fakeSelectorScanTransport) writeRegister(addr, value uint16) error {
	write := selectorWrite{addr: addr, value: value}
	f.writes = append(f.writes, write)
	if f.failWrite != nil && *f.failWrite == write {
		return errors.New("transport write failed")
	}

	switch addr {
	case RegSelectorTarget:
		f.currentSelector = int(value)
	case RegSelectorHole:
		f.currentHole = int(value)
	}
	return nil
}

func (f *fakeSelectorScanTransport) readRegister(addr uint16) (uint16, error) {
	if addr != RegSelectorCoord {
		return 0, fmt.Errorf("unexpected read register %d", addr)
	}
	f.readCalls++
	return uint16(f.currentSelector*1000 + f.currentHole), nil
}

func TestScanSelectorPositionsUsesReducedRequestBudget(t *testing.T) {
	transport := &fakeSelectorScanTransport{}
	progressCalls := 0
	lastCurrent := 0
	lastTotal := 0

	sel1, sel2, err := scanSelectorPositions(transport, func(current, total int, _ string) {
		progressCalls++
		lastCurrent = current
		lastTotal = total
	})
	if err != nil {
		t.Fatalf("scanSelectorPositions() error = %v", err)
	}

	if len(sel1) != 15 || len(sel2) != 15 {
		t.Fatalf("positions = %d/%d, want 15/15", len(sel1), len(sel2))
	}
	for hole := 0; hole <= 14; hole++ {
		if got, want := sel1[hole].Coord, 1000+hole; got != want {
			t.Errorf("selector 1 hole %d coord = %d, want %d", hole, got, want)
		}
		if got, want := sel2[hole].Coord, 2000+hole; got != want {
			t.Errorf("selector 2 hole %d coord = %d, want %d", hole, got, want)
		}
	}

	if transport.waitCalls != 1 {
		t.Errorf("ready checks = %d, want 1", transport.waitCalls)
	}
	if transport.readCalls != 30 {
		t.Errorf("coordinate reads = %d, want 30", transport.readCalls)
	}
	if len(transport.writes) != 32 {
		t.Fatalf("writes = %d, want 32 (2 selector + 30 hole)", len(transport.writes))
	}

	targetWrites := 0
	holeWrites := 0
	for _, write := range transport.writes {
		switch write.addr {
		case RegSelectorTarget:
			targetWrites++
		case RegSelectorHole:
			holeWrites++
		default:
			t.Errorf("unexpected write register %d", write.addr)
		}
	}
	if targetWrites != 2 || holeWrites != 30 {
		t.Errorf("writes by type = target:%d hole:%d, want 2/30", targetWrites, holeWrites)
	}

	// 1 ready read + 32 writes + 30 coordinate reads = 63 Modbus requests.
	if requests := transport.waitCalls + len(transport.writes) + transport.readCalls; requests != 63 {
		t.Errorf("request budget = %d, want 63", requests)
	}
	if progressCalls != 30 || lastCurrent != 30 || lastTotal != 30 {
		t.Errorf("progress = calls:%d last:%d/%d, want 30 and 30/30", progressCalls, lastCurrent, lastTotal)
	}
}

func TestScanSelectorPositionsReportsExactFailurePosition(t *testing.T) {
	failedWrite := selectorWrite{addr: RegSelectorHole, value: 3}
	transport := &fakeSelectorScanTransport{failWrite: &failedWrite}

	_, _, err := scanSelectorPositions(transport, nil)
	if err == nil {
		t.Fatal("scanSelectorPositions() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "селектор 1, отверстие 3") {
		t.Fatalf("error = %q, want selector and hole context", err)
	}
}

func TestScanSelectorPositionsStopsWhenControllerIsBusy(t *testing.T) {
	transport := &fakeSelectorScanTransport{waitErr: errors.New("busy")}

	_, _, err := scanSelectorPositions(transport, nil)
	if err == nil {
		t.Fatal("scanSelectorPositions() error = nil, want failure")
	}
	if len(transport.writes) != 0 || transport.readCalls != 0 {
		t.Fatalf("requests after ready failure: writes=%d reads=%d", len(transport.writes), transport.readCalls)
	}
}
