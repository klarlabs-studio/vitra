// Package domain defines the core domain model for Vitra — a secure,
// capability-oriented desktop application runtime for Go + web frontends.
//
// The domain imports nothing outside the Go standard library.
package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var validNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)

// AppID identifies a Vitra application (reverse-DNS, e.g. com.example.myapp).
type AppID string

// WindowID identifies a window / WebView instance within a runtime.
type WindowID string

// Origin is a frontend content origin (scheme + host, optional path root).
// Packaged local content uses the conventional "app://local" origin.
type Origin string

// PermissionName identifies a privileged native operation (e.g. "fs.read").
type PermissionName string

// CommandName identifies an explicitly registered application command.
type CommandName string

// GrantName identifies a capability grant declared by the application.
type GrantName string

// PluginID identifies a plugin contribution.
type PluginID string

// ResourceID identifies a long-lived native resource handle.
type ResourceID string

// NewAppID validates and returns an AppID.
func NewAppID(s string) (AppID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("app id must not be empty")
	}
	if strings.ContainsAny(s, " \t\n") {
		return "", fmt.Errorf("app id %q must not contain whitespace", s)
	}
	return AppID(s), nil
}

// NewWindowID validates and returns a WindowID.
func NewWindowID(s string) (WindowID, error) {
	if s == "" {
		return "", errors.New("window id must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("window id %q is invalid: must match %s", s, validNamePattern.String())
	}
	return WindowID(s), nil
}

// NewOrigin validates and returns an Origin.
// Accepted forms: absolute URLs with a non-empty scheme (e.g. app://local,
// https://trusted.example). Empty origins are rejected.
func NewOrigin(s string) (Origin, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("origin must not be empty")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("origin %q is invalid: %w", s, err)
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("origin %q must include a scheme", s)
	}
	// Normalize trailing slash differences for opaque/local schemes.
	return Origin(s), nil
}

// Scheme returns the URL scheme of the origin, or empty if unparsable.
func (o Origin) Scheme() string {
	u, err := url.Parse(string(o))
	if err != nil {
		return ""
	}
	return u.Scheme
}

// IsPackaged returns true for the conventional packaged-app origin.
func (o Origin) IsPackaged() bool {
	return o == OriginPackagedLocal
}

// OriginPackagedLocal is the trusted origin for packaged application content.
const OriginPackagedLocal Origin = "app://local"

// NewPermissionName validates and returns a PermissionName.
func NewPermissionName(s string) (PermissionName, error) {
	if s == "" {
		return "", errors.New("permission name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("permission name %q is invalid: must match %s", s, validNamePattern.String())
	}
	return PermissionName(s), nil
}

// NewCommandName validates and returns a CommandName.
func NewCommandName(s string) (CommandName, error) {
	if s == "" {
		return "", errors.New("command name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("command name %q is invalid: must match %s", s, validNamePattern.String())
	}
	return CommandName(s), nil
}

// NewGrantName validates and returns a GrantName.
func NewGrantName(s string) (GrantName, error) {
	if s == "" {
		return "", errors.New("grant name must not be empty")
	}
	if !validNamePattern.MatchString(s) {
		return "", fmt.Errorf("grant name %q is invalid: must match %s", s, validNamePattern.String())
	}
	return GrantName(s), nil
}

// NewPluginID validates and returns a PluginID.
func NewPluginID(s string) (PluginID, error) {
	if s == "" {
		return "", errors.New("plugin id must not be empty")
	}
	return PluginID(s), nil
}

// NewResourceID validates and returns a ResourceID.
func NewResourceID(s string) (ResourceID, error) {
	if s == "" {
		return "", errors.New("resource id must not be empty")
	}
	return ResourceID(s), nil
}
