// Package policy implements Phase 5 enterprise policy injection.
//
// Fleet administrators can tighten (never silently loosen production defaults)
// capability and update behaviour without baking secrets into the project.
package policy

import (
	"errors"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/updater"
)

// Document is an externally supplied enterprise policy.
type Document struct {
	// DenyPermissions permanently denies permissions even if the app grants them.
	DenyPermissions []domain.PermissionName `json:"deny_permissions"`
	// AllowedUpdateChannels restricts which update channels may be used.
	AllowedUpdateChannels []updater.Channel `json:"allowed_update_channels"`
	// RequireUpdateSignature is always treated as true in production evaluation.
	RequireUpdateSignature bool `json:"require_update_signature"`
	// DisableDevPrivileges ensures development-only privileges cannot leak
	// into release evaluation (security invariant 11).
	DisableDevPrivileges bool `json:"disable_dev_privileges"`
}

// Environment distinguishes development vs production evaluation.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

// Engine evaluates enterprise policy overlays.
type Engine struct {
	doc Document
	env Environment
}

// NewEngine constructs a policy engine.
func NewEngine(doc Document, env Environment) (*Engine, error) {
	if env == "" {
		return nil, errors.New("environment is required")
	}
	if env == EnvProduction {
		doc.RequireUpdateSignature = true
		doc.DisableDevPrivileges = true
	}
	return &Engine{doc: doc, env: env}, nil
}

// AllowsPermission reports whether a permission survives enterprise deny lists.
func (e *Engine) AllowsPermission(p domain.PermissionName) bool {
	for _, d := range e.doc.DenyPermissions {
		if d == p {
			return false
		}
	}
	if e.doc.DisableDevPrivileges && isDevPermission(p) {
		return false
	}
	return true
}

func isDevPermission(p domain.PermissionName) bool {
	switch p {
	case "dev.hot_reload", "dev.inspect_raw_ipc", "dev.bypass_csp":
		return true
	default:
		return false
	}
}

// AllowsUpdateChannel reports whether a channel is permitted.
func (e *Engine) AllowsUpdateChannel(ch updater.Channel) bool {
	if len(e.doc.AllowedUpdateChannels) == 0 {
		return true
	}
	for _, a := range e.doc.AllowedUpdateChannels {
		if a == ch {
			return true
		}
	}
	return false
}

// RequireUpdateSignature reports whether unsigned updates are forbidden.
func (e *Engine) RequireUpdateSignature() bool {
	if e.env == EnvProduction {
		return true
	}
	return e.doc.RequireUpdateSignature
}

// OverlayDecision applies enterprise denies on top of a capability decision.
func (e *Engine) OverlayDecision(permission domain.PermissionName, d domain.Decision) domain.Decision {
	if !d.Allowed {
		return d
	}
	if !e.AllowsPermission(permission) {
		return domain.Decision{
			Permission: permission,
			Code:       domain.DenialNoGrant,
			Reason:     "denied by enterprise policy",
		}
	}
	return d
}
