package domain

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// PathScope constrains filesystem (or path-like) operations within a permission.
//
// Candidates and patterns are normalized before matching: backslashes become
// forward slashes, and anything that is not an absolute POSIX path ("/a/b") or
// drive path ("C:/a/b") is rejected, as are ".." segments, NUL bytes, and UNC or
// device paths ("//host/share", `\\?\C:\`).
//
// Pattern syntax, applied per path segment:
//   - "**" matches zero or more whole segments
//   - any other segment is a path.Match pattern ("*", "?", "[a-z]"), so
//     "*" matches exactly one segment and "*.pem" matches one file name
//
// Deny always wins over allow, and deny matching ignores letter case so a deny
// cannot be sidestepped on case-insensitive filesystems (APFS, NTFS). Allow
// matching is case-sensitive, so ambiguity fails closed in both directions.
//
// PathScope is lexical: it cannot see symlinks. Adapters that touch the
// filesystem must resolve the real path and authorize that.
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

// Validate reports the first malformed allow or deny pattern.
func (s PathScope) Validate() error {
	for _, list := range [][]string{s.Allow, s.Deny} {
		for _, pattern := range list {
			if _, err := compilePathPattern(pattern); err != nil {
				return fmt.Errorf("path pattern %q: %w", pattern, err)
			}
		}
	}
	return nil
}

// Matches reports whether candidate is within this scope.
func (s PathScope) Matches(candidate string) (bool, DenialCode, string) {
	cleaned, err := normalizePath(candidate)
	if err != nil {
		return false, DenialPathDenied, err.Error()
	}
	segments := splitPath(cleaned)
	folded := splitPath(strings.ToLower(cleaned))
	for _, deny := range s.Deny {
		pattern, err := compilePathPattern(strings.ToLower(deny))
		// A deny pattern that cannot be compiled fails closed.
		if err != nil || matchSegments(pattern, folded) {
			return false, DenialPathDenied, "path matches deny pattern"
		}
	}
	if len(s.Allow) == 0 {
		return false, DenialPathOutOfScope, "no allow patterns configured"
	}
	for _, allow := range s.Allow {
		pattern, err := compilePathPattern(allow)
		if err == nil && matchSegments(pattern, segments) {
			return true, "", ""
		}
	}
	return false, DenialPathOutOfScope, "path not in allow list"
}

// normalizePath converts p to a clean, absolute, slash-separated path or
// rejects it.
func normalizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path must not be empty")
	}
	if strings.Contains(p, "\x00") {
		return "", errors.New("path must not contain NUL")
	}
	// Windows accepts both separators, so treat them alike everywhere: a
	// backslash must never hide a ".." segment or a directory boundary.
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.HasPrefix(p, "//") {
		return "", errors.New("UNC and device paths are not allowed")
	}
	// Reject explicit traversal before cleaning so "foo/../secret" cannot
	// sneak past an allow of "foo/**".
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return "", errors.New("path traversal is not allowed")
		}
	}
	if !isAbsolutePath(p) {
		return "", errors.New("path must be absolute")
	}
	return path.Clean(p), nil
}

// isAbsolutePath reports whether slash-separated p is rooted ("/a") or a
// Windows drive path ("C:/a"). Drive-relative paths ("C:a") are not absolute.
func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && p[2] == '/' &&
		(('a' <= p[0] && p[0] <= 'z') || ('A' <= p[0] && p[0] <= 'Z'))
}

// compilePathPattern normalizes pattern and splits it into segments, checking
// each glob segment's syntax.
func compilePathPattern(pattern string) ([]string, error) {
	cleaned, err := normalizePath(pattern)
	if err != nil {
		return nil, err
	}
	segments := splitPath(cleaned)
	for _, seg := range segments {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return nil, err
		}
	}
	return segments, nil
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
		if ok, _ := path.Match(pattern[0], candidate[0]); !ok {
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
		if p.PathScope != nil {
			if err := p.PathScope.Validate(); err != nil {
				return nil, &ErrValidation{Message: "permission " + string(p.Name) + ": " + err.Error()}
			}
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
