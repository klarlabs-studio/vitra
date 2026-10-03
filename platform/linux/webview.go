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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

func init() { runtime.LockOSThread() }

// mainThread is the tid of the process's main thread. On Linux the main
// thread's tid is the process id.
var mainThread = os.Getpid()

// Host is a WebKitGTK desktop host.
type Host struct {
	mu          sync.Mutex
	windows     map[domain.WindowID]*nativeWindow
	origins     map[domain.WindowID]domain.Origin
	onInvoke    func(domain.WindowID, domain.Origin, []byte) []byte
	onMessage   func(domain.WindowID, string, []byte) []byte
	onNav       func(domain.WindowID, string) bool
	onAction    func(id string)
	onDrop      func(windowID domain.WindowID, paths []string)
	onDestroy   func(windowID domain.WindowID)
	trayMu      sync.Mutex
	trayBackend trayBackend
	looping     bool
	inited      bool
	programName string   // WM_CLASS / StartupWMClass; empty → filepath.Base(os.Args[0])
	jobs        sync.Map // uint64 -> func()
	jobSeq      uint64
	// uiThread is the OS thread (Linux tid) that owns GTK: the process's
	// main thread, which init locks the main goroutine to and Run is
	// documented to run on. Before Run, only calls from this thread run
	// inline; calls from any other thread wait for the loop. It is fixed in
	// New rather than taken from the first thread to call gtk_init:
	// ensureInit runs inside dispatched calls, so that would bless whichever
	// goroutine happened to call first.
	uiThread int

	// Wayland global shortcuts via the GlobalShortcuts portal: the bound set,
	// in registration order. portalMu serializes rebinding.
	portalMu    sync.Mutex
	portalBinds []portalBind
	// forcePortal routes global shortcuts through the portal even on X11
	// (tests run under xvfb against a fake portal).
	forcePortal bool
}

// portalBind is one shortcut bound through the GlobalShortcuts portal. The
// portal shortcut id is the action id, so Activated maps straight to it.
type portalBind struct {
	actionID    string
	accelerator string
	trigger     string
}

type nativeWindow struct{ ptr *C.VitraWin }

var (
	activeMu sync.Mutex
	active   *Host
)

// New constructs a Linux host.
func New() *Host {
	h := &Host{
		windows:  make(map[domain.WindowID]*nativeWindow),
		origins:  make(map[domain.WindowID]domain.Origin),
		uiThread: mainThread,
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

// ProgramName returns the GTK program name, initializing GTK on the UI
// thread first (see dispatch).
func (h *Host) ProgramName() string {
	ch := make(chan string, 1)
	h.dispatch(func() {
		h.ensureInit()
		p := C.vitra_get_prgname()
		if p == nil {
			ch <- ""
			return
		}
		ch <- C.GoString(p)
	})
	return <-ch
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
		platform.FeatureTray:             trayFeature(),
		platform.FeatureSingleInstance:   {Feature: platform.FeatureSingleInstance, Available: true},
		platform.FeatureGlobalShortcut:   h.globalShortcutFeature(),
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

// SetMessageHandler registers the IPC callback that also receives the URL of
// the document that sent each message. When set, it is used instead of the
// invoke handler.
func (h *Host) SetMessageHandler(fn func(windowID domain.WindowID, senderURL string, raw []byte) []byte) {
	h.onMessage = fn
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

// BlurWindow resigns key focus and lowers a native window.
func (h *Host) BlurWindow(id domain.WindowID) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		w, ok := h.windows[id]
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		C.vitra_win_blur(w.ptr)
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
		h.mu.Unlock()
		// Destroy emits synchronously. Drop the map entry first so the
		// destroy callback does not free the native window under h.mu.
		C.vitra_win_close(ptr)
		C.vitra_win_free(ptr)
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

// Host can select several files in one open dialog.
var _ platform.MultiFileOpener = (*Host)(nil)

// OpenFilesDialog opens a native file chooser that can select several files
// (platform.MultiFileOpener). It returns nil when the user cancels.
func (h *Host) OpenFilesDialog(opts platform.DialogFileOptions) ([]string, error) {
	ch := make(chan []string, 1)
	h.dispatch(func() {
		h.ensureInit()
		ctitle := C.CString(opts.Title)
		cdefault := C.CString(opts.DefaultPath)
		cfilters := C.CString(platform.EncodeFileFilters(opts.Filters))
		var n C.int
		p := C.vitra_open_dialog_multi(ctitle, cdefault, cfilters, &n)
		C.free(unsafe.Pointer(ctitle))
		C.free(unsafe.Pointer(cdefault))
		C.free(unsafe.Pointer(cfilters))
		if p == nil {
			ch <- nil
			return
		}
		paths := platform.DecodePathList(C.GoBytes(unsafe.Pointer(p), n))
		C.g_free(C.gpointer(p))
		ch <- paths
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

// trayBackend is the tray implementation currently showing the icon.
type trayBackend string

const (
	trayNone trayBackend = ""
	traySNI  trayBackend = "sni" // StatusNotifierItem + dbusmenu over D-Bus
	trayGTK  trayBackend = "gtk" // GtkStatusIcon (XEmbed) fallback
)

// trayFeature reports which tray protocol SetTray will use.
func trayFeature() platform.Support {
	if C.vitra_sni_available() != 0 {
		return platform.Support{
			Feature: platform.FeatureTray, Available: true,
			Detail: "StatusNotifierItem + dbusmenu via org.kde.StatusNotifierWatcher " +
				"(KDE Plasma, GNOME with the AppIndicator extension, XFCE, Cinnamon, MATE, LXQt, …)",
		}
	}
	return platform.Support{
		Feature: platform.FeatureTray, Available: true,
		Detail: "GtkStatusIcon (XEmbed system tray): no StatusNotifierWatcher on the session bus; " +
			"stock GNOME and Wayland-only panels show nothing",
	}
}

// SetTray shows a tray icon with a tooltip and an optional context menu. It
// uses StatusNotifierItem when a StatusNotifierWatcher is on the session bus
// and falls back to GtkStatusIcon otherwise. Left click emits
// "tray.activate"; menu items emit their ID.
func (h *Host) SetTray(tooltip string, items []platform.MenuItem) error {
	h.trayMu.Lock()
	defer h.trayMu.Unlock()
	if setTraySNI(tooltip, items) {
		if h.trayBackend == trayGTK {
			h.dispatch(func() { C.vitra_tray_clear() })
		}
		h.trayBackend = traySNI
		return nil
	}
	if h.trayBackend == traySNI {
		C.vitra_sni_clear() // the watcher went away
	}
	h.trayBackend = trayGTK
	return h.setTrayGTK(tooltip, items)
}

// setTraySNI shows the tray through StatusNotifierItem; false when no
// StatusNotifierWatcher takes the item.
func setTraySNI(tooltip string, items []platform.MenuItem) bool {
	n := len(items)
	ids := make([]*C.char, n)
	labels := make([]*C.char, n)
	for i, it := range items {
		ids[i] = C.CString(it.ID)
		labels[i] = C.CString(it.Label)
	}
	defer func() {
		for i := range items {
			C.free(unsafe.Pointer(ids[i]))
			C.free(unsafe.Pointer(labels[i]))
		}
	}()
	var idp, lp **C.char
	if n > 0 {
		idp, lp = &ids[0], &labels[0]
	}
	ct := C.CString(tooltip)
	defer C.free(unsafe.Pointer(ct))
	return C.vitra_sni_set(ct, idp, lp, C.int(n)) != 0
}

func (h *Host) setTrayGTK(tooltip string, items []platform.MenuItem) error {
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
	h.trayMu.Lock()
	defer h.trayMu.Unlock()
	switch h.trayBackend {
	case traySNI:
		C.vitra_sni_clear()
	case trayGTK:
		h.dispatch(func() { C.vitra_tray_clear() })
	}
	h.trayBackend = trayNone
}

// RegisterGlobalShortcut binds an OS-wide accelerator: XGrabKey on X11, the
// org.freedesktop.portal.GlobalShortcuts portal on Wayland. Through the
// portal the accelerator is only the preferred trigger: the desktop may ask
// the user to approve the binding or pick another key, and a refusal is
// returned as an error. Without the portal it returns platform.ErrUnsupported.
func (h *Host) RegisterGlobalShortcut(accelerator, actionID string) error {
	if accelerator == "" || actionID == "" {
		return &domain.ErrValidation{Message: "accelerator and action id are required"}
	}
	if h.shortcutsViaPortal() {
		return h.portalRegister(accelerator, actionID)
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		if C.vitra_hotkey_supported() == 0 {
			errCh <- errPortalShortcutsUnsupported()
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

// UnregisterGlobalShortcut removes a previously registered accelerator. The
// portal has no unbind, so on Wayland the remaining shortcuts are rebound on
// a fresh portal session (the desktop may confirm them with the user again).
func (h *Host) UnregisterGlobalShortcut(accelerator string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	if h.shortcutsViaPortal() {
		return h.portalUnregister(accelerator)
	}
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.ensureInit()
		if C.vitra_hotkey_supported() == 0 {
			errCh <- errPortalShortcutsUnsupported()
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

// shortcutsViaPortal reports whether global shortcuts go through the portal:
// in a Wayland session, or whenever GDK is not on an X11 display.
func (h *Host) shortcutsViaPortal() bool {
	if h.forcePortal || waylandSession() {
		return true
	}
	ch := make(chan bool, 1)
	h.dispatch(func() {
		h.ensureInit()
		ch <- C.vitra_hotkey_supported() == 0
	})
	return <-ch
}

func errPortalShortcutsUnsupported() error {
	return &platform.ErrUnsupported{
		Feature: platform.FeatureGlobalShortcut,
		OS:      platform.OSLinux,
		Detail: "global shortcuts on Wayland need the org.freedesktop.portal.GlobalShortcuts portal " +
			"(e.g. GNOME 48+, KDE Plasma 6), which is not running; use MenuItem.Shortcut for in-window accelerators",
	}
}

func (h *Host) portalRegister(accelerator, actionID string) error {
	if C.vitra_gs_portal_version() == 0 {
		return errPortalShortcutsUnsupported()
	}
	trigger, err := portalTrigger(accelerator)
	if err != nil {
		return &domain.ErrValidation{Message: err.Error()}
	}
	h.portalMu.Lock()
	defer h.portalMu.Unlock()
	// One portal shortcut per action id (its portal id), and one action per
	// accelerator as on X11: a new registration replaces either.
	next := make([]portalBind, 0, len(h.portalBinds)+1)
	for _, b := range h.portalBinds {
		if b.actionID != actionID && b.accelerator != accelerator {
			next = append(next, b)
		}
	}
	next = append(next, portalBind{actionID: actionID, accelerator: accelerator, trigger: trigger})
	if err := portalBindAll(next); err != nil {
		return err
	}
	h.portalBinds = next
	return nil
}

func (h *Host) portalUnregister(accelerator string) error {
	h.portalMu.Lock()
	defer h.portalMu.Unlock()
	next := make([]portalBind, 0, len(h.portalBinds))
	for _, b := range h.portalBinds {
		if b.accelerator != accelerator {
			next = append(next, b)
		}
	}
	if len(next) == len(h.portalBinds) {
		return errors.New("global shortcut not found")
	}
	if len(next) == 0 {
		C.vitra_gs_portal_close()
	} else if err := portalBindAll(next); err != nil {
		return err
	}
	h.portalBinds = next
	return nil
}

// portalBindAll replaces the portal session's shortcuts with binds. It blocks
// until the portal (and possibly the user) answers, so it never runs on the
// GTK thread.
func portalBindAll(binds []portalBind) error {
	n := len(binds)
	ids := make([]*C.char, n)
	triggers := make([]*C.char, n)
	for i, b := range binds {
		ids[i] = C.CString(b.actionID)
		triggers[i] = C.CString(b.trigger)
	}
	defer func() {
		for i := range binds {
			C.free(unsafe.Pointer(ids[i]))
			C.free(unsafe.Pointer(triggers[i]))
		}
	}()
	var idp, trp **C.char
	if n > 0 {
		idp, trp = &ids[0], &triggers[0]
	}
	var cerr *C.char
	code := C.vitra_gs_portal_bind(idp, trp, C.int(n), &cerr)
	msg := ""
	if cerr != nil {
		msg = C.GoString(cerr)
		C.g_free(C.gpointer(cerr))
	}
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("global shortcut not granted: the user cancelled or refused it (%s)", msg)
	default:
		return fmt.Errorf("global shortcuts portal: %s", msg)
	}
}

// globalShortcutFeature reports OS-wide hotkeys: XGrabKey on X11, and on
// Wayland the GlobalShortcuts portal, available only while it is running.
func (h *Host) globalShortcutFeature() platform.Support {
	if h.forcePortal || waylandSession() {
		if v := C.vitra_gs_portal_version(); v > 0 {
			return platform.Support{
				Feature: platform.FeatureGlobalShortcut, Available: true,
				Detail: fmt.Sprintf("org.freedesktop.portal.GlobalShortcuts v%d on Wayland; "+
					"the desktop may ask the user to approve each binding", int(v)),
			}
		}
		return platform.Support{
			Feature: platform.FeatureGlobalShortcut, Available: false,
			Detail: "Wayland without the org.freedesktop.portal.GlobalShortcuts portal; use in-window menu accelerators (MenuItem.Shortcut)",
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
		Detail: "XGrabKey OS-wide accelerators on X11 → SetActionHandler (Wayland: GlobalShortcuts portal)",
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

// dispatch runs fn on the UI thread. It is queued while the loop runs, and
// also before the loop starts when the caller is not the UI thread:
// g_idle_add attaches to the default main context, which gtk_main drains once
// Run starts the loop. Only a call that is already on the UI thread before
// Run runs inline, so gtk_init and every widget call stay on that thread.
func (h *Host) dispatch(fn func()) {
	if h.enqueue(fn) {
		return
	}
	if !h.onUIThread() {
		h.post(fn)
		return
	}
	fn()
}

// onUIThread reports whether the caller runs on the thread that owns GTK.
func (h *Host) onUIThread() bool { return syscall.Gettid() == h.uiThread }

// enqueue queues fn for the UI thread and reports whether the loop is running
// to take it.
func (h *Host) enqueue(fn func()) bool {
	h.mu.Lock()
	looping := h.looping
	h.mu.Unlock()
	if !looping {
		return false
	}
	h.post(fn)
	return true
}

// post queues fn on the default main context, whether or not the loop runs
// yet.
func (h *Host) post(fn func()) {
	h.mu.Lock()
	h.jobSeq++
	id := h.jobSeq
	h.mu.Unlock()
	h.jobs.Store(id, fn)
	C.vitra_idle_add(C.ulonglong(id))
}

//export goVitraIdle
func goVitraIdle(cid C.ulonglong) {
	id := uint64(cid)
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
func goVitraMessage(windowID, msg, sender *C.char) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil || (h.onInvoke == nil && h.onMessage == nil) {
		return
	}
	id := domain.WindowID(C.GoString(windowID))
	h.mu.Lock()
	origin := h.origins[id]
	h.mu.Unlock()
	if origin == "" {
		origin = domain.OriginPackagedLocal
	}
	// Copy the message out of C memory before returning, then run the invoke
	// off the UI thread: executors call host methods (clipboard, dialogs,
	// Eval, Emit) that queue work to this thread and wait for it, so running
	// them here would deadlock the loop, and a slow command would freeze the
	// UI. The reply hops back to the UI thread.
	payload := []byte(C.GoString(msg))
	go h.handleMessage(id, origin, C.GoString(sender), payload)
}

func (h *Host) handleMessage(id domain.WindowID, origin domain.Origin, sender string, payload []byte) {
	var resp []byte
	if h.onMessage != nil {
		resp = h.onMessage(id, sender, payload)
	} else {
		resp = h.onInvoke(id, origin, payload)
	}
	if len(resp) == 0 {
		return
	}
	// If the loop has already stopped there is no page left to answer.
	h.enqueue(func() { h.replyOnGTKThread(id, resp) })
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
	h.mu.Unlock()
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
	// Handlers typically emit to the frontend, which waits on this thread;
	// run them off it (see goVitraMessage).
	go h.onAction(C.GoString(actionID))
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
	// Off the UI thread, like goVitraAction.
	go h.onDrop(domain.WindowID(C.GoString(windowID)), paths)
}

// SetDevTools enables or disables the web inspector for windows opened
// afterwards. It is off unless enabled; app.Run sets it before opening
// windows.
func (h *Host) SetDevTools(enabled bool) {
	v := C.int(0)
	if enabled {
		v = 1
	}
	C.vitra_set_devtools(v)
}
