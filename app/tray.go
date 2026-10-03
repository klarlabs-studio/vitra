package app

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"io/fs"
	"sync"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// TraySpec is the tray (menu bar status item) SetTray shows.
type TraySpec = platform.TraySpec

// MaxTrayIconBytes bounds a tray icon. Menu bar icons are a few hundred
// bytes; the bound keeps a page from making the host decode a huge image.
const MaxTrayIconBytes = 256 << 10

// SetTray shows or updates the tray from Go, for example from a poller that
// refreshes the status. App code is trusted, so it needs no grant; the
// page's tray.set still does. Items are delivered through OnAction (and as
// tray.action events with UseOfficialPlugins). It fails with
// *platform.ErrUnsupported when the host has no tray.
func (a *App) SetTray(spec TraySpec) error {
	h, err := a.trayHost()
	if err != nil {
		return err
	}
	if spec.Icon != nil {
		if err := checkTrayIcon(spec.Icon); err != nil {
			return err
		}
	}
	if err := checkMenuItems(spec.Items); err != nil {
		return err
	}
	if err := h.SetTray(spec); err != nil {
		return err
	}
	a.mu.Lock()
	a.tray = true
	a.mu.Unlock()
	return nil
}

// ClearTray removes the tray icon. It does nothing on hosts without a tray.
func (a *App) ClearTray() {
	if h, err := a.trayHost(); err == nil {
		h.ClearTray()
		a.mu.Lock()
		a.tray = false
		a.mu.Unlock()
	}
}

func (a *App) trayShown() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tray
}

func (a *App) trayHost() (platform.Tray, error) {
	if err := platform.Require(a.host, platform.FeatureTray); err != nil {
		return nil, err
	}
	h, ok := a.host.(platform.Tray)
	if !ok {
		return nil, &platform.ErrUnsupported{Feature: platform.FeatureTray, OS: a.host.OS(), Detail: "host has no tray adapter"}
	}
	return h, nil
}

// trayIcon reads the tray icon at name (an io/fs path) from the app's assets.
func (a *App) trayIcon(name string) ([]byte, error) {
	f, err := a.opts.Assets.Open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &domain.ErrNotFound{Entity: "tray icon", ID: name}
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxTrayIconBytes+1))
	if err != nil {
		return nil, err
	}
	if err := checkTrayIcon(data); err != nil {
		return nil, err
	}
	return data, nil
}

// checkTrayIcon accepts a PNG of at most MaxTrayIconBytes.
func checkTrayIcon(data []byte) error {
	if len(data) > MaxTrayIconBytes {
		return &domain.ErrValidation{Message: "tray icon is larger than 256 KiB"}
	}
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return &domain.ErrValidation{Message: "tray icon must be a PNG image"}
	}
	return nil
}

// checkMenuItems applies the menu item rules of the official inputs to
// items Go code passes directly.
func checkMenuItems(items []platform.MenuItem) error {
	for _, it := range items {
		if it.Separator {
			if it.ID != "" || it.Label != "" || it.Shortcut != "" || it.Disabled || it.Checked {
				return &domain.ErrValidation{Message: "a menu separator takes no id, label, shortcut, disabled or checked"}
			}
			continue
		}
		if it.ID == "" || it.Label == "" {
			return &domain.ErrValidation{Message: "menu item requires id and label"}
		}
	}
	return nil
}

// OnAction registers fn for native activations (menu items, tray items and
// global shortcuts), by action ID. Handlers run off the UI thread.
func (a *App) OnAction(fn func(id string)) {
	a.actions.onEach(fn)
	a.installActions()
}

// OnTrayClick registers fn for primary clicks on a tray whose spec sets
// ClickActivates. Handlers run off the UI thread.
func (a *App) OnTrayClick(fn func(platform.TrayClick)) {
	a.trayClicks.onEach(fn)
	a.installTrayClicks()
}

// TrayAnchor returns the tray icon's screen rectangle, for placing a window
// under it. It fails with *platform.ErrUnsupported when the host cannot
// tell.
func (a *App) TrayAnchor() (platform.Rect, error) {
	if err := platform.Require(a.host, platform.FeatureTrayAnchor); err != nil {
		return platform.Rect{}, err
	}
	h, ok := a.host.(platform.TrayAnchorer)
	if !ok {
		return platform.Rect{}, &platform.ErrUnsupported{Feature: platform.FeatureTrayAnchor, OS: a.host.OS(), Detail: "host cannot locate the tray icon"}
	}
	return h.TrayAnchor()
}

func (a *App) installActions() {
	a.actions.install(func(deliver func(string)) {
		if ar, ok := a.host.(platform.ActionReporter); ok {
			ar.SetActionHandler(deliver)
		}
	})
}

func (a *App) installTrayClicks() {
	a.trayClicks.install(func(deliver func(platform.TrayClick)) {
		if tr, ok := a.host.(platform.TrayClickReporter); ok {
			tr.SetTrayClickHandler(deliver)
		}
	})
}

// emitNativeEvents forwards native activations to the page: menu, tray and
// shortcut activations as menu.action, tray.action and shortcut.action, and
// tray clicks as tray.click to the windows that may react to them.
func (a *App) emitNativeEvents() {
	a.actions.setEmit(func(id string) {
		payload := map[string]any{"id": id}
		for _, ev := range []domain.EventName{"menu.action", "tray.action", "shortcut.action"} {
			_ = a.Emit(context.Background(), ev, payload)
		}
	})
	a.installActions()
	a.trayClicks.setEmit(func(c platform.TrayClick) {
		_ = a.emitTo(context.Background(), "tray.click", trayClickPayload(c), a.trayClickWindows())
	})
	a.installTrayClicks()
}

// trayClickEvent is the tray.click payload. Anchor is null when the host
// could not tell where the icon is.
type trayClickEvent struct {
	Anchor *trayRect `json:"anchor"`
}

type trayRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

func trayClickPayload(c platform.TrayClick) trayClickEvent {
	if !c.HasAnchor {
		return trayClickEvent{}
	}
	return trayClickEvent{Anchor: &trayRect{X: c.Anchor.X, Y: c.Anchor.Y, Width: c.Anchor.Width, Height: c.Anchor.Height}}
}

// trayClickWindows are the windows tray.click reaches: the primary window.
// Other windows have no business reacting to the tray.
func (a *App) trayClickWindows() map[domain.WindowID]bool {
	allowed := map[domain.WindowID]bool{}
	if a.opts.Window.ID != "" {
		allowed[a.opts.Window.ID] = true
	}
	return allowed
}

// fanout delivers each native activation of type T to the Go handlers and,
// once set, to the page through emit. The host handler is installed once.
type fanout[T any] struct {
	mu        sync.Mutex
	installed bool
	emit      func(T)
	handlers  []func(T)
}

func (f *fanout[T]) onEach(fn func(T)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers = append(f.handlers, fn)
}

func (f *fanout[T]) setEmit(fn func(T)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emit = fn
}

// install calls hook with the delivery function the first time only.
func (f *fanout[T]) install(hook func(deliver func(T))) {
	f.mu.Lock()
	done := f.installed
	f.installed = true
	f.mu.Unlock()
	if !done {
		hook(f.deliver)
	}
}

func (f *fanout[T]) deliver(v T) {
	f.mu.Lock()
	emit := f.emit
	handlers := append([]func(T){}, f.handlers...)
	f.mu.Unlock()
	if emit != nil {
		emit(v)
	}
	for _, fn := range handlers {
		fn(v)
	}
}
