// Command vitra is the Vitra developer CLI.
//
// Exact commands are not yet contractual (see docs/intent.md). This skeleton
// exposes version, inspect, and doctor so the DX shape from the charter is
// exercisable against the Phase 1 kernel.
package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
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
  vitra doctor               Diagnose environment prerequisites
  vitra inspect capabilities Demo capability inspection against an in-memory runtime
  vitra help                 Show this help

Phase 1 ships the secure runtime kernel. Windowing, packaging, and
frontend starters arrive in later phases — see docs/intent.md.`)
}

func doctor() error {
	fmt.Println("vitra doctor")
	fmt.Printf("  go:      %s\n", runtime.Version())
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  kernel:  %s\n", vitra.Version)
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("  webview: WKWebView (adapter not yet linked in Phase 1)")
	case "windows":
		fmt.Println("  webview: WebView2 (adapter not yet linked in Phase 1)")
	case "linux":
		fmt.Println("  webview: WebKitGTK (adapter not yet linked in Phase 1)")
	default:
		fmt.Printf("  webview: unsupported platform %q (explicit)\n", runtime.GOOS)
	}
	fmt.Println("  status:  kernel-only; platform adapters pending Phase 0 spikes")
	return nil
}

// inspectDemo builds a small in-memory app so `vitra inspect capabilities`
// demonstrates the inspectable privileged surface without a full desktop host.
func inspectDemo(args []string) error {
	if len(args) == 0 || args[0] != "capabilities" {
		return fmt.Errorf("usage: vitra inspect capabilities")
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.myapp"})
	if err != nil {
		return err
	}
	ctx := context.Background()
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
	fmt.Print(vitra.FormatInspect(rt.AppID(), surface))
	return nil
}
