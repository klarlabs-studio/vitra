// Command vitra is the Vitra developer CLI.
package main

import (
	"fmt"
	"os"

	"go.klarlabs.de/vitra"
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
  vitra new <dir> [--template vanilla|vite|react|svelte|vue] [--with fs,dialog,clipboard,notification,os]
                             Scaffold a starter desktop app (default: vanilla HTML; vite/react/svelte/vue add Vite frontends)
                             --with: register those official plugins with least-privilege grants and a demo call each
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
                     [--channel stable|beta] [--artifact-name name] [--expires-in 90d]
                             Digest + sign an update manifest (privkey: env:/file:/secret:; bare hex rejected)
  vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>
                             Fetch + verify a signed channel manifest (HTTP(S) client; does not install)
  vitra update-stage --out <dir> --manifest <json> --artifact <path>
                             Stage {out}/{app}/{channel}/manifest.json + artifact for static CDN upload
  vitra update-apply (--manifest <json> --artifact <path> | --base-url <url>) --app-id <id> [--channel name] --current-version <semver> --pubkey <hex> --dest <path> [--policy production|development]
                             Verify a signed update and install it (binary, or .tar.gz/.zip replacing a directory such as a .app)
  vitra notary-setup [--profile name]
                             Print a dry-run notarytool store-credentials plan (Darwin notarize bootstrap; not executed)
  vitra register-scheme <scheme> [app-id] [exec]
                             Register a URL scheme handler (Linux xdg / Darwin helper .app / Windows .reg)
  vitra register-files --mime <type> [--mime <type>] [--app-id id] [--exec path] [--name name]
                             Register MIME file associations (Linux xdg / Darwin helper .app / Windows .reg)
  vitra inspect capabilities Demo capability inspection against an in-memory runtime
  vitra help                 Show this help`)
}
