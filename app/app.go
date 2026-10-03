// Package app is the competitive desktop application runtime that binds the
// secure Vitra kernel to a native WebView host.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bridge"
	"go.klarlabs.de/vitra/internal/ipc"
	"go.klarlabs.de/vitra/platform"
)

// DesktopHost is the core a host implements to run an app. It is an alias
// of platform.DesktopHost; the optional capabilities (clipboard, dialogs,
// menus, tray, …) are separate interfaces in package platform, detected on
// the host when a command needs them.
type DesktopHost = platform.DesktopHost

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
	// DevTools enables the WebView inspector. Leave it off in shipped apps:
	// anyone with the inspector can run script with the page's bridge
	// access. `vitra dev` turns it on through EnvDevTools.
	DevTools bool
}

// EnvDevTools, when set to "1", enables the WebView inspector regardless of
// Options.DevTools. `vitra dev` sets it.
const EnvDevTools = "VITRA_DEVTOOLS"

// App is a runnable desktop application.
type App struct {
	opts    Options
	rt      *vitra.Runtime
	host    DesktopHost
	server  *http.Server
	addr    string
	mu      sync.Mutex
	windows map[domain.WindowID]WindowOptions
	// tokens holds each window's bridge sender token (see bridge.Preload).
	tokens map[domain.WindowID]string
	// actions is the native activation fan-out (see OnAction).
	actions actionFanout
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

// EnvGenerateTypeScript names an environment variable that switches Run into
// code generation: Run writes a TypeScript client for the runtime's
// registered commands to the path it names, then returns without serving
// assets or opening a window. `vitra generate typescript --app` sets it.
const EnvGenerateTypeScript = "VITRA_GENERATE_TYPESCRIPT"

// Run serves frontend assets, opens the primary window, and blocks on the UI loop.
func (a *App) Run(ctx context.Context) error {
	if out := os.Getenv(EnvGenerateTypeScript); out != "" {
		return a.writeTypeScript(out)
	}
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

	if dt, ok := a.host.(platform.DevToolsSetter); ok {
		dt.SetDevTools(a.opts.DevTools || os.Getenv(EnvDevTools) == "1")
	}
	a.host.SetInvokeHandler(a.handleInvoke)
	if mr, ok := a.host.(platform.MessageReporter); ok {
		mr.SetMessageHandler(a.handleMessage)
	}
	a.host.SetNavPolicy(a.allowNav)
	if rr, ok := a.host.(platform.RejectReporter); ok {
		rr.SetRejectHandler(func(id domain.WindowID, reason string) {
			a.audit(audit.Event{Kind: audit.KindBridgeReject, Window: string(id), Outcome: "denied", Detail: reason})
		})
	}
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
	token, err := newSenderToken()
	if err != nil {
		_ = a.rt.CloseWindow(ctx, opts.ID)
		return err
	}
	a.mu.Lock()
	if a.tokens == nil {
		a.tokens = map[domain.WindowID]string{}
	}
	a.tokens[opts.ID] = token
	a.mu.Unlock()
	if err := a.host.Open(spec, uri, bridge.Preload(token)); err != nil {
		a.mu.Lock()
		delete(a.tokens, opts.ID)
		a.mu.Unlock()
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
	delete(a.tokens, id)
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
		delete(a.tokens, id)
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
	a.mu.Lock()
	token := a.tokens[windowID]
	a.mu.Unlock()
	if token == "" {
		return nil // not a window this app opened
	}
	req, id, err := ipc.Bridge{Host: ipc.HostIdentity{Window: windowID, Origin: origin}, Token: token}.DecodeInvoke(raw)
	if errors.Is(err, ipc.ErrUntrustedSender) {
		// Not from the window's top frame (for example a subframe that can
		// reach the native handler): drop it without replying.
		a.audit(audit.Event{Kind: audit.KindBridgeReject, Window: string(windowID), Origin: string(origin), Outcome: "denied", Detail: "untrusted sender"})
		return nil
	}
	if err != nil {
		a.audit(audit.Event{Kind: audit.KindBridgeReject, Window: string(windowID), Origin: string(origin), Outcome: "error", Detail: err.Error()})
		return mustJSON(replyMsg{ID: id, OK: false, Error: err.Error(), Code: "bad_request"})
	}
	res, err := a.rt.Invoke(context.Background(), req)
	if err != nil {
		code := ipc.DenialCode(err)
		return mustJSON(replyMsg{ID: id, OK: false, Error: err.Error(), Code: code})
	}
	return mustJSON(replyMsg{ID: id, OK: true, Result: res.Output})
}

// handleMessage accepts a bridge message only from a document served by the
// app's asset server; the call's origin follows from that. Anything else,
// including about:blank and remote pages, is dropped without a reply.
func (a *App) handleMessage(windowID domain.WindowID, senderURL string, raw []byte) []byte {
	if senderURL == "about:blank" || !a.isLocal(senderURL) {
		a.audit(audit.Event{
			Kind: audit.KindBridgeReject, Window: string(windowID), Outcome: "denied",
			Detail: "message from a document outside the app: " + redactURL(senderURL),
		})
		return nil
	}
	return a.handleInvoke(windowID, domain.OriginPackagedLocal, raw)
}

func (a *App) allowNav(windowID domain.WindowID, uri string) bool {
	if a.isLocal(uri) {
		return true
	}
	a.audit(audit.Event{Kind: audit.KindNavigationBlock, Window: string(windowID), Outcome: "denied", Detail: redactURL(uri)})
	return false
}

func (a *App) isLocal(uri string) bool {
	// Allow only the local asset server and about:blank. Everything else is
	// external and must not keep privileged bridge access.
	if uri == "about:blank" {
		return true
	}
	// Compare the parsed origin exactly. A string prefix check on a.addr
	// admits "http://127.0.0.1:PORT@evil.example/" (userinfo) and
	// "http://127.0.0.1:PORT1/" (another local port).
	u, err := url.Parse(uri)
	if a.addr == "" || err != nil || u.User != nil || u.Opaque != "" {
		return false
	}
	return u.Scheme == "http" && u.Host == strings.TrimPrefix(a.addr, "http://")
}

// redactURL drops credentials, query, and fragment, which can carry secrets,
// before a URL goes into the audit log.
func redactURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Opaque != "" {
		return "(unparseable URL)"
	}
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	return u.String()
}

// audit records e in the runtime's audit sink, if one is installed.
func (a *App) audit(e audit.Event) {
	if s := a.rt.Audit(); s != nil {
		_ = s.Append(e)
	}
}

// newSenderToken returns 32 random bytes, hex-encoded.
func newSenderToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("sender token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"ok":false,"error":"encode"}`)
	}
	return b
}

func (a *App) writeTypeScript(path string) error {
	ts, err := a.rt.TypeScript(string(a.opts.AppID))
	if err != nil {
		return fmt.Errorf("generate typescript: %w", err)
	}
	return os.WriteFile(path, []byte(ts), 0o644)
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
