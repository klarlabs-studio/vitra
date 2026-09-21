package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// JSONLSink writes each event as one JSON object per line (NDJSON).
// Destinations are typically a file, stdout, or a pipe into a SIEM agent.
// List retains an in-memory copy for local inspection.
type JSONLSink struct {
	W io.Writer

	mu     sync.Mutex
	events []Event
}

// Append encodes the event as a single JSON line and retains it for List.
func (s *JSONLSink) Append(e Event) error {
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
	if s.W != nil {
		enc := json.NewEncoder(s.W)
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	s.events = append(s.events, e)
	return nil
}

// List returns a copy of retained events.
func (s *JSONLSink) List() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.events...)
}
