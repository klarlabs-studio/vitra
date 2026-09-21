// Package packaging models Phase 4 release artifacts and targets.
package packaging

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
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
	// SigningIdentityRef is a keychain/CI secret reference — never raw key material.
	// Accepted forms: env:<name>, keychain:<name>, file:<path>, secret:<name>.
	SigningIdentityRef string
	// IconPath is an optional filesystem path to an app icon (.png / .svg / .icns).
	// Empty leaves packages without a staged icon file.
	IconPath string
	// Maintainer is the Debian control Maintainer field (and Windows publisher when set).
	// Empty defaults to DefaultMaintainer for .deb; WiX/NSIS fall back to Name.
	Maintainer string
	// Description is a short package summary used as the Debian control extended
	// Description and FreeDesktop Comment=. Empty defaults to DefaultDescription.
	Description string
}

// DefaultMaintainer is used when Spec.Maintainer is empty for .deb packages.
const DefaultMaintainer = "Vitra Packaging <vitra@klarlabs.de>"

// DefaultDescription is used when Spec.Description is empty.
const DefaultDescription = "Secure Go + web desktop application packaged by Vitra."

// EffectiveMaintainer returns Spec.Maintainer or DefaultMaintainer.
func (s Spec) EffectiveMaintainer() string {
	if m := strings.TrimSpace(s.Maintainer); m != "" {
		return m
	}
	return DefaultMaintainer
}

// EffectivePublisher returns Spec.Maintainer when set, otherwise Spec.Name
// (WiX Manufacturer / NSIS PRODUCT_PUBLISHER).
func (s Spec) EffectivePublisher() string {
	if m := strings.TrimSpace(s.Maintainer); m != "" {
		return m
	}
	return s.Name
}

// EffectiveDescription returns Spec.Description or DefaultDescription.
func (s Spec) EffectiveDescription() string {
	if d := strings.TrimSpace(s.Description); d != "" {
		return d
	}
	return DefaultDescription
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
	if s.SigningIdentityRef != "" {
		if err := validateSigningIdentityRef(s.SigningIdentityRef); err != nil {
			return err
		}
	}
	return nil
}

func validateSigningIdentityRef(ref string) error {
	ref = strings.TrimSpace(ref)
	upper := strings.ToUpper(ref)
	for _, bad := range []string{"BEGIN ", "PRIVATE KEY", "-----"} {
		if strings.Contains(upper, bad) {
			return errors.New("SigningIdentityRef must be a reference (env:/keychain:/file:/secret:), not raw key material")
		}
	}
	switch {
	case strings.HasPrefix(ref, "env:"),
		strings.HasPrefix(ref, "keychain:"),
		strings.HasPrefix(ref, "file:"),
		strings.HasPrefix(ref, "secret:"):
		if len(ref) <= strings.IndexByte(ref, ':')+1 {
			return fmt.Errorf("SigningIdentityRef %q has empty name after prefix", ref)
		}
		return nil
	default:
		return fmt.Errorf("SigningIdentityRef %q must start with env:, keychain:, file:, or secret: prefix", ref)
	}
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
