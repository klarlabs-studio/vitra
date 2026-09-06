// Package inmemory provides in-memory adapters for Vitra domain ports.
// Suitable for tests and single-process development runtimes.
package inmemory

import (
	"sync"

	"go.klarlabs.de/vitra/domain"
)

// GrantRepo is a thread-safe GrantRepository.
type GrantRepo struct {
	mu     sync.RWMutex
	byName map[domain.GrantName]*domain.CapabilityGrant
}

// NewGrantRepo constructs an empty grant repository.
func NewGrantRepo() *GrantRepo {
	return &GrantRepo{byName: make(map[domain.GrantName]*domain.CapabilityGrant)}
}

// Save stores a grant.
func (r *GrantRepo) Save(grant *domain.CapabilityGrant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[grant.Name()] = grant
	return nil
}

// Get returns a grant by name.
func (r *GrantRepo) Get(name domain.GrantName) (*domain.CapabilityGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.byName[name]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "grant", ID: string(name)}
	}
	return g, nil
}

// List returns all grants.
func (r *GrantRepo) List() ([]*domain.CapabilityGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.CapabilityGrant, 0, len(r.byName))
	for _, g := range r.byName {
		out = append(out, g)
	}
	return out, nil
}

// WindowRepo is a thread-safe WindowRepository.
type WindowRepo struct {
	mu   sync.RWMutex
	byID map[domain.WindowID]*domain.Window
}

// NewWindowRepo constructs an empty window repository.
func NewWindowRepo() *WindowRepo {
	return &WindowRepo{byID: make(map[domain.WindowID]*domain.Window)}
}

// Save stores a window.
func (r *WindowRepo) Save(window *domain.Window) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[window.ID()] = window
	return nil
}

// Get returns a window by id.
func (r *WindowRepo) Get(id domain.WindowID) (*domain.Window, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.byID[id]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "window", ID: string(id)}
	}
	return w, nil
}

// Delete removes a window.
func (r *WindowRepo) Delete(id domain.WindowID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

// List returns all windows.
func (r *WindowRepo) List() ([]*domain.Window, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.Window, 0, len(r.byID))
	for _, w := range r.byID {
		out = append(out, w)
	}
	return out, nil
}

// CommandRepo is a thread-safe CommandRepository.
type CommandRepo struct {
	mu     sync.RWMutex
	byName map[domain.CommandName]*domain.CommandDefinition
}

// NewCommandRepo constructs an empty command repository.
func NewCommandRepo() *CommandRepo {
	return &CommandRepo{byName: make(map[domain.CommandName]*domain.CommandDefinition)}
}

// Save stores a command definition.
func (r *CommandRepo) Save(cmd *domain.CommandDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[cmd.Name()]; exists {
		return &domain.ErrConflict{Message: "command already registered: " + string(cmd.Name())}
	}
	r.byName[cmd.Name()] = cmd
	return nil
}

// Get returns a command by name.
func (r *CommandRepo) Get(name domain.CommandName) (*domain.CommandDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.byName[name]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "command", ID: string(name)}
	}
	return c, nil
}

// List returns all commands.
func (r *CommandRepo) List() ([]*domain.CommandDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.CommandDefinition, 0, len(r.byName))
	for _, c := range r.byName {
		out = append(out, c)
	}
	return out, nil
}

// ResourceRepo is a thread-safe ResourceRepository.
type ResourceRepo struct {
	mu   sync.RWMutex
	byID map[domain.ResourceID]*domain.ResourceHandle
}

// NewResourceRepo constructs an empty resource repository.
func NewResourceRepo() *ResourceRepo {
	return &ResourceRepo{byID: make(map[domain.ResourceID]*domain.ResourceHandle)}
}

// Save stores a resource handle.
func (r *ResourceRepo) Save(handle *domain.ResourceHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[handle.ID()] = handle
	return nil
}

// Get returns a handle by id.
func (r *ResourceRepo) Get(id domain.ResourceID) (*domain.ResourceHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.byID[id]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "resource", ID: string(id)}
	}
	return h, nil
}

// Delete removes a handle.
func (r *ResourceRepo) Delete(id domain.ResourceID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

// ListByOwner returns handles owned by window.
func (r *ResourceRepo) ListByOwner(window domain.WindowID) ([]*domain.ResourceHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.ResourceHandle, 0)
	for _, h := range r.byID {
		if h.Owner() == window {
			out = append(out, h)
		}
	}
	return out, nil
}

// ExecutorRegistry maps command names to executors.
type ExecutorRegistry struct {
	mu     sync.RWMutex
	byName map[domain.CommandName]domain.CommandExecutor
}

// NewExecutorRegistry constructs an empty registry.
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{byName: make(map[domain.CommandName]domain.CommandExecutor)}
}

// Register binds an executor to a command name.
func (r *ExecutorRegistry) Register(name domain.CommandName, exec domain.CommandExecutor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[name]; exists {
		return &domain.ErrConflict{Message: "executor already registered: " + string(name)}
	}
	r.byName[name] = exec
	return nil
}

// Get returns an executor by command name.
func (r *ExecutorRegistry) Get(name domain.CommandName) (domain.CommandExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.byName[name]
	return e, ok
}
