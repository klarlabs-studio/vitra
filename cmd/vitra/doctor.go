package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// Check statuses reported by vitra doctor.
const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

// doctorCheck is one result of vitra doctor. The text report and the JSON
// report are both rendered from the same checks; the unexported fields only
// control the text layout.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`

	heading string // text line printed before this check, if any
	indent  string // text indentation
	width   int    // text column width of "name:"
}

// doctorReport is the JSON document printed by vitra doctor --json.
type doctorReport struct {
	Checks []doctorCheck `json:"checks"`
}

var doctorFeatures = []platform.Feature{
	platform.FeatureWindowCreate,
	platform.FeatureWindowNavigate,
	platform.FeatureWebViewMessage,
	platform.FeatureClipboard,
	platform.FeatureDialogOpen,
	platform.FeatureDialogSave,
	platform.FeatureDialogOpenDirectory,
	platform.FeatureMenuBar,
	platform.FeatureTray,
	platform.FeatureTrayTitle,
	platform.FeatureTrayIcon,
	platform.FeaturePresentation,
	platform.FeatureSingleInstance,
	platform.FeatureGlobalShortcut,
	platform.FeatureDeepLink,
	platform.FeatureDragDrop,
	platform.FeatureFileAssociation,
	platform.FeatureWindowChrome,
	platform.FeatureOpenURL,
}

type packagingTool struct {
	name    string
	resolve func() (string, error)
	envHint string
}

var doctorFoldTools = []packagingTool{
	{"appimagetool", packaging.ResolveAppImageTool, "VITRA_APPIMAGETOOL"},
	{"rpmbuild", packaging.ResolveRpmbuild, "VITRA_RPMBUILD"},
	{"snapcraft", packaging.ResolveSnapcraft, "VITRA_SNAPCRAFT"},
	{"flatpak-builder", packaging.ResolveFlatpakBuilder, "VITRA_FLATPAK_BUILDER"},
	{"candle", packaging.ResolveCandle, "VITRA_CANDLE"},
	{"light", packaging.ResolveLight, "VITRA_LIGHT"},
	{"makensis", packaging.ResolveMakensis, "VITRA_MAKENSIS"},
	{"hdiutil", packaging.ResolveHdiutil, "VITRA_HDIUTIL"},
}

var doctorSignTools = []packagingTool{
	{"codesign", packaging.ResolveCodesign, "VITRA_CODESIGN"},
	{"signtool", packaging.ResolveSigntool, "VITRA_SIGNTOOL"},
	{"notarytool", packaging.ResolveNotarytool, "VITRA_NOTARYTOOL"},
	{"stapler", packaging.ResolveStapler, "VITRA_STAPLER"},
	{"gpg", packaging.ResolveGPG, "VITRA_GPG"},
	{"dpkg-sig", packaging.ResolveDpkgSig, "VITRA_DPKGSIG"},
	{"rpmsign", packaging.ResolveRpmsign, "VITRA_RPMSIGN"},
}

// doctor prints the doctor report as text, or as JSON with --json. The exit
// status does not depend on the output format.
func doctor(args []string) error {
	asJSON := false
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		default:
			return fmt.Errorf("unknown doctor flag %q", a)
		}
	}
	checks := runDoctorChecks()
	if asJSON {
		return writeDoctorJSON(os.Stdout, checks)
	}
	return writeDoctorText(os.Stdout, checks)
}

func writeDoctorJSON(w io.Writer, checks []doctorCheck) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doctorReport{Checks: checks})
}

func writeDoctorText(w io.Writer, checks []doctorCheck) error {
	var b strings.Builder
	b.WriteString("vitra doctor\n")
	for _, c := range checks {
		if c.heading != "" {
			b.WriteString(c.heading + "\n")
		}
		fmt.Fprintf(&b, "%s%-*s %s\n", c.indent, c.width, c.Name+":", c.Detail)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// runDoctorChecks gathers every doctor check in report order.
func runDoctorChecks() []doctorCheck {
	top := func(name, status, detail string) doctorCheck {
		return doctorCheck{Name: name, Status: status, Detail: detail, indent: "  ", width: 8}
	}
	cgoStatus, cgoDetail := statusOK, "enabled"
	if !cgoEnabled() {
		cgoStatus, cgoDetail = statusWarn, "disabled"
	}
	host := currentHost()
	checks := []doctorCheck{
		top("go", statusOK, runtime.Version()),
		top("os/arch", statusOK, runtime.GOOS+"/"+runtime.GOARCH),
		top("cgo", cgoStatus, cgoDetail),
		top("kernel", statusOK, vitra.Version),
		top("adapter", statusOK, string(host.OS())),
	}
	checks = append(checks, featureChecks(host)...)
	checks = append(checks, toolchainChecks()...)
	checks = append(checks, packagingChecks("  packaging fold tools:", doctorFoldTools)...)
	checks = append(checks, packagingChecks("  packaging sign tools (ExecuteSign via --sign-execute):", doctorSignTools)...)
	return checks
}

func featureChecks(host platform.Host) []doctorCheck {
	features := host.Features()
	checks := make([]doctorCheck, 0, len(doctorFeatures))
	for _, f := range doctorFeatures {
		s := features[f]
		status, detail := statusWarn, "missing"
		if s.Available {
			status, detail = statusOK, "ok"
		} else if s.Detail != "" {
			detail = "unavailable — " + s.Detail
		}
		checks = append(checks, doctorCheck{Name: string(f), Status: status, Detail: detail, indent: "  ", width: 18})
	}
	return checks
}

// toolchainChecks reports the OS-specific WebView toolchain.
func toolchainChecks() []doctorCheck {
	note := func(name, status, detail string) doctorCheck {
		return doctorCheck{Name: name, Status: status, Detail: detail, indent: "  ", width: 9}
	}
	const nativeBuild = "build/run with CGO_ENABLED=1 -tags vitra_native"
	switch runtime.GOOS {
	case "linux":
		return append(webkitChecks(note),
			note("native", statusOK, nativeBuild),
			note("note", statusOK, "global shortcuts via XGrabKey on X11; Wayland stays unsupported (use MenuItem.Shortcut)"))
	case "darwin":
		return []doctorCheck{
			{Name: "frameworks", Status: statusOK, Detail: "Cocoa + WebKit (system)", indent: "  "},
			note("native", statusOK, nativeBuild),
			note("note", statusOK, "WKWebView DesktopHost under -tags vitra_native; global shortcuts via RegisterEventHotKey (Ctrl→Command)"),
		}
	case "windows":
		return []doctorCheck{
			note("native", statusOK, nativeBuild+" (Win32 + WebView2Loader.dll)"),
			note("note", statusOK, "WebView2 Navigate/Eval/message need Evergreen Runtime; global shortcuts via RegisterHotKey"),
		}
	}
	return nil
}

// webkitChecks looks for the WebKitGTK development package on Linux.
func webkitChecks(note func(name, status, detail string) doctorCheck) []doctorCheck {
	out, err := exec.Command("pkg-config", "--exists", "webkit2gtk-4.1").CombinedOutput()
	if err == nil {
		return []doctorCheck{{Name: "pkg-config", Status: statusOK, Detail: "webkit2gtk-4.1 ok", indent: "  "}}
	}
	return []doctorCheck{
		{
			Name:   "pkg-config",
			Status: statusFail,
			Detail: fmt.Sprintf("webkit2gtk-4.1 not found (%v %s)", err, strings.TrimSpace(string(out))),
			indent: "  ",
		},
		note("hint", statusWarn, "sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev"),
	}
}

// packagingChecks resolves each tool; the first check carries the section heading.
func packagingChecks(heading string, tools []packagingTool) []doctorCheck {
	checks := make([]doctorCheck, 0, len(tools))
	for _, t := range tools {
		c := doctorCheck{Name: t.name, indent: "    ", width: 12}
		if path, err := t.resolve(); err != nil {
			c.Status, c.Detail = statusWarn, fmt.Sprintf("missing (set %s or install on PATH)", t.envHint)
		} else {
			c.Status, c.Detail = statusOK, fmt.Sprintf("ok (%s)", path)
		}
		checks = append(checks, c)
	}
	if len(checks) > 0 {
		checks[0].heading = heading
	}
	return checks
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
