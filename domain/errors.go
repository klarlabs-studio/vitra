package domain

import "fmt"

// ErrNotFound indicates a requested entity does not exist.
type ErrNotFound struct {
	Entity string
	ID     string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s %q not found", e.Entity, e.ID)
}

// Is enables errors.Is matching against *ErrNotFound.
func (e *ErrNotFound) Is(target error) bool {
	_, ok := target.(*ErrNotFound)
	return ok
}

// ErrConflict indicates a uniqueness or duplicate constraint violation.
type ErrConflict struct {
	Message string
}

func (e *ErrConflict) Error() string {
	return e.Message
}

// Is enables errors.Is matching against *ErrConflict.
func (e *ErrConflict) Is(target error) bool {
	_, ok := target.(*ErrConflict)
	return ok
}

// ErrValidation indicates input or configuration validation failure.
type ErrValidation struct {
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Message
}

// Is enables errors.Is matching against *ErrValidation.
func (e *ErrValidation) Is(target error) bool {
	_, ok := target.(*ErrValidation)
	return ok
}

// ErrDenied indicates a capability gateway denial.
// Denials are deterministic and inspectable — Reason and Code are stable
// enough for diagnostics and tests of security invariants.
type ErrDenied struct {
	Permission PermissionName
	Window     WindowID
	Origin     Origin
	Code       DenialCode
	Reason     string
}

func (e *ErrDenied) Error() string {
	if e == nil {
		return "capability denied"
	}
	return fmt.Sprintf("capability denied: permission %q window %q origin %q: %s (%s)",
		e.Permission, e.Window, e.Origin, e.Reason, e.Code)
}

// Is enables errors.Is matching against *ErrDenied.
func (e *ErrDenied) Is(target error) bool {
	_, ok := target.(*ErrDenied)
	return ok
}

// DenialCode classifies why a capability check failed.
type DenialCode string

const (
	DenialNoGrant          DenialCode = "no_grant"
	DenialWindowMismatch   DenialCode = "window_mismatch"
	DenialOriginMismatch   DenialCode = "origin_mismatch"
	DenialPermissionAbsent DenialCode = "permission_absent"
	DenialPathDenied       DenialCode = "path_denied"
	DenialPathOutOfScope   DenialCode = "path_out_of_scope"
	DenialWindowClosed     DenialCode = "window_closed"
	DenialCommandMissing   DenialCode = "command_missing"
	DenialUnsupported      DenialCode = "unsupported_platform"
)
