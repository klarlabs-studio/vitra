// Package plugin defines the Phase 3 plugin SDK: manifests, contributions,
// lifecycle, and a registry that prevents silent privilege expansion.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.klarlabs.de/vitra/domain"
)

// SemVer is a simple major.minor.patch version for plugin compatibility.
type SemVer struct {
	Major int
	Minor int
	Patch int
}

func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// CompatibleWith reports whether v satisfies a minimum required version
// (major must match; minor/patch must be >=).
func (v SemVer) CompatibleWith(min SemVer) bool {
	if v.Major != min.Major {
		return false
	}
	if v.Minor != min.Minor {
		return v.Minor > min.Minor
	}
	return v.Patch >= min.Patch
}

// Manifest declares a plugin's identity and privileged surface.
type Manifest struct {
	ID          domain.PluginID
	Name        string
	Version     SemVer
	Description string
	Permissions []domain.PermissionName // declared privileged surface
	MinKernel   SemVer
}

// Validate checks manifest invariants.
func (m Manifest) Validate() error {
	if m.ID == "" {
		return &domain.ErrValidation{Message: "plugin id is required"}
	}
	if m.Name == "" {
		return &domain.ErrValidation{Message: "plugin name is required"}
	}
	if len(m.Permissions) == 0 {
		return &domain.ErrValidation{Message: "plugin must declare at least one permission"}
	}
	seen := map[domain.PermissionName]struct{}{}
	for _, p := range m.Permissions {
		if p == "" {
			return &domain.ErrValidation{Message: "permission name must not be empty"}
		}
		if _, ok := seen[p]; ok {
			return &domain.ErrValidation{Message: "duplicate permission in manifest: " + string(p)}
		}
		seen[p] = struct{}{}
	}
	return nil
}

// Declares reports whether the manifest includes permission.
func (m Manifest) Declares(permission domain.PermissionName) bool {
	for _, p := range m.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// Contribution is what a plugin registers into the runtime.
type Contribution struct {
	Commands []*domain.CommandDefinition
	Events   []domain.EventName
}

// Plugin is the SDK contract for a Vitra plugin.
type Plugin interface {
	Manifest() Manifest
	Contribute() (Contribution, error)
}

// Lifecycle hooks are optional.
type Lifecycle interface {
	Init(ctx context.Context) error
	Close(ctx context.Context) error
}

// Registration is a loaded plugin plus its contribution.
type Registration struct {
	Manifest     Manifest
	Contribution Contribution
	Plugin       Plugin
}

// Registry manages plugin load order and permission isolation.
// Security invariant 6: a plugin cannot silently expand another plugin's
// permission scope.
type Registry struct {
	mu        sync.RWMutex
	byID      map[domain.PluginID]*Registration
	permOwner map[domain.PermissionName]domain.PluginID
	kernel    SemVer
}

// NewRegistry constructs an empty plugin registry for a kernel version.
func NewRegistry(kernel SemVer) *Registry {
	return &Registry{
		byID:      make(map[domain.PluginID]*Registration),
		permOwner: make(map[domain.PermissionName]domain.PluginID),
		kernel:    kernel,
	}
}

// Register validates and installs a plugin contribution.
func (r *Registry) Register(ctx context.Context, p Plugin) error {
	if p == nil {
		return errors.New("plugin is required")
	}
	m := p.Manifest()
	if err := m.Validate(); err != nil {
		return err
	}
	if !kernelMeets(r.kernel, m.MinKernel) {
		return &domain.ErrValidation{Message: fmt.Sprintf(
			"plugin %s requires kernel >= %s (have %s)", m.ID, m.MinKernel, r.kernel)}
	}
	contrib, err := p.Contribute()
	if err != nil {
		return fmt.Errorf("plugin %s contribute: %w", m.ID, err)
	}
	for _, cmd := range contrib.Commands {
		if cmd == nil {
			return &domain.ErrValidation{Message: "nil command in contribution"}
		}
		if !m.Declares(cmd.Permission()) {
			return &domain.ErrValidation{Message: fmt.Sprintf(
				"plugin %s command %s requires undeclared permission %s",
				m.ID, cmd.Name(), cmd.Permission())}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[m.ID]; exists {
		return &domain.ErrConflict{Message: "plugin already registered: " + string(m.ID)}
	}
	// Permission ownership: first plugin to declare a permission owns it.
	// Another plugin cannot claim the same permission (invariant 6).
	for _, perm := range m.Permissions {
		if owner, ok := r.permOwner[perm]; ok && owner != m.ID {
			return &domain.ErrConflict{Message: fmt.Sprintf(
				"permission %s already owned by plugin %s; %s cannot expand it",
				perm, owner, m.ID)}
		}
	}
	if lc, ok := p.(Lifecycle); ok {
		if err := lc.Init(ctx); err != nil {
			return fmt.Errorf("plugin %s init: %w", m.ID, err)
		}
	}
	for _, perm := range m.Permissions {
		r.permOwner[perm] = m.ID
	}
	r.byID[m.ID] = &Registration{Manifest: m, Contribution: contrib, Plugin: p}
	return nil
}

func kernelMeets(have, need SemVer) bool {
	if have.Major != need.Major {
		return have.Major > need.Major
	}
	if have.Minor != need.Minor {
		return have.Minor > need.Minor
	}
	return have.Patch >= need.Patch
}

// Get returns a registration by id.
func (r *Registry) Get(id domain.PluginID) (*Registration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.byID[id]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "plugin", ID: string(id)}
	}
	return reg, nil
}

// List returns all registrations.
func (r *Registry) List() []*Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Registration, 0, len(r.byID))
	for _, reg := range r.byID {
		out = append(out, reg)
	}
	return out
}

// OwnerOf returns which plugin owns a permission, if any.
func (r *Registry) OwnerOf(permission domain.PermissionName) (domain.PluginID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.permOwner[permission]
	return id, ok
}

// InspectSurface returns the combined declared privileged surface.
func (r *Registry) InspectSurface() []PermissionOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PermissionOwnership, 0, len(r.permOwner))
	for perm, owner := range r.permOwner {
		out = append(out, PermissionOwnership{Permission: perm, Plugin: owner})
	}
	return out
}

// PermissionOwnership is an inspectable permission→plugin mapping.
type PermissionOwnership struct {
	Permission domain.PermissionName
	Plugin     domain.PluginID
}

// CloseAll invokes Close on lifecycle plugins (best-effort reverse order).
func (r *Registry) CloseAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var first error
	for id, reg := range r.byID {
		if lc, ok := reg.Plugin.(Lifecycle); ok {
			if err := lc.Close(ctx); err != nil && first == nil {
				first = fmt.Errorf("plugin %s close: %w", id, err)
			}
		}
	}
	return first
}
