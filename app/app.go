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

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/bridge"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// DesktopHost is the native surface required to run a Vitra desktop app.
type DesktopHost interface {
	platform.Host
	Open(spec platform.WindowSpec, uri, preload string) error
	SetInvokeHandler(fn func(windowID domain.WindowID, origin domain.Origin, raw []byte) []byte)
	SetNavPolicy(fn func(windowID domain.WindowID, uri string) bool)
	Eval(id domain.WindowID, js string) error
	ClipboardGet() (string, error)
	ClipboardSet(text string) error
	OpenFileDialog() (string, error)
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
	opts   Options
	rt     *vitra.Runtime
	host   DesktopHost
	server *http.Server
	addr   string
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
	return &App{opts: opts, rt: rt, host: opts.Host}, nil
}

// Runtime returns the secure kernel.
func (a *App) Runtime() *vitra.Runtime { return a.rt }

// Run serves frontend assets, opens the native window, and blocks on the UI loop.
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

	if _, err := a.rt.OpenWindow(ctx, a.opts.Window.ID, domain.OriginPackagedLocal); err != nil {
		return err
	}
	uri := strings.TrimRight(a.addr, "/") + a.opts.Window.Path
	spec := platform.WindowSpec{
		ID:     a.opts.Window.ID,
		Title:  a.opts.Window.Title,
		Origin: domain.OriginPackagedLocal,
		Width:  a.opts.Window.Width,
		Height: a.opts.Window.Height,
	}
	if err := a.host.Open(spec, uri, bridge.PreloadJS); err != nil {
		return err
	}
	return a.host.Run()
}

// Quit requests application shutdown.
func (a *App) Quit() {
	a.host.Quit()
}

type invokeMsg struct {
	Type         string          `json:"type"`
	ID           string          `json:"id"`
	Command      string          `json:"command"`
	Input        json.RawMessage `json:"input"`
	ResourcePath string          `json:"resource_path"`
}

type replyMsg struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	Code   string `json:"code,omitempty"`
}

func (a *App) handleInvoke(windowID domain.WindowID, origin domain.Origin, raw []byte) []byte {
	var msg invokeMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		return mustJSON(replyMsg{OK: false, Error: "invalid invoke payload", Code: "bad_request"})
	}
	if msg.Type != "invoke" {
		return mustJSON(replyMsg{ID: msg.ID, OK: false, Error: "unsupported message type", Code: "bad_request"})
	}
	caller, err := domain.NewCaller(windowID, origin)
	if err != nil {
		return mustJSON(replyMsg{ID: msg.ID, OK: false, Error: err.Error(), Code: "bad_caller"})
	}
	var input any
	if len(msg.Input) > 0 && string(msg.Input) != "null" {
		_ = json.Unmarshal(msg.Input, &input)
	}
	res, err := a.rt.Invoke(context.Background(), domain.InvocationRequest{
		Caller:       caller,
		Command:      domain.CommandName(msg.Command),
		Input:        input,
		ResourcePath: msg.ResourcePath,
	})
	if err != nil {
		code := "error"
		var denied *domain.ErrDenied
		if errors.As(err, &denied) {
			code = string(denied.Code)
		}
		return mustJSON(replyMsg{ID: msg.ID, OK: false, Error: err.Error(), Code: code})
	}
	return mustJSON(replyMsg{ID: msg.ID, OK: true, Result: res.Output})
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
	return fmt.Sprintf("app=%s addr=%s window=%s", a.opts.AppID, a.addr, a.opts.Window.ID)
}
