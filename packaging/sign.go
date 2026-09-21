package packaging

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Overridable for tests.
var (
	lookPathCodesign   = exec.LookPath
	lookPathSigntool   = exec.LookPath
	lookPathNotarytool = exec.LookPath
	lookupEnv          = os.LookupEnv
	statPath           = os.Stat
)

// SignPlan is a dry-run description of how an artifact would be signed.
// It never embeds secret material — only the SigningIdentityRef and a
// non-secret resolvability note. Execution is intentionally out of scope.
type SignPlan struct {
	Target       Target
	ArtifactPath string
	Tool         string
	Args         []string
	IdentityRef  string
	IdentityOK   bool
	IdentityNote string
	Supported    bool
	Note         string
	// FollowUps are optional post-sign steps (Darwin notarize + staple).
	FollowUps []CommandPlan
}

// CommandPlan is a dry-run argv for a packaging host tool.
type CommandPlan struct {
	Tool string
	Args []string
	Note string
}

// PlanSign validates Spec signing fields and returns an argv plan for the
// target. It never invokes codesign/signtool/notary and never reads secret
// file bodies or env var values into the plan.
func PlanSign(spec Spec, artifactPath string) (SignPlan, error) {
	if err := spec.Validate(); err != nil {
		return SignPlan{}, err
	}
	if !spec.Sign {
		return SignPlan{}, fmt.Errorf("signing not requested (Sign=false)")
	}
	if artifactPath == "" {
		return SignPlan{}, fmt.Errorf("artifact path is required")
	}
	target := Target("")
	if len(spec.Targets) > 0 {
		target = spec.Targets[0]
	}
	ok, note := resolveIdentityStatus(spec.SigningIdentityRef)
	plan := SignPlan{
		Target:       target,
		ArtifactPath: artifactPath,
		IdentityRef:  spec.SigningIdentityRef,
		IdentityOK:   ok,
		IdentityNote: note,
	}
	display := identityDisplay(spec.SigningIdentityRef)
	switch target {
	case TargetDarwinApp, TargetDarwinDMG:
		plan.Supported = true
		plan.Tool = "codesign"
		plan.Args = []string{
			"--force", "--options", "runtime", "--timestamp",
			"--sign", display, artifactPath,
		}
		plan.Note = "dry-run only; codesign is not invoked and Artifact.Signed stays false"
		profile := notarizeProfileDisplay(spec.SigningIdentityRef)
		plan.FollowUps = []CommandPlan{
			{
				Tool: "notarytool",
				Args: []string{"submit", artifactPath, "--keychain-profile", profile, "--wait"},
				Note: "dry-run only; notarytool is not invoked (store-credentials first)",
			},
			{
				Tool: "stapler",
				Args: []string{"staple", artifactPath},
				Note: "dry-run only; stapler is not invoked",
			},
		}
	case TargetWindowsMSI, TargetWindowsNSIS:
		plan.Supported = true
		plan.Tool = "signtool"
		plan.Args = windowsSignArgs(spec.SigningIdentityRef, display, artifactPath)
		plan.Note = "dry-run only; signtool is not invoked and Artifact.Signed stays false"
	default:
		plan.Supported = false
		plan.Note = "package signing for this target is not orchestrated yet (dpkg-sig / rpmsign / AppImage); ref validated only"
	}
	return plan, nil
}

func (p SignPlan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "sign plan (%s)\n", p.Target)
	fmt.Fprintf(&b, "  artifact:  %s\n", p.ArtifactPath)
	fmt.Fprintf(&b, "  identity:  %s (%s)\n", p.IdentityRef, p.IdentityNote)
	if p.Supported {
		fmt.Fprintf(&b, "  tool:      %s\n", p.Tool)
		fmt.Fprintf(&b, "  argv:      %s %s\n", p.Tool, strings.Join(p.Args, " "))
	} else {
		fmt.Fprintf(&b, "  tool:      (none)\n")
	}
	fmt.Fprintf(&b, "  status:    %s\n", p.Note)
	for i, step := range p.FollowUps {
		fmt.Fprintf(&b, "  follow-up %d:\n", i+1)
		fmt.Fprintf(&b, "    tool:    %s\n", step.Tool)
		fmt.Fprintf(&b, "    argv:    %s %s\n", step.Tool, strings.Join(step.Args, " "))
		fmt.Fprintf(&b, "    status:  %s\n", step.Note)
	}
	if !p.IdentityOK {
		fmt.Fprintf(&b, "  warning:   identity ref is not resolvable\n")
	}
	return b.String()
}

// notarizeProfileDisplay picks a non-secret notarytool --keychain-profile
// placeholder. Prefer env:NOTARYTOOL_PROFILE; else keychain: name from the
// signing identity; else ${NOTARYTOOL_PROFILE}.
func notarizeProfileDisplay(signingRef string) string {
	if _, set := lookupEnv("NOTARYTOOL_PROFILE"); set {
		return "${NOTARYTOOL_PROFILE}"
	}
	ref := strings.TrimSpace(signingRef)
	if strings.HasPrefix(ref, "keychain:") {
		name := strings.TrimPrefix(ref, "keychain:")
		if name != "" {
			return name
		}
	}
	return "${NOTARYTOOL_PROFILE}"
}

func resolveIdentityStatus(ref string) (ok bool, note string) {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimPrefix(ref, "env:")
		val, set := lookupEnv(name)
		if set && val != "" {
			return true, "env " + name + " is set"
		}
		if set {
			return false, "env " + name + " is empty"
		}
		return false, "env " + name + " is unset"
	case strings.HasPrefix(ref, "file:"):
		path := strings.TrimPrefix(ref, "file:")
		if _, err := statPath(path); err != nil {
			return false, "file missing: " + path
		}
		return true, "file exists: " + path
	case strings.HasPrefix(ref, "keychain:"):
		return true, "keychain identity (not probed)"
	case strings.HasPrefix(ref, "secret:"):
		return true, "secret ref (not probed; inject via CI)"
	default:
		return false, "unrecognized ref"
	}
}

func identityDisplay(ref string) string {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "env:"):
		return "${" + strings.TrimPrefix(ref, "env:") + "}"
	case strings.HasPrefix(ref, "file:"):
		return strings.TrimPrefix(ref, "file:")
	case strings.HasPrefix(ref, "keychain:"):
		return strings.TrimPrefix(ref, "keychain:")
	case strings.HasPrefix(ref, "secret:"):
		return "<secret:" + strings.TrimPrefix(ref, "secret:") + ">"
	default:
		return ref
	}
}

func windowsSignArgs(ref, display, artifactPath string) []string {
	args := []string{"sign", "/fd", "SHA256"}
	if strings.HasPrefix(strings.TrimSpace(ref), "file:") {
		args = append(args, "/f", display)
	} else {
		args = append(args, "/n", display)
	}
	return append(args, artifactPath)
}

// ResolveCodesign returns the codesign binary (VITRA_CODESIGN or PATH).
func ResolveCodesign() (string, error) {
	return resolveSignTool("codesign", "VITRA_CODESIGN", lookPathCodesign)
}

// ResolveSigntool returns the signtool binary (VITRA_SIGNTOOL or PATH).
func ResolveSigntool() (string, error) {
	return resolveSignTool("signtool", "VITRA_SIGNTOOL", lookPathSigntool)
}

// ResolveNotarytool returns the notarytool binary (VITRA_NOTARYTOOL or PATH).
func ResolveNotarytool() (string, error) {
	return resolveSignTool("notarytool", "VITRA_NOTARYTOOL", lookPathNotarytool)
}

func resolveSignTool(name, envKey string, look func(string) (string, error)) (string, error) {
	if p := strings.TrimSpace(os.Getenv(envKey)); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s=%s: %w", envKey, p, err)
		}
		return p, nil
	}
	p, err := look(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH (set %s): %w", name, envKey, err)
	}
	return p, nil
}
