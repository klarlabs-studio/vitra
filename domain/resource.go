package domain

import (
	"errors"
	"sync"
	"time"
)

// ResourceHandle represents a long-lived native resource with ownership.
// Handles are not unrestricted pointers — closing the owning window must
// release them via the runtime.
type ResourceHandle struct {
	mu       sync.RWMutex
	id       ResourceID
	kind     string
	owner    WindowID
	closed   bool
	created  time.Time
	closedAt time.Time
}

// NewResourceHandle creates an open handle owned by window.
func NewResourceHandle(id ResourceID, kind string, owner WindowID) (*ResourceHandle, error) {
	if id == "" {
		return nil, errors.New("resource id is required")
	}
	if kind == "" {
		return nil, errors.New("resource kind is required")
	}
	if owner == "" {
		return nil, errors.New("resource owner window is required")
	}
	return &ResourceHandle{
		id:      id,
		kind:    kind,
		owner:   owner,
		created: time.Now().UTC(),
	}, nil
}

// ID returns the resource id.
func (h *ResourceHandle) ID() ResourceID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.id
}

// Kind returns the resource kind.
func (h *ResourceHandle) Kind() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.kind
}

// Owner returns the owning window.
func (h *ResourceHandle) Owner() WindowID {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.owner
}

// IsClosed reports whether the handle has been released.
func (h *ResourceHandle) IsClosed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.closed
}

// Close releases the handle.
func (h *ResourceHandle) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	h.closedAt = time.Now().UTC()
}
