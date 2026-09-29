package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/internal/packaging"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
)

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
