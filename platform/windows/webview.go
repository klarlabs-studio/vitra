//go:build windows && cgo && vitra_native

// Package windows provides a Win32 + WebView2 desktop host.
// Build with: CGO_ENABLED=1 go build -tags vitra_native
//
// Requires WebView2Loader.dll and the Evergreen WebView2 Runtime at runtime for
// Navigate/Eval/message. The HWND shell still opens when the loader is absent.
package windows

/*
#cgo LDFLAGS: -luser32 -lgdi32 -lcomdlg32 -lshell32 -lole32 -luuid
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

// init keeps the main goroutine on the main thread and remembers that thread.
func init() {
	runtime.LockOSThread()
	mainThread = currentThread()
}

// mainThread is the id of the process's main thread, the UI thread: it owns
// the host's Win32 windows and Run is documented to run on it. Before Run,
// only calls from this thread run inline; calls from any other thread are
// held until Run starts the loop.
var mainThread uint32

// currentThread returns the calling OS thread's id.
func currentThread() uint32 { return uint32(C.vitra_current_thread_id()) }

// Host is a Win32 desktop host scaffold.
type Host struct {
	mu          sync.Mutex
	windows     map[domain.WindowID]*nativeWindow
	origins     map[domain.WindowID]domain.Origin
	onInvoke    func(domain.WindowID, domain.Origin, []byte) []byte
	onMessage   func(domain.WindowID, string, []byte) []byte
	onNav       func(domain.WindowID, string) bool
	onAction    func(id string)
	onTrayClick func(platform.TrayClick)
	onDrop      func(windowID domain.WindowID, paths []string)
	onDestroy   func(windowID domain.WindowID)
	accessory   bool // windows get no taskbar button (under mu)
	looping     bool
	inited      bool
	programName string
	jobs        sync.Map
	jobSeq      uint64
	pending     []uint64 // jobs from other threads before Run, in order
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
		platform.FeatureDialogOpenDirectory: {
			Feature: platform.FeatureDialogOpenDirectory, Available: true,
			Detail: "IFileOpenDialog FOS_PICKFOLDERS",
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
		platform.FeatureTrayTitle: {
			Feature: platform.FeatureTrayTitle, Available: false,
			Detail: "the notification area shows icons only: the title is shown in the tooltip",
		},
		platform.FeatureTrayIcon: {
			Feature: platform.FeatureTrayIcon, Available: true,
			Detail: "PNG via CreateIconFromResourceEx at the small icon size",
		},
		platform.FeaturePresentation: {
			Feature: platform.FeaturePresentation, Available: true,
			Detail: "accessory windows are owned by a hidden window, so they get no taskbar button",
		},
		platform.FeatureTrayAnchor: {
			Feature: platform.FeatureTrayAnchor, Available: true,
			Detail: "Shell_NotifyIconGetRect, in screen pixels",
		},
		platform.FeatureWindowPanel: {
			Feature: platform.FeatureWindowPanel, Available: true,
			Detail: "borderless topmost tool window placed by the notification icon, in the monitor's work area",
		},
		platform.FeatureLoginItem: {
			Feature: platform.FeatureLoginItem, Available: runtime.GOOS == "windows",
			Detail: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run value; available without native WebView`,
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

// SetDestroyHandler registers callbacks when a native window is destroyed.
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
		var panel C.int
		if spec.Kind == platform.WindowKindPanel {
			panel = 1
		}
		ptr := C.vitra_win_new(cid, ctitle, C.int(spec.Width), C.int(spec.Height), curi, cjs, panel)
		if ptr == nil {
			errCh <- errors.New("failed to create Win32 window")
			return
		}
		if h.accessory {
			C.vitra_win_set_skip_taskbar(ptr, 1)
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
		C.vitra_win_close(ptr)
		C.vitra_win_free(ptr)
		errCh <- nil
	})
	return <-errCh
}

// OpenFileDialog opens a native file chooser (GetOpenFileName).
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
		C.free(unsafe.Pointer(p))
	})
	return <-ch, nil
}

// Host can select several files in one open dialog.
var _ platform.MultiFileOpener = (*Host)(nil)

// OpenFilesDialog opens an IFileOpenDialog that can select several files
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
		C.free(unsafe.Pointer(p))
		ch <- paths
	})
	return <-ch, nil
}

// SaveFileDialog opens a native save-file chooser (GetSaveFileName).
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
		C.free(unsafe.Pointer(p))
	})
	return <-ch, nil
}

// OpenDirectoryDialog opens a native folder chooser (IFileOpenDialog).
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
			C.vitra_win_add_menu_item(w.ptr, cmenu, cid, clabel, cshort, menuFlags(it))
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

// SetPresentation keeps an accessory app's windows off the taskbar, now and
// for windows opened later.
func (h *Host) SetPresentation(p platform.Presentation) error {
	var skip C.int
	switch p {
	case platform.PresentationRegular:
	case platform.PresentationAccessory:
		skip = 1
	default:
		return &domain.ErrValidation{Message: "unknown presentation " + string(p)}
	}
	done := make(chan struct{})
	h.dispatch(func() {
		h.ensureInit()
		h.mu.Lock()
		defer h.mu.Unlock()
		h.accessory = skip != 0
		for _, w := range h.windows {
			C.vitra_win_set_skip_taskbar(w.ptr, skip)
		}
		close(done)
	})
	<-done
	return nil
}

// SetTray shows a notification-area tray entry: icon, tooltip and context
// menu. The notification area shows no text, so the title joins the tooltip.
func (h *Host) SetTray(spec platform.TraySpec) error {
	done := make(chan struct{}, 1)
	h.dispatch(func() {
		h.ensureInit()
		ct := C.CString(spec.TooltipWithTitle())
		defer C.free(unsafe.Pointer(ct))
		var icon unsafe.Pointer
		if len(spec.Icon) > 0 {
			icon = C.CBytes(spec.Icon)
			defer C.free(icon)
		}
		var clicks C.int
		if spec.ClickActivates {
			clicks = 1
		}
		C.vitra_tray_set(ct, icon, C.int(len(spec.Icon)), clicks)
		C.vitra_tray_clear_menu()
		for _, it := range spec.Items {
			cid := C.CString(it.ID)
			clabel := C.CString(it.Label)
			C.vitra_tray_add_menu_item(cid, clabel, menuFlags(it))
			C.free(unsafe.Pointer(cid))
			C.free(unsafe.Pointer(clabel))
		}
		done <- struct{}{}
	})
	<-done
	return nil
}

// ShowPanel places a panel window under the anchor (centered without one),
// keeps it in the anchor monitor's work area, and shows it.
func (h *Host) ShowPanel(id domain.WindowID, anchor platform.Rect, hasAnchor bool) error {
	var has C.int
	if hasAnchor {
		has = 1
	}
	return h.onPanel(id, func(w *C.VitraWin) C.int {
		return C.vitra_panel_show(w, C.int(anchor.X), C.int(anchor.Y), C.int(anchor.Width), C.int(anchor.Height), has)
	})
}

// HidePanel hides a panel window.
func (h *Host) HidePanel(id domain.WindowID) error {
	return h.onPanel(id, func(w *C.VitraWin) C.int { return C.vitra_panel_hide(w) })
}

// PanelShown reports whether a panel is shown, or hid itself on losing
// focus a moment ago.
func (h *Host) PanelShown(id domain.WindowID) (bool, error) {
	var shown bool
	err := h.onPanel(id, func(w *C.VitraWin) C.int {
		r := C.vitra_panel_shown(w)
		shown = r == 1
		if r < 0 {
			return 0
		}
		return 1
	})
	return shown, err
}

// onPanel runs fn on the UI thread with the panel window id; fn returns 0
// when the window is not a panel.
func (h *Host) onPanel(id domain.WindowID, fn func(*C.VitraWin) C.int) error {
	errCh := make(chan error, 1)
	h.dispatch(func() {
		h.mu.Lock()
		w, ok := h.windows[id]
		h.mu.Unlock()
		if !ok {
			errCh <- &domain.ErrNotFound{Entity: "window", ID: string(id)}
			return
		}
		if fn(w.ptr) == 0 {
			errCh <- &domain.ErrValidation{Message: "window " + string(id) + " is not a panel"}
			return
		}
		errCh <- nil
	})
	return <-errCh
}

// SetTrayClickHandler registers fn for left clicks on a tray whose spec
// sets ClickActivates.
func (h *Host) SetTrayClickHandler(fn func(platform.TrayClick)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onTrayClick = fn
}

// TrayAnchor returns the notification icon's rectangle in screen pixels.
func (h *Host) TrayAnchor() (platform.Rect, error) {
	type result struct {
		r  platform.Rect
		ok bool
	}
	ch := make(chan result, 1)
	h.dispatch(func() {
		var x, y, w, hgt C.int
		ok := C.vitra_tray_anchor(&x, &y, &w, &hgt) != 0
		ch <- result{platform.Rect{X: int(x), Y: int(y), Width: int(w), Height: int(hgt)}, ok}
	})
	got := <-ch
	if !got.ok {
		return platform.Rect{}, &platform.ErrUnsupported{Feature: platform.FeatureTrayAnchor, OS: platform.OSWindows, Detail: "no notification icon is shown"}
	}
	return got.r, nil
}

//export goVitraTrayClick
func goVitraTrayClick(x, y, w, hgt, has C.int) {
	activeMu.Lock()
	h := active
	activeMu.Unlock()
	if h == nil {
		return
	}
	h.mu.Lock()
	fn := h.onTrayClick
	h.mu.Unlock()
	if fn == nil {
		return
	}
	click := platform.TrayClick{HasAnchor: has != 0}
	if click.HasAnchor {
		click.Anchor = platform.Rect{X: int(x), Y: int(y), Width: int(w), Height: int(hgt)}
	}
	go fn(click)
}

// menuFlags packs a menu item's separator, disabled and checked states.
func menuFlags(it platform.MenuItem) C.int {
	var f C.int
	if it.Separator {
		f |= C.VITRA_MENU_SEPARATOR
	}
	if it.Disabled {
		f |= C.VITRA_MENU_DISABLED
	}
	if it.Checked {
		f |= C.VITRA_MENU_CHECKED
	}
	return f
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

// Run runs the Win32 message loop (blocking). Must be called from the main OS thread.
func (h *Host) Run() error {
	h.ensureInit()
	// The job window must exist before looping is set: from then on, calls
	// from other threads are posted to it.
	C.vitra_win32_bind_ui_thread()
	h.mu.Lock()
	h.looping = true
	pending := h.pending
	h.pending = nil
	// Posted under the lock, so they run ahead of anything queued from now on.
	for _, id := range pending {
		C.vitra_idle_post(C.ulonglong(id))
	}
	h.mu.Unlock()
	C.vitra_win32_main()
	return nil
}

// Quit leaves the Win32 message loop.
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

// dispatch runs fn on the UI thread. It is queued while the loop runs, and
// also before the loop starts when the caller is not the UI thread: Win32
// windows belong to the thread that creates them, and only the UI thread's
// message loop pumps them. Such a call is held until Run posts it to that
// loop. Only a call that is already on the UI thread before Run runs inline.
func (h *Host) dispatch(fn func()) {
	h.mu.Lock()
	switch {
	case h.looping:
		id := h.addJobLocked(fn)
		h.mu.Unlock()
		C.vitra_idle_add(C.ulonglong(id))
	case currentThread() != mainThread:
		h.pending = append(h.pending, h.addJobLocked(fn))
		h.mu.Unlock()
	default:
		h.mu.Unlock()
		fn()
	}
}

// enqueue queues fn for the UI thread and reports whether the loop is running
// to take it.
func (h *Host) enqueue(fn func()) bool {
	h.mu.Lock()
	if !h.looping {
		h.mu.Unlock()
		return false
	}
	id := h.addJobLocked(fn)
	h.mu.Unlock()
	C.vitra_idle_add(C.ulonglong(id))
	return true
}

// addJobLocked stores fn under a new job id. h.mu must be held.
func (h *Host) addJobLocked(fn func()) uint64 {
	h.jobSeq++
	id := h.jobSeq
	h.jobs.Store(id, fn)
	return id
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
	h.enqueue(func() { h.replyOnUIThread(id, resp) })
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
