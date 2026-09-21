//go:build darwin && cgo && vitra_native

// Package darwin provides a real WKWebView desktop host.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
package darwin

/*
#cgo CFLAGS: -x objective-c -fno-objc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func init() { runtime.LockOSThread() }

// Host is a WKWebView desktop host.
type Host struct {
	mu        sync.Mutex
	windows   map[domain.WindowID]*nativeWindow
	origins   map[domain.WindowID]domain.Origin
	onInvoke  func(domain.WindowID, domain.Origin, []byte) []byte
	onNav     func(domain.WindowID, string) bool
	onAction  func(id string)
	onDrop    func(windowID domain.WindowID, paths []string)
	onDestroy func(windowID domain.WindowID)
	looping   bool
	inited    bool
	jobs      sync.Map // uint64 -> func()
	jobSeq    uint64
}

type nativeWindow struct{ ptr *C.VitraWin }

var (
	activeMu sync.Mutex
	active   *Host
)

// New constructs a Darwin host.
func New() *Host {
	h := &Host{
		windows: make(map[domain.WindowID]*nativeWindow),
		origins: make(map[domain.WindowID]domain.Origin),
	}
	activeMu.Lock()
	active = h
	activeMu.Unlock()
	return h
}

func (h *Host) ensureInit() {
	if !h.inited {
		C.vitra_app_init()
		h.inited = true
	}
}

// OS returns darwin.
func (h *Host) OS() platform.OS { return platform.OSDarwin }

// Features reports supported native capabilities for this first WKWebView slice.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate:   {Feature: platform.FeatureWindowCreate, Available: true},
		platform.FeatureWindowNavigate: {Feature: platform.FeatureWindowNavigate, Available: true},
		platform.FeatureWebViewMessage: {Feature: platform.FeatureWebViewMessage, Available: true},
		platform.FeatureClipboard: {
			Feature: platform.FeatureClipboard, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureDialogOpen: {
			Feature: platform.FeatureDialogOpen, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureDialogSave: {
			Feature: platform.FeatureDialogSave, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureFileAssociation: {
			Feature: platform.FeatureFileAssociation, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureWindowChrome: {
			Feature: platform.FeatureWindowChrome, Available: false,
			Detail: "not yet implemented on Darwin WKWebView host",
		},
		platform.FeatureOpenURL: {
			Feature: platform.FeatureOpenURL, Available: true,
			Detail: "open for http(s)/mailto; available without native WebView",
		},
	}
}

// SetInvokeHandler registers the IPC callback.
func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}

// SetNavPolicy registers navigation allow/deny.
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }

// SetActionHandler registers menu/tray action callbacks.
func (h *Host) SetActionHandler(fn func(id string)) { h.onAction = fn }

// SetDragDropHandler registers file-drop callbacks.
func (h *Host) SetDragDropHandler(fn func(windowID domain.WindowID, paths []string)) {
	h.onDrop = fn
}

// SetDestroyHandler registers callbacks when a native window is destroyed.
func (h *Host) SetDestroyHandler(fn func(windowID domain.WindowID)) {
	h.onDestroy = fn
}

// EnableDragDrop is not yet implemented on Darwin.
func (h *Host) EnableDragDrop(domain.WindowID, bool) error {
	return h.err(platform.FeatureDragDrop)
}

// InjectFileDrop is a no-op until drag-drop lands on Darwin.
func (h *Host) InjectFileDrop(domain.WindowID, []string) {}

// ApplyWindowChrome is not yet implemented on Darwin.
func (h *Host) ApplyWindowChrome(domain.WindowID, platform.WindowChrome) error {
	return h.err(platform.FeatureWindowChrome)
}

// ReadWindowChrome is not yet implemented on Darwin.
func (h *Host) ReadWindowChrome(domain.WindowID) (platform.WindowChrome, error) {
	return platform.WindowChrome{}, h.err(platform.FeatureWindowChrome)
}

// CreateWindow implements platform.Host.
func (h *Host) CreateWindow(_ context.Context, spec platform.WindowSpec) error {
	return h.Open(spec, "about:blank", "")
}

// Open creates a native window loading uri with optional preload JS.
func (h *Host) Open(spec platform.WindowSpec, uri, preload string) error {
	if spec.ID == "" {
		return errors.New("window id required")
	}
	if spec.Width <= 0 {
		spec.Width = 1024
	}
	if spec.Height <= 0 {
		spec.Height = 768
	}
	if spec.Title == "" {
		spec.Title = string(spec.ID)
	}
	if uri == "" {
		uri = "about:blank"
	}
	if spec.Origin == "" {
		spec.Origin = domain.OriginPackagedLocal
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, exists := h.windows[spec.ID]; exists {
			errCh <- errors.New("window already exists")
			return
		}
		cid := C.CString(string(spec.ID))
		ctitle := C.CString(spec.Title)
		curi := C.CString(uri)
		cjs := C.CString(preload)
		defer C.free(unsafe.Pointer(cid))
		defer C.free(unsafe.Pointer(ctitle))
		defer C.free(unsafe.Pointer(curi))
		defer C.free(unsafe.Pointer(cjs))
		ptr := C.vitra_win_new(cid, ctitle, C.int(spec.Width), C.int(spec.Height), curi, cjs)
		if ptr == nil {
			errCh <- errors.New("failed to create WKWebView window")
			return
		}
		h.windows[spec.ID] = &nativeWindow{ptr: ptr}
		h.origins[spec.ID] = spec.Origin
		errCh <- nil
	})
	return <-errCh
}

// NavigateWindow loads a URI and stores it as the window origin.
func (h *Host) NavigateWindow(_ context.Context, id domain.WindowID, origin domain.Origin) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		curi := C.CString(string(origin))
		defer C.free(unsafe.Pointer(curi))
		C.vitra_win_navigate(w.ptr, curi)
		h.origins[id] = origin
		errCh <- nil
	})
	return <-errCh
}

// PostMessage delivers a bridge payload to the frontend.
func (h *Host) PostMessage(_ context.Context, id domain.WindowID, message []byte) error {
	enc, err := json.Marshal(string(message))
	if err != nil {
		return err
	}
	return h.Eval(id, "window.__vitra&&window.__vitra.__recv("+string(enc)+");")
}

// Eval runs JavaScript in the given window.
func (h *Host) Eval(id domain.WindowID, js string) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		cjs := C.CString(js)
		defer C.free(unsafe.Pointer(cjs))
		C.vitra_win_eval(w.ptr, cjs)
		errCh <- nil
	})
	return <-errCh
}

// CloseWindow destroys a native window.
func (h *Host) CloseWindow(_ context.Context, id domain.WindowID) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		w, ok := h.windows[id]
		if !ok {
			h.mu.Unlock()
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		delete(h.windows, id)
		delete(h.origins, id)
		ptr := w.ptr
		empty := len(h.windows) == 0
		h.mu.Unlock()
		C.vitra_win_close(ptr)
		C.vitra_win_free(ptr)
		if empty {
			activeMu.Lock()
			if active == h {
				active = nil
			}
			activeMu.Unlock()
		}
		errCh <- nil
	})
	return <-errCh
}

// ClipboardGet is not yet implemented on Darwin.
func (h *Host) ClipboardGet() (string, error) {
	return "", h.err(platform.FeatureClipboard)
}

// ClipboardSet is not yet implemented on Darwin.
func (h *Host) ClipboardSet(string) error { return h.err(platform.FeatureClipboard) }

// OpenFileDialog is not yet implemented on Darwin.
func (h *Host) OpenFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogOpen)
}

// SaveFileDialog is not yet implemented on Darwin.
func (h *Host) SaveFileDialog() (string, error) {
	return "", h.err(platform.FeatureDialogSave)
}

// SetMenuBar is not yet implemented on Darwin.
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}

// SetTray is not yet implemented on Darwin.
func (h *Host) SetTray(string, []platform.MenuItem) error {
	return h.err(platform.FeatureTray)
}

// ClearTray is a no-op until tray lands on Darwin.
func (h *Host) ClearTray() {}

// TrySingleInstance is not yet implemented on Darwin.
func (h *Host) TrySingleInstance(string) (bool, func(), error) {
	return false, nil, h.err(platform.FeatureSingleInstance)
}

// StartDeepLinkBridge is not yet implemented on Darwin.
func (h *Host) StartDeepLinkBridge(string, func(string)) (func(), error) {
	return nil, h.err(platform.FeatureDeepLink)
}

// ForwardToPrimary is not yet implemented on Darwin.
func (h *Host) ForwardToPrimary(string, []string) (bool, error) {
	return false, h.err(platform.FeatureDeepLink)
}

// RegisterURLScheme is not yet implemented on Darwin.
func (h *Host) RegisterURLScheme(string, string, string) error {
	return h.err(platform.FeatureDeepLink)
}

// Run runs the Cocoa main loop (blocking). Must be called from the main OS thread.
func (h *Host) Run() error {
	h.ensureInit()
	h.mu.Lock()
	h.looping = true
	h.mu.Unlock()
	C.vitra_app_run()
	return nil
}

// Quit leaves the Cocoa main loop.
func (h *Host) Quit() {
	h.dispatch(func() { C.vitra_app_quit() })
}

func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSDarwin,
		Detail:  "not yet implemented on Darwin WKWebView host",
	}
}

func (h *Host) dispatch(fn func()) {
	h.mu.Lock()
	looping := h.looping
	h.mu.Unlock()
	if !looping {
		fn()
		return
	}
	h.mu.Lock()
	h.jobSeq++
	id := h.jobSeq
	h.mu.Unlock()
	h.jobs.Store(id, fn)
	C.vitra_idle_add(unsafe.Pointer(uintptr(id)))
}

//export goVitraIdle
func goVitraIdle(ptr unsafe.Pointer) {
	id := uint64(uintptr(ptr))
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil {
		return
	}
	v, ok := h.jobs.LoadAndDelete(id)
	if !ok {
		return
	}
	v.(func())()
}

//export goVitraMessage
func goVitraMessage(windowID, msg *C.char) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil || h.onInvoke == nil {
		return
	}
	id := domain.WindowID(C.GoString(windowID))
	h.mu.Lock()
	origin := h.origins[id]
	h.mu.Unlock()
	if origin == "" {
		origin = domain.OriginPackagedLocal
	}
	resp := h.onInvoke(id, origin, []byte(C.GoString(msg)))
	if len(resp) > 0 {
		h.replyOnMainThread(id, resp)
	}
}

func (h *Host) replyOnMainThread(id domain.WindowID, message []byte) {
	enc, err := json.Marshal(string(message))
	if err != nil {
		return
	}
	js := "window.__vitra&&window.__vitra.__recv(" + string(enc) + ");"
	h.mu.Lock()
	w, ok := h.windows[id]
	h.mu.Unlock()
	if !ok || w == nil || w.ptr == nil {
		return
	}
	cjs := C.CString(js)
	defer C.free(unsafe.Pointer(cjs))
	C.vitra_win_eval(w.ptr, cjs)
}

//export goVitraDestroy
func goVitraDestroy(windowID *C.char) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil {
		return
	}
	id := domain.WindowID(C.GoString(windowID))
	h.mu.Lock()
	if w, ok := h.windows[id]; ok {
		C.vitra_win_free(w.ptr)
		delete(h.windows, id)
		delete(h.origins, id)
	}
	onDestroy := h.onDestroy
	empty := len(h.windows) == 0
	h.mu.Unlock()
	if empty {
		activeMu.Lock()
		if active == h {
			active = nil
		}
		activeMu.Unlock()
	}
	if onDestroy != nil {
		onDestroy(id)
	}
}

//export goVitraNav
func goVitraNav(windowID, uri *C.char) C.int {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil || h.onNav == nil {
		return 1
	}
	if h.onNav(domain.WindowID(C.GoString(windowID)), C.GoString(uri)) {
		return 1
	}
	return 0
}
