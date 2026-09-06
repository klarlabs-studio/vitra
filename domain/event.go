package domain

import (
	"errors"
	"sync"
	"time"
)

// EventName identifies a runtime or application event channel.
type EventName string

// SubscriptionID identifies an active event subscription.
type SubscriptionID string

// NewEventName validates an event name.
func NewEventName(s string) (EventName, error) {
	if s == "" {
		return "", errors.New("event name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", errors.New("event name is invalid")
	}
	return EventName(s), nil
}

// NewSubscriptionID validates a subscription id.
func NewSubscriptionID(s string) (SubscriptionID, error) {
	if s == "" {
		return "", errors.New("subscription id must not be empty")
	}
	return SubscriptionID(s), nil
}

// Event is a one-way notification delivered to subscribed windows.
type Event struct {
	Name      EventName
	Payload   any
	EmittedAt time.Time
}

// Subscription binds an event channel to an owning window.
// Closing the owner must release the subscription (reliability invariant 2 /
// security invariant 15).
type Subscription struct {
	mu     sync.RWMutex
	id     SubscriptionID
	event  EventName
	owner  WindowID
	closed bool
}

// NewSubscription creates an open subscription owned by window.
func NewSubscription(id SubscriptionID, event EventName, owner WindowID) (*Subscription, error) {
	if id == "" {
		return nil, errors.New("subscription id is required")
	}
	if event == "" {
		return nil, errors.New("event name is required")
	}
	if owner == "" {
		return nil, errors.New("subscription owner window is required")
	}
	return &Subscription{id: id, event: event, owner: owner}, nil
}

// ID returns the subscription id.
func (s *Subscription) ID() SubscriptionID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// Event returns the subscribed event name.
func (s *Subscription) Event() EventName {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.event
}

// Owner returns the owning window.
func (s *Subscription) Owner() WindowID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owner
}

// IsClosed reports whether the subscription has been released.
func (s *Subscription) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// Close releases the subscription (idempotent).
func (s *Subscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}

// SubscriptionRepository stores event subscriptions.
type SubscriptionRepository interface {
	Save(sub *Subscription) error
	Get(id SubscriptionID) (*Subscription, error)
	Delete(id SubscriptionID) error
	ListByOwner(window WindowID) ([]*Subscription, error)
	ListByEvent(event EventName) ([]*Subscription, error)
}
