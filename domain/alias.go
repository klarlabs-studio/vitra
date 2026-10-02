package domain

import "strings"

// DirResolver returns the real path of directory dir (slash form), and
// whether it could be resolved. Adapters back it with the filesystem; the
// domain stays free of I/O.
type DirResolver func(dir string) (real string, ok bool)

// WithDenyAliases returns a copy of s whose deny patterns also cover the real
// spelling of their literal directory. A deny written as /var/app/secret/**
// then also refuses /private/var/app/secret/... on macOS, where /var is a
// link to /private/var. Allow patterns are unchanged, so this only ever
// tightens the scope.
func (s PathScope) WithDenyAliases(resolve DirResolver) PathScope {
	out := s.Clone()
	seen := make(map[string]bool, len(out.Deny))
	for _, p := range out.Deny {
		seen[p] = true
	}
	for _, raw := range s.Deny {
		alias, ok := realSpelling(raw, resolve)
		if ok && !seen[alias] {
			out.Deny = append(out.Deny, alias)
			seen[alias] = true
		}
	}
	return out
}

// realSpelling rewrites pattern's literal directory to its real path.
func realSpelling(pattern string, resolve DirResolver) (string, bool) {
	segments, err := compilePathPattern(pattern)
	if err != nil {
		return "", false
	}
	root := patternRoot(pattern, segments)
	cleaned, err := normalizePath(pattern)
	if err != nil || !strings.HasPrefix(cleaned, root) {
		return "", false
	}
	real, ok := resolve(root)
	if !ok {
		return "", false
	}
	real, err = normalizePath(real)
	if err != nil || real == root {
		return "", false
	}
	rest := strings.TrimPrefix(cleaned, root)
	if rest != "" && !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return strings.TrimSuffix(real, "/") + rest, true
}

// WithDenyAliases returns a copy of g with PathScope.WithDenyAliases applied
// to every path-scoped permission.
func (g *CapabilityGrant) WithDenyAliases(resolve DirResolver) (*CapabilityGrant, error) {
	perms := make([]PermissionSpec, 0, len(g.permissions))
	for _, p := range g.permissions {
		p = p.Clone()
		if p.PathScope != nil {
			scope := p.PathScope.WithDenyAliases(resolve)
			p.PathScope = &scope
		}
		perms = append(perms, p)
	}
	return NewCapabilityGrant(g.name, g.description, g.windows, g.origins, perms)
}
