package packaging

import (
	"fmt"
	"strings"
)

// PublishPlan describes store submission steps for a packaged artifact.
// It never invokes host tools or store APIs — print-only guidance.
type PublishPlan struct {
	Target       Target
	ArtifactPath string
	Store        string
	Steps        []CommandPlan
	Supported    bool
	Note         string
}

// PlanPublish returns a dry-run store submission plan for snap / flatpak
// (and a short note for other Linux package targets). Execution and Flathub
// PR automation remain out of scope.
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
		plan.Note = "plan only; snapcraft login + upload are not invoked"
		snapName := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
		plan.Steps = []CommandPlan{
			{
				Tool: "snapcraft",
				Args: []string{"login"},
				Note: "interactive Snap Store authentication (once per machine/CI)",
			},
			{
				Tool: "snapcraft",
				Args: []string{"register", snapName},
				Note: "register the snap name if not already owned",
			},
			{
				Tool: "snapcraft",
				Args: []string{"upload", artifactPath, "--release", "stable"},
				Note: "upload the folded .snap; adjust --release for candidate/beta/edge",
			},
		}
	case TargetLinuxFlatpak:
		plan.Supported = true
		plan.Store = "Flathub"
		plan.Note = "plan only; Flathub PR / flatpak-builder are not invoked"
		if appID == "" {
			appID = "com.example.App"
		}
		plan.Steps = []CommandPlan{
			{
				Tool: "git",
				Args: []string{"clone", "https://github.com/flathub/flathub.git"},
				Note: "fork + clone Flathub; add a new app branch per Flathub docs",
			},
			{
				Tool: "flatpak-builder",
				Args: []string{"--repo=repo", "--force-clean", "build-dir", "manifest.yml"},
				Note: "build from the staged Flatpak manifest (vitra package --format flatpak-dir)",
			},
			{
				Tool: "flatpak",
				Args: []string{"build-bundle", "repo", artifactPath, appID},
				Note: "optional local .flatpak bundle for smoke-testing before the Flathub PR",
			},
			{
				Tool: "gh",
				Args: []string{"pr", "create", "--repo", "flathub/flathub", "--title", "New app: " + appID},
				Note: "open the Flathub submission PR after pushing the app branch",
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
		}
	}
	fmt.Fprintf(&b, "  status:    %s\n", p.Note)
	return b.String()
}
