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
// global shortcuts), by action ID. Handlers run on a host thread: hand long
// work to a goroutine.
func (a *App) OnAction(fn func(id string)) {
	a.actions.add(a, fn)
}

// actionFanout installs the single host action handler and delivers each
// activation to the Go handlers and, once enabled, as page events.
type actionFanout struct {
	mu        sync.Mutex
	installed bool
	events    bool
	handlers  []func(string)
}

func (f *actionFanout) add(a *App, fn func(string)) {
	f.mu.Lock()
	f.handlers = append(f.handlers, fn)
	f.mu.Unlock()
	f.install(a)
}

// emitEvents forwards activations to the page as menu.action, tray.action
// and shortcut.action events.
func (f *actionFanout) emitEvents(a *App) {
	f.mu.Lock()
	f.events = true
	f.mu.Unlock()
	f.install(a)
}

func (f *actionFanout) install(a *App) {
	ar, ok := a.host.(platform.ActionReporter)
	if !ok {
		return
	}
	f.mu.Lock()
	done := f.installed
	f.installed = true
	f.mu.Unlock()
	if !done {
		ar.SetActionHandler(func(id string) { f.deliver(a, id) })
	}
}

func (f *actionFanout) deliver(a *App, id string) {
	f.mu.Lock()
	events := f.events
	handlers := append([]func(string){}, f.handlers...)
	f.mu.Unlock()
	if events {
		payload := map[string]any{"id": id}
		for _, ev := range []domain.EventName{"menu.action", "tray.action", "shortcut.action"} {
			_ = a.Emit(context.Background(), ev, payload)
		}
	}
	for _, fn := range handlers {
		fn(id)
	}
}
