// Package worker provides Phase 5 supervised worker processes for isolation.
//
// Crash-prone, privileged, or long-running work runs outside the WebView host.
// Worker failures must not corrupt runtime bookkeeping (reliability invariant 4).
package worker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State is the supervised worker lifecycle state.
type State string

const (
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateCrashed  State = "crashed"
)

// ID identifies a worker.
type ID string

// Spec describes a worker to supervise.
type Spec struct {
	ID          ID
	Name        string
	MaxRestarts int
	// Elevated indicates the worker may hold privileges the host must not.
	Elevated bool
}

// Validate checks the spec.
func (s Spec) Validate() error {
	if s.ID == "" {
		return errors.New("worker id is required")
	}
	if s.Name == "" {
		return errors.New("worker name is required")
	}
	if s.MaxRestarts < 0 {
		return errors.New("max restarts must be >= 0")
	}
	return nil
}

// Runner executes worker logic until ctx is cancelled or an error occurs.
type Runner func(ctx context.Context) error

// Record is inspectable worker bookkeeping that survives crashes.
type Record struct {
	Spec         Spec
	State        State
	Restarts     int
	LastError    string
	StartedAt    time.Time
	LastExitAt   time.Time
	LastExitCode int
}

// Supervisor manages worker lifecycles without sharing mutable host state
// that a crash could corrupt — bookkeeping lives in the supervisor.
type Supervisor struct {
	mu      sync.Mutex
	records map[ID]*Record
	cancels map[ID]context.CancelFunc
	runners map[ID]Runner
}

// NewSupervisor constructs an empty supervisor.
func NewSupervisor() *Supervisor {
	return &Supervisor{
		records: make(map[ID]*Record),
		cancels: make(map[ID]context.CancelFunc),
		runners: make(map[ID]Runner),
	}
}

// Start supervises a worker. The WebView host remains non-elevated; elevated
// work must run here (security invariant 8).
func (s *Supervisor) Start(parent context.Context, spec Spec, run Runner) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if run == nil {
		return errors.New("runner is required")
	}
	s.mu.Lock()
	if _, exists := s.records[spec.ID]; exists {
		s.mu.Unlock()
		return errors.New("worker already registered: " + string(spec.ID))
	}
	rec := &Record{Spec: spec, State: StateStarting, StartedAt: time.Now().UTC()}
	s.records[spec.ID] = rec
	s.runners[spec.ID] = run
	ctx, cancel := context.WithCancel(parent)
	s.cancels[spec.ID] = cancel
	s.mu.Unlock()

	go s.loop(ctx, spec.ID)
	return nil
}

func (s *Supervisor) loop(ctx context.Context, id ID) {
	for {
		s.mu.Lock()
		rec := s.records[id]
		run := s.runners[id]
		if rec == nil || run == nil {
			s.mu.Unlock()
			return
		}
		if rec.State == StateStopping {
			rec.State = StateStopped
			rec.LastExitAt = time.Now().UTC()
			s.mu.Unlock()
			return
		}
		rec.State = StateRunning
		max := rec.Spec.MaxRestarts
		restarts := rec.Restarts
		s.mu.Unlock()

		err := run(ctx)
		s.mu.Lock()
		rec = s.records[id]
		if rec == nil {
			s.mu.Unlock()
			return
		}
		rec.LastExitAt = time.Now().UTC()
		if ctx.Err() != nil || rec.State == StateStopping {
			rec.State = StateStopped
			s.mu.Unlock()
			return
		}
		if err != nil {
			rec.LastError = err.Error()
			rec.LastExitCode = 1
		}
		if restarts >= max {
			rec.State = StateCrashed
			s.mu.Unlock()
			return
		}
		rec.Restarts++
		rec.State = StateStarting
		s.mu.Unlock()
	}
}

// Stop requests a graceful stop.
func (s *Supervisor) Stop(id ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return errors.New("worker not found: " + string(id))
	}
	rec.State = StateStopping
	if cancel, ok := s.cancels[id]; ok {
		cancel()
	}
	return nil
}

// Get returns a copy of worker bookkeeping.
func (s *Supervisor) Get(id ID) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return Record{}, errors.New("worker not found: " + string(id))
	}
	cp := *rec
	return cp, nil
}

// List returns all worker records.
func (s *Supervisor) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.records))
	for _, rec := range s.records {
		cp := *rec
		out = append(out, cp)
	}
	return out
}
