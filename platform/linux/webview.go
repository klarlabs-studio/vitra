//go:build linux && cgo && vitra_native

// Package linux provides a real WebKitGTK/GTK3 desktop host.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
package linux

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11
#include "native.h"
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

// Host is a WebKitGTK desktop host.
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
	programName string   // WM_CLASS / StartupWMClass; empty → filepath.Base(os.Args[0])
	jobs        sync.Map // uint64 -> func()
	jobSeq      uint64
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

// SetProgramName sets the GTK/GDK program name used for WM_CLASS so docks
// can match FreeDesktop StartupWMClass. Must be called before the first
// window/event-loop call. Empty keeps the default (basename of os.Args[0]).
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
	C.vitra_gtk_init(cname)
}

// ProgramName returns the GTK program name after init (empty before).
func (h *Host) ProgramName() string {
	h.ensureInit()
	p := C.vitra_get_prgname()
	if p == nil {
		return ""
	}
	return C.GoString(p)
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
		platform.FeatureDialogSave:     {Feature: platform.FeatureDialogSave, Available: true},
		platform.FeatureDialogMessage:  {Feature: platform.FeatureDialogMessage, Available: true},
		platform.FeatureDialogOpenDirectory: {
			Feature: platform.FeatureDialogOpenDirectory, Available: true,
			Detail: "GTK SELECT_FOLDER",
		},
		platform.FeatureNotificationShow: {Feature: platform.FeatureNotificationShow, Available: true, Detail: "org.freedesktop.Notifications"},
		platform.FeatureClipboard:        {Feature: platform.FeatureClipboard, Available: true},
		platform.FeatureMenuBar:          {Feature: platform.FeatureMenuBar, Available: true},
		platform.FeatureTray:             {Feature: platform.FeatureTray, Available: true},
		platform.FeatureSingleInstance:   {Feature: platform.FeatureSingleInstance, Available: true},
		platform.FeatureGlobalShortcut:   linuxGlobalShortcutFeature(),
		platform.FeatureDeepLink: {
			Feature: platform.FeatureDeepLink, Available: true,
			Detail: "argv + socket handoff + xdg URL-scheme registration",
		},
		platform.FeatureDragDrop: {
			Feature: platform.FeatureDragDrop, Available: true,
			Detail: "GTK URI file drops on the WebView",
		},
		platform.FeatureFileAssociation: {
			Feature: platform.FeatureFileAssociation, Available: true,
			Detail: "xdg MIME desktop file",
		},
		platform.FeatureWindowChrome: {
			Feature: platform.FeatureWindowChrome, Available: true,
			Detail: "GTK title, size, maximize, fullscreen, keep-above, minimize, hide, and icon",
		},
		platform.FeatureOpenURL: {
			Feature: platform.FeatureOpenURL, Available: true,
			Detail: "xdg-open for http(s)/mailto",
		},
		platform.FeaturePathOpen: {
			Feature: platform.FeaturePathOpen, Available: true,
			Detail: "open absolute local paths with OS default handler",
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

// SetDragDropHandler registers file-drop callbacks (absolute paths).
func (h *Host) SetDragDropHandler(fn func(windowID domain.WindowID, paths []string)) {
	h.onDrop = fn
}

// SetDestroyHandler registers callbacks when a native window is destroyed
// (e.g. titlebar close). The handler runs after the host map entry is removed.
func (h *Host) SetDestroyHandler(fn func(windowID domain.WindowID)) {
	h.onDestroy = fn
}

// EnableDragDrop toggles GTK URI drop targets on a window's WebView.
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

// ReadWindowChrome returns the window presentation GTK last applied.
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

// FocusWindow raises and activates a native window.
func (h *Host) FocusWindow(id domain.WindowID) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		C.vitra_win_focus(w.ptr)
		errCh <- nil
	})
	return <-errCh
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
		// Destroy emits synchronously. Drop the map entry first so the
		// destroy callback does not free the native window under h.mu.
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
func (h *Host) OpenFileDialog(opts platform.DialogFileOptions) (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(opts.Title)
		cdefault := C.CString(opts.DefaultPath)
		cfilters := C.CString(platform.EncodeFileFilters(opts.Filters))
		p := C.vitra_open_dialog(ctitle, cdefault, cfilters)
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cdefault))
		C.free(unsafe.Pointer(cfilters))
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.g_free(C.gpointer(p))
	})
	return <-ch, nil
}

// SaveFileDialog opens a native save-file chooser.
func (h *Host) SaveFileDialog(opts platform.DialogFileOptions) (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(opts.Title)
		cdefault := C.CString(opts.DefaultPath)
		cfilters := C.CString(platform.EncodeFileFilters(opts.Filters))
		p := C.vitra_save_dialog(ctitle, cdefault, cfilters)
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cdefault))
		C.free(unsafe.Pointer(cfilters))
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.g_free(C.gpointer(p))
	})
	return <-ch, nil
}

// OpenDirectoryDialog opens a native folder chooser.
func (h *Host) OpenDirectoryDialog(opts platform.DialogFileOptions) (string, error) {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(opts.Title)
		cdefault := C.CString(opts.DefaultPath)
		p := C.vitra_open_directory_dialog(ctitle, cdefault)
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cdefault))
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
		C.g_free(C.gpointer(p))
	})
	return <-ch, nil
}

// MessageDialog shows a native info or confirm dialog. kind is "info" or "confirm".
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

// MenuItem is an alias for the portable chrome menu entry.
type MenuItem = platform.MenuItem

// SetMenuBar replaces the application menu bar on the given window.
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

// ActivateMenuAccel fires an in-window menu accelerator (tests / headless demos).
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

// SetTray shows a status-icon tray entry with tooltip and optional context menu.
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

// RegisterGlobalShortcut binds an OS-wide accelerator on X11 (unsupported on Wayland).
func (h *Host) RegisterGlobalShortcut(accelerator, actionID string) error {
	if accelerator == "" || actionID == "" {
		return &domain.ErrValidation{Message: "accelerator and action id are required"}
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		if C.vitra_hotkey_supported() == 0 {
			errCh <- &platform.ErrUnsupported{
				Feature: platform.FeatureGlobalShortcut,
				OS:      platform.OSLinux,
				Detail:  "global shortcuts unsupported on Wayland; use MenuItem.Shortcut for in-window accelerators",
			}
			return
		}
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

// UnregisterGlobalShortcut removes a previously registered X11 accelerator.
func (h *Host) UnregisterGlobalShortcut(accelerator string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		if C.vitra_hotkey_supported() == 0 {
			errCh <- &platform.ErrUnsupported{
				Feature: platform.FeatureGlobalShortcut,
				OS:      platform.OSLinux,
				Detail:  "global shortcuts unsupported on Wayland; use MenuItem.Shortcut for in-window accelerators",
			}
			return
		}
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

// linuxGlobalShortcutFeature reports X11-only OS-wide hotkeys. Wayland stays unsupported.
func linuxGlobalShortcutFeature() platform.Support {
	if waylandSession() {
		return platform.Support{
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "global shortcuts are not reliable on Wayland; use in-window menu accelerators (MenuItem.Shortcut)",
		}
	}
	if os.Getenv("DISPLAY") == "" {
		return platform.Support{
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "global shortcuts require an X11 DISPLAY",
		}
	}
	return platform.Support{
		Feature: platform.FeatureGlobalShortcut, Available: true,
		Detail: "XGrabKey OS-wide accelerators on X11 → SetActionHandler (Wayland unsupported)",
	}
}

func waylandSession() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
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
		// Reply on this GTK thread directly. PostMessage/Eval → dispatch+wait
		// would deadlock the main loop.
		h.replyOnGTKThread(id, resp)
	}
}

func (h *Host) replyOnGTKThread(id domain.WindowID, message []byte) {
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
