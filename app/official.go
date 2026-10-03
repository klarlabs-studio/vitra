package app

import (
	"context"
	"errors"
	"fmt"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/plugin"
	"go.klarlabs.de/vitra/plugin/official"
)

// UseOfficialPlugins registers official plugins on the app's runtime and
// binds each of their commands to the native host through the
// capability-checked services in package desktop. With no arguments it uses
// official.All(); pass constructors (official.Clipboard(), official.Dialog(),
// ...) to take only some.
//
// Every command is bound as a typed command (vitra.Bind) with the input and
// output types in package official: input is decoded strictly, so an unknown
// field or a wrong type is a validation error, and Runtime.TypeScript
// generates a typed client for them.
//
// It grants nothing. A window can use a command only if one of your grants
// gives it the command's permission, and path-scoped permissions (fs.read,
// fs.write, path.open) only within the grant's scope. Every service checks
// the grant again before touching the host.
//
// It also forwards native activations to the page: menu, tray, and shortcut
// activations are emitted as menu.action, tray.action, and shortcut.action
// ({"id": ...}), and file drops as dragdrop.drop ({"window", "paths"}).
// Windows receive them once subscribed (Runtime.SubscribeEvent). This
// replaces any action or drop handler set on the host before.
//
// Menus apply to the primary window. Deep links need an app-specific URL
// scheme, so the deeplink plugin's event is left for the app to emit.
//
// Call it after New and before Run.
func (a *App) UseOfficialPlugins(ctx context.Context, plugins ...plugin.Plugin) error {
	if len(plugins) == 0 {
		plugins = official.All()
	}
	binders := a.officialBinders()
	for _, p := range plugins {
		if _, ok := binders[p.Manifest().ID]; !ok {
			return fmt.Errorf("UseOfficialPlugins: %q is not an official plugin; register it with Runtime.RegisterPlugin", p.Manifest().ID)
		}
	}
	for _, p := range plugins {
		if err := a.rt.RegisterPlugin(ctx, p); err != nil {
			return err
		}
		if err := binders[p.Manifest().ID](); err != nil {
			return fmt.Errorf("bind %s: %w", p.Manifest().ID, err)
		}
	}
	a.host.SetActionHandler(func(id string) {
		payload := map[string]any{"id": id}
		for _, ev := range []domain.EventName{"menu.action", "tray.action", "shortcut.action"} {
			_ = a.Emit(context.Background(), ev, payload)
		}
	})
	a.host.SetDragDropHandler(func(window domain.WindowID, paths []string) {
		_ = a.Emit(context.Background(), "dragdrop.drop", map[string]any{"window": string(window), "paths": paths})
	})
	return nil
}

// none is the input of a command that takes none.
type none = struct{}

// binder binds official commands as typed commands and collects errors.
type binder struct {
	rt   *vitra.Runtime
	errs []error
}

// on binds name to fn, which acts as the invoking window: it receives the
// caller the gateway authorized, never one taken from the input.
func on[In, Out any](b *binder, name domain.CommandName, fn func(ctx context.Context, caller domain.Caller, in In) (Out, error)) {
	b.errs = append(b.errs, vitra.Bind(b.rt, name, func(ctx context.Context, inv domain.Invocation, in In) (Out, error) {
		return fn(ctx, inv.Caller, in)
	}))
}

// onVoid binds a command that returns nothing.
func onVoid[In any](b *binder, name domain.CommandName, fn func(ctx context.Context, caller domain.Caller, in In) error) {
	on(b, name, func(ctx context.Context, caller domain.Caller, in In) (vitra.Void, error) {
		return vitra.Void{}, fn(ctx, caller, in)
	})
}

func (b *binder) err() error { return errors.Join(b.errs...) }

func (a *App) binder() *binder { return &binder{rt: a.rt} }

func (a *App) officialBinders() map[domain.PluginID]func() error {
	return map[domain.PluginID]func() error{
		official.FSID:           a.bindFS,
		official.DialogID:       a.bindDialog,
		official.ClipboardID:    a.bindClipboard,
		official.BrowserID:      a.bindBrowser,
		official.OSID:           a.bindOS,
		official.NotificationID: a.bindNotification,
		official.PathID:         a.bindPath,
		official.WindowID:       a.bindWindow,
		official.MenuID:         a.bindMenu,
		official.TrayID:         a.bindTray,
		official.DragDropID:     a.bindDragDrop,
		official.ShortcutID:     a.bindShortcut,
		official.AppID:          a.bindApp,
		official.DeepLinkID:     func() error { return nil }, // events only
	}
}

func (a *App) bindFS() error {
	files := &desktop.FileService{Gateway: a.rt}
	b := a.binder()
	on(b, "fs.read", func(ctx context.Context, caller domain.Caller, path string) (string, error) {
		data, err := files.Read(ctx, caller, path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	})
	onVoid(b, "fs.write", func(ctx context.Context, caller domain.Caller, in official.WriteFileInput) error {
		return files.Write(ctx, caller, in.Path, []byte(in.Data))
	})
	return b.err()
}

// openFiles runs the host's open dialog. With opts.Multiple it needs a host
// that implements platform.MultiFileOpener and fails explicitly otherwise,
// so a caller asking for several files never silently gets one.
func openFiles(h DesktopHost, opts platform.DialogFileOptions) ([]string, error) {
	if opts.Multiple {
		multi, ok := h.(platform.MultiFileOpener)
		if !ok {
			return nil, &platform.ErrUnsupported{
				Feature: platform.FeatureDialogOpen,
				OS:      h.OS(),
				Detail:  "this host cannot select multiple files",
			}
		}
		return multi.OpenFilesDialog(opts)
	}
	path, err := h.OpenFileDialog(opts)
	if err != nil || path == "" {
		return nil, err
	}
	return []string{path}, nil
}

// dialogOptions converts dialog input for the host.
func dialogOptions(in official.DialogOptions) platform.DialogFileOptions {
	opts := platform.DialogFileOptions{Title: in.Title, DefaultPath: in.DefaultPath, Multiple: in.Multiple}
	for _, f := range in.Filters {
		opts.Filters = append(opts.Filters, platform.FileFilter{Name: f.Name, Extensions: f.Extensions})
	}
	return opts
}

func (a *App) bindDialog() error {
	h := a.host
	dialogs := &desktop.DialogService{
		Gateway: a.rt,
		Host:    h,
		OnOpen: func(_ context.Context, opts platform.DialogFileOptions) ([]string, error) {
			return openFiles(h, opts)
		},
		OnSave: func(_ context.Context, opts platform.DialogFileOptions) (string, error) {
			return h.SaveFileDialog(opts)
		},
		OnOpenDirectory: func(_ context.Context, opts platform.DialogFileOptions) (string, error) {
			return h.OpenDirectoryDialog(opts)
		},
		OnMessage: func(_ context.Context, title, message, kind string) (bool, error) {
			return h.MessageDialog(title, message, kind)
		},
	}
	b := a.binder()
	on(b, "dialog.open", func(ctx context.Context, caller domain.Caller, in official.DialogOptions) ([]string, error) {
		return dialogs.OpenFile(ctx, caller, dialogOptions(in))
	})
	on(b, "dialog.save", func(ctx context.Context, caller domain.Caller, in official.DialogOptions) (string, error) {
		return dialogs.SaveFile(ctx, caller, dialogOptions(in))
	})
	on(b, "dialog.openDirectory", func(ctx context.Context, caller domain.Caller, in official.DialogOptions) (string, error) {
		return dialogs.OpenDirectory(ctx, caller, dialogOptions(in))
	})
	on(b, "dialog.message", func(ctx context.Context, caller domain.Caller, in official.MessageDialogInput) (bool, error) {
		return dialogs.Message(ctx, caller, in.Title, in.Message, in.Kind)
	})
	return b.err()
}

func (a *App) bindClipboard() error {
	h := a.host
	clips := &desktop.ClipboardService{
		Gateway: a.rt,
		Host:    h,
		OnRead:  func(context.Context) (string, error) { return h.ClipboardGet() },
		OnWrite: func(_ context.Context, text string) error { return h.ClipboardSet(text) },
	}
	b := a.binder()
	on(b, "clipboard.read", func(ctx context.Context, caller domain.Caller, _ none) (string, error) {
		return clips.Read(ctx, caller)
	})
	onVoid(b, "clipboard.write", clips.Write)
	return b.err()
}

func (a *App) bindBrowser() error {
	h := a.host
	browser := &desktop.BrowserService{Gateway: a.rt, Host: h, OnOpen: h.OpenURL}
	b := a.binder()
	onVoid(b, "browser.open", browser.OpenURL)
	return b.err()
}

func (a *App) bindOS() error {
	osSvc := &desktop.OsService{Gateway: a.rt}
	b := a.binder()
	on(b, "os.info", func(ctx context.Context, caller domain.Caller, _ none) (desktop.OsInfo, error) {
		return osSvc.Info(ctx, caller)
	})
	return b.err()
}

func (a *App) bindNotification() error {
	h := a.host
	notes := &desktop.NotificationService{
		Gateway: a.rt,
		Host:    h,
		OnShow:  func(_ context.Context, title, body string) error { return h.ShowNotification(title, body) },
	}
	b := a.binder()
	onVoid(b, "notifications.show", func(ctx context.Context, caller domain.Caller, in official.NotificationInput) error {
		return notes.Show(ctx, caller, in.Title, in.Body)
	})
	return b.err()
}

func (a *App) bindPath() error {
	h := a.host
	paths := &desktop.PathService{Gateway: a.rt, Host: h, OnOpen: h.OpenPath}
	b := a.binder()
	onVoid(b, "path.open", paths.Open)
	return b.err()
}

func (a *App) bindWindow() error {
	h := a.host
	w := &desktop.WindowService{
		Gateway: a.rt,
		Host:    h,
		OnApply: func(_ context.Context, id domain.WindowID, c platform.WindowChrome) error {
			return h.ApplyWindowChrome(id, c)
		},
		OnRead: func(_ context.Context, id domain.WindowID) (platform.WindowChrome, error) {
			return h.ReadWindowChrome(id)
		},
		OnFocus: func(_ context.Context, id domain.WindowID) error {
			return h.FocusWindow(id)
		},
		OnBlur: func(_ context.Context, id domain.WindowID) error {
			return h.BlurWindow(id)
		},
		OnCreate: func(ctx context.Context, o desktop.WindowCreateOptions) error {
			return a.OpenWindow(ctx, WindowOptions{ID: o.ID, Title: o.Title, Path: o.Path, Width: o.Width, Height: o.Height})
		},
		OnClose: a.CloseWindow,
	}
	b := a.binder()
	on(b, "window.create", func(ctx context.Context, caller domain.Caller, in official.WindowCreateInput) (official.WindowCreated, error) {
		id, err := w.Create(ctx, caller, desktop.WindowCreateOptions{
			ID: in.ID, Title: in.Title, Path: in.Path, Width: in.Width, Height: in.Height,
		})
		if err != nil {
			return official.WindowCreated{}, err
		}
		return official.WindowCreated{ID: id}, nil
	})
	onVoid(b, "window.chrome", func(ctx context.Context, caller domain.Caller, in official.WindowChromeInput) error {
		return w.Apply(ctx, caller, in.ID, platform.WindowChrome{
			Title: in.Title, Width: in.Width, Height: in.Height,
			Maximized: in.Maximized, Fullscreen: in.Fullscreen, AlwaysOnTop: in.AlwaysOnTop,
			Minimized: in.Minimized, Hidden: in.Hidden, IconPath: in.IconPath,
		})
	})
	on(b, "window.getChrome", func(ctx context.Context, caller domain.Caller, in official.WindowRef) (platform.WindowChrome, error) {
		return w.Read(ctx, caller, in.ID)
	})
	onVoid(b, "window.setAlwaysOnTop", func(ctx context.Context, caller domain.Caller, in official.WindowAlwaysOnTopInput) error {
		return w.SetAlwaysOnTop(ctx, caller, in.ID, in.AlwaysOnTop)
	})
	onVoid(b, "window.setTitle", func(ctx context.Context, caller domain.Caller, in official.WindowTitleInput) error {
		return w.SetTitle(ctx, caller, in.ID, in.Title)
	})
	onVoid(b, "window.setSize", func(ctx context.Context, caller domain.Caller, in official.WindowSizeInput) error {
		return w.SetSize(ctx, caller, in.ID, in.Width, in.Height)
	})
	onVoid(b, "window.setIcon", func(ctx context.Context, caller domain.Caller, in official.WindowIconInput) error {
		return w.SetIcon(ctx, caller, in.ID, in.IconPath)
	})
	for name, fn := range map[domain.CommandName]func(context.Context, domain.Caller, domain.WindowID) error{
		"window.close":        w.Close,
		"window.focus":        w.Focus,
		"window.blur":         w.Blur,
		"window.hide":         w.Hide,
		"window.show":         w.Show,
		"window.minimize":     w.Minimize,
		"window.maximize":     w.Maximize,
		"window.unmaximize":   w.Unmaximize,
		"window.fullscreen":   w.Fullscreen,
		"window.unfullscreen": w.Unfullscreen,
		"window.restore":      w.Restore,
	} {
		onVoid(b, name, func(ctx context.Context, caller domain.Caller, in official.WindowRef) error {
			return fn(ctx, caller, in.ID)
		})
	}
	return b.err()
}

// nativeItems converts menu items for the host; items without a menu go in
// defaultMenu.
func nativeItems(items []desktop.MenuItem, defaultMenu string) []platform.MenuItem {
	out := make([]platform.MenuItem, 0, len(items))
	for _, it := range items {
		menu := it.Menu
		if menu == "" {
			menu = defaultMenu
		}
		out = append(out, platform.MenuItem{Menu: menu, ID: it.ID, Label: it.Label, Shortcut: it.Shortcut})
	}
	return out
}

// menuItems converts menu input for the desktop services.
func menuItems(items []official.MenuItem) []desktop.MenuItem {
	if items == nil {
		return nil
	}
	out := make([]desktop.MenuItem, 0, len(items))
	for _, it := range items {
		out = append(out, desktop.MenuItem{ID: it.ID, Label: it.Label, Menu: it.Menu, Shortcut: it.Shortcut})
	}
	return out
}

func (a *App) bindMenu() error {
	h, primary := a.host, a.opts.Window.ID
	menus := &desktop.MenuService{
		Gateway: a.rt,
		Host:    h,
		OnSet: func(_ context.Context, items []desktop.MenuItem) error {
			return h.SetMenuBar(primary, nativeItems(items, "App"))
		},
		OnClear: func(context.Context) error { return h.SetMenuBar(primary, nil) },
	}
	b := a.binder()
	onVoid(b, "menu.set", func(ctx context.Context, caller domain.Caller, in official.MenuInput) error {
		return menus.SetMenu(ctx, caller, menuItems(in.Items))
	})
	onVoid(b, "menu.clear", func(ctx context.Context, caller domain.Caller, _ none) error {
		return menus.ClearMenu(ctx, caller)
	})
	return b.err()
}

func (a *App) bindTray() error {
	h := a.host
	trays := &desktop.TrayService{
		Gateway: a.rt,
		Host:    h,
		OnSet: func(_ context.Context, tooltip string, items []desktop.MenuItem) error {
			return h.SetTray(tooltip, nativeItems(items, ""))
		},
		OnClear: func(context.Context) error { h.ClearTray(); return nil },
	}
	b := a.binder()
	onVoid(b, "tray.set", func(ctx context.Context, caller domain.Caller, in official.TrayInput) error {
		return trays.SetTray(ctx, caller, in.Tooltip, menuItems(in.Items))
	})
	onVoid(b, "tray.clear", func(ctx context.Context, caller domain.Caller, _ none) error {
		return trays.ClearTray(ctx, caller)
	})
	return b.err()
}

func (a *App) bindDragDrop() error {
	h := a.host
	drops := &desktop.DragDropService{
		Gateway:  a.rt,
		Host:     h,
		OnEnable: func(_ context.Context, id domain.WindowID, on bool) error { return h.EnableDragDrop(id, on) },
	}
	b := a.binder()
	onVoid(b, "dragdrop.receive", func(ctx context.Context, caller domain.Caller, in official.DragDropInput) error {
		return drops.Enable(ctx, caller, in.ID, in.Enabled)
	})
	return b.err()
}

func (a *App) bindShortcut() error {
	h := a.host
	shortcuts := &desktop.ShortcutService{
		Gateway: a.rt,
		Host:    h,
		OnRegister: func(_ context.Context, accelerator, action string) error {
			return h.RegisterGlobalShortcut(accelerator, action)
		},
		OnUnregister: func(_ context.Context, accelerator string) error {
			return h.UnregisterGlobalShortcut(accelerator)
		},
	}
	b := a.binder()
	onVoid(b, "shortcut.register", func(ctx context.Context, caller domain.Caller, in official.ShortcutInput) error {
		return shortcuts.Register(ctx, caller, in.Accelerator, in.Action)
	})
	onVoid(b, "shortcut.unregister", func(ctx context.Context, caller domain.Caller, in official.ShortcutRef) error {
		return shortcuts.Unregister(ctx, caller, in.Accelerator)
	})
	return b.err()
}

func (a *App) bindApp() error {
	appSvc := &desktop.AppService{Gateway: a.rt, OnQuit: func(context.Context) error { a.Quit(); return nil }}
	b := a.binder()
	onVoid(b, "app.quit", func(ctx context.Context, caller domain.Caller, _ none) error {
		return appSvc.Quit(ctx, caller)
	})
	return b.err()
}
