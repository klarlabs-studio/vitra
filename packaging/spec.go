// Package packaging models Phase 4 release artifacts and targets.
package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
	TargetLinuxRPM      Target = "linux-rpm"
	TargetLinuxSnap     Target = "linux-snap"
	TargetLinuxFlatpak  Target = "linux-flatpak"
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
	// Maintainer is the Debian control Maintainer field, snap contact:, DEP-5
	// Upstream-Contact, and Windows publisher when set.
	// Empty defaults to DefaultMaintainer for .deb/snap/copyright; WiX/NSIS fall back to Name.
	Maintainer string
	// Description is a short package summary used as the Debian control extended
	// Description, FreeDesktop Comment=, Windows ARP Comments / ARPCOMMENTS, and
	// Darwin Info.plist CFBundleGetInfoString.
	// Empty defaults to DefaultDescription.
	Description string
	// Homepage is an optional project URL (Debian Homepage, RPM URL, snap website,
	// AppStream <url type="homepage">, Windows ARP URLInfoAbout / ARPURLINFOABOUT
	// and HelpLink / ARPHELPLINK).
	Homepage string
	// Categories are FreeDesktop/AppStream categories (e.g. Utility, Development).
	// Empty defaults to []string{"Utility"}. The first mapped category also
	// selects the Debian control Section: (see DebianSection), RPM Group:
	// (see RPMGroup), and Darwin Info.plist LSApplicationCategoryType
	// (see LSApplicationCategoryType).
	Categories []string
	// Keywords are optional FreeDesktop/AppStream search terms (desktop Keywords=,
	// AppStream <keyword>, snap keywords). Empty omits them.
	Keywords []string
	// License is the SPDX id or LicenseRef-* used for AppStream <project_license>,
	// RPM License:, snap license:, Debian usr/share/doc/.../copyright, Windows
	// ARP LegalCopyright / ARPCOPYRIGHT, and Darwin Info.plist NSHumanReadableCopyright.
	// Empty defaults to DefaultLicense.
	License string
}

// DefaultMaintainer is used when Spec.Maintainer is empty for .deb packages.
const DefaultMaintainer = "Vitra Packaging <vitra@klarlabs.de>"

// DefaultDescription is used when Spec.Description is empty.
const DefaultDescription = "Secure Go + web desktop application packaged by Vitra."

// DefaultLicense is used when Spec.License is empty.
const DefaultLicense = "LicenseRef-proprietary"

// DefaultCategories is used when Spec.Categories is empty.
var DefaultCategories = []string{"Utility"}

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

// EffectiveLicense returns Spec.License or DefaultLicense.
func (s Spec) EffectiveLicense() string {
	if l := strings.TrimSpace(s.License); l != "" {
		return l
	}
	return DefaultLicense
}

// EffectiveCategories returns Spec.Categories or DefaultCategories.
func (s Spec) EffectiveCategories() []string {
	out := make([]string, 0, len(s.Categories))
	for _, c := range s.Categories {
		c = strings.TrimSpace(c)
		if c != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), DefaultCategories...)
	}
	return out
}

// DesktopCategories returns a FreeDesktop Categories= value (trailing semicolon).
func (s Spec) DesktopCategories() string {
	cats := s.EffectiveCategories()
	return strings.Join(cats, ";") + ";"
}

// DebianSection returns the Debian control Section: derived from
// EffectiveCategories. Unknown categories fall back to "utils".
func (s Spec) DebianSection() string {
	for _, cat := range s.EffectiveCategories() {
		switch strings.ToLower(strings.TrimSpace(cat)) {
		case "audiovideo", "video":
			return "video"
		case "audio":
			return "sound"
		case "development":
			return "devel"
		case "education":
			return "education"
		case "game":
			return "games"
		case "graphics":
			return "graphics"
		case "network":
			return "net"
		case "office":
			return "text"
		case "science":
			return "science"
		case "settings", "system":
			return "admin"
		case "utility":
			return "utils"
		}
	}
	return "utils"
}

// RPMGroup returns the RPM .spec Group: derived from EffectiveCategories.
// Unknown categories fall back to "Applications/System".
func (s Spec) RPMGroup() string {
	for _, cat := range s.EffectiveCategories() {
		switch strings.ToLower(strings.TrimSpace(cat)) {
		case "audiovideo", "audio", "video", "graphics":
			return "Applications/Multimedia"
		case "development":
			return "Development/Tools"
		case "education":
			return "Applications/Education"
		case "game":
			return "Amusements/Games"
		case "network":
			return "Applications/Internet"
		case "office":
			return "Applications/Productivity"
		case "science":
			return "Applications/Engineering"
		case "settings", "system":
			return "System Environment/Base"
		case "utility":
			return "Applications/System"
		}
	}
	return "Applications/System"
}

// LSApplicationCategoryType returns the Darwin Info.plist
// LSApplicationCategoryType UTI derived from EffectiveCategories.
// Unknown categories fall back to "public.app-category.utilities".
func (s Spec) LSApplicationCategoryType() string {
	for _, cat := range s.EffectiveCategories() {
		switch strings.ToLower(strings.TrimSpace(cat)) {
		case "audiovideo", "audio", "music":
			return "public.app-category.music"
		case "video":
			return "public.app-category.video"
		case "development":
			return "public.app-category.developer-tools"
		case "education":
			return "public.app-category.education"
		case "game":
			return "public.app-category.games"
		case "graphics":
			return "public.app-category.graphics-design"
		case "network":
			return "public.app-category.social-networking"
		case "office":
			return "public.app-category.productivity"
		case "science":
			return "public.app-category.reference"
		case "settings", "system", "utility":
			return "public.app-category.utilities"
		}
	}
	return "public.app-category.utilities"
}

// EffectiveKeywords returns trimmed Spec.Keywords (no default; empty means omit).
func (s Spec) EffectiveKeywords() []string {
	out := make([]string, 0, len(s.Keywords))
	for _, k := range s.Keywords {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// DesktopKeywords returns a FreeDesktop Keywords= value (trailing semicolon), or "".
func (s Spec) DesktopKeywords() string {
	kws := s.EffectiveKeywords()
	if len(kws) == 0 {
		return ""
	}
	return strings.Join(kws, ";") + ";"
}

// DesktopKeywordsLine returns "Keywords=…\n" when keywords are set, else "".
func (s Spec) DesktopKeywordsLine() string {
	if kw := s.DesktopKeywords(); kw != "" {
		return "Keywords=" + kw + "\n"
	}
	return ""
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

// RefreshArtifactDigest recomputes SHA-256 for a file artifact (e.g. after
// codesign/signtool mutates bytes). Directory bundles keep the prior digest.
func RefreshArtifactDigest(art *Artifact) error {
	if art == nil || art.Path == "" {
		return fmt.Errorf("artifact path is required")
	}
	info, err := os.Stat(art.Path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	raw, err := os.ReadFile(art.Path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	art.SHA256 = hex.EncodeToString(sum[:])
	return nil
}
