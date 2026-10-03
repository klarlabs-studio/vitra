// Package platform defines the ports that OS-native desktop hosts implement.
//
// Each OS has a first-class adapter. Unsupported operations must surface as
// explicit FeatureUnavailable results — never silent no-ops (security
// invariant 14 / design principle 5).
package platform

import (
	"context"
	"strings"

	"go.klarlabs.de/vitra/domain"
)

// OS identifies a desktop operating system family.
type OS string

const (
	OSDarwin  OS = "darwin"
	OSWindows OS = "windows"
	OSLinux   OS = "linux"
	OSUnknown OS = "unknown"
)

// Feature names portable desktop capabilities.
type Feature string

const (
	FeatureWindowCreate        Feature = "window.create"
	FeatureWindowNavigate      Feature = "window.navigate"
	FeatureWebViewMessage      Feature = "webview.message"
	FeatureMenuBar             Feature = "menu.bar"
	FeatureTray                Feature = "tray"
	FeatureTrayTitle           Feature = "tray.title"
	FeatureTrayIcon            Feature = "tray.icon"
	FeatureTrayAnchor          Feature = "tray.anchor"
	FeatureWindowPanel         Feature = "window.panel"
	FeaturePresentation        Feature = "app.presentation"
	FeatureLoginItem           Feature = "app.login_item"
	FeatureDialogOpen          Feature = "dialog.open"
	FeatureDialogSave          Feature = "dialog.save"
	FeatureDialogMessage       Feature = "dialog.message"
	FeatureDialogOpenDirectory Feature = "dialog.openDirectory"
	FeatureNotificationShow    Feature = "notifications.show"
	FeatureClipboard           Feature = "clipboard"
	FeatureGlobalShortcut      Feature = "shortcut.global"
	FeatureDeepLink            Feature = "deeplink"
	FeatureSingleInstance      Feature = "single_instance"
	FeatureDragDrop            Feature = "drag_drop"
	FeatureFileAssociation     Feature = "file_association"
	FeatureWindowChrome        Feature = "window.chrome"
	FeatureOpenURL             Feature = "browser.open"
	FeaturePathOpen            Feature = "path.open"
)

// WindowChrome is the native window presentation a host can apply and read back.
type WindowChrome struct {
	Title       string
	Width       int
	Height      int
	Maximized   bool
	Fullscreen  bool
	AlwaysOnTop bool
	Minimized   bool
	// Hidden hides the window when true; the zero value keeps it visible.
	Hidden bool
	// IconPath is an optional filesystem path to a window icon image.
	// Empty leaves the current icon unchanged.
	IconPath string
}

// FileFilter describes a named set of file extensions for open/save dialogs.
// Extensions are without a leading dot (e.g. "png", not ".png").
type FileFilter struct {
	Name       string
	Extensions []string
}

// DialogFileOptions configures native open/save/directory dialogs.
// Zero values preserve previous unfiltered, untitled behavior.
// Filters apply to open/save only; directory dialogs use Title and DefaultPath.
type DialogFileOptions struct {
	Title       string
	DefaultPath string
	Filters     []FileFilter
	// Multiple lets the open dialog select several files. Hosts honour it
	// through MultiFileOpener; it is ignored by save and directory dialogs.
	Multiple bool
}

// DecodePathList splits a buffer of NUL-terminated paths, the format native
// hosts use to hand a multi-file selection to Go. NUL is the one byte no path
// can contain on any supported OS (unlike newline, which Linux allows), so
// no path is ever split or merged. Empty entries are dropped; nil or an empty
// buffer yields nil.
func DecodePathList(buf []byte) []string {
	var paths []string
	for _, p := range strings.Split(string(buf), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// EncodeFileFilters serializes filters as "Name:ext1,ext2;Other:txt" for C hosts.
// Empty input yields "".
func EncodeFileFilters(filters []FileFilter) string {
	if len(filters) == 0 {
		return ""
	}
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		name := strings.TrimSpace(f.Name)
		exts := make([]string, 0, len(f.Extensions))
		for _, e := range f.Extensions {
			e = strings.TrimSpace(e)
			e = strings.TrimPrefix(e, ".")
			if e == "" {
				continue
			}
			exts = append(exts, e)
		}
		if name == "" && len(exts) == 0 {
			continue
		}
		if name == "" {
			name = strings.Join(exts, ",")
		}
		parts = append(parts, name+":"+strings.Join(exts, ","))
	}
	return strings.Join(parts, ";")
}

// Support describes whether a feature is available on the current host.
type Support struct {
	Feature   Feature
	Available bool
	// Detail explains unavailability (missing WebView2 runtime, GTK version, …).
	Detail string
}

// FeatureSet is the discoverable support matrix for an adapter.
type FeatureSet map[Feature]Support

// Available reports whether feature is supported.
func (fs FeatureSet) Available(f Feature) bool {
	s, ok := fs[f]
	return ok && s.Available
}

// ErrUnsupported is returned when an adapter cannot perform an operation.
// Callers must treat this as an explicit capability/diagnostic result.
type ErrUnsupported struct {
	Feature Feature
	OS      OS
	Detail  string
}

func (e *ErrUnsupported) Error() string {
	if e.Detail != "" {
		return "platform unsupported: " + string(e.Feature) + " on " + string(e.OS) + ": " + e.Detail
	}
	return "platform unsupported: " + string(e.Feature) + " on " + string(e.OS)
}

// WindowKind is the kind of native window a WindowSpec asks for.
type WindowKind string

const (
	// WindowKindNormal is a regular, decorated application window.
	WindowKindNormal WindowKind = ""
	// WindowKindPanel is a tray panel: frameless, not resizable, above other
	// windows, without a taskbar entry, opened hidden, and hidden again when
	// it loses focus. Hosts that implement Panels show it under the tray.
	WindowKindPanel WindowKind = "panel"
)

// WindowSpec describes a window to create.
type WindowSpec struct {
	ID     domain.WindowID
	Title  string
	Origin domain.Origin
	Width  int
	Height int
	Kind   WindowKind
}

// Host is the base native adapter port: the feature matrix and window
// lifecycle. The desktop services take it to check features. DesktopHost
// adds what running an app needs; see capabilities.go for the optional
// interfaces.
type Host interface {
	OS() OS
	Features() FeatureSet
	CreateWindow(ctx context.Context, spec WindowSpec) error
	NavigateWindow(ctx context.Context, id domain.WindowID, origin domain.Origin) error
	PostMessage(ctx context.Context, id domain.WindowID, message []byte) error
	CloseWindow(ctx context.Context, id domain.WindowID) error
}

// Require returns ErrUnsupported if feature is not available.
func Require(h Host, f Feature) error {
	fs := h.Features()
	if fs.Available(f) {
		return nil
	}
	detail := ""
	if s, ok := fs[f]; ok {
		detail = s.Detail
	} else {
		detail = "feature not declared in adapter matrix"
	}
	return &ErrUnsupported{Feature: f, OS: h.OS(), Detail: detail}
}
