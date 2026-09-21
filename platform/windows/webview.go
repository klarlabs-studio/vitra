//go:build windows && cgo && vitra_native

// Package windows provides a Win32 + WebView2 desktop host.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
//
// Requires WebView2Loader.dll and the Evergreen WebView2 Runtime at runtime for
// Navigate/Eval/message. The HWND shell still opens when the loader is absent.
package windows

/*
#cgo LDFLAGS: -luser32 -lgdi32 -lcomdlg32 -lshell32 -lole32
#include "native.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func init() { runtime.LockOSThread() }

// Host is a Win32 desktop host scaffold.
type Host struct {
	mu          sync.Mutex
	windows     map[domain.WindowID]*nativeWindow
	origins     map[domain.WindowID]domain.Origin
	onInvoke    func(domain.WindowID, domain.Origin, []byte) []byte
	onNav       func(domain.WindowID, string) bool
	onAction    func(id string)
	onDrop      func(windowID domain.WindowID, paths []string)
	onDestroy   func(windowID domain.WindowID)
	looping     bool
	inited      bool
	programName string
	jobs        sync.Map
	jobSeq      uint64
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

// SetProgramName sets the process AppUserModelID for taskbar identity. Must be
// called before the first window/event-loop call. Empty keeps the default
// (basename of os.Args[0]).
func (h *Host) SetProgramName(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.inited {
		return
	}
	h.programName = strings.TrimSpace(name)
}

func (h *Host) ensureInit() {
	if h.inited {
		return
	}
	h.mu.Lock()
	if h.inited {
		h.mu.Unlock()
		return
	}
	name := h.programName
	h.inited = true
	h.mu.Unlock()
	if name == "" && len(os.Args) > 0 {
		name = filepath.Base(os.Args[0])
	}
	var cname *C.char
	if name != "" {
		cname = C.CString(name)
		defer C.free(unsafe.Pointer(cname))
	}
	C.vitra_win32_init(cname)
}

// ProgramName returns the AppUserModelID after init (empty before).
func (h *Host) ProgramName() string {
	h.ensureInit()
	p := C.vitra_get_program_name()
	if p == nil {
		return ""
	}
	return C.GoString(p)
}

// OS returns windows.
func (h *Host) OS() platform.OS { return platform.OSWindows }

// Features reports supported capabilities for this Win32 shell slice.
func (h *Host) Features() platform.FeatureSet {
	return platform.FeatureSet{
		platform.FeatureWindowCreate: {Feature: platform.FeatureWindowCreate, Available: true, Detail: "Win32 HWND shell"},
		platform.FeatureWindowNavigate: {
			Feature: platform.FeatureWindowNavigate, Available: true,
			Detail: "WebView2 Navigate (requires WebView2Loader.dll + Evergreen Runtime)",
		},
		platform.FeatureWebViewMessage: {
			Feature: platform.FeatureWebViewMessage, Available: true,
			Detail: "WebView2 ExecuteScript + chrome.webview.postMessage bridge",
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
		platform.FeatureDialogMessage: {
			Feature: platform.FeatureDialogMessage, Available: true,
			Detail: "Win32 MessageBox info/confirm",
		},
		platform.FeatureNotificationShow: {
			Feature: platform.FeatureNotificationShow, Available: true,
			Detail: "Shell_NotifyIcon balloon title+body",
		},
		platform.FeatureMenuBar: {
			Feature: platform.FeatureMenuBar, Available: true,
			Detail: "Win32 CreateMenu menubar; MenuItem.Shortcut as in-window HACCEL",
		},
		platform.FeatureTray: {
			Feature: platform.FeatureTray, Available: true,
			Detail: "Shell_NotifyIcon tray with TrackPopupMenu context menu",
		},
		platform.FeatureSingleInstance: {
			Feature: platform.FeatureSingleInstance, Available: true,
			Detail: "exclusive lock file; available without native WebView",
		},
		platform.FeatureGlobalShortcut: {
			Feature: platform.FeatureGlobalShortcut, Available: true,
			Detail: "RegisterHotKey OS-wide accelerators → SetActionHandler",
		},
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: true,
			Detail: "argv + socket handoff + HKCU Classes .reg URL-scheme registration",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: true,
			Detail: "WM_DROPFILES on the HWND (DragAcceptFiles)",
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
		platform.FeaturePathOpen: {
			Feature: platform.FeaturePathOpen, Available: true,
			Detail: "open absolute local paths with OS default handler",
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

// EnableDragDrop toggles file-drop acceptance on a window.
func (h *Host) EnableDragDrop(id domain.WindowID, enabled bool) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		en := C.int(0)
		if enabled {
			en = 1
		}
		C.vitra_win_set_drag_drop(w.ptr, en)
		errCh <- nil
	})
	return <-errCh
}

// InjectFileDrop synthesizes a file drop for tests/headless demos.
func (h *Host) InjectFileDrop(id domain.WindowID, paths []string) {
	if h.onDrop == nil {
		return
	}
	h.onDrop(id, append([]string(nil), paths...))
}

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

// Eval runs JavaScript in the given window via WebView2 ExecuteScript.
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
		if C.vitra_win_eval(w.ptr, cjs) == 0 {
			errCh <- h.err(platform.FeatureWebViewMessage)
			return
		}
		errCh <- nil
	})
	return <-errCh
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

// MessageDialog shows a native MessageBox info or confirm dialog.
func (h *Host) MessageDialog(title, message, kind string) (bool, error) {
	confirm := 0
	if strings.EqualFold(kind, "confirm") {
		confirm = 1
	}
	ch := make(chan bool, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(title)
		cmsg := C.CString(message)
		ok := C.vitra_message_dialog(ctitle, cmsg, C.int(confirm)) != 0
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cmsg))
		ch <- ok
	})
	return <-ch, nil
}

// ShowNotification displays a title+body desktop notification.
func (h *Host) ShowNotification(title, body string) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(title)
		cbody := C.CString(body)
		ok := C.vitra_show_notification(ctitle, cbody) != 0
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cbody))
		if !ok {
			errCh <- errors.New("show notification failed")
			return
		}
		errCh <- nil
	})
	return <-errCh
}

// SetMenuBar replaces the window menu bar with the given flat items.
func (h *Host) SetMenuBar(id domain.WindowID, items []platform.MenuItem) error {
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
			cshort := C.CString(it.Shortcut)
			C.vitra_win_add_menu_item(w.ptr, cmenu, cid, clabel, cshort)
			C.free(unsafe.Pointer(cmenu))
			C.free(unsafe.Pointer(cid))
			C.free(unsafe.Pointer(clabel))
			C.free(unsafe.Pointer(cshort))
		}
		errCh <- nil
	})
	return <-errCh
}

// ActivateMenuAccel fires an in-window menu accelerator (tests / demos).
func (h *Host) ActivateMenuAccel(id domain.WindowID, shortcut string) (bool, error) {
	type result struct {
		ok  bool
		err error
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
		cshort := C.CString(shortcut)
		defer C.free(unsafe.Pointer(cshort))
		ch <- result{ok: C.vitra_win_activate_accel(w.ptr, cshort) != 0}
	})
	got := <-ch
	return got.ok, got.err
}

// SetTray shows a notification-area tray entry with tooltip and optional context menu.
func (h *Host) SetTray(tooltip string, items []platform.MenuItem) error {
	done := make(chan struct{}, 1)
	h.dispatch(func() {
		h.ensureInit()
		ct := C.CString(tooltip)
		defer C.free(unsafe.Pointer(ct))
		C.vitra_tray_set(ct)
		C.vitra_tray_clear_menu()
		for _, it := range items {
			cid := C.CString(it.ID)
			clabel := C.CString(it.Label)
			C.vitra_tray_add_menu_item(cid, clabel)
			C.free(unsafe.Pointer(cid))
			C.free(unsafe.Pointer(clabel))
		}
		done <- struct{}{}
	})
	<-done
	return nil
}

// ClearTray hides the tray icon.
func (h *Host) ClearTray() {
	h.dispatch(func() { C.vitra_tray_clear() })
}

// RegisterGlobalShortcut binds an OS-wide accelerator to an action id.
func (h *Host) RegisterGlobalShortcut(accelerator, actionID string) error {
	if accelerator == "" || actionID == "" {
		return &domain.ErrValidation{Message: "accelerator and action id are required"}
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		ca := C.CString(accelerator)
		cid := C.CString(actionID)
		defer C.free(unsafe.Pointer(ca))
		defer C.free(unsafe.Pointer(cid))
		if C.vitra_register_hotkey(ca, cid) == 0 {
			errCh <- errors.New("failed to register global shortcut")
			return
		}
		errCh <- nil
	})
	return <-errCh
}

// UnregisterGlobalShortcut removes a previously registered accelerator.
func (h *Host) UnregisterGlobalShortcut(accelerator string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		ca := C.CString(accelerator)
		defer C.free(unsafe.Pointer(ca))
		if C.vitra_unregister_hotkey(ca) == 0 {
			errCh <- errors.New("global shortcut not found")
			return
		}
		errCh <- nil
	})
	return <-errCh
}

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
		h.replyOnUIThread(id, resp)
	}
}

func (h *Host) replyOnUIThread(id domain.WindowID, message []byte) {
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

//export goVitraDrop
func goVitraDrop(windowID, pathsJoined *C.char) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil || h.onDrop == nil {
		return
	}
	raw := C.GoString(pathsJoined)
	var paths []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) == 0 {
		return
	}
	h.onDrop(domain.WindowID(C.GoString(windowID)), paths)
}
