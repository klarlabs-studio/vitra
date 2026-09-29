// Command vitra is the Vitra developer CLI.
package main

import (
	"context"
	"crypto/ed25519"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bindings"
	"go.klarlabs.de/vitra/internal/packaging"
	"go.klarlabs.de/vitra/internal/provenance"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
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
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/updater"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "vitra: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("vitra %s\n", vitra.Version)
		return nil
	case "doctor":
		return doctor()
	case "inspect":
		return inspectDemo(args[1:])
	case "new":
		return scaffoldNew(args[1:])
	case "dev":
		return runDev(args[1:])
	case "build":
		return runBuild(args[1:])
	case "package":
		return runPackage(args[1:])
	case "generate":
		return runGenerate(args[1:])
	case "update-apply":
		return runUpdateApply(args[1:])
	case "update-check":
		return runUpdateCheck(args[1:])
	case "update-stage":
		return runUpdateStage(args[1:])
	case "update-sign":
		return runUpdateSign(args[1:])
	case "update-keygen":
		return runUpdateKeygen(args[1:])
	case "notary-setup":
		return runNotarySetup(args[1:])
	case "register-scheme":
		return registerScheme(args[1:])
	case "register-files":
		return registerFiles(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\nRun 'vitra help' for usage", args[0])
	}
}

func printUsage() {
	fmt.Println(`vitra — secure Go + web desktop runtime

Usage:
  vitra version              Print kernel version
  vitra doctor               Diagnose WebView / CGO prerequisites
  vitra new <dir> [--template vanilla|vite|react|svelte|vue]
                             Scaffold a starter desktop app (default: vanilla HTML; vite/react/svelte/vue add Vite frontends)
  vitra dev [dir]            Watch + run the app with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra build [dir]          Build the app binary with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra package --out <dir> [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path] [--maintainer name] [--description text] [--homepage url] [--categories list] [--keywords list] [--license spdx] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref] [--publish [--publish-execute]]
                             Stage Linux dir, build .deb / .rpm / .snap / .flatpak / AppDir / .AppImage, Windows win-dir/WiX/NSIS, Darwin .app/.dmg + provenance.json; --sign prints PlanSign; --sign-execute runs host tools; --publish prints store PlanPublish; --publish-execute runs non-interactive Executable steps
  vitra generate typescript [--app dir] [--out path] [--module name]
                             --app: build and run the app in dir to emit its own typed client
                             Emit TypeScript client stubs for official plugin commands
  vitra update-keygen [--out <dir>]
                             Generate an ed25519 update-signing key pair (writes priv.key + pub.key hex)
  vitra update-sign --artifact <path> --app-id <id> --version <ver> --privkey <ref> --out <manifest.json>
                     [--channel stable|beta] [--artifact-name name]
                             Digest + sign an update manifest (privkey: env:/file:/secret:; bare hex rejected)
  vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>
                             Fetch + verify a signed channel manifest (HTTP(S) client; does not install)
  vitra update-stage --out <dir> --manifest <json> --artifact <path>
                             Stage {out}/{app}/{channel}/manifest.json + artifact for static CDN upload
  vitra update-apply (--manifest <json> --artifact <path> | --base-url <url>) --app-id <id> [--channel name] --current-version <semver> --pubkey <hex> --dest <path> [--policy production|development]
                             Verify a signed update and atomically install it (local files or HTTP channel fetch)
  vitra notary-setup [--profile name]
                             Print a dry-run notarytool store-credentials plan (Darwin notarize bootstrap; not executed)
  vitra register-scheme <scheme> [app-id] [exec]
                             Register a URL scheme handler (Linux xdg / Darwin helper .app / Windows .reg)
  vitra register-files --mime <type> [--mime <type>] [--app-id id] [--exec path] [--name name]
                             Register MIME file associations (Linux xdg / Darwin helper .app / Windows .reg)
  vitra inspect capabilities Demo capability inspection against an in-memory runtime
  vitra help                 Show this help`)
}

func doctor() error {
	fmt.Println("vitra doctor")
	fmt.Printf("  go:      %s\n", runtime.Version())
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  cgo:     %s\n", map[bool]string{true: "enabled", false: "disabled"}[cgoEnabled()])
	fmt.Printf("  kernel:  %s\n", vitra.Version)

	host := currentHost()
	fmt.Printf("  adapter: %s\n", host.OS())
	for _, f := range []platform.Feature{
		platform.FeatureWindowCreate,
		platform.FeatureWindowNavigate,
		platform.FeatureWebViewMessage,
		platform.FeatureClipboard,
		platform.FeatureDialogOpen,
		platform.FeatureDialogSave,
		platform.FeatureDialogOpenDirectory,
		platform.FeatureMenuBar,
		platform.FeatureTray,
		platform.FeatureSingleInstance,
		platform.FeatureGlobalShortcut,
		platform.FeatureDeepLink,
		platform.FeatureDragDrop,
		platform.FeatureFileAssociation,
		platform.FeatureWindowChrome,
		platform.FeatureOpenURL,
	} {
		s := host.Features()[f]
		status := "missing"
		if s.Available {
			status = "ok"
		} else if s.Detail != "" {
			status = "unavailable — " + s.Detail
		}
		fmt.Printf("  %-18s %s\n", string(f)+":", status)
	}

	switch runtime.GOOS {
	case "linux":
		if out, err := exec.Command("pkg-config", "--exists", "webkit2gtk-4.1").CombinedOutput(); err != nil {
			fmt.Printf("  pkg-config: webkit2gtk-4.1 not found (%v %s)\n", err, strings.TrimSpace(string(out)))
			fmt.Println("  hint:     sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev")
		} else {
			fmt.Println("  pkg-config: webkit2gtk-4.1 ok")
		}
		fmt.Println("  native:   build/run with CGO_ENABLED=1 -tags vitra_native")
		fmt.Println("  note:     global shortcuts via XGrabKey on X11; Wayland stays unsupported (use MenuItem.Shortcut)")
	case "darwin":
		fmt.Println("  frameworks: Cocoa + WebKit (system)")
		fmt.Println("  native:   build/run with CGO_ENABLED=1 -tags vitra_native")
		fmt.Println("  note:     WKWebView DesktopHost under -tags vitra_native; global shortcuts via RegisterEventHotKey (Ctrl→Command)")
	case "windows":
		fmt.Println("  native:   build/run with CGO_ENABLED=1 -tags vitra_native (Win32 + WebView2Loader.dll)")
		fmt.Println("  note:     WebView2 Navigate/Eval/message need Evergreen Runtime; global shortcuts via RegisterHotKey")
	}

	fmt.Println("  packaging fold tools:")
	reportPackagingTool("appimagetool", packaging.ResolveAppImageTool, "VITRA_APPIMAGETOOL")
	reportPackagingTool("rpmbuild", packaging.ResolveRpmbuild, "VITRA_RPMBUILD")
	reportPackagingTool("snapcraft", packaging.ResolveSnapcraft, "VITRA_SNAPCRAFT")
	reportPackagingTool("flatpak-builder", packaging.ResolveFlatpakBuilder, "VITRA_FLATPAK_BUILDER")
	reportPackagingTool("candle", packaging.ResolveCandle, "VITRA_CANDLE")
	reportPackagingTool("light", packaging.ResolveLight, "VITRA_LIGHT")
	reportPackagingTool("makensis", packaging.ResolveMakensis, "VITRA_MAKENSIS")
	reportPackagingTool("hdiutil", packaging.ResolveHdiutil, "VITRA_HDIUTIL")
	fmt.Println("  packaging sign tools (ExecuteSign via --sign-execute):")
	reportPackagingTool("codesign", packaging.ResolveCodesign, "VITRA_CODESIGN")
	reportPackagingTool("signtool", packaging.ResolveSigntool, "VITRA_SIGNTOOL")
	reportPackagingTool("notarytool", packaging.ResolveNotarytool, "VITRA_NOTARYTOOL")
	reportPackagingTool("stapler", packaging.ResolveStapler, "VITRA_STAPLER")
	reportPackagingTool("gpg", packaging.ResolveGPG, "VITRA_GPG")
	reportPackagingTool("dpkg-sig", packaging.ResolveDpkgSig, "VITRA_DPKGSIG")
	reportPackagingTool("rpmsign", packaging.ResolveRpmsign, "VITRA_RPMSIGN")
	return nil
}

func reportPackagingTool(name string, resolve func() (string, error), envHint string) {
	path, err := resolve()
	if err != nil {
		fmt.Printf("    %-12s missing (set %s or install on PATH)\n", name+":", envHint)
		return
	}
	fmt.Printf("    %-12s ok (%s)\n", name+":", path)
}

func currentHost() platform.Host {
	switch runtime.GOOS {
	case "darwin":
		return darwin.New()
	case "windows":
		return windows.New()
	default:
		return linux.New()
	}
}

func cgoEnabled() bool {
	switch os.Getenv("CGO_ENABLED") {
	case "0":
		return false
	case "1":
		return true
	default:
		// Default Go toolchain enables cgo on platforms with a C compiler.
		_, err := exec.LookPath("gcc")
		if err != nil {
			_, err = exec.LookPath("cc")
		}
		return err == nil
	}
}

func scaffoldTypeScriptClient() (string, error) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.app"})
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialclipboard.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialbrowser.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialos.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialwindow.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialmenu.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialtray.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdragdrop.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdeeplink.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialshortcut.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialapp.New()); err != nil {
		return "", err
	}
	greet, err := domain.NewCommandDefinition("demo.greet", "Greet", "demo.greet")
	if err != nil {
		return "", err
	}
	cmds := []*domain.CommandDefinition{greet}
	var events []domain.EventName
	for _, reg := range rt.Plugins().List() {
		cmds = append(cmds, reg.Contribution.Commands...)
		events = append(events, reg.Contribution.Events...)
	}
	return bindings.GenerateTypeScript("vitra", vitra.Version, bindings.Untyped(cmds...), events)
}

func runDev(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	fmt.Println("vitra dev: watching for .go/.html/.css/.js changes (ctrl-c to stop)")
	var (
		cmd   *exec.Cmd
		stamp = map[string]time.Time{}
		first = true
	)
	restart := func() error {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		argsGo := appendNativeHostTags([]string{"run"})
		argsGo = append(argsGo, ".")
		cmd = exec.Command("go", argsGo...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("vitra dev: starting…")
		return cmd.Start()
	}
	for {
		changed := first
		first = false
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && (d.Name() == ".git" || d.Name() == "node_modules") {
					return fs.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".go", ".html", ".css", ".js", ".json":
			default:
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			prev, ok := stamp[path]
			if !ok || info.ModTime().After(prev) {
				stamp[path] = info.ModTime()
				if ok {
					changed = true
				}
			}
			return nil
		})
		if changed {
			if err := restart(); err != nil {
				fmt.Fprintf(os.Stderr, "vitra dev: start failed: %v\n", err)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func runBuild(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	argsGo := appendNativeHostTags([]string{"build"})
	argsGo = append(argsGo, "-o", "vitra-app", ".")
	return execGo(dir, argsGo...)
}

// appendNativeHostTags adds -tags vitra_native on desktop OSes that ship a
// DesktopHost adapter (Linux WebKitGTK, Darwin WKWebView, Windows WebView2).
func appendNativeHostTags(args []string) []string {
	if supportsNativeHostTag(runtime.GOOS) {
		return append(args, "-tags", "vitra_native")
	}
	return args
}

func supportsNativeHostTag(goos string) bool {
	switch goos {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

// generateFromApp runs the app in dir in code-generation mode (see
// app.EnvGenerateTypeScript) so the client covers the app's own typed
// commands, then writes it to outPath or stdout.
func generateFromApp(dir, outPath string) error {
	target := outPath
	if target == "" {
		f, err := os.CreateTemp("", "vitra-client-*.ts")
		if err != nil {
			return err
		}
		_ = f.Close()
		defer func() { _ = os.Remove(f.Name()) }()
		target = f.Name()
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), app.EnvGenerateTypeScript+"="+abs)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run app in %s: %w", dir, err)
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("app did not write a client (does it reach app.Run?): %w", err)
	}
	if outPath == "" {
		fmt.Print(string(body))
		return nil
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}

func runGenerate(args []string) error {
	if len(args) == 0 || args[0] != "typescript" {
		return fmt.Errorf("usage: vitra generate typescript [--app dir] [--out path] [--module name]")
	}
	outPath := ""
	module := "vitra"
	appDir := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--app":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app requires a directory")
			}
			appDir = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a path")
			}
			outPath = args[i]
		case "--module":
			i++
			if i >= len(args) {
				return fmt.Errorf("--module requires a name")
			}
			module = args[i]
		default:
			return fmt.Errorf("unknown generate flag %q", args[i])
		}
	}
	if appDir != "" {
		return generateFromApp(appDir, outPath)
	}

	rt, err := vitra.New(vitra.Config{AppID: "com.vitra.generate"})
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialclipboard.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialbrowser.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialos.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialwindow.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialmenu.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialtray.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdragdrop.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdeeplink.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialshortcut.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialapp.New()); err != nil {
		return err
	}
	var cmds []*domain.CommandDefinition
	var events []domain.EventName
	for _, reg := range rt.Plugins().List() {
		cmds = append(cmds, reg.Contribution.Commands...)
		events = append(events, reg.Contribution.Events...)
	}
	body, err := bindings.GenerateTypeScript(module, vitra.Version, bindings.Untyped(cmds...), events)
	if err != nil {
		return err
	}
	if outPath == "" {
		fmt.Print(body)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d commands)\n", outPath, len(cmds))
	return nil
}

func runUpdateApply(args []string) error {
	manifestPath, artifactPath, pubkeyHex, dest, policyEnv := "", "", "", "", ""
	baseURL, appID, channel, currentVersion := "", "", "stable", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--manifest requires a path")
			}
			manifestPath = args[i]
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		case "--base-url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--base-url requires a URL")
			}
			baseURL = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--pubkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--pubkey requires hex-encoded ed25519 public key")
			}
			pubkeyHex = args[i]
		case "--dest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--dest requires a path")
			}
			dest = args[i]
		case "--policy":
			i++
			if i >= len(args) {
				return fmt.Errorf("--policy requires production or development")
			}
			policyEnv = args[i]
		case "--current-version":
			i++
			if i >= len(args) {
				return fmt.Errorf("--current-version requires the installed version")
			}
			currentVersion = args[i]
		default:
			return fmt.Errorf("unknown update-apply flag %q", args[i])
		}
	}
	usage := "usage: vitra update-apply (--manifest <json> --artifact <path> | --base-url <url>) --app-id <id> [--channel name] --current-version <semver> --pubkey <hex> --dest <path> [--policy production|development]"
	if pubkeyHex == "" || dest == "" || appID == "" || currentVersion == "" {
		return fmt.Errorf("%s", usage)
	}
	localMode := manifestPath != "" || artifactPath != ""
	channelMode := baseURL != ""
	if localMode && channelMode {
		return fmt.Errorf("update-apply: use either local --manifest/--artifact or channel --base-url, not both")
	}
	if localMode && (manifestPath == "" || artifactPath == "") {
		return fmt.Errorf("%s", usage)
	}
	if !localMode && !channelMode {
		return fmt.Errorf("%s", usage)
	}

	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubkeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("pubkey must be %d-byte ed25519 key as hex", ed25519.PublicKeySize)
	}

	var m updater.Manifest
	var artifact []byte
	if channelMode {
		src := updater.ChannelSource{
			BaseURL: baseURL,
			AppID:   appID,
			Channel: updater.Channel(channel),
		}
		f := &updater.Fetcher{}
		m, err = f.FetchManifest(context.Background(), src)
		if err != nil {
			return err
		}
		artifact, err = f.FetchArtifact(context.Background(), src, m)
		if err != nil {
			return err
		}
	} else {
		rawManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(rawManifest, &m); err != nil {
			return fmt.Errorf("manifest: %w", err)
		}
		artifact, err = os.ReadFile(artifactPath)
		if err != nil {
			return err
		}
	}

	rt, err := vitra.New(vitra.Config{AppID: domain.AppID(appID)})
	if err != nil {
		return fmt.Errorf("--app-id: %w", err)
	}
	if policyEnv != "" {
		var env policy.Environment
		switch policyEnv {
		case string(policy.EnvProduction):
			env = policy.EnvProduction
		case string(policy.EnvDevelopment):
			env = policy.EnvDevelopment
		default:
			return fmt.Errorf("--policy: want %q or %q", policy.EnvProduction, policy.EnvDevelopment)
		}
		eng, err := policy.NewEngine(policy.Document{}, env)
		if err != nil {
			return err
		}
		rt.SetPolicy(eng)
	}
	installed := updater.Installed{AppID: appID, Channel: updater.Channel(channel), Version: currentVersion}
	plan, err := rt.ApplyUpdate(m, ed25519.PublicKey(pubBytes), artifact, dest, installed)
	if err != nil {
		return err
	}
	fmt.Printf("installed %s v%s (%s) → %s\n  sha256: %s\n", plan.AppID, plan.Version, plan.Channel, dest, plan.SHA256)
	return nil
}

func runUpdateCheck(args []string) error {
	baseURL, appID, channel, pubkeyHex := "", "", "stable", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--base-url":
			i++
			if i >= len(args) {
				return fmt.Errorf("--base-url requires a URL")
			}
			baseURL = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--pubkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--pubkey requires hex-encoded ed25519 public key")
			}
			pubkeyHex = args[i]
		default:
			return fmt.Errorf("unknown update-check flag %q", args[i])
		}
	}
	if baseURL == "" || appID == "" || pubkeyHex == "" {
		return fmt.Errorf("usage: vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>")
	}
	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubkeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("pubkey must be %d-byte ed25519 key as hex", ed25519.PublicKeySize)
	}
	src := updater.ChannelSource{
		BaseURL: baseURL,
		AppID:   appID,
		Channel: updater.Channel(channel),
	}
	manifestURL, err := src.ManifestURL()
	if err != nil {
		return err
	}
	f := &updater.Fetcher{}
	m, err := f.FetchManifest(context.Background(), src)
	if err != nil {
		return err
	}
	if err := updater.VerifyManifest(m, ed25519.PublicKey(pubBytes)); err != nil {
		return err
	}
	fmt.Printf("update available: %s v%s (%s)\n  manifest: %s\n  artifact: %s\n  sha256: %s\n",
		m.AppID, m.Version, m.Channel, manifestURL, m.Artifact, m.SHA256)
	return nil
}

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

func runUpdateStage(args []string) error {
	outDir, manifestPath, artifactPath := "", "", ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a directory")
			}
			outDir = args[i]
		case "--manifest":
			i++
			if i >= len(args) {
				return fmt.Errorf("--manifest requires a path")
			}
			manifestPath = args[i]
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		default:
			return fmt.Errorf("unknown update-stage flag %q", args[i])
		}
	}
	if outDir == "" || manifestPath == "" || artifactPath == "" {
		return fmt.Errorf("usage: vitra update-stage --out <dir> --manifest <json> --artifact <path>")
	}
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m updater.Manifest
	if err := json.Unmarshal(rawManifest, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	stage, err := updater.StageChannel(outDir, m, artifact)
	if err != nil {
		return err
	}
	fmt.Printf("staged update channel\n  root:     %s\n  manifest: %s\n  artifact: %s\n  version:  %s (%s)\n",
		stage.Root, stage.ManifestPath, stage.ArtifactPath, m.Version, m.Channel)
	return nil
}

func runUpdateKeygen(args []string) error {
	outDir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a directory")
			}
			outDir = args[i]
		default:
			return fmt.Errorf("unknown update-keygen flag %q", args[i])
		}
	}
	kp, err := updater.GenerateKeyPair()
	if err != nil {
		return err
	}
	if outDir == "" {
		fmt.Printf("update signing key pair\n  public:  %s\n  private: %s\n", kp.PublicHex, kp.PrivateHex)
		fmt.Println("store the private key via env:/file:/secret: refs; never pass bare hex to --privkey")
		return nil
	}
	privPath, pubPath, err := updater.WriteKeyPair(outDir, kp)
	if err != nil {
		return err
	}
	fmt.Printf("wrote update signing key pair\n  private: %s\n  public:  %s\n", privPath, pubPath)
	return nil
}

func runUpdateSign(args []string) error {
	artifactPath, appID, version, privRef, outPath, artifactName := "", "", "", "", "", ""
	channel := string(updater.ChannelStable)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--artifact":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact requires a path")
			}
			artifactPath = args[i]
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires an id")
			}
			appID = args[i]
		case "--version":
			i++
			if i >= len(args) {
				return fmt.Errorf("--version requires a version")
			}
			version = args[i]
		case "--privkey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--privkey requires env:/file:/secret: ref")
			}
			privRef = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a path")
			}
			outPath = args[i]
		case "--channel":
			i++
			if i >= len(args) {
				return fmt.Errorf("--channel requires a name")
			}
			channel = args[i]
		case "--artifact-name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--artifact-name requires a name")
			}
			artifactName = args[i]
		default:
			return fmt.Errorf("unknown update-sign flag %q", args[i])
		}
	}
	if artifactPath == "" || appID == "" || version == "" || privRef == "" || outPath == "" {
		return fmt.Errorf("usage: vitra update-sign --artifact <path> --app-id <id> --version <ver> --privkey <ref> --out <manifest.json> [--channel stable|beta] [--artifact-name name]")
	}
	if artifactName == "" {
		artifactName = filepath.Base(artifactPath)
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	priv, err := updater.LoadPrivateKeyRef(privRef)
	if err != nil {
		return err
	}
	m, err := updater.BuildSignedManifest(appID, version, updater.Channel(channel), artifactName, artifact, priv)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("signed update manifest\n  out:      %s\n  app:      %s\n  version:  %s (%s)\n  artifact: %s\n  sha256:   %s\n",
		outPath, m.AppID, m.Version, m.Channel, m.Artifact, m.SHA256)
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

func registerScheme(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vitra register-scheme <scheme> [app-id] [exec]")
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("register-scheme is only implemented on Linux (xdg), Darwin (helper .app), and Windows (.reg)")
	}
	scheme := args[0]
	appID := "com.vitra.app"
	if len(args) > 1 {
		appID = args[1]
	}
	execPath := ""
	if len(args) > 2 {
		execPath = args[2]
	} else {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	host := currentHost()
	type schemeRegistrar interface {
		RegisterURLScheme(scheme, appID, execPath string) error
	}
	reg, ok := host.(schemeRegistrar)
	if !ok {
		return fmt.Errorf("host does not support URL scheme registration")
	}
	if err := reg.RegisterURLScheme(scheme, appID, execPath); err != nil {
		return err
	}
	fmt.Printf("registered URL handler for %s:// → %s (%s)\n", scheme, execPath, appID)
	return nil
}

func registerFiles(args []string) error {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("register-files is only implemented on Linux (xdg), Darwin (helper .app), and Windows (.reg)")
	}
	var mimes []string
	appID := "com.vitra.app"
	name := ""
	execPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mime":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mime requires a type")
			}
			mimes = append(mimes, args[i])
		case "--app-id":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app-id requires a value")
			}
			appID = args[i]
		case "--exec":
			i++
			if i >= len(args) {
				return fmt.Errorf("--exec requires a path")
			}
			execPath = args[i]
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			name = args[i]
		default:
			return fmt.Errorf("unknown register-files flag %q", args[i])
		}
	}
	if len(mimes) == 0 {
		return fmt.Errorf("usage: vitra register-files --mime <type> [--mime <type>] [--app-id id] [--exec path] [--name name]")
	}
	if execPath == "" {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return err
		}
	}
	if name == "" {
		name = appID
	}
	host := currentHost()
	type fileRegistrar interface {
		RegisterFileAssociations(appID, execPath, name string, mimeTypes []string) error
	}
	reg, ok := host.(fileRegistrar)
	if !ok {
		return fmt.Errorf("host does not support file association registration")
	}
	if err := reg.RegisterFileAssociations(appID, execPath, name, mimes); err != nil {
		return err
	}
	fmt.Printf("registered file associations %s → %s (%s)\n", strings.Join(mimes, ","), execPath, appID)
	return nil
}

func execGo(dir string, goArgs ...string) error {
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func inspectDemo(args []string) error {
	if len(args) == 0 || args[0] != "capabilities" {
		return fmt.Errorf("usage: vitra inspect capabilities")
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.myapp"})
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialclipboard.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialbrowser.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialos.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialwindow.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialmenu.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialtray.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdragdrop.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdeeplink.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialshortcut.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialapp.New()); err != nil {
		return err
	}
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		return err
	}
	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"project file access",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{
				Name: "fs.read",
				PathScope: &domain.PathScope{
					Allow: []string{"/project/**"},
					Deny:  []string{"/project/.secrets/**"},
				},
			},
			{Name: "dialog.open"},
			{Name: "clipboard.read"},
			{Name: "browser.open"},
			{Name: "os.info"},
			{Name: "notifications.show"},
			{Name: "path.open", PathScope: &domain.PathScope{Allow: []string{"/project/**"}}},
			{Name: "window.create"},
			{Name: "window.close"},
			{Name: "window.chrome"},
			{Name: "menu.set"},
			{Name: "tray.set"},
			{Name: "dragdrop.receive"},
			{Name: "shortcut.register"},
			{Name: "app.quit"},
		},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(grant); err != nil {
		return err
	}
	surface, err := rt.InspectCapabilities("main")
	if err != nil {
		return err
	}
	fmt.Print(vitra.FormatInspectFull(rt.AppID(), rt.Plugins().InspectSurface(), surface))
	return nil
}
