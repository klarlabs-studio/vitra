//go:build windows && cgo && vitra_native

// Package windows provides a Win32 desktop host scaffold toward WebView2.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
//
// This first slice creates real HWND windows and a message loop. Navigate/Eval
// messaging requires the WebView2 Evergreen Runtime + SDK (explicit unsupported
// until that slice lands).
package windows

/*
#cgo LDFLAGS: -luser32 -lgdi32 -lcomdlg32
#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func init() { runtime.LockOSThread() }

// Host is a Win32 desktop host scaffold.
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
	jobs      sync.Map
	jobSeq    uint64
}

type nativeWindow struct{ ptr *C.VitraWin }

var (
	activeMu sync.Mutex
	active   *Host
)

// New constructs a Windows host.
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
		C.vitra_win32_init()
		h.inited = true
	}
}

// OS returns windows.
func (h *Host) OS() platform.OS { return platform.OSWindows }

// Features reports supported capabilities for this Win32 shell slice.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {Feature: platform.FeatureWindowCreate, Available: true, Detail: "Win32 HWND shell"},
		platform.FeatureWindowNavigate: {
			Feature: platform.FeatureWindowNavigate, Available: false,
			Detail: "requires WebView2 Evergreen Runtime + SDK (next slice)",
		},
		platform.FeatureWebViewMessage: {
			Feature: platform.FeatureWebViewMessage, Available: false,
			Detail: "requires WebView2 Evergreen Runtime + SDK (next slice)",
		},
		platform.FeatureClipboard: {
			Feature: platform.FeatureClipboard, Available: true,
			Detail: "PowerShell Get/Set-Clipboard; available without native WebView",
		},
		platform.FeatureDialogOpen: {
			Feature: platform.FeatureDialogOpen, Available: true,
			Detail: "Win32 GetOpenFileName open-file dialog",
		},
		platform.FeatureDialogSave: {
			Feature: platform.FeatureDialogSave, Available: true,
			Detail: "Win32 GetSaveFileName save-file dialog",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: false,
			Detail: "not yet implemented on Windows host",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: false,
			Detail: "not yet implemented on Windows host",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: true,
			Detail: "exclusive lock file; available without native WebView",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "not yet implemented on Windows host",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: true,
			Detail: "argv + socket handoff + HKCU Classes .reg URL-scheme registration",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: false,
			Detail: "not yet implemented on Windows host",
		},
		platform.FeatureFileAssociation: {
			Feature: platform.FeatureFileAssociation, Available: true,
			Detail: "HKCU ProgID + MIME .reg; available without native WebView",
		},
		platform.FeatureWindowChrome: {
			Feature: platform.FeatureWindowChrome, Available: true,
			Detail: "Win32 title, size, maximize, fullscreen, topmost, minimize, hide, icon",
		},
		platform.FeatureOpenURL: {
			Feature: platform.FeatureOpenURL, Available: true,
			Detail: "cmd start for http(s)/mailto; available without native WebView",
		},
	}
}

func (h *Host) SetInvokeHandler(fn func(domain.WindowID, domain.Origin, []byte) []byte) {
	h.onInvoke = fn
}
func (h *Host) SetNavPolicy(fn func(domain.WindowID, string) bool) { h.onNav = fn }
func (h *Host) SetActionHandler(fn func(id string))                { h.onAction = fn }
func (h *Host) SetDragDropHandler(fn func(windowID domain.WindowID, paths []string)) {
	h.onDrop = fn
}
func (h *Host) SetDestroyHandler(fn func(windowID domain.WindowID)) {
	h.onDestroy = fn
}
func (h *Host) EnableDragDrop(domain.WindowID, bool) error {
	return h.err(platform.FeatureDragDrop)
}
func (h *Host) InjectFileDrop(domain.WindowID, []string) {}

// ApplyWindowChrome sets title, size, and presentation hints on a native window.
func (h *Host) ApplyWindowChrome(id domain.WindowID, chrome platform.WindowChrome) error {
	if chrome.Width <= 0 || chrome.Height <= 0 {
		return &domain.ErrValidation{Message: "window width and height must be positive"}
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		ctitle := C.CString(chrome.Title)
		defer C.free(unsafe.Pointer(ctitle))
		cicon := C.CString(chrome.IconPath)
		defer C.free(unsafe.Pointer(cicon))
		maxed, full, above, mini, hid := C.int(0), C.int(0), C.int(0), C.int(0), C.int(0)
		if chrome.Maximized {
			maxed = 1
		}
		if chrome.Fullscreen {
			full = 1
		}
		if chrome.AlwaysOnTop {
			above = 1
		}
		if chrome.Minimized {
			mini = 1
		}
		if chrome.Hidden {
			hid = 1
		}
		C.vitra_win_apply_chrome(w.ptr, ctitle, C.int(chrome.Width), C.int(chrome.Height), maxed, full, above, mini, hid, cicon)
		errCh <- nil
	})
	return <-errCh
}

// ReadWindowChrome returns the window presentation last applied / observed.
func (h *Host) ReadWindowChrome(id domain.WindowID) (platform.WindowChrome, error) {
	type result struct {
		chrome platform.WindowChrome
		err    error
	}
	ch := make(chan result, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			ch <- result{err: &domain.ErrNotFound{Entity: "window", ID: string(id)}}
			return
		}
		raw := C.vitra_win_chrome(w.ptr)
		defer C.free(unsafe.Pointer(raw.title))
		defer C.free(unsafe.Pointer(raw.icon_path))
		ch <- result{chrome: platform.WindowChrome{
			Title:       C.GoString(raw.title),
			Width:       int(raw.width),
			Height:      int(raw.height),
			Maximized:   raw.maximized != 0,
			Fullscreen:  raw.fullscreen != 0,
			AlwaysOnTop: raw.above != 0,
			Minimized:   raw.minimized != 0,
			Hidden:      raw.hidden != 0,
			IconPath:    C.GoString(raw.icon_path),
		}}
	})
	got := <-ch
	return got.chrome, got.err
}

func (h *Host) CreateWindow(_ context.Context, spec platform.WindowSpec) error {
	return h.Open(spec, "about:blank", "")
}

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
			errCh <- errors.New("failed to create Win32 window")
			return
		}
		h.windows[spec.ID] = &nativeWindow{ptr: ptr}
		h.origins[spec.ID] = spec.Origin
		errCh <- nil
	})
	return <-errCh
}

func (h *Host) NavigateWindow(_ context.Context, id domain.WindowID, origin domain.Origin) error {
	return h.err(platform.FeatureWindowNavigate)
}

func (h *Host) PostMessage(context.Context, domain.WindowID, []byte) error {
	return h.err(platform.FeatureWebViewMessage)
}

func (h *Host) Eval(domain.WindowID, string) error {
	return h.err(platform.FeatureWebViewMessage)
}

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

// OpenFileDialog opens a native file chooser (GetOpenFileName).
func (h *Host) OpenFileDialog() (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		p := C.vitra_open_dialog()
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.free(unsafe.Pointer(p))
	})
	return <-ch, nil
}

// SaveFileDialog opens a native save-file chooser (GetSaveFileName).
func (h *Host) SaveFileDialog() (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		p := C.vitra_save_dialog()
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.free(unsafe.Pointer(p))
	})
	return <-ch, nil
}
func (h *Host) SetMenuBar(domain.WindowID, []platform.MenuItem) error {
	return h.err(platform.FeatureMenuBar)
}
func (h *Host) SetTray(string, []platform.MenuItem) error {
	return h.err(platform.FeatureTray)
}
func (h *Host) ClearTray() {}

func (h *Host) Run() error {
	h.ensureInit()
	h.mu.Lock()
	h.looping = true
	h.mu.Unlock()
	C.vitra_win32_main()
	return nil
}

func (h *Host) Quit() {
	h.dispatch(func() { C.vitra_win32_quit() })
}

func (h *Host) err(f platform.Feature) error {
	return &platform.ErrUnsupported{
		Feature: f,
		OS:      platform.OSWindows,
		Detail:  "not yet implemented on Windows host",
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
