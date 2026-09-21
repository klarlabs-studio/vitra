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
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
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
	case "update-check":
		return runUpdateCheck(args[1:])
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
  vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid]
                             Scaffold a starter desktop app (default: vanilla HTML; vite/react/svelte/vue/solid add Vite frontends)
  vitra dev [dir]            Watch + run the app with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra build [dir]          Build the app binary with the native host (-tags vitra_native on Linux/Darwin/Windows)
  vitra package --out <dir> [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path] [--maintainer name] [--description text] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref]
                             Stage Linux dir, build .deb / .rpm / .snap / .flatpak / AppDir / .AppImage, Windows win-dir/WiX/NSIS, Darwin .app/.dmg + provenance.json; --sign prints PlanSign; --sign-execute runs host tools
  vitra generate typescript [--out path] [--module name]
                             Emit TypeScript client stubs for official plugin commands
  vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>
                             Fetch + verify a signed channel manifest (HTTP(S) client; does not install)
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
				return fmt.Errorf("--template requires vanilla, vite, react, svelte, vue, or solid")
			}
			tmpl = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown new flag %q", args[i])
			}
			if dir != "" {
				return fmt.Errorf("usage: vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid]")
			}
			dir = args[i]
		}
	}
	if dir == "" {
		return fmt.Errorf("usage: vitra new <dir> [--template vanilla|vite|react|svelte|vue|solid]")
	}
	switch tmpl {
	case "vanilla", "vite", "react", "svelte", "vue", "solid":
	default:
		return fmt.Errorf("unknown template %q (want vanilla, vite, react, svelte, vue, or solid)", tmpl)
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
	case "vite", "react", "svelte", "vue", "solid":
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
	case "vite", "react", "svelte", "vue", "solid":
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
	case "solid":
		jsx = `
    "jsx": "preserve",
    "jsxImportSource": "solid-js",`
		include = `"src"`
	case "svelte":
		include = `"src/**/*.ts", "src/**/*.svelte", "vitra-client.ts"`
	case "vue":
		include = `"src/**/*.ts", "src/**/*.vue", "vitra-client.ts"`
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
  <p>Vite + Svelte starter (official fs + dialog + clipboard plugins).</p>
  <button onclick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>
  <button onclick={() => run(() => client.dialogOpen())}>dialog.open</button>
  <button onclick={() => run(() => client.clipboardRead())}>clipboard.read</button>
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
    <p>Vite + Vue starter (official fs + dialog + clipboard plugins).</p>
    <button @click="run(() => client.demoGreet('Vitra'))">demo.greet</button>
    <button @click="run(() => client.dialogOpen())">dialog.open</button>
    <button @click="run(() => client.clipboardRead())">clipboard.read</button>
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
      <p>Vite + Solid starter (official fs + dialog + clipboard plugins).</p>
      <button onClick={() => run(() => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run(() => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run(() => client.clipboardRead())}>clipboard.read</button>
      <pre>{out()}</pre>
    </div>
  );
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
  <p>Vite + TypeScript starter (official fs + dialog + clipboard plugins).</p>
  <button id="greet">demo.greet</button>
  <button id="open">dialog.open</button>
  <button id="clip">clipboard.read</button>
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
      <p>Vite + React starter (official fs + dialog + clipboard plugins).</p>
      <button onClick={() => run("greet", () => client.demoGreet("Vitra"))}>demo.greet</button>{" "}
      <button onClick={() => run("open", () => client.dialogOpen())}>dialog.open</button>{" "}
      <button onClick={() => run("clip", () => client.clipboardRead())}>clipboard.read</button>
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
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
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
	caller := domain.Caller{WindowID: "main", Origin: domain.OriginPackagedLocal}

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
	dialogs := &desktop.DialogService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context) ([]string, error) {
			path, err := host.OpenFileDialog()
			if err != nil || path == "" {
				return nil, err
			}
			return []string{path}, nil
		},
		OnSave: func(ctx context.Context) (string, error) {
			return host.SaveFileDialog()
		},
	}
	if err := rt.BindExecutor("dialog.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.OpenFile(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.save", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.SaveFile(ctx, caller)
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

	grant, _ := domain.NewCapabilityGrant(
		"demo", "demo", []domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "demo.greet"},
			{Name: desktop.PermDialogOpen},
			{Name: desktop.PermDialogSave},
			{Name: desktop.PermClipboardRead},
			{Name: desktop.PermClipboardWrite},
			{Name: desktop.PermFSRead, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
			{Name: desktop.PermFSWrite, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
		},
	)
	_ = rt.RegisterGrant(grant)

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
`
}

func scaffoldIndexHTML() string {
	return `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><title>Vitra App</title>
<style>body{font-family:Georgia,serif;margin:2rem;background:#111;color:#eee}
button{padding:.75rem 1rem;cursor:pointer;margin-right:.5rem}</style></head>
<body><h1>Vitra</h1><p>Secure desktop runtime starter (official fs + dialog + clipboard plugins).</p>
<button id="greet">demo.greet</button>
<button id="open">dialog.open</button>
<button id="clip">clipboard.read</button>
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
document.getElementById("clip").onclick = async () => {
  try { out.textContent = JSON.stringify(await invoke("clipboard.read"), null, 2); }
  catch (e) { out.textContent = String(e); }
};
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
	sign := false
	signExecute := false
	signFollowUps := false
	signingIdentity := ""
	usage := "usage: vitra package --out <dir> [--format dir|deb|rpm-dir|rpm|snap-dir|snap|flatpak-dir|flatpak|appdir|appimage|win-dir|wix|nsis-dir|msi|nsis|app-dir|dmg] [--bin path] [--app-id id] [--name name] [--version ver] [--icon path] [--maintainer name] [--description text] [--sign [--sign-execute] [--sign-follow-ups] --signing-identity ref]"
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
		case "--sign":
			sign = true
		case "--sign-execute":
			sign = true
			signExecute = true
		case "--sign-follow-ups":
			signFollowUps = true
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
		Sign:               sign,
		SigningIdentityRef: signingIdentity,
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
			fmt.Printf("signed %s with %s\n", art.Path, plan.Tool)
		}
	}
	return nil
}

// officialPluginInventory returns Manifest-derived plugin rows for packaging
// provenance (declared surface, not a claim that --bin embeds them).
func officialPluginInventory() []provenance.PluginInfo {
	out := make([]provenance.PluginInfo, 0, 3)
	for _, p := range []plugin.Plugin{officialfs.New(), officialdialog.New(), officialclipboard.New()} {
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
