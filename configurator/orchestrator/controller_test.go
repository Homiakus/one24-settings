package orchestrator

import (
	"context"
	"testing"
)

func TestControllerReopensDurableLifecycle(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	c, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := c.Dispatch(ctx, Started{PID: 42, Address: "127.0.0.1:1234"}); err != nil {
		t.Fatalf("Dispatch(started) error = %v", err)
	}
	if err := c.Dispatch(ctx, Ready{Address: "127.0.0.1:1234"}); err != nil {
		t.Fatalf("Dispatch(ready) error = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	c, err = Open(dir)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer c.Close()
	state, err := c.State(ctx)
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Phase != "ready" || state.Generation != 2 || state.LastEvent != "ready" {
		t.Fatalf("reopened state = %#v, want ready generation 2", state)
	}
}

func TestControllerPersistsFailure(t *testing.T) {
	c, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer c.Close()
	if err := c.Dispatch(context.Background(), Failed{Component: "modbus", Err: "serial timeout"}); err != nil {
		t.Fatalf("Dispatch(failed) error = %v", err)
	}
	state, err := c.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Phase != "degraded" || state.LastError != "modbus: serial timeout" {
		t.Fatalf("failure state = %#v", state)
	}
}
