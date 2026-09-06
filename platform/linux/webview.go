//go:build linux && cgo && vitra_native

// Package linux provides a real WebKitGTK/GTK3 desktop host.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
package linux

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include "native.h"
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

// Host is a WebKitGTK desktop host.
type Host struct {
	mu       sync.Mutex
	windows  map[domain.WindowID]*nativeWindow
	origins  map[domain.WindowID]domain.Origin
	onInvoke func(domain.WindowID, domain.Origin, []byte) []byte
	onNav    func(domain.WindowID, string) bool
	onAction func(id string)
	looping  bool
	inited   bool
	jobs     sync.Map // uint64 -> func()
	jobSeq   uint64
}

type nativeWindow struct{ ptr *C.VitraWin }

var (
	activeMu sync.Mutex
	active   *Host
)

// New constructs a Linux host.
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
		C.vitra_gtk_init()
		h.inited = true
	}
}

// OS returns linux.
func (h *Host) OS() platform.OS { return platform.OSLinux }

// Features reports supported native capabilities.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate:   {Feature: platform.FeatureWindowCreate, Available: true},
		platform.FeatureWindowNavigate: {Feature: platform.FeatureWindowNavigate, Available: true},
		platform.FeatureWebViewMessage: {Feature: platform.FeatureWebViewMessage, Available: true},
		platform.FeatureDialogOpen:     {Feature: platform.FeatureDialogOpen, Available: true},
		platform.FeatureClipboard:      {Feature: platform.FeatureClipboard, Available: true},
		platform.FeatureMenuBar:        {Feature: platform.FeatureMenuBar, Available: true},
		platform.FeatureTray:           {Feature: platform.FeatureTray, Available: true},
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
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		C.vitra_win_close(w.ptr)
		C.vitra_win_free(w.ptr)
		delete(h.windows, id)
		delete(h.origins, id)
		errCh <- nil
	})
	return <-errCh
}

// ClipboardGet returns clipboard text.
func (h *Host) ClipboardGet() (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		p := C.vitra_clip_get()
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.g_free(C.gpointer(p))
	})
	return <-ch, nil
}

// ClipboardSet sets clipboard text.
func (h *Host) ClipboardSet(text string) error {
	done := make(chan struct{}, 1)
	h.dispatch(func() {
		h.ensureInit()
		ct := C.CString(text)
		defer C.free(unsafe.Pointer(ct))
		C.vitra_clip_set(ct)
		done <- struct{}{}
	})
	<-done
	return nil
}

// OpenFileDialog opens a native file chooser.
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
		C.g_free(C.gpointer(p))
	})
	return <-ch, nil
}

// MenuItem is a native menu entry.
type MenuItem struct {
	Menu  string // top-level menu label, e.g. "File"
	ID    string
	Label string
}

// SetMenuBar replaces the application menu bar on the given window.
func (h *Host) SetMenuBar(id domain.WindowID, items []MenuItem) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		C.vitra_win_clear_menu(w.ptr)
		for _, it := range items {
			cmenu := C.CString(it.Menu)
			cid := C.CString(it.ID)
			clabel := C.CString(it.Label)
			C.vitra_win_add_menu_item(w.ptr, cmenu, cid, clabel)
			C.free(unsafe.Pointer(cmenu))
			C.free(unsafe.Pointer(cid))
			C.free(unsafe.Pointer(clabel))
		}
		errCh <- nil
	})
	return <-errCh
}

// SetTray shows a status-icon tray entry with tooltip.
func (h *Host) SetTray(tooltip string) error {
	done := make(chan struct{}, 1)
	h.dispatch(func() {
		h.ensureInit()
		ct := C.CString(tooltip)
		defer C.free(unsafe.Pointer(ct))
		C.vitra_tray_set(ct)
		done <- struct{}{}
	})
	<-done
	return nil
}

// ClearTray hides the tray icon.
func (h *Host) ClearTray() {
	h.dispatch(func() { C.vitra_tray_clear() })
}

// Run runs the GTK main loop (blocking). Must be called from the main OS thread.
func (h *Host) Run() error {
	h.ensureInit()
	h.mu.Lock()
	h.looping = true
	h.mu.Unlock()
	C.vitra_gtk_main()
	return nil
}

// Quit leaves the GTK main loop.
func (h *Host) Quit() {
	h.dispatch(func() { C.vitra_gtk_quit() })
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
		_ = h.PostMessage(context.Background(), id, resp)
	}
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
	h.mu.Unlock()
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

//export goVitraAction
func goVitraAction(actionID *C.char) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil || h.onAction == nil {
		return
	}
	h.onAction(C.GoString(actionID))
}
