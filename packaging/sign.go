package packaging

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Overridable for tests.
var (
	lookPathCodesign   = exec.LookPath
	lookPathSigntool   = exec.LookPath
	lookPathNotarytool = exec.LookPath
	lookPathStapler    = exec.LookPath
	lookPathGPG        = exec.LookPath
	lookPathDpkgSig    = exec.LookPath
	lookPathRpmsign    = exec.LookPath
	lookupEnv          = os.LookupEnv
	statPath           = os.Stat
	runSignCommand     = func(tool string, args []string) error {
		cmd := exec.Command(tool, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
)

// SignPlan is a dry-run description of how an artifact would be signed.
// It never embeds secret material — only the SigningIdentityRef and a
// non-secret resolvability note. Use ExecuteSign to invoke host tools.
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
	// Prep are optional one-time setup steps (Darwin notarytool store-credentials).
	Prep []CommandPlan
	// FollowUps are optional post-sign steps (Darwin notarize + staple).
	FollowUps []CommandPlan
}

// CommandPlan is a dry-run argv for a packaging host tool.
type CommandPlan struct {
	Tool string
	Args []string
	Note string
}

// ExecuteSignOptions controls whether Darwin notarize/staple follow-ups run.
type ExecuteSignOptions struct {
	// FollowUps runs plan.FollowUps after the primary sign tool succeeds.
	FollowUps bool
}

// NotaryCredentialsPlan is a dry-run for one-time notarytool store-credentials
// bootstrap. Vitra never runs it (interactive / secret password).
type NotaryCredentialsPlan struct {
	Profile string
	Steps   []CommandPlan
	Note    string
}

// PlanNotaryCredentials returns argv guidance for creating a keychain profile
// used by Darwin PlanSign follow-ups. Profile defaults to "vitra-notary" when
// empty or still a ${NOTARYTOOL_PROFILE} placeholder.
func PlanNotaryCredentials(profile string) NotaryCredentialsPlan {
	profile = strings.TrimSpace(profile)
	if profile == "" || profile == "${NOTARYTOOL_PROFILE}" {
		profile = "vitra-notary"
	}
	return NotaryCredentialsPlan{
		Profile: profile,
		Steps: []CommandPlan{
			{
				Tool: "notarytool",
				Args: []string{
					"store-credentials", profile,
					"--apple-id", "${APPLE_ID}",
					"--team-id", "${APPLE_TEAM_ID}",
					"--password", "${APP_SPECIFIC_PASSWORD}",
				},
				Note: "one-time interactive bootstrap; set APPLE_ID / APPLE_TEAM_ID / APP_SPECIFIC_PASSWORD (app-specific password) — values never printed by Vitra",
			},
		},
		Note: "plan only; notarytool store-credentials is not invoked",
	}
}

func (p NotaryCredentialsPlan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "notary credentials plan\n")
	fmt.Fprintf(&b, "  profile:   %s\n", p.Profile)
	for i, step := range p.Steps {
		fmt.Fprintf(&b, "  step %d:\n", i+1)
		fmt.Fprintf(&b, "    tool:    %s\n", step.Tool)
		fmt.Fprintf(&b, "    argv:    %s %s\n", step.Tool, strings.Join(step.Args, " "))
		fmt.Fprintf(&b, "    status:  %s\n", step.Note)
	}
	fmt.Fprintf(&b, "  status:    %s\n", p.Note)
	return b.String()
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
		plan.Note = "plan only until ExecuteSign; Artifact.Signed set after successful ExecuteSign"
		profile := notarizeProfileDisplay(spec.SigningIdentityRef)
		cred := PlanNotaryCredentials(profile)
		plan.Prep = append([]CommandPlan(nil), cred.Steps...)
		for i := range plan.Prep {
			plan.Prep[i].Note = "prep; " + plan.Prep[i].Note + " (see PlanNotaryCredentials / --notary-setup)"
		}
		plan.FollowUps = []CommandPlan{
			{
				Tool: "notarytool",
				Args: []string{"submit", artifactPath, "--keychain-profile", profile, "--wait"},
				Note: "follow-up; run via ExecuteSign with FollowUps (store-credentials first)",
			},
			{
				Tool: "stapler",
				Args: []string{"staple", artifactPath},
				Note: "follow-up; run via ExecuteSign with FollowUps",
			},
		}
	case TargetWindowsMSI, TargetWindowsNSIS:
		plan.Supported = true
		plan.Tool = "signtool"
		plan.Args = windowsSignArgs(spec.SigningIdentityRef, display, artifactPath)
		plan.Note = "plan only until ExecuteSign; Artifact.Signed set after successful ExecuteSign"
	case TargetLinuxDeb:
		plan.Supported = true
		plan.Tool = "dpkg-sig"
		plan.Args = []string{"--sign", "builder", "-k", display, artifactPath}
		plan.Note = "plan only until ExecuteSign; Artifact.Signed set after successful ExecuteSign"
	case TargetLinuxRPM:
		plan.Supported = true
		plan.Tool = "rpmsign"
		plan.Args = []string{"--addsign", artifactPath}
		plan.Note = "plan only until ExecuteSign; configure %_gpg_name to " + display + "; Artifact.Signed set after success"
	case TargetLinuxSnap:
		plan.Supported = true
		plan.Tool = "snapcraft"
		plan.Args = []string{"upload", artifactPath, "--release", "stable"}
		plan.Note = "plan only until ExecuteSign; Snap Store login required; Artifact.Signed set after success"
	case TargetLinuxAppImage, TargetLinuxFlatpak:
		plan.Supported = true
		plan.Tool = "gpg"
		plan.Args = []string{"--local-user", display, "--detach-sign", "--armor", artifactPath}
		plan.Note = "plan only until ExecuteSign; Artifact.Signed set after successful ExecuteSign"
	default:
		plan.Supported = false
		plan.Note = "package signing for this target is not orchestrated yet; ref validated only"
	}
	return plan, nil
}

// ExecuteSign resolves host tools, expands ${ENV} and <secret:…> placeholders
// in argv (secret:NAME → VITRA_SECRET_<NAME>), and runs the primary sign
// command. Follow-ups (notarytool/stapler) run only when opts.FollowUps.
func ExecuteSign(plan SignPlan, opts ExecuteSignOptions) error {
	if !plan.Supported {
		return fmt.Errorf("sign plan is not supported for target %s", plan.Target)
	}
	if plan.Tool == "" {
		return fmt.Errorf("sign plan has no tool")
	}
	if !plan.IdentityOK {
		return fmt.Errorf("signing identity is not resolvable: %s", plan.IdentityNote)
	}
	toolPath, err := resolvePlanTool(plan.Tool)
	if err != nil {
		return err
	}
	args, err := expandPlanArgs(plan.Args)
	if err != nil {
		return err
	}
	if err := runSignCommand(toolPath, args); err != nil {
		return fmt.Errorf("%s: %w", plan.Tool, err)
	}
	if !opts.FollowUps {
		return nil
	}
	for i, step := range plan.FollowUps {
		stepPath, err := resolvePlanTool(step.Tool)
		if err != nil {
			return fmt.Errorf("follow-up %d (%s): %w", i+1, step.Tool, err)
		}
		stepArgs, err := expandPlanArgs(step.Args)
		if err != nil {
			return fmt.Errorf("follow-up %d (%s): %w", i+1, step.Tool, err)
		}
		if err := runSignCommand(stepPath, stepArgs); err != nil {
			return fmt.Errorf("follow-up %d (%s): %w", i+1, step.Tool, err)
		}
	}
	return nil
}

func resolvePlanTool(name string) (string, error) {
	switch name {
	case "codesign":
		return ResolveCodesign()
	case "signtool":
		return ResolveSigntool()
	case "notarytool":
		return ResolveNotarytool()
	case "stapler":
		return ResolveStapler()
	case "gpg":
		return ResolveGPG()
	case "dpkg-sig":
		return ResolveDpkgSig()
	case "rpmsign":
		return ResolveRpmsign()
	case "snapcraft":
		return ResolveSnapcraft()
	default:
		return "", fmt.Errorf("unknown sign tool %q", name)
	}
}

var (
	envPlaceholder    = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	secretPlaceholder = regexp.MustCompile(`<secret:([^>]+)>`)
)

func expandPlanArgs(args []string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		expanded, err := expandEnvPlaceholders(arg)
		if err != nil {
			return nil, err
		}
		expanded, err = expandSecretPlaceholders(expanded)
		if err != nil {
			return nil, err
		}
		out[i] = expanded
	}
	return out, nil
}

func expandEnvPlaceholders(s string) (string, error) {
	var firstErr error
	out := envPlaceholder.ReplaceAllStringFunc(s, func(match string) string {
		if firstErr != nil {
			return match
		}
		name := envPlaceholder.FindStringSubmatch(match)[1]
		val, set := lookupEnv(name)
		if !set || val == "" {
			firstErr = fmt.Errorf("env %s is unset or empty (needed to expand %s)", name, match)
			return match
		}
		return val
	})
	return out, firstErr
}

// expandSecretPlaceholders maps <secret:name> to VITRA_SECRET_<NAME>
// (non-alnum → _). Plan output keeps the opaque placeholder (invariant 10).
func expandSecretPlaceholders(s string) (string, error) {
	var firstErr error
	out := secretPlaceholder.ReplaceAllStringFunc(s, func(match string) string {
		if firstErr != nil {
			return match
		}
		name := secretPlaceholder.FindStringSubmatch(match)[1]
		envName := secretEnvKey(name)
		val, set := lookupEnv(envName)
		if !set || val == "" {
			firstErr = fmt.Errorf("secret %q unset: set %s (or use env:/file:/keychain:)", name, envName)
			return match
		}
		return val
	})
	return out, firstErr
}

func secretEnvKey(name string) string {
	var b strings.Builder
	b.WriteString("VITRA_SECRET_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
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
	for i, step := range p.Prep {
		fmt.Fprintf(&b, "  prep %d:\n", i+1)
		fmt.Fprintf(&b, "    tool:    %s\n", step.Tool)
		fmt.Fprintf(&b, "    argv:    %s %s\n", step.Tool, strings.Join(step.Args, " "))
		fmt.Fprintf(&b, "    status:  %s\n", step.Note)
	}
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
		key := secretEnvKey(strings.TrimPrefix(ref, "secret:"))
		return true, "secret ref (ExecuteSign reads " + key + "; value never printed)"
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

// ResolveStapler returns the stapler binary (VITRA_STAPLER or PATH).
func ResolveStapler() (string, error) {
	return resolveSignTool("stapler", "VITRA_STAPLER", lookPathStapler)
}

// ResolveGPG returns the gpg binary (VITRA_GPG or PATH).
func ResolveGPG() (string, error) {
	return resolveSignTool("gpg", "VITRA_GPG", lookPathGPG)
}

// ResolveDpkgSig returns the dpkg-sig binary (VITRA_DPKGSIG or PATH).
func ResolveDpkgSig() (string, error) {
	return resolveSignTool("dpkg-sig", "VITRA_DPKGSIG", lookPathDpkgSig)
}

// ResolveRpmsign returns the rpmsign binary (VITRA_RPMSIGN or PATH).
func ResolveRpmsign() (string, error) {
	return resolveSignTool("rpmsign", "VITRA_RPMSIGN", lookPathRpmsign)
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
