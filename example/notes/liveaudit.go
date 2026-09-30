package main

import (
	"sync"
	"time"

	"go.klarlabs.de/vitra/audit"
)

const (
	// maxAuditEvents bounds the retained log; a page that floods the bridge
	// cannot grow it without limit.
	maxAuditEvents = 500
	// publishQueue bounds events waiting to be pushed to the window. When it
	// is full, new events are still retained but not pushed live.
	publishQueue = 256
)

// liveAudit is an audit.Sink that keeps the most recent events and pushes
// each new one, in order, to a subscriber (the window).
type liveAudit struct {
	mu      sync.Mutex
	events  []audit.Event
	pending chan audit.Event
}

// Append records e and queues it for the subscriber without blocking: the
// runtime appends from the UI thread (navigation policy) and from invokes.
func (s *liveAudit) Append(e audit.Event) error {
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if n := len(s.events); n > maxAuditEvents {
		s.events = append([]audit.Event(nil), s.events[n-maxAuditEvents:]...)
	}
	if s.pending != nil {
		select {
		case s.pending <- e:
		default:
		}
	}
	return nil
}

// List returns a copy of the retained events, oldest first. It is never nil,
// so an empty log reaches the page as [] rather than null.
func (s *liveAudit) List() []audit.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]audit.Event{}, s.events...)
}

// Subscribe starts pushing new events to publish, one at a time and in
// order, on a goroutine of its own. Call it once.
func (s *liveAudit) Subscribe(publish func(audit.Event)) {
	ch := make(chan audit.Event, publishQueue)
	s.mu.Lock()
	s.pending = ch
	s.mu.Unlock()
	go func() {
		for e := range ch {
			publish(e)
		}
	}()
}
