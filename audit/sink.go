// Package audit provides Phase 5 structured audit logging for enterprise
// diagnostics and policy review.
package audit

import (
	"sync"
	"time"
)

// Kind classifies audit events.
type Kind string

const (
	KindCapabilityDecision Kind = "capability.decision"
	KindCommandInvoke      Kind = "command.invoke"
	KindPluginRegister     Kind = "plugin.register"
	KindWorkerLifecycle    Kind = "worker.lifecycle"
	KindUpdatePlan         Kind = "update.plan"
	KindPolicyOverride     Kind = "policy.override"
)

// Event is an append-only audit record.
type Event struct {
	At       time.Time      `json:"at"`
	Kind     Kind           `json:"kind"`
	Actor    string         `json:"actor,omitempty"`
	Window   string         `json:"window,omitempty"`
	Origin   string         `json:"origin,omitempty"`
	Action   string         `json:"action,omitempty"`
	Outcome  string         `json:"outcome,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Sink stores audit events.
type Sink interface {
	Append(Event) error
	List() []Event
}

// MemorySink is an in-memory audit sink for tests and single-node hosts.
type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

// Append records an event.
func (s *MemorySink) Append(e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.Metadata != nil {
		cp := make(map[string]any, len(e.Metadata))
		for k, v := range e.Metadata {
			cp[k] = v
		}
		e.Metadata = cp
	}
	s.events = append(s.events, e)
	return nil
}

// List returns a copy of events.
func (s *MemorySink) List() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}
