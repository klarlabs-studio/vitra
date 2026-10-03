// Command menubar is a menu bar (tray) app: it shows the free space on a
// disk next to its tray icon and drops down an HTML panel with the details
// when you click it, as menu bar apps such as CodexBar do. It has no Dock
// icon or taskbar entry and no other window.
//
//	CGO_ENABLED=1 go run -tags vitra_native ./example/menubar
//	CGO_ENABLED=1 go run -tags vitra_native ./example/menubar -path /Volumes/Data
//
// The panel is an ordinary window to the security model: it may read the
// usage and close itself, and nothing else. It tries to read the clipboard
// to show the refusal.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"sync"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
	"go.klarlabs.de/vitra/plugin/official"
)

const (
	appID = "de.klarlabs.vitra.menubar"
	// panelWindow is the tray panel, the app's only window.
	panelWindow domain.WindowID = "panel"
	// usageEvent carries each new Usage to subscribed windows.
	usageEvent domain.EventName = "usage.update"
	// refreshEvery is how often the tray status is refreshed.
	refreshEvery = 30 * time.Second
)

// Tray menu action IDs.
const (
	actionRefresh = "refresh"
	actionPanel   = "panel"
	actionLogin   = "login"
	actionQuit    = "quit"
)

//go:embed frontend
var frontendFS embed.FS

func main() {
	home, _ := os.UserHomeDir()
	path := flag.String("path", home, "a folder on the disk to watch")
	flag.Parse()
	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, "menubar:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	m, err := newMenubar(newHost(), path, &audit.MemorySink{})
	if err != nil {
		return err
	}
	return m.Run(context.Background())
}

// menubar is the app: the tray, its panel, and the usage they show.
type menubar struct {
	app  *app.App
	rt   *vitra.Runtime
	path string

	mu   sync.Mutex
	last Usage
}

// newMenubar wires the runtime, the panel's grant and commands, and the
// tray for the disk holding path.
func newMenubar(host app.DesktopHost, path string, sink audit.Sink) (*menubar, error) {
	rt, err := vitra.New(vitra.Config{AppID: appID})
	if err != nil {
		return nil, err
	}
	rt.SetAudit(sink)
	assets, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return nil, err
	}
	a, err := app.New(app.Options{
		AppID: appID, Title: "Disk Usage", Assets: assets, Host: host, Runtime: rt,
		Presentation: app.PresentationAccessory,
		Window:       app.WindowOptions{ID: panelWindow, Width: 340, Height: 280, Kind: app.WindowKindPanel},
	})
	if err != nil {
		return nil, err
	}
	m := &menubar{app: a, rt: rt, path: path}
	if err := m.register(); err != nil {
		return nil, err
	}
	// The clipboard plugin is registered but never granted: the panel's
	// attempt to read it is refused, which the panel shows.
	if err := a.UseOfficialPlugins(context.Background(), official.Clipboard()); err != nil {
		return nil, err
	}
	a.OnAction(m.onAction)
	return m, nil
}

// register grants the panel what it needs and registers its commands.
func (m *menubar) register() error {
	grant, err := domain.NewCapabilityGrant(
		"panel", "read the disk usage and close the panel",
		[]domain.WindowID{panelWindow},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "usage.read"}, {Name: "panel.close"}},
	)
	if err != nil {
		return err
	}
	return errors.Join(
		m.rt.RegisterGrant(grant),
		vitra.Register(m.rt, vitra.Command[struct{}, Usage]{
			Name:        "usage.follow",
			Description: "Send usage updates to the calling window; returns the current usage",
			Permission:  "usage.read",
			Handler: func(_ context.Context, inv domain.Invocation, _ struct{}) (Usage, error) {
				id := domain.SubscriptionID("usage-" + string(inv.Caller.Window))
				if _, err := m.rt.SubscribeEvent(id, usageEvent, inv.Caller.Window); err != nil {
					return Usage{}, err
				}
				return m.refresh()
			},
		}),
		vitra.Register(m.rt, vitra.Command[struct{}, vitra.Void]{
			Name:        "panel.close",
			Description: "Hide the tray panel",
			Permission:  "panel.close",
			Handler: func(context.Context, domain.Invocation, struct{}) (vitra.Void, error) {
				return vitra.Void{}, m.app.HideTrayPanel()
			},
		}),
	)
}

// Run shows the tray and blocks until the app quits.
func (m *menubar) Run(ctx context.Context) error {
	if _, err := m.refresh(); err != nil {
		return err
	}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	go m.poll(ctx)
	return m.app.Run(ctx)
}

func (m *menubar) poll(ctx context.Context) {
	tick := time.NewTicker(refreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if _, err := m.refresh(); err != nil {
				fmt.Fprintln(os.Stderr, "menubar:", err)
			}
		}
	}
}

// refresh measures the disk, updates the tray, and pushes the usage to the
// panel.
func (m *menubar) refresh() (Usage, error) {
	u, err := readUsage(m.path)
	if err != nil {
		return Usage{}, err
	}
	m.mu.Lock()
	m.last = u
	m.mu.Unlock()
	err = m.app.SetTray(app.TraySpec{
		Title: u.trayTitle(), Tooltip: u.tooltip(),
		Icon: trayIcon(u.UsedPercent), Template: true,
		Panel: panelWindow,
		Items: m.menu(),
	})
	if err != nil {
		return Usage{}, err
	}
	_ = m.app.Emit(context.Background(), usageEvent, u)
	return u, nil
}

// menu is the tray menu. "Launch at Login" is checked while the app starts
// at login, and disabled where the host cannot register it (an unbundled
// macOS binary, for example).
func (m *menubar) menu() []platform.MenuItem {
	login, err := m.app.LoginItemEnabled()
	return []platform.MenuItem{
		{ID: actionPanel, Label: "Show Details"},
		{ID: actionRefresh, Label: "Refresh"},
		{Separator: true},
		{ID: actionLogin, Label: "Launch at Login", Checked: login, Disabled: err != nil},
		{Separator: true},
		{ID: actionQuit, Label: "Quit"},
	}
}

// onAction handles the tray menu.
func (m *menubar) onAction(id string) {
	switch id {
	case actionRefresh:
		if _, err := m.refresh(); err != nil {
			fmt.Fprintln(os.Stderr, "menubar:", err)
		}
	case actionPanel:
		_ = m.app.ShowTrayPanel()
	case actionLogin:
		on, err := m.app.LoginItemEnabled()
		if err == nil {
			err = m.app.SetLoginItem(!on)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "menubar: launch at login:", err)
		}
		if _, err := m.refresh(); err != nil { // show the new checkmark
			fmt.Fprintln(os.Stderr, "menubar:", err)
		}
	case actionQuit:
		m.app.Quit()
	}
}

func newHost() app.DesktopHost {
	switch runtime.GOOS {
	case "darwin":
		h := darwin.New()
		h.SetProgramName("vitra-menubar")
		return h
	case "windows":
		h := windows.New()
		h.SetProgramName("vitra-menubar")
		return h
	default:
		h := linux.New()
		h.SetProgramName("vitra-menubar")
		return h
	}
}
