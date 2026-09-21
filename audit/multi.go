package audit

import "sync"

// MultiSink fans Append out to every sink. List concatenates List results
// from sinks that implement Sink (order preserved). Append stops on the
// first error; earlier sinks may have already recorded the event.
type MultiSink struct {
	Sinks []Sink

	mu sync.Mutex
}

// Append writes to each sink in order.
func (m *MultiSink) Append(e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.Sinks {
		if s == nil {
			continue
		}
		if err := s.Append(e); err != nil {
			return err
		}
	}
	return nil
}

// List returns events from the first non-nil sink (typically MemorySink).
func (m *MultiSink) List() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.Sinks {
		if s == nil {
			continue
		}
		return s.List()
	}
	return nil
}
