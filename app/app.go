// Package app is the competitive desktop application runtime that binds the
// secure Vitra kernel to a native WebView host.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/bridge"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/ipc"
	"go.klarlabs.de/vitra/platform"
)

// DesktopHost is the native surface required to run a Vitra desktop app.
type DesktopHost interface {
	platform.Host
	Open(spec platform.WindowSpec, uri, preload string) error
	SetInvokeHandler(fn func(windowID domain.WindowID, origin domain.Origin, raw []byte) []byte)
	SetNavPolicy(fn func(windowID domain.WindowID, uri string) bool)
	SetActionHandler(fn func(id string))
	SetDragDropHandler(fn func(windowID domain.WindowID, paths []string))
	SetDestroyHandler(fn func(windowID domain.WindowID))
	EnableDragDrop(id domain.WindowID, enabled bool) error
	Eval(id domain.WindowID, js string) error
	ClipboardGet() (string, error)
	ClipboardSet(text string) error
	OpenFileDialog() (string, error)
	SaveFileDialog() (string, error)
	OpenDirectoryDialog() (string, error)
	MessageDialog(title, message, kind string) (bool, error)
	ShowNotification(title, body string) error
	SetMenuBar(id domain.WindowID, items []platform.MenuItem) error
	SetTray(tooltip string, items []platform.MenuItem) error
	ClearTray()
	TrySingleInstance(appID string) (held bool, release func(), err error)
	StartDeepLinkBridge(appID string, onLink func(raw string)) (stop func(), err error)
	ForwardToPrimary(appID string, urls []string) (ok bool, err error)
	RegisterURLScheme(scheme, appID, execPath string) error
	RegisterFileAssociations(appID, execPath, name string, mimeTypes []string) error
	InjectFileDrop(id domain.WindowID, paths []string)
	ApplyWindowChrome(id domain.WindowID, chrome platform.WindowChrome) error
	ReadWindowChrome(id domain.WindowID) (platform.WindowChrome, error)
	OpenURL(ctx context.Context, rawURL string) error
	OpenPath(ctx context.Context, path string) error
	RegisterGlobalShortcut(accelerator, actionID string) error
	UnregisterGlobalShortcut(accelerator string) error
	Run() error
	Quit()
}

// WindowOptions configures an application window.
type WindowOptions struct {
	ID     domain.WindowID
	Title  string
	Width  int
	Height int
	Path   string // URL path served from Assets, default "/"
}

// Options configures App.
type Options struct {
	AppID   domain.AppID
	Title   string
	Assets  fs.FS
	Host    DesktopHost
	Runtime *vitra.Runtime
	Window  WindowOptions
}

// App is a runnable desktop application.
type App struct {
	opts    Options
	rt      *vitra.Runtime
	host    DesktopHost
	server  *http.Server
	addr    string
	mu      sync.Mutex
	windows map[domain.WindowID]WindowOptions
}

// New constructs an App. Host must be a native desktop host (e.g. linux.New()).
func New(opts Options) (*App, error) {
	if opts.AppID == "" {
		return nil, &domain.ErrValidation{Message: "app id is required"}
	}
	if opts.Host == nil {
		return nil, &domain.ErrValidation{Message: "desktop host is required"}
	}
	rt := opts.Runtime
	if rt == nil {
		var err error
		rt, err = vitra.New(vitra.Config{AppID: opts.AppID})
		if err != nil {
			return nil, err
		}
	}
	if opts.Window.ID == "" {
		opts.Window.ID = "main"
	}
	if opts.Window.Title == "" {
		opts.Window.Title = opts.Title
		if opts.Window.Title == "" {
			opts.Window.Title = string(opts.AppID)
		}
	}
	if opts.Window.Path == "" {
		opts.Window.Path = "/"
	}
	return &App{
		opts:    opts,
		rt:      rt,
		host:    opts.Host,
		windows: make(map[domain.WindowID]WindowOptions),
	}, nil
}

// Runtime returns the secure kernel.
func (a *App) Runtime() *vitra.Runtime { return a.rt }

// Run serves frontend assets, opens the primary window, and blocks on the UI loop.
func (a *App) Run(ctx context.Context) error {
	if a.opts.Assets == nil {
		return errors.New("frontend assets are required")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	a.addr = "http://" + ln.Addr().String()
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(a.opts.Assets)))
	a.server = &http.Server{Handler: mux}
	go func() { _ = a.server.Serve(ln) }()
	defer func() {
		_ = a.server.Close()
	}()

	a.host.SetInvokeHandler(a.handleInvoke)
	a.host.SetNavPolicy(a.allowNav)
	a.host.SetDestroyHandler(a.onNativeDestroy)

	if err := a.OpenWindow(ctx, a.opts.Window); err != nil {
		return err
	}
	return a.host.Run()
}

// OpenWindow opens an additional (or the primary) window against the asset server.
// The asset server must already be listening (after Run starts, or during Run setup).
func (a *App) OpenWindow(ctx context.Context, opts WindowOptions) error {
	if a.addr == "" {
		return errors.New("asset server is not running; call Run first")
	}
	if opts.ID == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if opts.Title == "" {
		opts.Title = a.opts.Title
		if opts.Title == "" {
			opts.Title = string(a.opts.AppID)
		}
	}
	if opts.Path == "" {
		opts.Path = "/"
	}
	if opts.Width <= 0 {
		opts.Width = 960
	}
	if opts.Height <= 0 {
		opts.Height = 640
	}

	a.mu.Lock()
	if _, exists := a.windows[opts.ID]; exists {
		a.mu.Unlock()
		return fmt.Errorf("window already open: %s", opts.ID)
	}
	a.mu.Unlock()

	if _, err := a.rt.OpenWindow(ctx, opts.ID, domain.OriginPackagedLocal); err != nil {
		return err
	}
	uri := strings.TrimRight(a.addr, "/") + opts.Path
	spec := platform.WindowSpec{
		ID:     opts.ID,
		Title:  opts.Title,
		Origin: domain.OriginPackagedLocal,
		Width:  opts.Width,
		Height: opts.Height,
	}
	if err := a.host.Open(spec, uri, bridge.PreloadJS); err != nil {
		_ = a.rt.CloseWindow(ctx, opts.ID)
		return err
	}
	a.mu.Lock()
	a.windows[opts.ID] = opts
	a.mu.Unlock()
	return nil
}

// CloseWindow closes a window. Closing the last open window quits the app.
func (a *App) CloseWindow(ctx context.Context, id domain.WindowID) error {
	if id == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	a.mu.Lock()
	if _, ok := a.windows[id]; !ok {
		a.mu.Unlock()
		return &domain.ErrNotFound{Entity: "window", ID: string(id)}
	}
	delete(a.windows, id)
	remaining := len(a.windows)
	a.mu.Unlock()

	hostErr := a.host.CloseWindow(ctx, id)
	rtErr := a.rt.CloseWindow(ctx, id)
	if remaining == 0 {
		a.host.Quit()
	}
	if hostErr != nil {
		return hostErr
	}
	return rtErr
}

// onNativeDestroy syncs App/Runtime when the user closes a GTK window (titlebar).
// API CloseWindow already drops the id before host.CloseWindow, so this is a
// no-op when the destroy was driven by CloseWindow (avoids double Quit).
func (a *App) onNativeDestroy(id domain.WindowID) {
	a.mu.Lock()
	_, tracked := a.windows[id]
	if tracked {
		delete(a.windows, id)
	}
	remaining := len(a.windows)
	a.mu.Unlock()
	if !tracked {
		return
	}
	_ = a.rt.CloseWindow(context.Background(), id)
	if remaining == 0 {
		a.host.Quit()
	}
}

// Windows returns a snapshot of open window ids.
func (a *App) Windows() []domain.WindowID {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.WindowID, 0, len(a.windows))
	for id := range a.windows {
		out = append(out, id)
	}
	return out
}

// Quit requests application shutdown.
func (a *App) Quit() {
	a.host.Quit()
}

type eventMsg struct {
	Type    string `json:"type"`
	Event   string `json:"event"`
	Payload any    `json:"payload,omitempty"`
}

// Emit pushes a host→frontend event to every open window subscribed to name.
func (a *App) Emit(ctx context.Context, name domain.EventName, payload any) error {
	deliveries, err := a.rt.EmitEvent(name, payload)
	if err != nil {
		return err
	}
	var first error
	for _, d := range deliveries {
		msg := mustJSON(eventMsg{Type: "event", Event: string(d.Event.Name), Payload: d.Event.Payload})
		if err := a.host.PostMessage(ctx, d.Window, msg); err != nil && first == nil {
			first = err
		}
	}
	return first
}

type replyMsg struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	Code   string `json:"code,omitempty"`
}

func (a *App) handleInvoke(windowID domain.WindowID, origin domain.Origin, raw []byte) []byte {
	req, id, err := ipc.Bridge{Host: ipc.HostIdentity{Window: windowID, Origin: origin}}.DecodeInvoke(raw)
	if err != nil {
		return mustJSON(replyMsg{ID: id, OK: false, Error: err.Error(), Code: "bad_request"})
	}
	res, err := a.rt.Invoke(context.Background(), req)
	if err != nil {
		code := ipc.DenialCode(err)
		return mustJSON(replyMsg{ID: id, OK: false, Error: err.Error(), Code: code})
	}
	return mustJSON(replyMsg{ID: id, OK: true, Result: res.Output})
}

func (a *App) allowNav(windowID domain.WindowID, uri string) bool {
	// Allow only the local asset server and about:blank. Everything else is
	// external and must not keep privileged bridge access.
	if uri == "about:blank" || strings.HasPrefix(uri, a.addr) {
		return true
	}
	return false
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"ok":false,"error":"encode"}`)
	}
	return b
}

// Addr returns the local asset server address after Run starts listening.
func (a *App) Addr() string { return a.addr }

// DebugString summarizes the running app for doctor/inspect.
func (a *App) DebugString() string {
	a.mu.Lock()
	n := len(a.windows)
	a.mu.Unlock()
	return fmt.Sprintf("app=%s addr=%s windows=%d primary=%s", a.opts.AppID, a.addr, n, a.opts.Window.ID)
}
