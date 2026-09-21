// Command vitra is the Vitra developer CLI.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/bindings"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/packaging"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
	"go.klarlabs.de/vitra/policy"
	"go.klarlabs.de/vitra/provenance"
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
  vitra new <dir>            Scaffold a starter desktop app
  vitra dev [dir]            Watch + run the app with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra build [dir]          Build the app binary with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra package --out <dir> [--format dir|deb|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path]
                             Stage Linux dir, build .deb / AppDir / .AppImage, Windows win-dir/WiX/NSIS, Darwin .app/.dmg + provenance.json
  vitra generate typescript [--out path] [--module name]
                             Emit TypeScript client stubs for official plugin commands
  vitra update-apply --manifest <json> --artifact <path> --pubkey <hex> --dest <path> [--policy production|development]
                             Verify a signed update and atomically install it
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
	return nil
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

func scaffoldNew(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vitra new <dir>")
	}
	dir := args[0]
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		return err
	}
	modPath := "example.com/" + filepath.Base(dir)
	if modPath == "example.com/." {
		modPath = "example.com/vitra-app"
	}
	files := map[string]string{
		"go.mod": scaffoldGoMod(modPath),
		"main.go": `package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"runtime"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
)

//go:embed frontend/*
var frontendRoot embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "app: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	assets, err := fs.Sub(frontendRoot, "frontend")
	if err != nil {
		return err
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.app"})
	if err != nil {
		return err
	}
	greet, _ := domain.NewCommandDefinition("demo.greet", "Greet", "demo.greet")
	_ = rt.RegisterCommand(greet, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		who, _ := input.(string)
		if who == "" {
			who = "world"
		}
		return map[string]any{"message": "Hello, " + who}, nil
	}))
	grant, _ := domain.NewCapabilityGrant(
		"demo", "demo", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "demo.greet"}},
	)
	_ = rt.RegisterGrant(grant)

	host := desktopHost()
	application, err := app.New(app.Options{
		AppID: "com.example.app", Title: "Vitra App", Assets: assets, Host: host, Runtime: rt,
		Window: app.WindowOptions{ID: "main", Width: 960, Height: 640},
	})
	if err != nil {
		return err
	}
	return application.Run(context.Background())
}

func desktopHost() app.DesktopHost {
	switch runtime.GOOS {
	case "darwin":
		return darwin.New()
	case "windows":
		return windows.New()
	default:
		return linux.New()
	}
}
`,
		"frontend/index.html": `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><title>Vitra App</title>
<style>body{font-family:Georgia,serif;margin:2rem;background:#111;color:#eee}
button{padding:.75rem 1rem;cursor:pointer}</style></head>
<body><h1>Vitra</h1><p>Secure desktop runtime starter.</p>
<button id="go">Invoke demo.greet</button><pre id="out"></pre>
<script>
document.getElementById("go").onclick=async()=>{
  try{document.getElementById("out").textContent=JSON.stringify(await window.vitra.invoke("demo.greet","Vitra"),null,2)}
  catch(e){document.getElementById("out").textContent=String(e)}
};
</script></body></html>
`,
		"README.md": "# Vitra app\n\n```bash\n# Native DesktopHost (Linux WebKitGTK / Darwin WKWebView / Windows WebView2)\nCGO_ENABLED=1 go run -tags vitra_native .\n# or\nvitra dev\n```\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("created %s\n", dir)
	fmt.Println("next: cd", dir, "&& vitra dev")
	return nil
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

func runGenerate(args []string) error {
	if len(args) == 0 || args[0] != "typescript" {
		return fmt.Errorf("usage: vitra generate typescript [--out path] [--module name]")
	}
	outPath := ""
	module := "vitra"
	for i := 1; i < len(args); i++ {
		switch args[i] {
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
	var cmds []*domain.CommandDefinition
	for _, reg := range rt.Plugins().List() {
		cmds = append(cmds, reg.Contribution.Commands...)
	}
	body := bindings.GenerateTypeScript(module, vitra.Version, cmds)
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
		default:
			return fmt.Errorf("unknown update-apply flag %q", args[i])
		}
	}
	if manifestPath == "" || artifactPath == "" || pubkeyHex == "" || dest == "" {
		return fmt.Errorf("usage: vitra update-apply --manifest <json> --artifact <path> --pubkey <hex> --dest <path> [--policy production|development]")
	}
	rawManifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var m updater.Manifest
	if err := json.Unmarshal(rawManifest, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	pubBytes, err := hex.DecodeString(strings.TrimSpace(pubkeyHex))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("pubkey must be %d-byte ed25519 key as hex", ed25519.PublicKeySize)
	}
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	rt, err := vitra.New(vitra.Config{AppID: domain.AppID(m.AppID)})
	if err != nil {
		// Fall back if manifest app id empty / invalid for Config.
		rt, err = vitra.New(vitra.Config{AppID: "com.vitra.update"})
		if err != nil {
			return err
		}
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
	plan, err := rt.ApplyUpdate(m, ed25519.PublicKey(pubBytes), artifact, dest)
	if err != nil {
		return err
	}
	fmt.Printf("installed %s v%s (%s) → %s\n  sha256: %s\n", plan.AppID, plan.Version, plan.Channel, dest, plan.SHA256)
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
	usage := "usage: vitra package --out <dir> [--format dir|deb|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path]"
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
		case "--format":
			i++
			if i >= len(args) {
				return fmt.Errorf("--format requires dir, deb, appdir, appimage, win-dir, wix, nsis-dir, msi, nsis, app-dir, or dmg")
			}
			format = args[i]
		default:
			return fmt.Errorf("unknown package flag %q", args[i])
		}
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
		return fmt.Errorf("unknown format %q (want dir, deb, appdir, appimage, win-dir, wix, nsis-dir, msi, nsis, app-dir, or dmg)", format)
	}
	spec := packaging.Spec{
		AppID:    appID,
		Version:  version,
		Name:     name,
		Targets:  []packaging.Target{target},
		Arch:     packaging.DefaultArch(),
		IconPath: icon,
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
		WithArtifactDigest(raw)
	doc.Plugins = []provenance.PluginInfo{
		{ID: string(officialfs.PluginID), Version: "1.0.0", Perms: []string{"fs.read", "fs.write"}},
		{ID: string(officialdialog.PluginID), Version: "1.0.0", Perms: []string{"dialog.open", "dialog.save"}},
	}
	doc.Capabilities = []string{"fs.read", "fs.write", "dialog.open", "dialog.save"}
	body, err := doc.JSON()
	if err != nil {
		return err
	}
	provDir := art.Path
	if format == "deb" || format == "appimage" || format == "msi" || format == "nsis" || format == "app-dir" || format == "darwin-app" || format == "dmg" || format == "darwin-dmg" {
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
	return nil
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

func scaffoldGoMod(modPath string) string {
	body := "module " + modPath + "\n\ngo 1.26\n\nrequire go.klarlabs.de/vitra v0.0.0\n"
	if root := os.Getenv("VITRA_MODULE_PATH"); root != "" {
		body += "\nreplace go.klarlabs.de/vitra => " + root + "\n"
	}
	return body
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
					Allow: []string{"${PROJECT_DIR}/**"},
					Deny:  []string{"${PROJECT_DIR}/.secrets/**"},
				},
			},
			{Name: "dialog.open"},
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
