// Package platform defines Phase 0 ports for OS-native desktop adapters.
//
// Each OS has a first-class adapter. Unsupported operations must surface as
// explicit FeatureUnavailable results — never silent no-ops (security
// invariant 14 / design principle 5).
package platform

import (
	"context"

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
	FeatureWindowCreate    Feature = "window.create"
	FeatureWindowNavigate  Feature = "window.navigate"
	FeatureWebViewMessage  Feature = "webview.message"
	FeatureMenuBar         Feature = "menu.bar"
	FeatureTray            Feature = "tray"
	FeatureDialogOpen      Feature = "dialog.open"
	FeatureDialogSave      Feature = "dialog.save"
	FeatureClipboard       Feature = "clipboard"
	FeatureGlobalShortcut  Feature = "shortcut.global"
	FeatureDeepLink        Feature = "deeplink"
	FeatureSingleInstance  Feature = "single_instance"
	FeatureDragDrop        Feature = "drag_drop"
	FeatureFileAssociation Feature = "file_association"
)

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

// WindowSpec describes a window to create.
type WindowSpec struct {
	ID     domain.WindowID
	Title  string
	Origin domain.Origin
	Width  int
	Height int
}

// Host is the Phase 0 native adapter port. Real OS adapters (darwin/windows/
// linux) implement this; the null adapter documents unsupported semantics.
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
