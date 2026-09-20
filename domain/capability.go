package domain

import (
	"errors"
	"path"
	"strings"
)

// PathScope constrains filesystem (or path-like) operations within a permission.
// Deny patterns always win over allow patterns. Matching is intentional and
// path-traversal-safe: requests containing ".." segments are rejected.
type PathScope struct {
	Allow []string
	Deny  []string
}

// Clone returns a defensive copy of the scope.
func (s PathScope) Clone() PathScope {
	out := PathScope{}
	if len(s.Allow) > 0 {
		out.Allow = append([]string(nil), s.Allow...)
	}
	if len(s.Deny) > 0 {
		out.Deny = append([]string(nil), s.Deny...)
	}
	return out
}

// Matches reports whether candidate is within this scope.
func (s PathScope) Matches(candidate string) (bool, DenialCode, string) {
	cleaned, err := normalizePath(candidate)
	if err != nil {
		return false, DenialPathDenied, err.Error()
	}
	for _, deny := range s.Deny {
		if matchPathPattern(deny, cleaned) {
			return false, DenialPathDenied, "path matches deny pattern"
		}
	}
	if len(s.Allow) == 0 {
		return false, DenialPathOutOfScope, "no allow patterns configured"
	}
	for _, allow := range s.Allow {
		if matchPathPattern(allow, cleaned) {
			return true, "", ""
		}
	}
	return false, DenialPathOutOfScope, "path not in allow list"
}

func normalizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path must not be empty")
	}
	if strings.Contains(p, "\x00") {
		return "", errors.New("path must not contain NUL")
	}
	// Reject explicit traversal before cleaning so "foo/../secret" cannot
	// sneak past an allow of "foo/**".
	parts := strings.Split(p, "/")
	for _, part := range parts {
		if part == ".." {
			return "", errors.New("path traversal is not allowed")
		}
	}
	cleaned := path.Clean(p)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path traversal is not allowed")
	}
	return cleaned, nil
}

// matchPathPattern supports a small intentional glob:
//   - "**" matches any remaining path segments
//   - "*" matches a single path segment
//   - otherwise exact segment equality
func matchPathPattern(pattern, candidate string) bool {
	pattern = path.Clean(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	return matchSegments(splitPath(pattern), splitPath(candidate))
}

func splitPath(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" || p == "." {
		return nil
	}
	return strings.Split(p, "/")
}

func matchSegments(pattern, candidate []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			if len(pattern) == 1 {
				return true
			}
			// Try consuming zero or more candidate segments.
			for i := 0; i <= len(candidate); i++ {
				if matchSegments(pattern[1:], candidate[i:]) {
					return true
				}
			}
			return false
		}
		if len(candidate) == 0 {
			return false
		}
		if pattern[0] != "*" && pattern[0] != candidate[0] {
			return false
		}
		pattern = pattern[1:]
		candidate = candidate[1:]
	}
	return len(candidate) == 0
}

// PermissionSpec declares one permission inside a capability grant, with an
// optional path scope for filesystem-like operations.
type PermissionSpec struct {
	Name      PermissionName
	PathScope *PathScope
}

// Clone returns a defensive copy.
func (p PermissionSpec) Clone() PermissionSpec {
	out := PermissionSpec{Name: p.Name}
	if p.PathScope != nil {
		scope := p.PathScope.Clone()
		out.PathScope = &scope
	}
	return out
}

// CapabilityGrant is the aggregate root for a named capability grant.
// Grants are deny-by-default: a caller must match window, origin, and
// permission (and path scope when present).
type CapabilityGrant struct {
	name        GrantName
	description string
	windows     []WindowID
	origins     []Origin
	permissions []PermissionSpec
}

// NewCapabilityGrant creates a validated grant. Windows and origins must be
// non-empty — ambient "all windows / all origins" grants are intentionally
// unsupported so least privilege remains the default.
func NewCapabilityGrant(
	name GrantName,
	description string,
	windows []WindowID,
	origins []Origin,
	permissions []PermissionSpec,
) (*CapabilityGrant, error) {
	if name == "" {
		return nil, &ErrValidation{Message: "grant name is required"}
	}
	if len(windows) == 0 {
		return nil, &ErrValidation{Message: "grant must list at least one window"}
	}
	if len(origins) == 0 {
		return nil, &ErrValidation{Message: "grant must list at least one origin"}
	}
	if len(permissions) == 0 {
		return nil, &ErrValidation{Message: "grant must list at least one permission"}
	}
	seenPerm := make(map[PermissionName]struct{}, len(permissions))
	perms := make([]PermissionSpec, 0, len(permissions))
	for _, p := range permissions {
		if p.Name == "" {
			return nil, &ErrValidation{Message: "permission name is required"}
		}
		if _, dup := seenPerm[p.Name]; dup {
			return nil, &ErrValidation{Message: "duplicate permission in grant: " + string(p.Name)}
		}
		seenPerm[p.Name] = struct{}{}
		perms = append(perms, p.Clone())
	}
	wins := append([]WindowID(nil), windows...)
	orgs := append([]Origin(nil), origins...)
	return &CapabilityGrant{
		name:        name,
		description: description,
		windows:     wins,
		origins:     orgs,
		permissions: perms,
	}, nil
}

// Name returns the grant name.
func (g *CapabilityGrant) Name() GrantName { return g.name }

// Description returns the human-readable description.
func (g *CapabilityGrant) Description() string { return g.description }

// Windows returns a copy of allowed window ids.
func (g *CapabilityGrant) Windows() []WindowID {
	return append([]WindowID(nil), g.windows...)
}

// Origins returns a copy of allowed origins.
func (g *CapabilityGrant) Origins() []Origin {
	return append([]Origin(nil), g.origins...)
}

// Permissions returns a defensive copy of permission specs.
func (g *CapabilityGrant) Permissions() []PermissionSpec {
	out := make([]PermissionSpec, len(g.permissions))
	for i, p := range g.permissions {
		out[i] = p.Clone()
	}
	return out
}

// Decision is the result of evaluating a capability request.
type Decision struct {
	Allowed    bool
	Grant      GrantName
	Permission PermissionName
	Code       DenialCode
	Reason     string
}

// Authorize evaluates whether caller may exercise permission, optionally
// against a path resource. Path is ignored when the matching permission has
// no PathScope.
func (g *CapabilityGrant) Authorize(caller Caller, permission PermissionName, resourcePath string) Decision {
	if !containsWindow(g.windows, caller.Window) {
		return Decision{
			Permission: permission,
			Code:       DenialWindowMismatch,
			Reason:     "caller window is not in grant",
		}
	}
	if !containsOrigin(g.origins, caller.Origin) {
		return Decision{
			Permission: permission,
			Code:       DenialOriginMismatch,
			Reason:     "caller origin is not in grant",
		}
	}
	spec, ok := findPermission(g.permissions, permission)
	if !ok {
		return Decision{
			Permission: permission,
			Code:       DenialPermissionAbsent,
			Reason:     "permission not present in grant",
		}
	}
	if spec.PathScope != nil {
		ok, code, reason := spec.PathScope.Matches(resourcePath)
		if !ok {
			return Decision{
				Permission: permission,
				Code:       code,
				Reason:     reason,
			}
		}
	}
	return Decision{
		Allowed:    true,
		Grant:      g.name,
		Permission: permission,
		Reason:     "granted",
	}
}

func containsWindow(windows []WindowID, id WindowID) bool {
	for _, w := range windows {
		if w == id {
			return true
		}
	}
	return false
}

func containsOrigin(origins []Origin, o Origin) bool {
	for _, x := range origins {
		if x == o {
			return true
		}
	}
	return false
}

func findPermission(perms []PermissionSpec, name PermissionName) (PermissionSpec, bool) {
	for _, p := range perms {
		if p.Name == name {
			return p, true
		}
	}
	return PermissionSpec{}, false
}
