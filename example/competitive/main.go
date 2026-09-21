package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
	officialapp "go.klarlabs.de/vitra/plugin/official/app"
	officialbrowser "go.klarlabs.de/vitra/plugin/official/browser"
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialdragdrop "go.klarlabs.de/vitra/plugin/official/dragdrop"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
	officialmenu "go.klarlabs.de/vitra/plugin/official/menu"
	officialnotification "go.klarlabs.de/vitra/plugin/official/notification"
	officialos "go.klarlabs.de/vitra/plugin/official/os"
	officialpath "go.klarlabs.de/vitra/plugin/official/path"
	officialshortcut "go.klarlabs.de/vitra/plugin/official/shortcut"
	officialtray "go.klarlabs.de/vitra/plugin/official/tray"
	officialwindow "go.klarlabs.de/vitra/plugin/official/window"
	"go.klarlabs.de/vitra/policy"
)

//go:embed frontend/*
var frontendRoot embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "competitive demo: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	assets, err := fs.Sub(frontendRoot, "frontend")
	if err != nil {
		return err
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.vitra.competitive"})
	if err != nil {
		return err
	}
	if err := installOptionalPolicy(rt); err != nil {
		return err
	}
	if err := installOptionalAudit(rt); err != nil {
		return err
	}
	host := newDesktopHost()
	const appID = "com.vitra.competitive"

	if os.Getenv("VITRA_REGISTER_SCHEME") == "1" {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		if err := host.RegisterURLScheme("vitra", appID, execPath); err != nil {
			return fmt.Errorf("register URL scheme: %w", err)
		}
		fmt.Println("registered URL scheme handler for vitra://")
	}
	if raw := os.Getenv("VITRA_REGISTER_FILES"); raw != "" {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		var mimes []string
		for _, m := range strings.Split(raw, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				mimes = append(mimes, m)
			}
		}
		if err := host.RegisterFileAssociations(appID, execPath, "Vitra Competitive", mimes); err != nil {
			return fmt.Errorf("register file associations: %w", err)
		}
		fmt.Println("registered file associations:", strings.Join(mimes, ","))
	}

	held, release, err := host.TrySingleInstance(appID)
	if err != nil {
		return fmt.Errorf("single-instance: %w", err)
	}
	if !held {
		urls := deepLinksFromArgs(os.Args[1:])
		if len(urls) == 0 {
			return fmt.Errorf("another Vitra competitive instance is already running")
		}
		ok, err := host.ForwardToPrimary(appID, urls)
		if err != nil {
			return fmt.Errorf("deep-link handoff: %w", err)
		}
		if !ok {
			return fmt.Errorf("primary instance is running but deep-link bridge is unavailable")
		}
		fmt.Println("forwarded deep link(s) to primary instance")
		return nil
	}
	defer release()

	var e2eOK atomic.Bool
	caller, err := domain.NewCaller("main", domain.OriginPackagedLocal)
	if err != nil {
		return err
	}

	chromeGrant, err := domain.NewCapabilityGrant(
		"desktop-chrome",
		"clipboard, dialogs, menu, tray, shortcuts, single-instance, deeplink, drag-drop, window chrome/create/close, open url, os info, notifications",
		[]domain.WindowID{"main", "aux"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: desktop.PermClipboardRead},
			{Name: desktop.PermClipboardWrite},
			{Name: desktop.PermDialogOpen},
			{Name: desktop.PermDialogSave},
			{Name: desktop.PermDialogOpenDirectory},
			{Name: desktop.PermDialogMessage},
			{Name: desktop.PermMenuSet},
			{Name: desktop.PermTraySet},
			{Name: desktop.PermShortcutRegister},
			{Name: desktop.PermAppQuit},
			{Name: desktop.PermSingleInstance},
			{Name: desktop.PermDeepLinkHandle},
			{Name: desktop.PermDragDrop},
			{Name: desktop.PermWindowChrome},
			{Name: desktop.PermWindowCreate},
			{Name: desktop.PermWindowClose},
			{Name: desktop.PermOpenURL},
			{Name: desktop.PermOsInfo},
			{Name: desktop.PermNotificationShow},
			{Name: desktop.PermPathOpen, PathScope: &domain.PathScope{Allow: []string{"/**"}}},
		},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(chromeGrant); err != nil {
		return err
	}

	menus := &desktop.MenuService{
		Gateway: rt,
		Host:    host,
		OnSet: func(ctx context.Context, items []desktop.MenuItem) error {
			native := make([]platform.MenuItem, 0, len(items))
			for _, it := range items {
				menu := it.Menu
				if menu == "" {
					menu = "App"
				}
				native = append(native, platform.MenuItem{Menu: menu, ID: it.ID, Label: it.Label, Shortcut: it.Shortcut})
			}
			return host.SetMenuBar("main", native)
		},
	}
	trays := &desktop.TrayService{
		Gateway: rt,
		Host:    host,
		OnSet: func(ctx context.Context, tooltip string, items []desktop.MenuItem) error {
			native := make([]platform.MenuItem, 0, len(items))
			for _, it := range items {
				native = append(native, platform.MenuItem{ID: it.ID, Label: it.Label})
			}
			return host.SetTray(tooltip, native)
		},
		OnClear: func(ctx context.Context) error {
			host.ClearTray()
			return nil
		},
	}
	dialogs := &desktop.DialogService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, opts platform.DialogFileOptions) ([]string, error) {
			path, err := host.OpenFileDialog(opts)
			if err != nil || path == "" {
				return nil, err
			}
			return []string{path}, nil
		},
		OnSave: func(ctx context.Context, opts platform.DialogFileOptions) (string, error) {
			return host.SaveFileDialog(opts)
		},
		OnOpenDirectory: func(ctx context.Context) (string, error) {
			return host.OpenDirectoryDialog()
		},
		OnMessage: func(ctx context.Context, title, message, kind string) (bool, error) {
			return host.MessageDialog(title, message, kind)
		},
	}
	clips := &desktop.ClipboardService{
		Gateway: rt,
		Host:    host,
		OnRead:  func(ctx context.Context) (string, error) { return host.ClipboardGet() },
		OnWrite: func(ctx context.Context, text string) error { return host.ClipboardSet(text) },
	}
	shortcuts := &desktop.ShortcutService{
		Gateway: rt,
		Host:    host,
		OnRegister: func(_ context.Context, accelerator, actionID string) error {
			return host.RegisterGlobalShortcut(accelerator, actionID)
		},
		OnUnregister: func(_ context.Context, accelerator string) error {
			return host.UnregisterGlobalShortcut(accelerator)
		},
	}
	single := &desktop.SingleInstanceService{
		Gateway: rt,
		Host:    host,
		OnLock:  func(ctx context.Context) (bool, error) { return true, nil },
	}
	if ok, err := single.Acquire(context.Background(), caller); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("single-instance acquire failed")
	}

	deepLinks := &desktop.DeepLinkService{
		Gateway:  rt,
		Host:     host,
		Patterns: []domain.DeepLinkPattern{{Scheme: "vitra"}},
	}
	handleDeepLink := func(raw string) {
		ok, err := deepLinks.Handle(caller, raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "deep link rejected: %v\n", err)
			return
		}
		if !ok {
			return
		}
		fmt.Println("deep link:", raw)
		js := fmt.Sprintf(`(function(){var el=document.getElementById("out");if(el){el.textContent=%q;}})()`, "deep link: "+raw)
		_ = host.Eval("main", js)
	}
	stopBridge, err := host.StartDeepLinkBridge(appID, handleDeepLink)
	if err != nil {
		return fmt.Errorf("deep-link bridge: %w", err)
	}
	defer stopBridge()
	for _, u := range deepLinksFromArgs(os.Args[1:]) {
		handleDeepLink(u)
	}

	greet, err := domain.NewCommandDefinition("demo.greet", "Greet the user", "demo.greet")
	if err != nil {
		return err
	}
	if err := rt.RegisterCommand(greet, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		who, _ := input.(string)
		if who == "" {
			who = "Vitra"
		}
		if who == "E2E" {
			e2eOK.Store(true)
			fmt.Println("VITRA_E2E_OK")
		}
		return map[string]any{
			"message": "Hello from native Go, " + who,
			"at":      time.Now().UTC().Format(time.RFC3339),
		}, nil
	})); err != nil {
		return err
	}
	grant, err := domain.NewCapabilityGrant(
		"demo",
		"demo surface",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "demo.greet"}},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(grant); err != nil {
		return err
	}

	register := func(name, desc string, perm domain.PermissionName, exec domain.CommandExecutor) error {
		def, err := domain.NewCommandDefinition(domain.CommandName(name), desc, perm)
		if err != nil {
			return err
		}
		return rt.RegisterCommand(def, exec)
	}
	_ = register // kept for local demo commands if needed

	// Official plugins own dialog.* / fs.* / clipboard.* / browser.* / os.* / notifications.* / path.* / window.* permissions (invariant 6).
	if err := rt.RegisterPlugin(context.Background(), officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialfs.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialclipboard.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialbrowser.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialos.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialnotification.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialpath.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialwindow.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialmenu.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialtray.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialdragdrop.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialshortcut.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialapp.New()); err != nil {
		return err
	}
	if err := rt.BindExecutor("clipboard.read", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return clips.Read(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("clipboard.write", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		text, _ := input.(string)
		return nil, clips.Write(ctx, caller, text)
	})); err != nil {
		return err
	}
	browserSvc := &desktop.BrowserService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, rawURL string) error {
			return host.OpenURL(ctx, rawURL)
		},
	}
	if err := rt.BindExecutor("browser.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		rawURL, _ := input.(string)
		return nil, browserSvc.OpenURL(ctx, caller, rawURL)
	})); err != nil {
		return err
	}
	osSvc := &desktop.OsService{Gateway: rt}
	if err := rt.BindExecutor("os.info", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return osSvc.Info(ctx, caller)
	})); err != nil {
		return err
	}
	notifs := &desktop.NotificationService{
		Gateway: rt,
		Host:    host,
		OnShow: func(ctx context.Context, title, body string) error {
			return host.ShowNotification(title, body)
		},
	}
	if err := rt.BindExecutor("notifications.show", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		title, body := "", ""
		switch v := input.(type) {
		case string:
			body = v
		case map[string]any:
			title, _ = v["title"].(string)
			body, _ = v["body"].(string)
		}
		return nil, notifs.Show(ctx, caller, title, body)
	})); err != nil {
		return err
	}
	pathSvc := &desktop.PathService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context, path string) error {
			return host.OpenPath(ctx, path)
		},
	}
	if err := rt.BindExecutor("path.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		path, _ := input.(string)
		return nil, pathSvc.Open(ctx, caller, path)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		return dialogs.OpenFile(ctx, caller, desktop.ParseDialogFileOptions(input))
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.save", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		return dialogs.SaveFile(ctx, caller, desktop.ParseDialogFileOptions(input))
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.openDirectory", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.OpenDirectory(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.message", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
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
	})); err != nil {
		return err
	}

	demoRoot := filepath.Join(os.TempDir(), "vitra-competitive-fs")
	if err := os.MkdirAll(demoRoot, 0o755); err != nil {
		return err
	}
	fsGrant, err := domain.NewCapabilityGrant(
		"demo-files",
		"scoped demo filesystem",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: desktop.PermFSRead, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
			{Name: desktop.PermFSWrite, PathScope: &domain.PathScope{Allow: []string{demoRoot + "/**"}}},
		},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(fsGrant); err != nil {
		return err
	}
	files := &desktop.FileService{Gateway: rt}
	if err := rt.BindExecutor("fs.read", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		path, _ := input.(string)
		data, err := files.Read(ctx, caller, path)
		if err != nil {
			return nil, err
		}
		return string(data), nil
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("fs.write", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		m, _ := input.(map[string]any)
		path, _ := m["path"].(string)
		data, _ := m["data"].(string)
		return nil, files.Write(ctx, caller, path, []byte(data))
	})); err != nil {
		return err
	}

	var application *app.App
	winSvc := &desktop.WindowService{
		Gateway: rt,
		Host:    host,
		OnApply: func(_ context.Context, window domain.WindowID, chrome platform.WindowChrome) error {
			return host.ApplyWindowChrome(window, chrome)
		},
		OnRead: func(_ context.Context, window domain.WindowID) (platform.WindowChrome, error) {
			return host.ReadWindowChrome(window)
		},
		OnCreate: func(ctx context.Context, opts desktop.WindowCreateOptions) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			return application.OpenWindow(ctx, app.WindowOptions{
				ID: opts.ID, Title: opts.Title, Path: opts.Path, Width: opts.Width, Height: opts.Height,
			})
		},
		OnClose: func(ctx context.Context, id domain.WindowID) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			return application.CloseWindow(ctx, id)
		},
	}
	if err := rt.BindExecutor("window.create", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		opts, err := desktop.ParseWindowCreateOptions(input)
		if err != nil {
			return nil, err
		}
		id, err := winSvc.Create(ctx, caller, opts)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": string(id)}, nil
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.close", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, err := desktop.ParseWindowID(input)
		if err != nil {
			return nil, err
		}
		return nil, winSvc.Close(ctx, caller, id)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.chrome", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, chrome, err := desktop.ParseWindowChromeApply(input)
		if err != nil {
			return nil, err
		}
		return nil, winSvc.Apply(ctx, caller, id, chrome)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("window.getChrome", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		id, err := desktop.ParseWindowID(input)
		if err != nil {
			return nil, err
		}
		return winSvc.Read(ctx, caller, id)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("menu.set", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		items, err := desktop.ParseMenuItems(input)
		if err != nil {
			return nil, err
		}
		return nil, menus.SetMenu(ctx, caller, items)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("tray.set", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		tooltip, items, err := desktop.ParseTraySet(input)
		if err != nil {
			return nil, err
		}
		return nil, trays.SetTray(ctx, caller, tooltip, items)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("tray.clear", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return nil, trays.ClearTray(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("shortcut.register", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		acc, action, err := desktop.ParseShortcutRegister(input)
		if err != nil {
			return nil, err
		}
		return nil, shortcuts.Register(ctx, caller, acc, action)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("shortcut.unregister", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		acc, err := desktop.ParseShortcutUnregister(input)
		if err != nil {
			return nil, err
		}
		return nil, shortcuts.Unregister(ctx, caller, acc)
	})); err != nil {
		return err
	}
	appSvc := &desktop.AppService{
		Gateway: rt,
		OnQuit: func(ctx context.Context) error {
			if application == nil {
				return fmt.Errorf("app is not ready")
			}
			application.Quit()
			return nil
		},
	}
	if err := rt.BindExecutor("app.quit", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return nil, appSvc.Quit(ctx, caller)
	})); err != nil {
		return err
	}

	application, err = app.New(app.Options{
		AppID:   "com.vitra.competitive",
		Title:   "Vitra Competitive Demo",
		Assets:  assets,
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main", Width: 960, Height: 640, Path: "/"},
	})
	if err != nil {
		return err
	}

	host.SetActionHandler(func(id string) {
		fmt.Println("native action:", id)
		payload := map[string]any{"id": id}
		_ = application.Emit(context.Background(), "menu.action", payload)
		_ = application.Emit(context.Background(), "tray.action", payload)
		_ = application.Emit(context.Background(), "shortcut.action", payload)
		if id == "app.quit" || id == "tray.quit" || id == "tray.activate" {
			application.Quit()
		}
	})
	host.SetDragDropHandler(func(windowID domain.WindowID, paths []string) {
		fmt.Println("file drop:", windowID, paths)
		_ = application.Emit(context.Background(), "dragdrop.drop", map[string]any{
			"window": string(windowID),
			"paths":  paths,
		})
	})
	drops := &desktop.DragDropService{
		Gateway: rt,
		Host:    host,
		OnEnable: func(_ context.Context, window domain.WindowID, enabled bool) error {
			return host.EnableDragDrop(window, enabled)
		},
	}
	if err := rt.BindExecutor("dragdrop.receive", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		win, enabled, err := desktop.ParseDragDropEnable(input)
		if err != nil {
			return nil, err
		}
		return nil, drops.Enable(ctx, caller, win, enabled)
	})); err != nil {
		return err
	}

	go func() {
		// Wait until the GTK loop is up; cold WebKit on CI can exceed 500ms.
		time.Sleep(1500 * time.Millisecond)
		_ = menus.SetMenu(context.Background(), caller, []desktop.MenuItem{
			{Menu: "File", ID: "app.quit", Label: "Quit", Shortcut: "Ctrl+Q"},
			{Menu: "Help", ID: "help.about", Label: "About Vitra"},
		})
		_ = trays.SetTray(context.Background(), caller, "Vitra competitive demo", []desktop.MenuItem{
			{ID: "help.about", Label: "About Vitra"},
			{ID: "tray.quit", Label: "Quit"},
		})
		if err := shortcuts.Register(context.Background(), caller, "Ctrl+Shift+Q", "app.quit"); err != nil {
			fmt.Fprintf(os.Stderr, "global shortcut: %v\n", err)
		}
		if err := drops.Enable(context.Background(), caller, "main", true); err != nil {
			fmt.Fprintf(os.Stderr, "drag-drop enable: %v\n", err)
		} else if _, err := rt.SubscribeEvent("drop-sub", "dragdrop.drop", "main"); err == nil {
			if os.Getenv("VITRA_INJECT_DROP") == "1" {
				host.InjectFileDrop("main", []string{"/tmp/vitra-demo-drop.txt"})
			}
		}
		if os.Getenv("VITRA_SECOND_WINDOW") == "1" {
			if err := application.OpenWindow(context.Background(), app.WindowOptions{
				ID: "aux", Title: "Vitra Aux", Width: 480, Height: 360,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "second window: %v\n", err)
			} else {
				fmt.Println("opened second window: aux")
			}
		}
		if title := os.Getenv("VITRA_WINDOW_TITLE"); title != "" {
			winChrome := &desktop.WindowService{
				Gateway: rt,
				Host:    host,
				OnApply: func(_ context.Context, window domain.WindowID, chrome platform.WindowChrome) error {
					return host.ApplyWindowChrome(window, chrome)
				},
			}
			cur, err := host.ReadWindowChrome("main")
			if err != nil {
				fmt.Fprintf(os.Stderr, "window chrome: %v\n", err)
			} else {
				cur.Title = title
				if err := winChrome.Apply(context.Background(), caller, "main", cur); err != nil {
					fmt.Fprintf(os.Stderr, "window chrome apply: %v\n", err)
				}
			}
		}
		if openTarget := os.Getenv("VITRA_OPEN_URL"); openTarget != "" {
			if err := browserSvc.OpenURL(context.Background(), caller, openTarget); err != nil {
				fmt.Fprintf(os.Stderr, "open url: %v\n", err)
			} else {
				fmt.Println("opened url:", openTarget)
			}
		}
		if _, err := rt.SubscribeEvent("demo-tick", "demo.tick", "main"); err == nil {
			_ = application.Emit(context.Background(), "demo.tick", map[string]any{"source": "competitive"})
		}
		if os.Getenv("VITRA_E2E") == "1" {
			// Retry Eval until the preload bridge is live or the demo timer quits.
			js := `(function(){function go(){if(!window.vitra||!window.vitra.invoke){setTimeout(go,200);return;}window.vitra.invoke("demo.greet","E2E").then(function(r){var el=document.getElementById("out");if(el){el.textContent=JSON.stringify(r,null,2);}}).catch(function(e){var el=document.getElementById("out");if(el){el.textContent=String(e);}}); } go();})();`
			for i := 0; i < 20 && !e2eOK.Load(); i++ {
				_ = host.Eval("main", js)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	if secs := os.Getenv("VITRA_DEMO_SECONDS"); secs != "" {
		var n int
		if _, err := fmt.Sscanf(secs, "%d", &n); err != nil {
			return fmt.Errorf("VITRA_DEMO_SECONDS: %w", err)
		}
		if n > 0 {
			go func() {
				time.Sleep(time.Duration(n) * time.Second)
				if os.Getenv("VITRA_E2E") == "1" && !e2eOK.Load() {
					fmt.Fprintln(os.Stderr, "VITRA_E2E_FAIL: demo.greet did not complete")
				}
				if sink := rt.Audit(); sink != nil {
					fmt.Printf("audit events: %d\n", len(sink.List()))
				}
				application.Quit()
			}()
		}
	}

	fmt.Println("starting competitive desktop runtime…")
	return application.Run(context.Background())
}

// installOptionalPolicy wires Phase 5 enterprise overlay when VITRA_POLICY is set
// to "production" or "development". Optional VITRA_POLICY_FILE loads an MDM JSON
// document; VITRA_POLICY_DENY appends a comma-separated permission deny list.
func installOptionalPolicy(rt *vitra.Runtime) error {
	envName := os.Getenv("VITRA_POLICY")
	if envName == "" {
		return nil
	}
	var env policy.Environment
	switch envName {
	case string(policy.EnvProduction):
		env = policy.EnvProduction
	case string(policy.EnvDevelopment):
		env = policy.EnvDevelopment
	default:
		return fmt.Errorf("VITRA_POLICY: want %q or %q, got %q", policy.EnvProduction, policy.EnvDevelopment, envName)
	}
	doc := policy.Document{}
	if path := os.Getenv("VITRA_POLICY_FILE"); path != "" {
		loaded, err := policy.LoadDocument(path)
		if err != nil {
			return fmt.Errorf("VITRA_POLICY_FILE: %w", err)
		}
		doc = loaded
	}
	if raw := os.Getenv("VITRA_POLICY_DENY"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			doc.DenyPermissions = append(doc.DenyPermissions, domain.PermissionName(p))
		}
	}
	eng, err := policy.NewEngine(doc, env)
	if err != nil {
		return err
	}
	rt.SetPolicy(eng)
	fmt.Printf("enterprise policy: env=%s deny=%v file=%q\n", env, eng.Document().DenyPermissions, os.Getenv("VITRA_POLICY_FILE"))
	return nil
}

// installOptionalAudit wires Phase 5 audit sinks when VITRA_AUDIT is set.
// Values: "1"/"memory" (in-memory), "jsonl" (NDJSON), "cef" (Common Event Format).
// VITRA_AUDIT_PATH selects the export file (default: stderr for jsonl/cef).
func installOptionalAudit(rt *vitra.Runtime) error {
	mode := os.Getenv("VITRA_AUDIT")
	if mode == "" {
		return nil
	}
	mem := &audit.MemorySink{}
	sinks := []audit.Sink{mem}
	path := os.Getenv("VITRA_AUDIT_PATH")
	switch mode {
	case "1", "memory":
		rt.SetAudit(mem)
		fmt.Println("audit sink: memory")
		return nil
	case "jsonl":
		w, label, err := openAuditWriter(path)
		if err != nil {
			return err
		}
		sinks = append(sinks, &audit.JSONLSink{W: w})
		rt.SetAudit(&audit.MultiSink{Sinks: sinks})
		fmt.Printf("audit sink: memory+jsonl (%s)\n", label)
		return nil
	case "cef":
		w, label, err := openAuditWriter(path)
		if err != nil {
			return err
		}
		sinks = append(sinks, &audit.CEFSink{W: w})
		rt.SetAudit(&audit.MultiSink{Sinks: sinks})
		fmt.Printf("audit sink: memory+cef (%s)\n", label)
		return nil
	default:
		return fmt.Errorf("VITRA_AUDIT: want 1|memory|jsonl|cef, got %q", mode)
	}
}

func openAuditWriter(path string) (io.Writer, string, error) {
	if path == "" {
		return os.Stderr, "stderr", nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("VITRA_AUDIT_PATH: %w", err)
	}
	return f, path, nil
}

// newDesktopHost selects the OS DesktopHost adapter (Linux / Darwin / Windows).
func newDesktopHost() app.DesktopHost {
	const prog = "vitra-competitive"
	switch runtime.GOOS {
	case "darwin":
		h := darwin.New()
		h.SetProgramName(prog)
		return h
	case "windows":
		h := windows.New()
		h.SetProgramName(prog)
		return h
	default:
		h := linux.New()
		h.SetProgramName(prog)
		return h
	}
}

func deepLinksFromArgs(args []string) []string {
	switch runtime.GOOS {
	case "darwin":
		return darwin.DeepLinksFromArgs(args)
	case "windows":
		return windows.DeepLinksFromArgs(args)
	default:
		return linux.DeepLinksFromArgs(args)
	}
}
