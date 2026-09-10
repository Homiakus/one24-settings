package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/axiom"
	"github.com/Homiakus/axiom/profile"
)

// State is the durable lifecycle state of the configurator process.
// It is intentionally small: Axiom records facts and transitions, while
// process supervision remains owned by the host process.
type State struct {
	Phase      string    `json:"phase"`
	Generation uint64    `json:"generation"`
	LastEvent  string    `json:"last_event"`
	LastError  string    `json:"last_error,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Started struct {
	PID     int
	Address string
}
type Ready struct{ Address string }
type Failed struct {
	Component string
	Err       string
}
type Stopping struct{ Reason string }

type Controller struct {
	profile *profile.DurableSingleNode
	engine  *axiom.FlowEngine[State]
	exec    *axiom.FlowExecution[State]
	mu      sync.Mutex
}

// Open creates or reopens the crash-durable process lifecycle journal.
func Open(dir string) (*Controller, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("orchestrator state directory is required")
	}
	p, err := profile.OpenDurableSingleNode(profile.DurableSingleNodeConfig{Dir: dir, SyncWrites: true})
	if err != nil {
		return nil, fmt.Errorf("open axiom profile: %w", err)
	}
	flow := axiom.NewFlow("onepap-process", State{Phase: "created"})
	axiom.Handle(flow, func(_ context.Context, s State, e Started) (axiom.FlowResult[State], error) {
		s.Phase, s.LastEvent, s.LastError = "starting", "started", ""
		s.Generation++
		s.UpdatedAt = time.Now().UTC()
		return axiom.Next(s), nil
	})
	axiom.Handle(flow, func(_ context.Context, s State, e Ready) (axiom.FlowResult[State], error) {
		s.Phase, s.LastEvent = "ready", "ready"
		s.Generation++
		s.UpdatedAt = time.Now().UTC()
		return axiom.Next(s), nil
	})
	axiom.Handle(flow, func(_ context.Context, s State, e Failed) (axiom.FlowResult[State], error) {
		s.Phase, s.LastEvent, s.LastError = "degraded", "failed", e.Component+": "+e.Err
		s.Generation++
		s.UpdatedAt = time.Now().UTC()
		return axiom.Next(s), nil
	})
	axiom.Handle(flow, func(_ context.Context, s State, e Stopping) (axiom.FlowResult[State], error) {
		s.Phase, s.LastEvent, s.LastError = "stopping", "stopping", ""
		s.Generation++
		s.UpdatedAt = time.Now().UTC()
		return axiom.Next(s), nil
	})
	engine, err := profile.OpenDurableFlow(p, flow)
	if err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("open axiom durable flow: %w", err)
	}
	return &Controller{profile: p, engine: engine, exec: engine.Execution("configurator")}, nil
}

func (c *Controller) Dispatch(ctx context.Context, event any) error {
	if c == nil || c.exec == nil {
		return fmt.Errorf("orchestrator is not initialized")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.exec.Dispatch(ctx, event)
}

func (c *Controller) State(ctx context.Context) (State, error) {
	if c == nil || c.exec == nil {
		return State{}, fmt.Errorf("orchestrator is not initialized")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.exec.State(ctx)
}

func (c *Controller) Close() error {
	if c == nil || c.profile == nil {
		return nil
	}
	return c.profile.Close()
}
