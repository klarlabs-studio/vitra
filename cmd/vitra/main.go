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
	"go.klarlabs.de/vitra/bindings"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/packaging"
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
  vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid|preact|lit|alpine|htmx|angular|qwik|mithril|riot|inferno|stencil|marko]
                             Scaffold a starter desktop app (default: vanilla HTML; vite/react/svelte/vue/solid/preact/lit/alpine/htmx/angular/qwik/mithril/riot/inferno/stencil/marko add Vite frontends)
  vitra dev [dir]            Watch + run the app with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra build [dir]          Build the app binary with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra package --out <dir> [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path] [--maintainer name] [--description text] [--homepage url] [--categories list] [--keywords list] [--license spdx] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref] [--publish [--publish-execute]]
                             Stage Linux dir, build .deb / .rpm / .snap / .flatpak / AppDir / .AppImage, Windows win-dir/WiX/NSIS, Darwin .app/.dmg + provenance.json; --sign prints PlanSign; --sign-execute runs host tools; --publish prints store PlanPublish; --publish-execute runs non-interactive Executable steps
  vitra generate typescript [--out path] [--module name]
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
  vitra update-apply (--manifest <json> --artifact <path> | --base-url <url> --app-id <id> [--channel name]) --pubkey <hex> --dest <path> [--policy production|development]
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

func scaffoldNew(args []string) error {
	dir := ""
	tmpl := "vanilla"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--template":
			i++
			if i >= len(args) {
				return fmt.Errorf("--template requires vanilla, vite, react, svelte, vue, solid, preact, lit, alpine, htmx, angular, qwik, mithril, riot, inferno, stencil, or marko")
			}
			tmpl = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown new flag %q", args[i])
			}
			if dir != "" {
				return fmt.Errorf("usage: vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid|preact|lit|alpine|htmx|angular|qwik|mithril|riot|inferno|stencil|marko]")
			}
			dir = args[i]
		}
	}
	if dir == "" {
		return fmt.Errorf("usage: vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid|preact|lit|alpine|htmx|angular|qwik|mithril|riot|inferno|stencil|marko]")
	}
	switch tmpl {
	case "vanilla", "vite", "react", "svelte", "vue", "solid", "preact", "lit", "alpine", "htmx", "angular", "qwik", "mithril", "riot", "inferno", "stencil", "marko":
	default:
		return fmt.Errorf("unknown template %q (want vanilla, vite, react, svelte, vue, solid, preact, lit, alpine, htmx, angular, qwik, mithril, riot, inferno, stencil, or marko)", tmpl)
	}

	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		return err
	}
	modPath := "example.com/" + filepath.Base(dir)
	if modPath == "example.com/." {
		modPath = "example.com/vitra-app"
	}
	tsClient, err := scaffoldTypeScriptClient()
	if err != nil {
		return err
	}

	var files map[string]string
	switch tmpl {
	case "vite":
		files = scaffoldViteFiles(modPath, tsClient)
	case "react":
		files = scaffoldReactFiles(modPath, tsClient)
	case "svelte":
		files = scaffoldSvelteFiles(modPath, tsClient)
	case "vue":
		files = scaffoldVueFiles(modPath, tsClient)
	case "solid":
		files = scaffoldSolidFiles(modPath, tsClient)
	case "preact":
		files = scaffoldPreactFiles(modPath, tsClient)
	case "lit":
		files = scaffoldLitFiles(modPath, tsClient)
	case "alpine":
		files = scaffoldAlpineFiles(modPath, tsClient)
	case "htmx":
		files = scaffoldHtmxFiles(modPath, tsClient)
	case "angular":
		files = scaffoldAngularFiles(modPath, tsClient)
	case "qwik":
		files = scaffoldQwikFiles(modPath, tsClient)
	case "mithril":
		files = scaffoldMithrilFiles(modPath, tsClient)
	case "riot":
		files = scaffoldRiotFiles(modPath, tsClient)
	case "inferno":
		files = scaffoldInfernoFiles(modPath, tsClient)
	case "stencil":
		files = scaffoldStencilFiles(modPath, tsClient)
	case "marko":
		files = scaffoldMarkoFiles(modPath, tsClient)
	default:
		files = scaffoldVanillaFiles(modPath, tsClient)
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
	fmt.Printf("created %s (template=%s)\n", dir, tmpl)
	switch tmpl {
	case "vite", "react", "svelte", "vue", "solid", "preact", "lit", "alpine", "htmx", "angular", "qwik", "mithril", "riot", "inferno", "stencil", "marko":
		fmt.Println("next: cd", dir, "&& (optional: cd frontend && npm install && npm run build) && vitra dev")
	default:
		fmt.Println("next: cd", dir, "&& vitra dev")
	}
	return nil
}

func scaffoldVanillaFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                   scaffoldGoMod(modPath),
		"main.go":                  scaffoldMainGo("frontend/*", "frontend"),
		"frontend/index.html":      scaffoldIndexHTML(),
		"frontend/vitra-client.ts": tsClient,
		"README.md":                scaffoldREADME("vanilla"),
	}
}

func scaffoldViteFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                   scaffoldGoMod(modPath),
		"main.go":                  scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":    scaffoldVitePackageJSON(),
		"frontend/vite.config.js":  scaffoldViteConfig(""),
		"frontend/tsconfig.json":   scaffoldViteTSConfig(""),
		"frontend/index.html":      scaffoldViteIndexHTML("main.ts"),
		"frontend/src/main.ts":     scaffoldViteMainTS(),
		"frontend/vitra-client.ts": tsClient,
		"frontend/dist/index.html": scaffoldIndexHTML(), // works before first npm run build
		".gitignore":               "frontend/node_modules/\nvitra-app\n",
		"README.md":                scaffoldREADME("vite"),
	}
}

func scaffoldReactFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                      scaffoldGoMod(modPath),
		"main.go":                     scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":       scaffoldReactPackageJSON(),
		"frontend/vite.config.js":     scaffoldViteConfig("react"),
		"frontend/tsconfig.json":      scaffoldViteTSConfig("react"),
		"frontend/tsconfig.node.json": scaffoldReactTSConfigNode(),
		"frontend/index.html":         scaffoldViteIndexHTML("main.tsx"),
		"frontend/src/main.tsx":       scaffoldReactMainTSX(),
		"frontend/src/App.tsx":        scaffoldReactAppTSX(),
		"frontend/src/vite-env.d.ts":  "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":    tsClient,
		"frontend/dist/index.html":    scaffoldIndexHTML(),
		".gitignore":                  "frontend/node_modules/\nvitra-app\n",
		"README.md":                   scaffoldREADME("react"),
	}
}

func scaffoldSvelteFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldSveltePackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig("svelte"),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("svelte"),
		"frontend/svelte.config.js":  scaffoldSvelteConfig(),
		"frontend/index.html":        scaffoldViteIndexHTML("main.ts"),
		"frontend/src/main.ts":       scaffoldSvelteMainTS(),
		"frontend/src/App.svelte":    scaffoldSvelteApp(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"svelte\" />\n/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("svelte"),
	}
}

func scaffoldVueFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldVuePackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig("vue"),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("vue"),
		"frontend/index.html":        scaffoldViteIndexHTML("main.ts"),
		"frontend/src/main.ts":       scaffoldVueMainTS(),
		"frontend/src/App.vue":       scaffoldVueApp(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("vue"),
	}
}

func scaffoldSolidFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                      scaffoldGoMod(modPath),
		"main.go":                     scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":       scaffoldSolidPackageJSON(),
		"frontend/vite.config.js":     scaffoldViteConfig("solid"),
		"frontend/tsconfig.json":      scaffoldViteTSConfig("solid"),
		"frontend/tsconfig.node.json": scaffoldReactTSConfigNode(),
		"frontend/index.html":         scaffoldViteIndexHTML("main.tsx"),
		"frontend/src/main.tsx":       scaffoldSolidMainTSX(),
		"frontend/src/App.tsx":        scaffoldSolidAppTSX(),
		"frontend/src/vite-env.d.ts":  "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":    tsClient,
		"frontend/dist/index.html":    scaffoldIndexHTML(),
		".gitignore":                  "frontend/node_modules/\nvitra-app\n",
		"README.md":                   scaffoldREADME("solid"),
	}
}

func scaffoldPreactFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                      scaffoldGoMod(modPath),
		"main.go":                     scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":       scaffoldPreactPackageJSON(),
		"frontend/vite.config.js":     scaffoldViteConfig("preact"),
		"frontend/tsconfig.json":      scaffoldViteTSConfig("preact"),
		"frontend/tsconfig.node.json": scaffoldReactTSConfigNode(),
		"frontend/index.html":         scaffoldViteIndexHTML("main.tsx"),
		"frontend/src/main.tsx":       scaffoldPreactMainTSX(),
		"frontend/src/App.tsx":        scaffoldPreactAppTSX(),
		"frontend/src/vite-env.d.ts":  "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":    tsClient,
		"frontend/dist/index.html":    scaffoldIndexHTML(),
		".gitignore":                  "frontend/node_modules/\nvitra-app\n",
		"README.md":                   scaffoldREADME("preact"),
	}
}

func scaffoldLitFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldLitPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig(""),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("lit"),
		"frontend/index.html":        scaffoldLitIndexHTML(),
		"frontend/src/main.ts":       scaffoldLitMainTS(),
		"frontend/src/vitra-app.ts":  scaffoldLitApp(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("lit"),
	}
}

func scaffoldAlpineFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldAlpinePackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig(""),
		"frontend/tsconfig.json":     scaffoldViteTSConfig(""),
		"frontend/index.html":        scaffoldAlpineIndexHTML(),
		"frontend/src/main.ts":       scaffoldAlpineMainTS(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("alpine"),
	}
}

func scaffoldMithrilFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldMithrilPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig(""),
		"frontend/tsconfig.json":     scaffoldViteTSConfig(""),
		"frontend/index.html":        scaffoldMithrilIndexHTML(),
		"frontend/src/main.ts":       scaffoldMithrilMainTS(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("mithril"),
	}
}

func scaffoldMithrilPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "mithril": "^2.2.2"
  },
  "devDependencies": {
    "@types/mithril": "^2.0.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldMithrilMainTS() string {
	return `import m from "mithril";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

let out = "";

const App: m.Component = {
  view() {
    const run = async (fn: () => Promise<unknown>) => {
      try {
        out = JSON.stringify(await fn(), null, 2);
      } catch (e) {
        out = String(e);
      }
      m.redraw();
    };
    return m(
      "div",
      {
        style: {
          fontFamily: "Georgia, serif",
          margin: "2rem",
          background: "#111",
          color: "#eee",
          minHeight: "100vh",
        },
      },
      m("h1", "Vitra"),
      m("p", "Vite + Mithril starter (official fs + dialog + clipboard + browser + os + notification + path plugins)."),
      m("button", { onclick: () => run(() => client.demoGreet("Vitra")) }, "demo.greet"),
      " ",
      m("button", { onclick: () => run(() => client.dialogOpen()) }, "dialog.open"),
      " ",
      m("button", { onclick: () => run(() => client.clipboardRead()) }, "clipboard.read"),
      " ",
      m("button", { onclick: () => run(() => client.browserOpen("https://go.klarlabs.de/vitra")) }, "browser.open"),
      " ",
      m("button", { onclick: () => run(() => client.osInfo()) }, "os.info"),
      " ",
      m(
        "button",
        { onclick: () => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" })) },
        "notifications.show",
      ),
      m("pre", out),
    );
  },
};

m.mount(document.getElementById("app")!, App);
`
}

func scaffoldMithrilIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldRiotFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldRiotPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig("riot"),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("riot"),
		"frontend/index.html":        scaffoldRiotIndexHTML(),
		"frontend/src/main.ts":       scaffoldRiotMainTS(),
		"frontend/src/app.riot":      scaffoldRiotAppRiot(),
		"frontend/src/vite-env.d.ts": scaffoldRiotViteEnv(),
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("riot"),
	}
}

func scaffoldRiotPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "riot": "^9.4.0"
  },
  "devDependencies": {
    "@riotjs/compiler": "^9.4.0",
    "rollup-plugin-riot": "^9.0.2",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldRiotMainTS() string {
	return `import { component } from "riot";
import App from "./app.riot";

component(App)(document.getElementById("app")!);
`
}

func scaffoldRiotAppRiot() string {
	return `<app>
  <div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh">
    <h1>Vitra</h1>
    <p>Vite + Riot starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button onclick={greet}>demo.greet</button>
    <button onclick={openDialog}>dialog.open</button>
    <button onclick={readClipboard}>clipboard.read</button>
    <button onclick={openDocs}>browser.open</button>
    <button onclick={showOs}>os.info</button>
    <button onclick={notify}>notifications.show</button>
    <pre>{ state.out }</pre>
  </div>

  <script>
    import { createClient } from "../vitra-client";

    const client = createClient(window.vitra.invoke);

    export default {
      state: {
        out: "",
      },
      async run(fn) {
        try {
          this.update({ out: JSON.stringify(await fn(), null, 2) });
        } catch (e) {
          this.update({ out: String(e) });
        }
      },
      greet() {
        return this.run(() => client.demoGreet("Vitra"));
      },
      openDialog() {
        return this.run(() => client.dialogOpen());
      },
      readClipboard() {
        return this.run(() => client.clipboardRead());
      },
      openDocs() {
        return this.run(() => client.browserOpen("https://go.klarlabs.de/vitra"));
      },
      showOs() {
        return this.run(() => client.osInfo());
      },
      notify() {
        return this.run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }));
      },
    };
  </script>
</app>
`
}

func scaffoldRiotIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldRiotViteEnv() string {
	return `/// <reference types="vite/client" />

declare module "*.riot" {
  import type { RiotComponentWrapper } from "riot";
  const component: RiotComponentWrapper;
  export default component;
}
`
}

func scaffoldInfernoFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                      scaffoldGoMod(modPath),
		"main.go":                     scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":       scaffoldInfernoPackageJSON(),
		"frontend/vite.config.js":     scaffoldViteConfig("inferno"),
		"frontend/tsconfig.json":      scaffoldViteTSConfig("inferno"),
		"frontend/tsconfig.node.json": scaffoldReactTSConfigNode(),
		"frontend/index.html":         scaffoldViteIndexHTML("main.tsx"),
		"frontend/src/main.tsx":       scaffoldInfernoMainTSX(),
		"frontend/src/App.tsx":        scaffoldInfernoAppTSX(),
		"frontend/src/vite-env.d.ts":  "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":    tsClient,
		"frontend/dist/index.html":    scaffoldIndexHTML(),
		".gitignore":                  "frontend/node_modules/\nvitra-app\n",
		"README.md":                   scaffoldREADME("inferno"),
	}
}

func scaffoldInfernoPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "inferno": "^9.0.0"
  },
  "devDependencies": {
    "@babel/core": "^7.25.0",
    "@babel/preset-typescript": "^7.25.0",
    "babel-plugin-inferno": "^6.7.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0",
    "vite-plugin-babel": "^1.2.0"
  }
}
`
}

func scaffoldInfernoMainTSX() string {
	return `import { render } from "inferno";
import { App } from "./App";

const root = document.getElementById("app");
if (root) {
  render(<App />, root);
}
`
}

func scaffoldInfernoAppTSX() string {
	return `import { Component } from "inferno";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

type AppState = { out: string };

export class App extends Component<object, AppState> {
  state: AppState = { out: "" };

  run = async (fn: () => Promise<unknown>) => {
    try {
      this.setState({ out: JSON.stringify(await fn(), null, 2) });
    } catch (e) {
      this.setState({ out: String(e) });
    }
  };

  render() {
    return (
      <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
        <h1>Vitra</h1>
        <p>Vite + Inferno starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
        <button onClick={() => this.run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
        <button onClick={() => this.run(() => client.dialogOpen())}>dialog.open</button>{" "}
        <button onClick={() => this.run(() => client.clipboardRead())}>clipboard.read</button>{" "}
        <button onClick={() => this.run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
        <button onClick={() => this.run(() => client.osInfo())}>os.info</button>{" "}
		<button onClick={() => this.run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
        <pre>{this.state.out}</pre>
      </div>
    );
  }
}
`
}

func scaffoldStencilFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                                scaffoldGoMod(modPath),
		"main.go":                               scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":                 scaffoldStencilPackageJSON(),
		"frontend/vite.config.js":               scaffoldViteConfig("stencil"),
		"frontend/tsconfig.json":                scaffoldViteTSConfig("stencil"),
		"frontend/stencil.config.ts":            scaffoldStencilConfigTS(),
		"frontend/index.html":                   scaffoldStencilIndexHTML(),
		"frontend/src/main.ts":                  scaffoldStencilMainTS(),
		"frontend/src/components/vitra-app.tsx": scaffoldStencilAppTSX(),
		"frontend/src/vite-env.d.ts":            "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":              tsClient,
		"frontend/dist/index.html":              scaffoldIndexHTML(),
		".gitignore":                            "frontend/node_modules/\nfrontend/.stencil/\nvitra-app\n",
		"README.md":                             scaffoldREADME("stencil"),
	}
}

func scaffoldStencilPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@stencil/core": "^4.45.0"
  },
  "devDependencies": {
    "@stencil-community/unplugin-stencil": "^0.5.2",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldStencilConfigTS() string {
	return `import { Config } from "@stencil/core";

export const config: Config = {
  namespace: "vitra",
  srcDir: "src",
  outputTargets: [
    {
      type: "dist-custom-elements",
      dir: ".stencil/components",
      customElementsExportBehavior: "auto-define-custom-elements",
      externalRuntime: false,
    },
  ],
};
`
}

func scaffoldStencilMainTS() string {
	return `import "./components/vitra-app";
`
}

func scaffoldStencilIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <vitra-app></vitra-app>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldStencilAppTSX() string {
	return `import { Component, h, State } from "@stencil/core";
import { createClient } from "../../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

@Component({
  tag: "vitra-app",
  shadow: true,
})
export class VitraApp {
  @State() out = "";

  private async run(fn: () => Promise<unknown>) {
    try {
      this.out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      this.out = String(e);
    }
  }

  render() {
    return (
      <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
        <h1>Vitra</h1>
        <p>Vite + Stencil starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
        <button onClick={() => this.run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
        <button onClick={() => this.run(() => client.dialogOpen())}>dialog.open</button>{" "}
        <button onClick={() => this.run(() => client.clipboardRead())}>clipboard.read</button>{" "}
        <button onClick={() => this.run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
        <button onClick={() => this.run(() => client.osInfo())}>os.info</button>{" "}
        <button onClick={() => this.run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
        <pre>{this.out}</pre>
      </div>
    );
  }
}
`
}

func scaffoldMarkoFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldMarkoPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig("marko"),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("marko"),
		"frontend/index.html":        scaffoldViteIndexHTML("main.ts"),
		"frontend/src/main.ts":       scaffoldMarkoMainTS(),
		"frontend/src/App.marko":     scaffoldMarkoAppMarko(),
		"frontend/src/vite-env.d.ts": scaffoldMarkoViteEnv(),
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("marko"),
	}
}

func scaffoldMarkoPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "marko": "^5.39.0"
  },
  "devDependencies": {
    "@marko/compiler": "^5.42.0",
    "@marko/vite": "^5.4.10",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldMarkoMainTS() string {
	return `import App from "./App.marko";

App.renderSync({}).appendTo(document.getElementById("app")!);
`
}

func scaffoldMarkoAppMarko() string {
	return `import { createClient } from "../vitra-client";

class {
  onCreate() {
    this.state = { out: "" };
    this.client = createClient(window.vitra.invoke);
  }
  async run(fn) {
    try {
      this.state.out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      this.state.out = String(e);
    }
  }
  greet() {
    return this.run(() => this.client.demoGreet("Vitra"));
  }
  openDialog() {
    return this.run(() => this.client.dialogOpen());
  }
  readClipboard() {
    return this.run(() => this.client.clipboardRead());
  }
  openDocs() {
    return this.run(() => this.client.browserOpen("https://go.klarlabs.de/vitra"));
  }
  showOs() {
    return this.run(() => this.client.osInfo());
  }
  notify() {
    return this.run(() => this.client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }));
  }
}

<div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh">
  <h1>Vitra</h1>
  <p>Vite + Marko starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
  <button on-click("greet")>demo.greet</button>
  <button on-click("openDialog")>dialog.open</button>
  <button on-click("readClipboard")>clipboard.read</button>
  <button on-click("openDocs")>browser.open</button>
  <button on-click("showOs")>os.info</button>
  <button on-click("notify")>notifications.show</button>
  <pre>${state.out}</pre>
</div>
`
}

func scaffoldMarkoViteEnv() string {
	return `/// <reference types="vite/client" />

declare module "*.marko" {
  const template: {
    renderSync: (input?: object) => { appendTo: (el: Element) => unknown };
  };
  export default template;
}
`
}

func scaffoldAngularFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                        scaffoldGoMod(modPath),
		"main.go":                       scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":         scaffoldAngularPackageJSON(),
		"frontend/vite.config.js":       scaffoldViteConfig("angular"),
		"frontend/tsconfig.json":        scaffoldViteTSConfig("angular"),
		"frontend/index.html":           scaffoldAngularIndexHTML(),
		"frontend/src/main.ts":          scaffoldAngularMainTS(),
		"frontend/src/app.component.ts": scaffoldAngularAppComponent(),
		"frontend/src/vite-env.d.ts":    "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":      tsClient,
		"frontend/dist/index.html":      scaffoldIndexHTML(),
		".gitignore":                    "frontend/node_modules/\nvitra-app\n",
		"README.md":                     scaffoldREADME("angular"),
	}
}

func scaffoldQwikFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldQwikPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig("qwik"),
		"frontend/tsconfig.json":     scaffoldViteTSConfig("qwik"),
		"frontend/index.html":        scaffoldViteIndexHTML("main.tsx"),
		"frontend/src/main.tsx":      scaffoldQwikMainTSX(),
		"frontend/src/app.tsx":       scaffoldQwikAppTSX(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("qwik"),
	}
}

func scaffoldQwikPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@builder.io/qwik": "^1.20.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldQwikMainTSX() string {
	return `import { render, jsx } from "@builder.io/qwik";
import { App } from "./app";

render(document.getElementById("app")!, jsx(App, {}));
`
}

func scaffoldQwikAppTSX() string {
	return `import { component$, useSignal } from "@builder.io/qwik";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

export const App = component$(() => {
  const out = useSignal("");
  const run = async (fn: () => Promise<unknown>) => {
    try {
      out.value = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      out.value = String(e);
    }
  };
  return (
    <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
      <h1>Vitra</h1>
      <p>Vite + Qwik starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button onClick$={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick$={() => run(() => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick$={() => run(() => client.clipboardRead())}>clipboard.read</button>{" "}
      <button onClick$={() => run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
      <button onClick$={() => run(() => client.osInfo())}>os.info</button>{" "}
      <button onClick$={() => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>{out.value}</pre>
    </div>
  );
});
`
}

func scaffoldAngularPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@angular/common": "^19.0.0",
    "@angular/core": "^19.0.0",
    "@angular/platform-browser": "^19.0.0",
    "rxjs": "^7.8.0",
    "tslib": "^2.8.0"
  },
  "devDependencies": {
    "@analogjs/vite-plugin-angular": "^1.10.0",
    "@angular/compiler": "^19.0.0",
    "@angular/compiler-cli": "^19.0.0",
    "typescript": "~5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldAngularIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <vitra-app></vitra-app>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldAngularMainTS() string {
	return `import { bootstrapApplication } from "@angular/platform-browser";
import { AppComponent } from "./app.component";

bootstrapApplication(AppComponent).catch((err) => console.error(err));
`
}

func scaffoldAngularAppComponent() string {
	bt := "`"
	return `import { Component } from "@angular/core";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

@Component({
  selector: "vitra-app",
  standalone: true,
  template: ` + bt + `
    <h1>Vitra</h1>
    <p>Vite + Angular starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button type="button" (click)="run(() => client.demoGreet('Vitra'))">demo.greet</button>
    <button type="button" (click)="run(() => client.dialogOpen())">dialog.open</button>
    <button type="button" (click)="run(() => client.clipboardRead())">clipboard.read</button>
    <button type="button" (click)="run(() => client.browserOpen('https://go.klarlabs.de/vitra'))">browser.open</button>
    <button type="button" (click)="run(() => client.osInfo())">os.info</button>
    <button type="button" (click)="run(() => client.notificationsShow({ title: 'Vitra', body: 'Hello from scaffold' }))">notifications.show</button>
    <pre>{{ out }}</pre>
  ` + bt + `,
  styles: [
    ` + bt + `
      :host {
        display: block;
        font-family: Georgia, serif;
        margin: 2rem;
        background: #111;
        color: #eee;
        min-height: 100vh;
      }
    ` + bt + `,
  ],
})
export class AppComponent {
  out = "";

  async run(fn: () => Promise<unknown>) {
    try {
      this.out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      this.out = String(e);
    }
  }
}
`
}

func scaffoldREADME(tmpl string) string {
	body := `# Vitra app

` + "```bash" + `
# Native DesktopHost (Linux WebKitGTK / Darwin WKWebView / Windows WebView2)
vitra dev
# or
CGO_ENABLED=1 go run -tags vitra_native .

# Refresh typed frontend stubs after changing commands/plugins
vitra generate typescript --out frontend/vitra-client.ts

# Stage a package (optional)
vitra package --out dist/ --format dir
` + "```" + `
`
	switch tmpl {
	case "vite", "react", "svelte", "vue", "solid", "preact", "lit", "alpine", "htmx", "angular", "qwik", "mithril", "riot", "inferno", "stencil", "marko":
		label := "Vite"
		switch tmpl {
		case "react":
			label = "Vite + React"
		case "svelte":
			label = "Vite + Svelte"
		case "vue":
			label = "Vite + Vue"
		case "solid":
			label = "Vite + Solid"
		case "preact":
			label = "Vite + Preact"
		case "lit":
			label = "Vite + Lit"
		case "alpine":
			label = "Vite + Alpine"
		case "htmx":
			label = "Vite + HTMX"
		case "angular":
			label = "Vite + Angular"
		case "qwik":
			label = "Vite + Qwik"
		case "mithril":
			label = "Vite + Mithril"
		case "riot":
			label = "Vite + Riot"
		case "inferno":
			label = "Vite + Inferno"
		case "stencil":
			label = "Vite + Stencil"
		case "marko":
			label = "Vite + Marko"
		}
		body += `
## ` + label + ` frontend

Go embeds ` + "`frontend/dist`" + `. A starter ` + "`dist/index.html`" + ` is included so
` + "`vitra dev`" + ` works immediately. To rebuild from the sources:

` + "```bash" + `
cd frontend
npm install
npm run build
` + "```" + `

Then re-run ` + "`vitra generate typescript --out frontend/vitra-client.ts`" + ` and
` + "`npm run build`" + ` after changing plugin commands.
`
	}
	return body
}

func scaffoldVitePackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldReactPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.3",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldViteConfig(framework string) string {
	switch framework {
	case "react":
		return `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "svelte":
		return `import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "vue":
		return `import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "solid":
		return `import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "preact":
		return `import { defineConfig } from "vite";
import preact from "@preact/preset-vite";

export default defineConfig({
  plugins: [preact()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "angular":
		return `import { defineConfig } from "vite";
import angular from "@analogjs/vite-plugin-angular";

export default defineConfig({
  plugins: [angular()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "qwik":
		return `import { defineConfig } from "vite";
import { qwikVite } from "@builder.io/qwik/optimizer";

export default defineConfig({
  plugins: [qwikVite({ csr: true })],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "riot":
		return `import { defineConfig } from "vite";
import riot from "rollup-plugin-riot";

export default defineConfig({
  plugins: [riot()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "inferno":
		return `import { defineConfig } from "vite";
import babel from "vite-plugin-babel";

export default defineConfig({
  plugins: [
    babel({
      include: /\.[jt]sx?$/,
      babelConfig: {
        babelrc: false,
        configFile: false,
        presets: [["@babel/preset-typescript", { isTSX: true, allExtensions: true }]],
        plugins: [["babel-plugin-inferno", { imports: true }]],
      },
    }),
  ],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "stencil":
		return `import { defineConfig } from "vite";
import stencil from "@stencil-community/unplugin-stencil/vite";

export default defineConfig({
  plugins: [stencil()],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	case "marko":
		return `import { defineConfig } from "vite";
import marko from "@marko/vite";

export default defineConfig({
  plugins: [marko({ linked: false })],
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	default:
		return `import { defineConfig } from "vite";

export default defineConfig({
  root: ".",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
`
	}
}

func scaffoldViteTSConfig(framework string) string {
	jsx := ""
	include := `"src", "vitra-client.ts"`
	switch framework {
	case "react":
		jsx = `
    "jsx": "react-jsx",`
		include = `"src"`
	case "preact":
		jsx = `
    "jsx": "react-jsx",
    "jsxImportSource": "preact",`
		include = `"src"`
	case "solid":
		jsx = `
    "jsx": "preserve",
    "jsxImportSource": "solid-js",`
		include = `"src"`
	case "lit":
		jsx = `
    "experimentalDecorators": true,
    "useDefineForClassFields": false,`
		include = `"src", "vitra-client.ts"`
	case "svelte":
		include = `"src/**/*.ts", "src/**/*.svelte", "vitra-client.ts"`
	case "vue":
		include = `"src/**/*.ts", "src/**/*.vue", "vitra-client.ts"`
	case "angular":
		jsx = `
    "experimentalDecorators": true,
    "emitDecoratorMetadata": false,
    "useDefineForClassFields": false,`
		include = `"src", "vitra-client.ts"`
	case "qwik":
		jsx = `
    "jsx": "react-jsx",
    "jsxImportSource": "@builder.io/qwik",`
		include = `"src"`
	case "riot":
		include = `"src/**/*.ts", "src/**/*.riot", "vitra-client.ts"`
	case "inferno":
		jsx = `
    "jsx": "preserve",`
		include = `"src"`
	case "stencil":
		jsx = `
    "experimentalDecorators": true,
    "jsx": "react",
    "jsxFactory": "h",
    "jsxFragmentFactory": "Fragment",`
		include = `"src", "vitra-client.ts", "stencil.config.ts"`
	case "marko":
		include = `"src/**/*.ts", "src/**/*.marko", "vitra-client.ts"`
	}
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "skipLibCheck": true,` + jsx + `
    "lib": ["ES2022", "DOM"]
  },
  "include": [` + include + `]
}
`
}

func scaffoldSveltePackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^4.0.0",
    "svelte": "^5.0.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldSvelteConfig() string {
	return `import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
  preprocess: vitePreprocess(),
};
`
}

func scaffoldSvelteMainTS() string {
	return `import { mount } from "svelte";
import App from "./App.svelte";

mount(App, { target: document.getElementById("app")! });
`
}

func scaffoldSvelteApp() string {
	return `<script lang="ts">
  import { createClient } from "../vitra-client";

  declare global {
    interface Window {
      vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
    }
  }

  const client = createClient(window.vitra.invoke);
  let out = $state("");

  async function run(fn: () => Promise<unknown>) {
    try {
      out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      out = String(e);
    }
  }
</script>

<div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh;">
  <h1>Vitra</h1>
  <p>Vite + Svelte starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
  <button onclick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>
  <button onclick={() => run(() => client.dialogOpen())}>dialog.open</button>
  <button onclick={() => run(() => client.clipboardRead())}>clipboard.read</button>
  <button onclick={() => run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>
  <button onclick={() => run(() => client.osInfo())}>os.info</button>
  <button onclick={() => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
  <pre>{out}</pre>
</div>
`
}

func scaffoldVuePackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.5.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.1.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0",
    "vue-tsc": "^2.1.0"
  }
}
`
}

func scaffoldVueMainTS() string {
	return `import { createApp } from "vue";
import App from "./App.vue";

createApp(App).mount("#app");
`
}

func scaffoldVueApp() string {
	return `<script setup lang="ts">
import { ref } from "vue";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);
const out = ref("");

async function run(fn: () => Promise<unknown>) {
  try {
    out.value = JSON.stringify(await fn(), null, 2);
  } catch (e) {
    out.value = String(e);
  }
}
</script>

<template>
  <div style="font-family: Georgia, serif; margin: 2rem; background: #111; color: #eee; min-height: 100vh;">
    <h1>Vitra</h1>
    <p>Vite + Vue starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button @click="run(() => client.demoGreet('Vitra'))">demo.greet</button>
    <button @click="run(() => client.dialogOpen())">dialog.open</button>
    <button @click="run(() => client.clipboardRead())">clipboard.read</button>
    <button @click="run(() => client.browserOpen('https://go.klarlabs.de/vitra'))">browser.open</button>
    <button @click="run(() => client.osInfo())">os.info</button>
    <button @click="run(() => client.notificationsShow({ title: 'Vitra', body: 'Hello from scaffold' }))">notifications.show</button>
    <pre>{{ out }}</pre>
  </div>
</template>
`
}

func scaffoldSolidPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "solid-js": "^1.9.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0",
    "vite-plugin-solid": "^2.10.0"
  }
}
`
}

func scaffoldSolidMainTSX() string {
	return `import { render } from "solid-js/web";
import { App } from "./App";

render(() => <App />, document.getElementById("app")!);
`
}

func scaffoldSolidAppTSX() string {
	return `import { createSignal } from "solid-js";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

export function App() {
  const [out, setOut] = createSignal("");
  const run = async (fn: () => Promise<unknown>) => {
    try {
      setOut(JSON.stringify(await fn(), null, 2));
    } catch (e) {
      setOut(String(e));
    }
  };
  return (
    <div style={{ "font-family": "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", "min-height": "100vh" }}>
      <h1>Vitra</h1>
      <p>Vite + Solid starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button onClick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run(() => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run(() => client.clipboardRead())}>clipboard.read</button>{" "}
      <button onClick={() => run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
      <button onClick={() => run(() => client.osInfo())}>os.info</button>{" "}
      <button onClick={() => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>{out()}</pre>
    </div>
  );
}
`
}

func scaffoldPreactPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "preact": "^10.24.0"
  },
  "devDependencies": {
    "@preact/preset-vite": "^2.9.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldPreactMainTSX() string {
	return `import { render } from "preact";
import { App } from "./App";

render(<App />, document.getElementById("app")!);
`
}

func scaffoldPreactAppTSX() string {
	return `import { useState } from "preact/hooks";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

export function App() {
  const [out, setOut] = useState("");
  const run = async (fn: () => Promise<unknown>) => {
    try {
      setOut(JSON.stringify(await fn(), null, 2));
    } catch (e) {
      setOut(String(e));
    }
  };
  return (
    <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
      <h1>Vitra</h1>
      <p>Vite + Preact starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button onClick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run(() => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run(() => client.clipboardRead())}>clipboard.read</button>{" "}
      <button onClick={() => run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
      <button onClick={() => run(() => client.osInfo())}>os.info</button>{" "}
      <button onClick={() => run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>{out}</pre>
    </div>
  );
}
`
}

func scaffoldLitPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "lit": "^3.2.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldAlpinePackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "alpinejs": "^3.14.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldAlpineMainTS() string {
	return `import Alpine from "alpinejs";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    Alpine: typeof Alpine;
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

Alpine.data("vitraApp", () => ({
  out: "",
  async run(fn: () => Promise<unknown>) {
    try {
      this.out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      this.out = String(e);
    }
  },
  greet() {
    return this.run(() => client.demoGreet("Vitra"));
  },
  openDialog() {
    return this.run(() => client.dialogOpen());
  },
  readClipboard() {
    return this.run(() => client.clipboardRead());
  },
  openBrowser() {
    return this.run(() => client.browserOpen("https://go.klarlabs.de/vitra"));
  },
  readOsInfo() {
    return this.run(() => client.osInfo());
  },
  showNotification() {
    return this.run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }));
  },
}));

window.Alpine = Alpine;
Alpine.start();
`
}

func scaffoldAlpineIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <div id="app" x-data="vitraApp">
    <h1>Vitra</h1>
    <p>Vite + Alpine starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button type="button" @click="greet()">demo.greet</button>
    <button type="button" @click="openDialog()">dialog.open</button>
    <button type="button" @click="readClipboard()">clipboard.read</button>
    <button type="button" @click="openBrowser()">browser.open</button>
    <button type="button" @click="readOsInfo()">os.info</button>
    <button type="button" @click="showNotification()">notifications.show</button>
    <pre x-text="out"></pre>
  </div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldHtmxFiles(modPath, tsClient string) map[string]string {
	return map[string]string{
		"go.mod":                     scaffoldGoMod(modPath),
		"main.go":                    scaffoldMainGo("all:frontend/dist", "frontend/dist"),
		"frontend/package.json":      scaffoldHtmxPackageJSON(),
		"frontend/vite.config.js":    scaffoldViteConfig(""),
		"frontend/tsconfig.json":     scaffoldViteTSConfig(""),
		"frontend/index.html":        scaffoldHtmxIndexHTML(),
		"frontend/src/main.ts":       scaffoldHtmxMainTS(),
		"frontend/src/vite-env.d.ts": "/// <reference types=\"vite/client\" />\n",
		"frontend/vitra-client.ts":   tsClient,
		"frontend/dist/index.html":   scaffoldIndexHTML(),
		".gitignore":                 "frontend/node_modules/\nvitra-app\n",
		"README.md":                  scaffoldREADME("htmx"),
	}
}

func scaffoldHtmxPackageJSON() string {
	return `{
  "name": "vitra-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "htmx.org": "^2.0.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
`
}

func scaffoldHtmxMainTS() string {
	return `import "htmx.org";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
    vitraRun: (fn: () => Promise<unknown>) => Promise<void>;
    vitraGreet: () => Promise<void>;
    vitraOpen: () => Promise<void>;
    vitraClip: () => Promise<void>;
    vitraBrowser: () => Promise<void>;
    vitraOs: () => Promise<void>;
    vitraNotify: () => Promise<void>;
  }
}

const client = createClient(window.vitra.invoke);
const out = () => document.getElementById("out")!;

window.vitraRun = async (fn) => {
  try {
    out().textContent = JSON.stringify(await fn(), null, 2);
  } catch (e) {
    out().textContent = String(e);
  }
};
window.vitraGreet = () => window.vitraRun(() => client.demoGreet("Vitra"));
window.vitraOpen = () => window.vitraRun(() => client.dialogOpen());
window.vitraClip = () => window.vitraRun(() => client.clipboardRead());
window.vitraBrowser = () =>
  window.vitraRun(() => client.browserOpen("https://go.klarlabs.de/vitra"));
window.vitraOs = () => window.vitraRun(() => client.osInfo());
window.vitraNotify = () =>
  window.vitraRun(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }));
`
}

func scaffoldHtmxIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <div id="app">
    <h1>Vitra</h1>
    <p>Vite + HTMX starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
    <button type="button" hx-on:click="vitraGreet()">demo.greet</button>
    <button type="button" hx-on:click="vitraOpen()">dialog.open</button>
    <button type="button" hx-on:click="vitraClip()">clipboard.read</button>
    <button type="button" hx-on:click="vitraBrowser()">browser.open</button>
    <button type="button" hx-on:click="vitraOs()">os.info</button>
    <button type="button" hx-on:click="vitraNotify()">notifications.show</button>
    <pre id="out"></pre>
  </div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldLitMainTS() string {
	return `import "./vitra-app";
`
}

func scaffoldLitApp() string {
	bt := "`"
	return `import { LitElement, css, html } from "lit";
import { customElement, state } from "lit/decorators.js";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
  interface HTMLElementTagNameMap {
    "vitra-app": VitraApp;
  }
}

const client = createClient(window.vitra.invoke);

@customElement("vitra-app")
export class VitraApp extends LitElement {
  static styles = css` + bt + `
    :host {
      display: block;
      font-family: Georgia, serif;
      margin: 2rem;
      background: #111;
      color: #eee;
      min-height: 100vh;
    }
  ` + bt + `;

  @state() private out = "";

  private async run(fn: () => Promise<unknown>) {
    try {
      this.out = JSON.stringify(await fn(), null, 2);
    } catch (e) {
      this.out = String(e);
    }
  }

  render() {
    return html` + bt + `
      <h1>Vitra</h1>
      <p>Vite + Lit starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button @click=${() => this.run(() => client.demoGreet("Vitra"))}>demo.greet</button>
      <button @click=${() => this.run(() => client.dialogOpen())}>dialog.open</button>
      <button @click=${() => this.run(() => client.clipboardRead())}>clipboard.read</button>
      <button @click=${() => this.run(() => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>
      <button @click=${() => this.run(() => client.osInfo())}>os.info</button>
      <button @click=${() => this.run(() => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>${this.out}</pre>
    ` + bt + `;
  }
}
`
}

func scaffoldReactTSConfigNode() string {
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "skipLibCheck": true
  },
  "include": ["vite.config.js"]
}
`
}

func scaffoldViteIndexHTML(entry string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/` + entry + `"></script>
</body>
</html>
`
}

func scaffoldLitIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <title>Vitra App</title>
</head>
<body>
  <vitra-app></vitra-app>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
`
}

func scaffoldViteMainTS() string {
	return `import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);
const root = document.getElementById("app")!;
root.innerHTML = ` + "`" + `
  <h1>Vitra</h1>
  <p>Vite + TypeScript starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
  <button id="greet">demo.greet</button>
  <button id="open">dialog.open</button>
  <button id="clip">clipboard.read</button>
  <button id="browser">browser.open</button>
  <button id="os">os.info</button>
  <button id="notify">notifications.show</button>
  <pre id="out"></pre>
` + "`" + `;

const out = document.getElementById("out")!;
document.getElementById("greet")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.demoGreet("Vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("open")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.dialogOpen(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("clip")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.clipboardRead(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("browser")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.browserOpen("https://go.klarlabs.de/vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("os")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.osInfo(), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("notify")!.onclick = async () => {
  try { out.textContent = JSON.stringify(await client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
`
}

func scaffoldReactMainTSX() string {
	return `import React from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";

createRoot(document.getElementById("app")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
`
}

func scaffoldReactAppTSX() string {
	return `import { useState } from "react";
import { createClient } from "../vitra-client";

declare global {
  interface Window {
    vitra: { invoke: (cmd: string, input?: unknown) => Promise<unknown> };
  }
}

const client = createClient(window.vitra.invoke);

export function App() {
  const [out, setOut] = useState("");
  const run = async (label: string, fn: () => Promise<unknown>) => {
    try {
      setOut(JSON.stringify(await fn(), null, 2));
    } catch (e) {
      setOut(String(e));
    }
  };
  return (
    <div style={{ fontFamily: "Georgia, serif", margin: "2rem", background: "#111", color: "#eee", minHeight: "100vh" }}>
      <h1>Vitra</h1>
      <p>Vite + React starter (official fs + dialog + clipboard + browser + os + notification + path plugins).</p>
      <button onClick={() => run("greet", () => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run("open", () => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run("clip", () => client.clipboardRead())}>clipboard.read</button>{" "}
      <button onClick={() => run("browser", () => client.browserOpen("https://go.klarlabs.de/vitra"))}>browser.open</button>{" "}
      <button onClick={() => run("os", () => client.osInfo())}>os.info</button>{" "}
      <button onClick={() => run("notify", () => client.notificationsShow({ title: "Vitra", body: "Hello from scaffold" }))}>notifications.show</button>
      <pre>{out}</pre>
    </div>
  );
}
`
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
	return bindings.GenerateTypeScript("vitra", vitra.Version, cmds, events), nil
}

func scaffoldMainGo(embedPattern, subPath string) string {
	return `package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
	officialapp "go.klarlabs.de/vitra/plugin/official/app"
	officialbrowser "go.klarlabs.de/vitra/plugin/official/browser"
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialdeeplink "go.klarlabs.de/vitra/plugin/official/deeplink"
	officialdragdrop "go.klarlabs.de/vitra/plugin/official/dragdrop"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
	officialmenu "go.klarlabs.de/vitra/plugin/official/menu"
	officialnotification "go.klarlabs.de/vitra/plugin/official/notification"
	officialos "go.klarlabs.de/vitra/plugin/official/os"
	officialpath "go.klarlabs.de/vitra/plugin/official/path"
	officialshortcut "go.klarlabs.de/vitra/plugin/official/shortcut"
	officialtray "go.klarlabs.de/vitra/plugin/official/tray"
	officialwindow "go.klarlabs.de/vitra/plugin/official/window"
)

//go:embed ` + embedPattern + `
var frontendRoot embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "app: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	assets, err := fs.Sub(frontendRoot, "` + subPath + `")
	if err != nil {
		return err
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.app"})
	if err != nil {
		return err
	}
	host := desktopHost()
	caller := domain.Caller{Window: "main", Origin: domain.OriginPackagedLocal}
	var application *app.App

	greet, _ := domain.NewCommandDefinition("demo.greet", "Greet", "demo.greet")
	_ = rt.RegisterCommand(greet, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		who, _ := input.(string)
		if who == "" {
			who = "world"
		}
		return map[string]any{"message": "Hello, " + who}, nil
	}))

	if err := rt.RegisterPlugin(context.Background(), officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialfs.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialclipboard.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialbrowser.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialos.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialnotification.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialpath.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialwindow.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialmenu.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialtray.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialdragdrop.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialdeeplink.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialshortcut.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialapp.New()); err != nil {
		return err
	}
	dialogs := &desktop.DialogService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, opts platform.DialogFileOptions) ([]string, error) {
			path, err := host.OpenFileDialog(opts)
			if err != nil || path == "" {
				return nil, err
			}
			return []string{path}, nil
		},
		OnSave: func(ctx context.Context, opts platform.DialogFileOptions) (string, error) {
			return host.SaveFileDialog(opts)
		},
		OnOpenDirectory: func(ctx context.Context, opts platform.DialogFileOptions) (string, error) {
			return host.OpenDirectoryDialog(opts)
		},
		OnMessage: func(ctx context.Context, title, message, kind string) (bool, error) {
			return host.MessageDialog(title, message, kind)
		},
	}
	if err := rt.BindExecutor("dialog.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		return dialogs.OpenFile(ctx, caller, desktop.ParseDialogFileOptions(input))
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.save", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		return dialogs.SaveFile(ctx, caller, desktop.ParseDialogFileOptions(input))
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.openDirectory", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		return dialogs.OpenDirectory(ctx, caller, desktop.ParseDialogFileOptions(input))
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.message", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		title, message, kind := "", "", "info"
		switch v := input.(type) {
		case string:
			message = v
		case map[string]any:
			title, _ = v["title"].(string)
			message, _ = v["message"].(string)
			if k, ok := v["kind"].(string); ok {
				kind = k
			}
		}
		return dialogs.Message(ctx, caller, title, message, kind)
	})); err != nil {
		return err
	}
	clips := &desktop.ClipboardService{
		Gateway: rt,
		Host:    host,
		OnRead:  func(ctx context.Context) (string, error) { return host.ClipboardGet() },
		OnWrite: func(ctx context.Context, text string) error { return host.ClipboardSet(text) },
	}
	if err := rt.BindExecutor("clipboard.read", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return clips.Read(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("clipboard.write", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		text, _ := input.(string)
		return nil, clips.Write(ctx, caller, text)
	})); err != nil {
		return err
	}
	browser := &desktop.BrowserService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, rawURL string) error {
			return host.OpenURL(ctx, rawURL)
		},
	}
	if err := rt.BindExecutor("browser.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		rawURL, _ := input.(string)
		return nil, browser.OpenURL(ctx, caller, rawURL)
	})); err != nil {
		return err
	}
	osInfo := &desktop.OsService{Gateway: rt}
	if err := rt.BindExecutor("os.info", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return osInfo.Info(ctx, caller)
	})); err != nil {
		return err
	}
	notifs := &desktop.NotificationService{
		Gateway: rt,
		Host:    host,
		OnShow: func(ctx context.Context, title, body string) error {
			return host.ShowNotification(title, body)
		},
	}
	if err := rt.BindExecutor("notifications.show", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		title, body := "", ""
		switch v := input.(type) {
		case string:
			body = v
		case map[string]any:
			title, _ = v["title"].(string)
			body, _ = v["body"].(string)
		}
		return nil, notifs.Show(ctx, caller, title, body)
	})); err != nil {
		return err
	}

	demoRoot := filepath.Join(os.TempDir(), "vitra-scaffold-fs")
	if err := os.MkdirAll(demoRoot, 0o755); err != nil {
		return err
	}
	files := &desktop.FileService{Gateway: rt}
	if err := rt.BindExecutor("fs.read", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		path, _ := input.(string)
		data, err := files.Read(ctx, caller, path)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("fs.write", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		m, _ := input.(map[string]any)
		path, _ := m["path"].(string)
		data, _ := m["data"].(string)
		return nil, files.Write(ctx, caller, path, []byte(data))
	})); err != nil {
		return err
	}
	paths := &desktop.PathService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, path string) error {
			return host.OpenPath(ctx, path)
		},
	}
	if err := rt.BindExecutor("path.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		path, _ := input.(string)
		return nil, paths.Open(ctx, caller, path)
	})); err != nil {
		return err
	}
	winSvc := &desktop.WindowService{
		Gateway: rt,
		Host:    host,
		OnApply: func(_ context.Context, window domain.WindowID, chrome platform.WindowChrome) error {
			return host.ApplyWindowChrome(window, chrome)
		},
		OnRead: func(_ context.Context, window domain.WindowID) (platform.WindowChrome, error) {
			return host.ReadWindowChrome(window)
		},
		OnFocus: func(_ context.Context, window domain.WindowID) error {
			return host.FocusWindow(window)
		},
		OnCreate: func(ctx context.Context, opts desktop.WindowCreateOptions) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			return application.OpenWindow(ctx, app.WindowOptions{
				ID: opts.ID, Title: opts.Title, Path: opts.Path, Width: opts.Width, Height: opts.Height,
			})
		},
		OnClose: func(ctx context.Context, id domain.WindowID) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			return application.CloseWindow(ctx, id)
		},
	}
	if err := rt.BindExecutor("window.create", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		opts, err := desktop.ParseWindowCreateOptions(input)
		if err != nil {
			return nil, err
		}
		id, err := winSvc.Create(ctx, caller, opts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": string(id)}, nil
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.close", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, err := desktop.ParseWindowID(input)
		if err != nil {
			return nil, err
		}
		return nil, winSvc.Close(ctx, caller, id)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.chrome", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, chrome, err := desktop.ParseWindowChromeApply(input)
		if err != nil {
			return nil, err
		}
		return nil, winSvc.Apply(ctx, caller, id, chrome)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.getChrome", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, err := desktop.ParseWindowID(input)
		if err != nil {
			return nil, err
		}
		return winSvc.Read(ctx, caller, id)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.focus", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, err := desktop.ParseWindowID(input)
		if err != nil {
			return nil, err
		}
		return nil, winSvc.Focus(ctx, caller, id)
	})); err != nil {
		return err
	}
	menus := &desktop.MenuService{
		Gateway: rt,
		Host:    host,
		OnSet: func(ctx context.Context, items []desktop.MenuItem) error {
			native := make([]platform.MenuItem, 0, len(items))
			for _, it := range items {
				menu := it.Menu
				if menu == "" {
					menu = "App"
				}
				native = append(native, platform.MenuItem{Menu: menu, ID: it.ID, Label: it.Label, Shortcut: it.Shortcut})
			}
			return host.SetMenuBar("main", native)
		},
		OnClear: func(ctx context.Context) error {
			return host.SetMenuBar("main", nil)
		},
	}
	if err := rt.BindExecutor("menu.set", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		items, err := desktop.ParseMenuItems(input)
		if err != nil {
			return nil, err
		}
		return nil, menus.SetMenu(ctx, caller, items)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("menu.clear", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return nil, menus.ClearMenu(ctx, caller)
	})); err != nil {
		return err
	}
	trays := &desktop.TrayService{
		Gateway: rt,
		Host:    host,
		OnSet: func(ctx context.Context, tooltip string, items []desktop.MenuItem) error {
			native := make([]platform.MenuItem, 0, len(items))
			for _, it := range items {
				native = append(native, platform.MenuItem{ID: it.ID, Label: it.Label})
			}
			return host.SetTray(tooltip, native)
		},
		OnClear: func(ctx context.Context) error {
			host.ClearTray()
			return nil
		},
	}
	if err := rt.BindExecutor("tray.set", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		tooltip, items, err := desktop.ParseTraySet(input)
		if err != nil {
			return nil, err
		}
		return nil, trays.SetTray(ctx, caller, tooltip, items)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("tray.clear", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return nil, trays.ClearTray(ctx, caller)
	})); err != nil {
		return err
	}
	drops := &desktop.DragDropService{
		Gateway: rt,
		Host:    host,
		OnEnable: func(_ context.Context, window domain.WindowID, enabled bool) error {
			return host.EnableDragDrop(window, enabled)
		},
	}
	if err := rt.BindExecutor("dragdrop.receive", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		win, enabled, err := desktop.ParseDragDropEnable(input)
		if err != nil {
			return nil, err
		}
		return nil, drops.Enable(ctx, caller, win, enabled)
	})); err != nil {
		return err
	}
	shortcuts := &desktop.ShortcutService{
		Gateway: rt,
		Host:    host,
		OnRegister: func(_ context.Context, accelerator, actionID string) error {
			return host.RegisterGlobalShortcut(accelerator, actionID)
		},
		OnUnregister: func(_ context.Context, accelerator string) error {
			return host.UnregisterGlobalShortcut(accelerator)
		},
	}
	if err := rt.BindExecutor("shortcut.register", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		acc, action, err := desktop.ParseShortcutRegister(input)
		if err != nil {
			return nil, err
		}
		return nil, shortcuts.Register(ctx, caller, acc, action)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("shortcut.unregister", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		acc, err := desktop.ParseShortcutUnregister(input)
		if err != nil {
			return nil, err
		}
		return nil, shortcuts.Unregister(ctx, caller, acc)
	})); err != nil {
		return err
	}
	appSvc := &desktop.AppService{
		Gateway: rt,
		OnQuit: func(ctx context.Context) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			application.Quit()
			return nil
		},
	}
	if err := rt.BindExecutor("app.quit", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return nil, appSvc.Quit(ctx, caller)
	})); err != nil {
		return err
	}

	grant, _ := domain.NewCapabilityGrant(
		"demo", "demo", []domain.WindowID{"main", "aux"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "demo.greet"},
			{Name: desktop.PermDialogOpen},
			{Name: desktop.PermDialogSave},
			{Name: desktop.PermDialogOpenDirectory},
			{Name: desktop.PermDialogMessage},
			{Name: desktop.PermClipboardRead},
			{Name: desktop.PermClipboardWrite},
			{Name: desktop.PermOpenURL},
			{Name: desktop.PermOsInfo},
			{Name: desktop.PermNotificationShow},
			{Name: desktop.PermWindowCreate},
			{Name: desktop.PermWindowClose},
			{Name: desktop.PermWindowChrome},
			{Name: desktop.PermMenuSet},
			{Name: desktop.PermTraySet},
			{Name: desktop.PermDragDrop},
			{Name: desktop.PermDeepLinkHandle},
			{Name: desktop.PermShortcutRegister},
			{Name: desktop.PermAppQuit},
			{Name: desktop.PermFSRead, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
			{Name: desktop.PermFSWrite, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
			{Name: desktop.PermPathOpen, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
		},
	)
	_ = rt.RegisterGrant(grant)

	application, err = app.New(app.Options{
		AppID: "com.example.app", Title: "Vitra App", Assets: assets, Host: host, Runtime: rt,
		Window: app.WindowOptions{ID: "main", Width: 960, Height: 640},
	})
	if err != nil {
		return err
	}
	host.SetActionHandler(func(id string) {
		payload := map[string]any{"id": id}
		_ = application.Emit(context.Background(), "menu.action", payload)
		_ = application.Emit(context.Background(), "tray.action", payload)
		_ = application.Emit(context.Background(), "shortcut.action", payload)
		if id == "app.quit" || id == "tray.quit" {
			application.Quit()
		}
	})
	host.SetDragDropHandler(func(windowID domain.WindowID, paths []string) {
		_ = application.Emit(context.Background(), "dragdrop.drop", map[string]any{
			"window": string(windowID),
			"paths":  paths,
		})
	})
	deepLinks := &desktop.DeepLinkService{
		Gateway:  rt,
		Host:     host,
		Patterns: []domain.DeepLinkPattern{{Scheme: "vitra"}},
	}
	handleDeepLink := func(raw string) {
		ok, err := deepLinks.Handle(caller, raw)
		if err != nil || !ok {
			return
		}
		_ = application.Emit(context.Background(), "deeplink.open", map[string]any{"url": raw})
	}
	if stop, err := host.StartDeepLinkBridge("com.example.app", handleDeepLink); err == nil {
		defer stop()
	}
	for _, u := range deepLinksFromArgs(os.Args[1:]) {
		handleDeepLink(u)
	}
	return application.Run(context.Background())
}

func deepLinksFromArgs(args []string) []string {
	switch runtime.GOOS {
	case "darwin":
		return darwin.DeepLinksFromArgs(args)
	case "windows":
		return windows.DeepLinksFromArgs(args)
	default:
		return linux.DeepLinksFromArgs(args)
	}
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
`
}

func scaffoldIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><title>Vitra App</title>
<style>body{font-family:Georgia,serif;margin:2rem;background:#111;color:#eee}
button{padding:.75rem 1rem;cursor:pointer;margin-right:.5rem}</style></head>
<body><h1>Vitra</h1><p>Secure desktop runtime starter (official fs + dialog + clipboard + browser + os + notification + path + window + menu + tray + dragdrop + deeplink + shortcut + app plugins).</p>
<button id="greet">demo.greet</button>
<button id="open">dialog.open</button>
<button id="opendir">dialog.openDirectory</button>
<button id="clip">clipboard.read</button>
<button id="browser">browser.open</button>
<button id="os">os.info</button>
<button id="notify">notifications.show</button>
<button id="win">window.create</button>
<button id="chrome">window.chrome</button>
<button id="getChrome">window.getChrome</button>
<button id="focus">window.focus</button>
<button id="menu">menu.set</button>
<button id="menuClear">menu.clear</button>
<button id="tray">tray.set</button>
<button id="trayClear">tray.clear</button>
<button id="drop">dragdrop.receive</button>
<button id="shortcut">shortcut.register</button>
<button id="shortcutUnreg">shortcut.unregister</button>
<button id="quit">app.quit</button>
<pre id="out"></pre>
<script>
const out = document.getElementById("out");
const invoke = (cmd, input) => window.vitra.invoke(cmd, input);
document.getElementById("greet").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("demo.greet", "Vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("open").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("dialog.open"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("opendir").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("dialog.openDirectory", { title: "Pick a folder", defaultPath: "/tmp" }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("clip").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("clipboard.read"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("browser").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("browser.open", "https://go.klarlabs.de/vitra"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("os").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("os.info"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("notify").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("notifications.show", { title: "Vitra", body: "Hello from scaffold" }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("win").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("window.create", { id: "aux", title: "Aux", width: 480, height: 360 }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("chrome").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("window.chrome", { id: "main", title: "Vitra Chrome", width: 900, height: 600 }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("getChrome").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("window.getChrome", { id: "main" }), null, 2); }
  catch (e) { out.textContent = String(e); }
};
document.getElementById("focus").onclick = async () => {
  try {
    await invoke("window.focus", { id: "main" });
    out.textContent = JSON.stringify({ focus: "main" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("menu").onclick = async () => {
  try {
    await invoke("menu.set", [
      { menu: "File", id: "app.quit", label: "Quit", shortcut: "Ctrl+Q" },
      { menu: "Help", id: "help.about", label: "About Vitra" },
    ]);
    out.textContent = JSON.stringify({ menu: "set" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("menuClear").onclick = async () => {
  try {
    await invoke("menu.clear");
    out.textContent = JSON.stringify({ menu: "cleared" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("tray").onclick = async () => {
  try {
    await invoke("tray.set", {
      tooltip: "Vitra",
      items: [
        { id: "help.about", label: "About Vitra" },
        { id: "tray.quit", label: "Quit" },
      ],
    });
    out.textContent = JSON.stringify({ tray: "set" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("trayClear").onclick = async () => {
  try {
    await invoke("tray.clear");
    out.textContent = JSON.stringify({ tray: "cleared" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("drop").onclick = async () => {
  try {
    await invoke("dragdrop.receive", { id: "main", enabled: true });
    out.textContent = JSON.stringify({ dragdrop: "enabled" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("shortcut").onclick = async () => {
  try {
    await invoke("shortcut.register", { accelerator: "Ctrl+Shift+Q", action: "app.quit" });
    out.textContent = JSON.stringify({ shortcut: "registered" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("shortcutUnreg").onclick = async () => {
  try {
    await invoke("shortcut.unregister", { accelerator: "Ctrl+Shift+Q" });
    out.textContent = JSON.stringify({ shortcut: "unregistered" }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
document.getElementById("quit").onclick = async () => {
  try {
    await invoke("app.quit");
    out.textContent = JSON.stringify({ quit: true }, null, 2);
  } catch (e) { out.textContent = String(e); }
};
if (window.vitra && window.vitra.on) {
  window.vitra.on("menu.action", (payload) => { out.textContent = JSON.stringify({ event: "menu.action", payload }, null, 2); });
  window.vitra.on("tray.action", (payload) => { out.textContent = JSON.stringify({ event: "tray.action", payload }, null, 2); });
  window.vitra.on("dragdrop.drop", (payload) => { out.textContent = JSON.stringify({ event: "dragdrop.drop", payload }, null, 2); });
  window.vitra.on("deeplink.open", (payload) => { out.textContent = JSON.stringify({ event: "deeplink.open", payload }, null, 2); });
  window.vitra.on("shortcut.action", (payload) => { out.textContent = JSON.stringify({ event: "shortcut.action", payload }, null, 2); });
}
// Typed stubs: frontend/vitra-client.ts (vitra generate typescript)
</script></body></html>
`
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
	body := bindings.GenerateTypeScript(module, vitra.Version, cmds, events)
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
	baseURL, appID, channel := "", "", "stable"
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
		default:
			return fmt.Errorf("unknown update-apply flag %q", args[i])
		}
	}
	usage := "usage: vitra update-apply (--manifest <json> --artifact <path> | --base-url <url> --app-id <id> [--channel name]) --pubkey <hex> --dest <path> [--policy production|development]"
	if pubkeyHex == "" || dest == "" {
		return fmt.Errorf("%s", usage)
	}
	localMode := manifestPath != "" || artifactPath != ""
	channelMode := baseURL != "" || appID != ""
	if localMode && channelMode {
		return fmt.Errorf("update-apply: use either local --manifest/--artifact or channel --base-url/--app-id, not both")
	}
	if localMode && (manifestPath == "" || artifactPath == "") {
		return fmt.Errorf("%s", usage)
	}
	if channelMode && (baseURL == "" || appID == "") {
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
					Allow: []string{"${PROJECT_DIR}/**"},
					Deny:  []string{"${PROJECT_DIR}/.secrets/**"},
				},
			},
			{Name: "dialog.open"},
			{Name: "clipboard.read"},
			{Name: "browser.open"},
			{Name: "os.info"},
			{Name: "notifications.show"},
			{Name: "path.open", PathScope: &domain.PathScope{Allow: []string{"${PROJECT_DIR}/**"}}},
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
