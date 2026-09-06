// Package packaging models Phase 4 release artifacts and targets.
package packaging

import (
	"errors"
	"fmt"
	"runtime"
)

// Target is a packaging destination.
type Target string

const (
	TargetDarwinDMG     Target = "darwin-dmg"
	TargetDarwinApp     Target = "darwin-app"
	TargetWindowsMSI    Target = "windows-msi"
	TargetWindowsNSIS   Target = "windows-nsis"
	TargetLinuxAppImage Target = "linux-appimage"
	TargetLinuxDeb      Target = "linux-deb"
)

// Spec describes a package to build.
type Spec struct {
	AppID   string
	Version string
	Name    string
	Targets []Target
	Arch    string // amd64, arm64, …
	Sign    bool
	// SigningIdentityRef is a keychain/CI secret reference — never a raw secret.
	SigningIdentityRef string
}

// Validate checks packaging invariants.
func (s Spec) Validate() error {
	if s.AppID == "" {
		return errors.New("app id is required")
	}
	if s.Version == "" {
		return errors.New("version is required")
	}
	if s.Name == "" {
		return errors.New("name is required")
	}
	if len(s.Targets) == 0 {
		return errors.New("at least one target is required")
	}
	if s.Sign && s.SigningIdentityRef == "" {
		return errors.New("signing requires SigningIdentityRef (secret must not live in project config)")
	}
	if !s.Sign && s.SigningIdentityRef != "" {
		return errors.New("SigningIdentityRef set but Sign is false")
	}
	return nil
}

// DefaultArch returns GOARCH.
func DefaultArch() string { return runtime.GOARCH }

// Artifact is a built package path + metadata.
type Artifact struct {
	Target   Target
	Path     string
	SHA256   string
	Signed   bool
	SBOMPath string
}

func (a Artifact) String() string {
	sig := "unsigned"
	if a.Signed {
		sig = "signed"
	}
	return fmt.Sprintf("%s %s (%s)", a.Target, a.Path, sig)
}
