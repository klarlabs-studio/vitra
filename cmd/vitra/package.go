package main

import (
	"debug/buildinfo"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"go.klarlabs.de/vitra/plugin"

	officialapp "go.klarlabs.de/vitra/plugin/official/app"
	officialbrowser "go.klarlabs.de/vitra/plugin/official/browser"
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
	officialdeeplink "go.klarlabs.de/vitra/plugin/official/deeplink"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialdragdrop "go.klarlabs.de/vitra/plugin/official/dragdrop"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
	officialmenu "go.klarlabs.de/vitra/plugin/official/menu"
	officialnotification "go.klarlabs.de/vitra/plugin/official/notification"
	officialos "go.klarlabs.de/vitra/plugin/official/os"
	officialpath "go.klarlabs.de/vitra/plugin/official/path"
	officialshortcut "go.klarlabs.de/vitra/plugin/official/shortcut"
	officialtray "go.klarlabs.de/vitra/plugin/official/tray"
	officialwindow "go.klarlabs.de/vitra/plugin/official/window"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/internal/packaging"
	"go.klarlabs.de/vitra/internal/provenance"
)

func runNotarySetup(args []string) error {
	profile := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			i++
			if i >= len(args) {
				return fmt.Errorf("--profile requires a name")
			}
			profile = args[i]
		default:
			return fmt.Errorf("unknown notary-setup flag %q", args[i])
		}
	}
	fmt.Print(packaging.PlanNotaryCredentials(profile).String())
	return nil
}

func runPackage(args []string) error {
	outDir := ""
	bin := "vitra-app"
	appID := "com.vitra.app"
	name := "Vitra App"
	version := vitra.Version
	format := "dir"
	icon := ""
	maintainer := ""
	description := ""
	homepage := ""
	categories := ""
	keywords := ""
	license := ""
	sign := false
	signExecute := false
	signFollowUps := false
	publish := false
	publishExecute := false
	signingIdentity := ""
	usage := "usage: vitra package --out <dir> [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path] [--maintainer name] [--description text] [--homepage url] [--categories list] [--keywords list] [--license spdx] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref] [--publish [--publish-execute]]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s", usage)
			}
			outDir = args[i]
		case "--bin":
			i++
			if i >= len(args) {
				return fmt.Errorf("--bin requires a path")
			}
			bin = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires a value")
			}
			appID = args[i]
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			name = args[i]
		case "--version":
			i++
			if i >= len(args) {
				return fmt.Errorf("--version requires a value")
			}
			version = args[i]
		case "--icon":
			i++
			if i >= len(args) {
				return fmt.Errorf("--icon requires a path")
			}
			icon = args[i]
		case "--maintainer":
			i++
			if i >= len(args) {
				return fmt.Errorf("--maintainer requires a value")
			}
			maintainer = args[i]
		case "--description":
			i++
			if i >= len(args) {
				return fmt.Errorf("--description requires a value")
			}
			description = args[i]
		case "--homepage":
			i++
			if i >= len(args) {
				return fmt.Errorf("--homepage requires a URL")
			}
			homepage = args[i]
		case "--categories":
			i++
			if i >= len(args) {
				return fmt.Errorf("--categories requires a comma-separated list")
			}
			categories = args[i]
		case "--keywords":
			i++
			if i >= len(args) {
				return fmt.Errorf("--keywords requires a comma-separated list")
			}
			keywords = args[i]
		case "--license":
			i++
			if i >= len(args) {
				return fmt.Errorf("--license requires an SPDX id or LicenseRef-*")
			}
			license = args[i]
		case "--sign":
			sign = true
		case "--sign-execute":
			sign = true
			signExecute = true
		case "--sign-follow-ups":
			signFollowUps = true
		case "--publish":
			publish = true
		case "--publish-execute":
			publish = true
			publishExecute = true
		case "--signing-identity":
			i++
			if i >= len(args) {
				return fmt.Errorf("--signing-identity requires a ref (env:/keychain:/file:/secret:)")
			}
			signingIdentity = args[i]
		case "--format":
			i++
			if i >= len(args) {
				return fmt.Errorf("--format requires dir, deb, rpm-dir, rpm, snap-dir, snap, flatpak-dir, flatpak, appdir, appimage, win-dir, wix, nsis-dir, msi, nsis, app-dir, or dmg")
			}
			format = args[i]
		default:
			return fmt.Errorf("unknown package flag %q", args[i])
		}
	}
	if signFollowUps && !signExecute {
		return fmt.Errorf("--sign-follow-ups requires --sign-execute")
	}
	if outDir == "" {
		return fmt.Errorf("%s", usage)
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("binary %q: %w (run vitra build first)", bin, err)
	}
	if icon != "" {
		if _, err := os.Stat(icon); err != nil {
			return fmt.Errorf("icon %q: %w", icon, err)
		}
	}

	var target packaging.Target
	switch format {
	case "dir":
		target = packaging.TargetLinuxDir
	case "deb":
		target = packaging.TargetLinuxDeb
	case "rpm-dir", "rpm":
		target = packaging.TargetLinuxRPM
	case "snap-dir", "snap":
		target = packaging.TargetLinuxSnap
	case "flatpak-dir", "flatpak":
		target = packaging.TargetLinuxFlatpak
	case "appdir", "appimage":
		target = packaging.TargetLinuxAppImage
	case "win-dir":
		target = packaging.TargetWindowsDir
	case "wix", "msi":
		target = packaging.TargetWindowsMSI
	case "nsis-dir", "nsis":
		target = packaging.TargetWindowsNSIS
	case "app-dir", "darwin-app":
		target = packaging.TargetDarwinApp
	case "dmg", "darwin-dmg":
		target = packaging.TargetDarwinDMG
	default:
		return fmt.Errorf("unknown format %q (want dir, deb, rpm-dir, rpm, snap-dir, snap, flatpak-dir, flatpak, appdir, appimage, win-dir, wix, nsis-dir, msi, nsis, app-dir, or dmg)", format)
	}
	spec := packaging.Spec{
		AppID:              appID,
		Version:            version,
		Name:               name,
		Targets:            []packaging.Target{target},
		Arch:               packaging.DefaultArch(),
		IconPath:           icon,
		Maintainer:         maintainer,
		Description:        description,
		Homepage:           homepage,
		License:            license,
		Sign:               sign,
		SigningIdentityRef: signingIdentity,
	}
	if categories != "" {
		for _, part := range strings.Split(categories, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				spec.Categories = append(spec.Categories, part)
			}
		}
	}
	if keywords != "" {
		for _, part := range strings.Split(keywords, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				spec.Keywords = append(spec.Keywords, part)
			}
		}
	}

	var art packaging.Artifact
	var err error
	switch format {
	case "dir":
		art, err = packaging.StageLinux(spec, bin, outDir)
	case "appdir":
		art, err = packaging.BuildAppDir(spec, bin, outDir)
	case "appimage":
		imgPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".appimage") {
			safe := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			imgPath = filepath.Join(outDir, fmt.Sprintf("%s-%s.AppImage", safe, packaging.DefaultArch()))
		}
		art, err = packaging.BuildAppImage(spec, bin, imgPath)
	case "deb":
		debPath := outDir
		if !strings.HasSuffix(outDir, ".deb") {
			pkg := strings.ReplaceAll(strings.ToLower(appID), ".", "-")
			debPath = filepath.Join(outDir, fmt.Sprintf("%s_%s_%s.deb", pkg, version, packaging.DefaultArch()))
		}
		art, err = packaging.BuildDeb(spec, bin, debPath)
	case "rpm-dir":
		art, err = packaging.BuildRPMDir(spec, bin, outDir)
	case "rpm":
		rpmPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".rpm") {
			pkg := strings.ReplaceAll(strings.ToLower(appID), ".", "-")
			rpmPath = filepath.Join(outDir, fmt.Sprintf("%s-%s.%s.rpm", pkg, version, packaging.DefaultArch()))
		}
		art, err = packaging.BuildRPM(spec, bin, rpmPath)
	case "snap-dir":
		art, err = packaging.BuildSnapDir(spec, bin, outDir)
	case "snap":
		snapPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".snap") {
			pkg := strings.ReplaceAll(strings.ToLower(appID), ".", "-")
			snapPath = filepath.Join(outDir, fmt.Sprintf("%s_%s_%s.snap", pkg, version, packaging.DefaultArch()))
		}
		art, err = packaging.BuildSnap(spec, bin, snapPath)
	case "flatpak-dir":
		art, err = packaging.BuildFlatpakDir(spec, bin, outDir)
	case "flatpak":
		fpPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".flatpak") {
			pkg := strings.ReplaceAll(strings.ToLower(appID), ".", "-")
			fpPath = filepath.Join(outDir, fmt.Sprintf("%s-%s.flatpak", pkg, version))
		}
		art, err = packaging.BuildFlatpak(spec, bin, fpPath)
	case "win-dir":
		art, err = packaging.StageWindows(spec, bin, outDir)
	case "wix":
		art, err = packaging.BuildWiXDir(spec, bin, outDir)
	case "nsis-dir":
		art, err = packaging.BuildNSISDir(spec, bin, outDir)
	case "msi":
		msiPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".msi") {
			safe := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			msiPath = filepath.Join(outDir, fmt.Sprintf("%s-%s.msi", safe, version))
		}
		art, err = packaging.BuildMSI(spec, bin, msiPath)
	case "nsis":
		nsisPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".exe") {
			safe := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			nsisPath = filepath.Join(outDir, fmt.Sprintf("%s-%s-setup.exe", safe, version))
		}
		art, err = packaging.BuildNSIS(spec, bin, nsisPath)
	case "app-dir", "darwin-app":
		art, err = packaging.StageDarwinApp(spec, bin, outDir)
	case "dmg", "darwin-dmg":
		dmgPath := outDir
		if !strings.HasSuffix(strings.ToLower(outDir), ".dmg") {
			safe := strings.ReplaceAll(strings.ToLower(name), " ", "-")
			dmgPath = filepath.Join(outDir, fmt.Sprintf("%s-%s.dmg", safe, version))
		}
		art, err = packaging.BuildDMG(spec, bin, dmgPath)
	}
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	doc := provenance.NewDocument(appID, version, runtime.Version()).
		WithArtifactDigest(raw).
		WithPluginInventory(officialPluginInventory())
	if bi, err := buildinfo.ReadFile(bin); err == nil {
		doc = doc.WithModulesFromBuildInfo(bi)
	} else if bi, ok := debug.ReadBuildInfo(); ok {
		doc = doc.WithModulesFromBuildInfo(bi)
	}
	body, err := doc.JSON()
	if err != nil {
		return err
	}
	provDir := art.Path
	if format == "deb" || format == "rpm" || format == "snap" || format == "flatpak" || format == "appimage" || format == "msi" || format == "nsis" || format == "app-dir" || format == "darwin-app" || format == "dmg" || format == "darwin-dmg" {
		provDir = filepath.Dir(art.Path)
	}
	if err := os.MkdirAll(provDir, 0o755); err != nil {
		return err
	}
	provPath := filepath.Join(provDir, "provenance.json")
	if err := os.WriteFile(provPath, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("packaged %s (%s)\n  artifact sha256: %s\n  provenance:      %s\n", art.Path, art.Target, art.SHA256, provPath)
	if spec.Sign {
		plan, err := packaging.PlanSign(spec, art.Path)
		if err != nil {
			return err
		}
		fmt.Print(plan.String())
		if signExecute {
			if err := packaging.ExecuteSign(plan, packaging.ExecuteSignOptions{FollowUps: signFollowUps}); err != nil {
				return err
			}
			art.Signed = true
			if err := packaging.RefreshArtifactDigest(&art); err != nil {
				return fmt.Errorf("refresh artifact digest after sign: %w", err)
			}
			fmt.Printf("signed %s with %s\n  status: %s\n  sha256: %s\n", art.Path, plan.Tool, art.String(), art.SHA256)
		}
	}
	if publish {
		pubPlan, err := packaging.PlanPublish(spec, art.Path)
		if err != nil {
			return err
		}
		fmt.Print(pubPlan.String())
		if publishExecute {
			if err := packaging.ExecutePublish(pubPlan, packaging.ExecutePublishOptions{}); err != nil {
				return err
			}
			fmt.Printf("published %s via %s (executable steps only)\n", art.Path, pubPlan.Store)
		}
	}
	return nil
}

// officialPluginInventory returns Manifest-derived plugin rows for packaging
// provenance (declared surface, not a claim that --bin embeds them).
func officialPluginInventory() []provenance.PluginInfo {
	out := make([]provenance.PluginInfo, 0, 7)
	for _, p := range []plugin.Plugin{officialfs.New(), officialdialog.New(), officialclipboard.New(), officialbrowser.New(), officialos.New(), officialnotification.New(), officialpath.New(), officialwindow.New(), officialmenu.New(), officialtray.New(), officialdragdrop.New(), officialdeeplink.New(), officialshortcut.New(), officialapp.New()} {
		m := p.Manifest()
		perms := make([]string, 0, len(m.Permissions))
		for _, perm := range m.Permissions {
			perms = append(perms, string(perm))
		}
		out = append(out, provenance.PluginInfo{
			ID:      string(m.ID),
			Version: m.Version.String(),
			Perms:   perms,
		})
	}
	return out
}
