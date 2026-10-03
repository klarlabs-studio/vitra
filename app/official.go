package app

import (
	"context"
	"errors"
	"fmt"

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
	if ar, ok := a.host.(platform.ActionReporter); ok {
		ar.SetActionHandler(func(id string) {
			payload := map[string]any{"id": id}
			for _, ev := range []domain.EventName{"menu.action", "tray.action", "shortcut.action"} {
				_ = a.Emit(context.Background(), ev, payload)
			}
		})
	}
	if dd, ok := a.host.(platform.DragDrop); ok {
		dd.SetDragDropHandler(func(window domain.WindowID, paths []string) {
			_ = a.Emit(context.Background(), "dragdrop.drop", map[string]any{"window": string(window), "paths": paths})
		})
	}
	return nil
}

// callerFunc is a command executor that acts as the invoking window.
type callerFunc = func(ctx context.Context, caller domain.Caller, input any) (any, error)

func (a *App) bind(execs map[domain.CommandName]callerFunc) error {
	var errs []error
	for name, fn := range execs {
		errs = append(errs, a.rt.BindExecutor(name, domain.CallerExecutorFunc(fn)))
	}
	return errors.Join(errs...)
}

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
	return a.bind(map[domain.CommandName]callerFunc{
		"fs.read": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			path, _ := input.(string)
			b, err := files.Read(ctx, caller, path)
			if err != nil {
				return nil, err
			}
			return string(b), nil
		},
		"fs.write": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			m, _ := input.(map[string]any)
			path, _ := m["path"].(string)
			data, _ := m["data"].(string)
			return nil, files.Write(ctx, caller, path, []byte(data))
		},
	})
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
	dialogs, ok := h.(platform.Dialogs)
	if !ok {
		return nil, &platform.ErrUnsupported{
			Feature: platform.FeatureDialogOpen,
			OS:      h.OS(),
			Detail:  "host does not implement platform.Dialogs",
		}
	}
	path, err := dialogs.OpenFileDialog(opts)
	if err != nil || path == "" {
		return nil, err
	}
	return []string{path}, nil
}

func (a *App) bindDialog() error {
	h := a.host
	dialogs := &desktop.DialogService{
		Gateway: a.rt,
		Host:    h,
		OnOpen: func(_ context.Context, opts platform.DialogFileOptions) ([]string, error) {
			return openFiles(h, opts)
		},
	}
	if d, ok := h.(platform.Dialogs); ok {
		dialogs.OnSave = func(_ context.Context, opts platform.DialogFileOptions) (string, error) {
			return d.SaveFileDialog(opts)
		}
		dialogs.OnOpenDirectory = func(_ context.Context, opts platform.DialogFileOptions) (string, error) {
			return d.OpenDirectoryDialog(opts)
		}
		dialogs.OnMessage = func(_ context.Context, title, message, kind string) (bool, error) {
			return d.MessageDialog(title, message, kind)
		}
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"dialog.open": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			return dialogs.OpenFile(ctx, caller, desktop.ParseDialogFileOptions(input))
		},
		"dialog.save": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			return dialogs.SaveFile(ctx, caller, desktop.ParseDialogFileOptions(input))
		},
		"dialog.openDirectory": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			return dialogs.OpenDirectory(ctx, caller, desktop.ParseDialogFileOptions(input))
		},
		"dialog.message": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			title, message, kind := "", "", "info"
			switch v := input.(type) {
			case string:
				message = v
			case map[string]any:
				title, _ = v["title"].(string)
				message, _ = v["message"].(string)
				if k, ok := v["kind"].(string); ok {
					kind = k
				}
			}
			return dialogs.Message(ctx, caller, title, message, kind)
		},
	})
}

func (a *App) bindClipboard() error {
	clips := &desktop.ClipboardService{Gateway: a.rt, Host: a.host}
	if c, ok := a.host.(platform.Clipboard); ok {
		clips.OnRead = func(context.Context) (string, error) { return c.ClipboardGet() }
		clips.OnWrite = func(_ context.Context, text string) error { return c.ClipboardSet(text) }
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"clipboard.read": func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
			return clips.Read(ctx, caller)
		},
		"clipboard.write": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			text, _ := input.(string)
			return nil, clips.Write(ctx, caller, text)
		},
	})
}

func (a *App) bindBrowser() error {
	browser := &desktop.BrowserService{Gateway: a.rt, Host: a.host}
	if o, ok := a.host.(platform.URLOpener); ok {
		browser.OnOpen = o.OpenURL
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"browser.open": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			rawURL, _ := input.(string)
			return nil, browser.OpenURL(ctx, caller, rawURL)
		},
	})
}

func (a *App) bindOS() error {
	osSvc := &desktop.OsService{Gateway: a.rt}
	return a.bind(map[domain.CommandName]callerFunc{
		"os.info": func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
			return osSvc.Info(ctx, caller)
		},
	})
}

func (a *App) bindNotification() error {
	notes := &desktop.NotificationService{Gateway: a.rt, Host: a.host}
	if n, ok := a.host.(platform.Notifier); ok {
		notes.OnShow = func(_ context.Context, title, body string) error { return n.ShowNotification(title, body) }
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"notifications.show": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			title, body := "", ""
			switch v := input.(type) {
			case string:
				body = v
			case map[string]any:
				title, _ = v["title"].(string)
				body, _ = v["body"].(string)
			}
			return nil, notes.Show(ctx, caller, title, body)
		},
	})
}

func (a *App) bindPath() error {
	paths := &desktop.PathService{Gateway: a.rt, Host: a.host}
	if o, ok := a.host.(platform.PathOpener); ok {
		paths.OnOpen = o.OpenPath
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"path.open": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			path, _ := input.(string)
			return nil, paths.Open(ctx, caller, path)
		},
	})
}

func (a *App) bindWindow() error {
	w := &desktop.WindowService{
		Gateway: a.rt,
		Host:    a.host,
		OnCreate: func(ctx context.Context, o desktop.WindowCreateOptions) error {
			return a.OpenWindow(ctx, WindowOptions{ID: o.ID, Title: o.Title, Path: o.Path, Width: o.Width, Height: o.Height})
		},
		OnClose: a.CloseWindow,
	}
	if h, ok := a.host.(platform.WindowControls); ok {
		w.OnApply = func(_ context.Context, id domain.WindowID, c platform.WindowChrome) error {
			return h.ApplyWindowChrome(id, c)
		}
		w.OnRead = func(_ context.Context, id domain.WindowID) (platform.WindowChrome, error) {
			return h.ReadWindowChrome(id)
		}
		w.OnFocus = func(_ context.Context, id domain.WindowID) error { return h.FocusWindow(id) }
		w.OnBlur = func(_ context.Context, id domain.WindowID) error { return h.BlurWindow(id) }
	}
	byID := func(fn func(context.Context, domain.Caller, domain.WindowID) error) callerFunc {
		return func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, err := desktop.ParseWindowID(input)
			if err != nil {
				return nil, err
			}
			return nil, fn(ctx, caller, id)
		}
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"window.create": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			opts, err := desktop.ParseWindowCreateOptions(input)
			if err != nil {
				return nil, err
			}
			id, err := w.Create(ctx, caller, opts)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": string(id)}, nil
		},
		"window.chrome": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, chrome, err := desktop.ParseWindowChromeApply(input)
			if err != nil {
				return nil, err
			}
			return nil, w.Apply(ctx, caller, id, chrome)
		},
		"window.getChrome": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, err := desktop.ParseWindowID(input)
			if err != nil {
				return nil, err
			}
			return w.Read(ctx, caller, id)
		},
		"window.setAlwaysOnTop": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, onTop, err := desktop.ParseWindowAlwaysOnTop(input)
			if err != nil {
				return nil, err
			}
			return nil, w.SetAlwaysOnTop(ctx, caller, id, onTop)
		},
		"window.setTitle": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, title, err := desktop.ParseWindowSetTitle(input)
			if err != nil {
				return nil, err
			}
			return nil, w.SetTitle(ctx, caller, id, title)
		},
		"window.setSize": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, width, height, err := desktop.ParseWindowSetSize(input)
			if err != nil {
				return nil, err
			}
			return nil, w.SetSize(ctx, caller, id, width, height)
		},
		"window.setIcon": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, icon, err := desktop.ParseWindowSetIcon(input)
			if err != nil {
				return nil, err
			}
			return nil, w.SetIcon(ctx, caller, id, icon)
		},
		"window.close":        byID(w.Close),
		"window.focus":        byID(w.Focus),
		"window.blur":         byID(w.Blur),
		"window.hide":         byID(w.Hide),
		"window.show":         byID(w.Show),
		"window.minimize":     byID(w.Minimize),
		"window.maximize":     byID(w.Maximize),
		"window.unmaximize":   byID(w.Unmaximize),
		"window.fullscreen":   byID(w.Fullscreen),
		"window.unfullscreen": byID(w.Unfullscreen),
		"window.restore":      byID(w.Restore),
	})
}

// nativeItems converts menu items for the host; items without a menu go in
// an "App" menu.
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

func (a *App) bindMenu() error {
	primary := a.opts.Window.ID
	menus := &desktop.MenuService{Gateway: a.rt, Host: a.host}
	if h, ok := a.host.(platform.MenuBar); ok {
		menus.OnSet = func(_ context.Context, items []desktop.MenuItem) error {
			return h.SetMenuBar(primary, nativeItems(items, "App"))
		}
		menus.OnClear = func(context.Context) error { return h.SetMenuBar(primary, nil) }
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"menu.set": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			items, err := desktop.ParseMenuItems(input)
			if err != nil {
				return nil, err
			}
			return nil, menus.SetMenu(ctx, caller, items)
		},
		"menu.clear": func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
			return nil, menus.ClearMenu(ctx, caller)
		},
	})
}

func (a *App) bindTray() error {
	trays := &desktop.TrayService{Gateway: a.rt, Host: a.host}
	if h, ok := a.host.(platform.Tray); ok {
		trays.OnSet = func(_ context.Context, tooltip string, items []desktop.MenuItem) error {
			return h.SetTray(tooltip, nativeItems(items, ""))
		}
		trays.OnClear = func(context.Context) error { h.ClearTray(); return nil }
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"tray.set": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			tooltip, items, err := desktop.ParseTraySet(input)
			if err != nil {
				return nil, err
			}
			return nil, trays.SetTray(ctx, caller, tooltip, items)
		},
		"tray.clear": func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
			return nil, trays.ClearTray(ctx, caller)
		},
	})
}

func (a *App) bindDragDrop() error {
	drops := &desktop.DragDropService{Gateway: a.rt, Host: a.host}
	if h, ok := a.host.(platform.DragDrop); ok {
		drops.OnEnable = func(_ context.Context, id domain.WindowID, on bool) error { return h.EnableDragDrop(id, on) }
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"dragdrop.receive": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			id, on, err := desktop.ParseDragDropEnable(input)
			if err != nil {
				return nil, err
			}
			return nil, drops.Enable(ctx, caller, id, on)
		},
	})
}

func (a *App) bindShortcut() error {
	shortcuts := &desktop.ShortcutService{Gateway: a.rt, Host: a.host}
	if h, ok := a.host.(platform.GlobalShortcuts); ok {
		shortcuts.OnRegister = func(_ context.Context, accelerator, action string) error {
			return h.RegisterGlobalShortcut(accelerator, action)
		}
		shortcuts.OnUnregister = func(_ context.Context, accelerator string) error {
			return h.UnregisterGlobalShortcut(accelerator)
		}
	}
	return a.bind(map[domain.CommandName]callerFunc{
		"shortcut.register": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			acc, action, err := desktop.ParseShortcutRegister(input)
			if err != nil {
				return nil, err
			}
			return nil, shortcuts.Register(ctx, caller, acc, action)
		},
		"shortcut.unregister": func(ctx context.Context, caller domain.Caller, input any) (any, error) {
			acc, err := desktop.ParseShortcutUnregister(input)
			if err != nil {
				return nil, err
			}
			return nil, shortcuts.Unregister(ctx, caller, acc)
		},
	})
}

func (a *App) bindApp() error {
	appSvc := &desktop.AppService{Gateway: a.rt, OnQuit: func(context.Context) error { a.Quit(); return nil }}
	return a.bind(map[domain.CommandName]callerFunc{
		"app.quit": func(ctx context.Context, caller domain.Caller, _ any) (any, error) {
			return nil, appSvc.Quit(ctx, caller)
		},
	})
}
