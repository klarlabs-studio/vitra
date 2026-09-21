package packaging

import (
	"fmt"
	"strings"
)

// PublishPlan describes store submission steps for a packaged artifact.
// PlanPublish is print-only; ExecutePublish runs steps marked Executable
// (never interactive login or Flathub PR creation).
type PublishPlan struct {
	Target       Target
	ArtifactPath string
	Store        string
	Steps        []CommandPlan
	Supported    bool
	Note         string
}

// ExecutePublishOptions is reserved for future publish knobs (kept for API
// symmetry with ExecuteSignOptions).
type ExecutePublishOptions struct{}

// PlanPublish returns a store submission plan for snap / flatpak (and a short
// note for other Linux package targets). Interactive login and Flathub PR
// automation remain operator-owned; ExecutePublish runs Executable steps only.
func PlanPublish(spec Spec, artifactPath string) (PublishPlan, error) {
	if err := spec.Validate(); err != nil {
		return PublishPlan{}, err
	}
	if artifactPath == "" {
		return PublishPlan{}, fmt.Errorf("artifact path is required")
	}
	target := Target("")
	if len(spec.Targets) > 0 {
		target = spec.Targets[0]
	}
	plan := PublishPlan{
		Target:       target,
		ArtifactPath: artifactPath,
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = "vitra-app"
	}
	appID := strings.TrimSpace(spec.AppID)
	switch target {
	case TargetLinuxSnap:
		plan.Supported = true
		plan.Store = "Snap Store"
		plan.Note = "ExecutePublish runs snapcraft upload only; login/register stay interactive"
		snapName := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		plan.Steps = []CommandPlan{
			{
				Tool: "snapcraft",
				Args: []string{"login"},
				Note: "interactive Snap Store authentication (once per machine/CI); not executed",
			},
			{
				Tool: "snapcraft",
				Args: []string{"register", snapName},
				Note: "register the snap name if not already owned; not executed",
			},
			{
				Tool:       "snapcraft",
				Args:       []string{"upload", artifactPath, "--release", "stable"},
				Note:       "upload the folded .snap; adjust --release for candidate/beta/edge",
				Executable: true,
			},
		}
	case TargetLinuxFlatpak:
		plan.Supported = true
		plan.Store = "Flathub"
		plan.Note = "ExecutePublish runs flatpak-builder only; Flathub PR / git clone stay operator-owned"
		if appID == "" {
			appID = "com.example.App"
		}
		plan.Steps = []CommandPlan{
			{
				Tool: "git",
				Args: []string{"clone", "https://github.com/flathub/flathub.git"},
				Note: "fork + clone Flathub; add a new app branch per Flathub docs; not executed",
			},
			{
				Tool:       "flatpak-builder",
				Args:       []string{"--repo=repo", "--force-clean", "build-dir", "manifest.yml"},
				Note:       "build from the staged Flatpak manifest (vitra package --format flatpak-dir)",
				Executable: true,
			},
			{
				Tool: "flatpak",
				Args: []string{"build-bundle", "repo", artifactPath, appID},
				Note: "optional local .flatpak bundle for smoke-testing before the Flathub PR; not executed",
			},
			{
				Tool: "gh",
				Args: []string{"pr", "create", "--repo", "flathub/flathub", "--title", "New app: " + appID},
				Note: "Flathub PR automation out of scope; not executed",
			},
		}
	case TargetLinuxDeb, TargetLinuxRPM, TargetLinuxAppImage:
		plan.Supported = false
		plan.Store = ""
		plan.Note = "no first-party store publish plan for " + string(target) + "; distribute the artifact directly or via your own apt/yum/AppImage update channel"
	default:
		plan.Supported = false
		plan.Note = "store publish planning is only defined for linux-snap and linux-flatpak"
	}
	return plan, nil
}

// ExecutePublish runs Supported plan steps marked Executable. It never runs
// snapcraft login/register, git clone, flatpak build-bundle, or gh pr create.
func ExecutePublish(plan PublishPlan, _ ExecutePublishOptions) error {
	if !plan.Supported {
		return fmt.Errorf("publish plan is not supported for target %s", plan.Target)
	}
	ran := 0
	for i, step := range plan.Steps {
		if !step.Executable {
			continue
		}
		if step.Tool == "" {
			return fmt.Errorf("publish step %d has no tool", i+1)
		}
		toolPath, err := resolvePublishTool(step.Tool)
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Tool, err)
		}
		args, err := expandPlanArgs(step.Args)
		if err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Tool, err)
		}
		if err := runSignCommand(toolPath, args); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Tool, err)
		}
		ran++
	}
	if ran == 0 {
		return fmt.Errorf("publish plan has no executable steps")
	}
	return nil
}

func resolvePublishTool(name string) (string, error) {
	switch name {
	case "snapcraft":
		return ResolveSnapcraft()
	case "flatpak-builder":
		return ResolveFlatpakBuilder()
	default:
		return "", fmt.Errorf("tool %q is not executable via ExecutePublish (interactive or out of scope)", name)
	}
}

func (p PublishPlan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "publish plan (%s)\n", p.Target)
	fmt.Fprintf(&b, "  artifact:  %s\n", p.ArtifactPath)
	if p.Store != "" {
		fmt.Fprintf(&b, "  store:     %s\n", p.Store)
	}
	if p.Supported {
		for i, step := range p.Steps {
			fmt.Fprintf(&b, "  step %d:\n", i+1)
			fmt.Fprintf(&b, "    tool:    %s\n", step.Tool)
			fmt.Fprintf(&b, "    argv:    %s %s\n", step.Tool, strings.Join(step.Args, " "))
			fmt.Fprintf(&b, "    status:  %s\n", step.Note)
			if step.Executable {
				fmt.Fprintf(&b, "    run:     ExecutePublish / --publish-execute\n")
			}
		}
	}
	fmt.Fprintf(&b, "  status:    %s\n", p.Note)
	return b.String()
}
