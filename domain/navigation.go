package domain

import (
	"errors"
	"strings"
)

// NavigationKind classifies a navigation target for policy decisions.
type NavigationKind string

const (
	NavigationPackaged  NavigationKind = "packaged"
	NavigationTrusted   NavigationKind = "trusted_remote"
	NavigationUntrusted NavigationKind = "untrusted_remote"
	NavigationExternal  NavigationKind = "external_browser"
)

// NavigationPolicy decides how a window may change origin.
// Native APIs must not remain available merely because a WebView navigated
// from trusted packaged content to another origin (security invariant 2).
type NavigationPolicy struct {
	trustedOrigins []Origin
	allowExternal  bool
}

// NewNavigationPolicy builds a policy. Packaged local origin is always trusted.
func NewNavigationPolicy(trusted []Origin, allowExternal bool) (*NavigationPolicy, error) {
	seen := map[Origin]struct{}{OriginPackagedLocal: {}}
	out := []Origin{OriginPackagedLocal}
	for _, o := range trusted {
		if o == "" {
			return nil, errors.New("trusted origin must not be empty")
		}
		if _, ok := seen[o]; ok {
			continue
		}
		seen[o] = struct{}{}
		out = append(out, o)
	}
	return &NavigationPolicy{trustedOrigins: out, allowExternal: allowExternal}, nil
}

// Classify returns the navigation kind for an origin.
func (p *NavigationPolicy) Classify(origin Origin) NavigationKind {
	if origin.IsPackaged() {
		return NavigationPackaged
	}
	for _, t := range p.trustedOrigins {
		if t == origin {
			return NavigationTrusted
		}
	}
	scheme := origin.Scheme()
	if scheme == "http" || scheme == "https" {
		return NavigationUntrusted
	}
	return NavigationExternal
}

// AllowInWebView reports whether origin may load inside the application WebView.
// Untrusted remote content is denied by default (security invariant 12).
func (p *NavigationPolicy) AllowInWebView(origin Origin) Decision {
	kind := p.Classify(origin)
	switch kind {
	case NavigationPackaged, NavigationTrusted:
		return Decision{Allowed: true, Reason: "navigation permitted (" + string(kind) + ")"}
	case NavigationUntrusted:
		return Decision{
			Code:   DenialOriginMismatch,
			Reason: "untrusted remote content has no in-webview navigation by default",
		}
	default:
		if p.allowExternal {
			return Decision{Allowed: true, Reason: "external browser navigation permitted by policy"}
		}
		return Decision{
			Code:   DenialUnsupported,
			Reason: "external navigation is disabled by policy",
		}
	}
}

// DeepLinkPattern matches an application deep-link URL.
type DeepLinkPattern struct {
	Scheme string
	Host   string // empty = any
	Path   string // prefix; empty = any
}

// Match reports whether raw URL matches the pattern.
func (p DeepLinkPattern) Match(raw string) bool {
	o, err := NewOrigin(raw)
	if err != nil {
		return false
	}
	if p.Scheme != "" && o.Scheme() != p.Scheme {
		return false
	}
	if p.Host != "" {
		// Compare host portion after scheme://
		without := strings.TrimPrefix(string(o), p.Scheme+"://")
		host := without
		if i := strings.IndexByte(without, '/'); i >= 0 {
			host = without[:i]
		}
		if host != p.Host {
			return false
		}
	}
	if p.Path != "" && !strings.Contains(string(o), p.Path) {
		return false
	}
	return true
}
