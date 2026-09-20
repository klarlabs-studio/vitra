package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/linux"
	officialdialog "go.klarlabs.de/vitra/plugin/official/dialog"
	officialfs "go.klarlabs.de/vitra/plugin/official/fs"
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
	host := linux.New()
	const appID = "com.vitra.competitive"

	if os.Getenv("VITRA_REGISTER_SCHEME") == "1" {
		execPath, err := os.Executable()
		if err != nil {
			return err
		}
		if err := host.RegisterURLScheme("vitra", appID, execPath); err != nil {
			return fmt.Errorf("register URL scheme: %w", err)
		}
		fmt.Println("registered xdg handler for vitra://")
	}

	held, release, err := host.TrySingleInstance(appID)
	if err != nil {
		return fmt.Errorf("single-instance: %w", err)
	}
	if !held {
		urls := linux.DeepLinksFromArgs(os.Args[1:])
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
		"clipboard, dialogs, menu, tray, single-instance, deeplink",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: desktop.PermClipboardRead},
			{Name: desktop.PermClipboardWrite},
			{Name: desktop.PermDialogOpen},
			{Name: desktop.PermDialogSave},
			{Name: desktop.PermMenuSet},
			{Name: desktop.PermTraySet},
			{Name: desktop.PermSingleInstance},
			{Name: desktop.PermDeepLinkHandle},
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
				native = append(native, platform.MenuItem{Menu: menu, ID: it.ID, Label: it.Label})
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
	}
	dialogs := &desktop.DialogService{
		Gateway: rt,
		Host:    host,
		OnOpen: func(ctx context.Context) ([]string, error) {
			path, err := host.OpenFileDialog()
			if err != nil || path == "" {
				return nil, err
			}
			return []string{path}, nil
		},
		OnSave: func(ctx context.Context) (string, error) {
			return host.SaveFileDialog()
		},
	}
	clips := &desktop.ClipboardService{
		Gateway: rt,
		Host:    host,
		OnRead:  func(ctx context.Context) (string, error) { return host.ClipboardGet() },
		OnWrite: func(ctx context.Context, text string) error { return host.ClipboardSet(text) },
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
	for _, u := range linux.DeepLinksFromArgs(os.Args[1:]) {
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
	if err := register("clipboard.read", "Read clipboard", desktop.PermClipboardRead, domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return clips.Read(ctx, caller)
	})); err != nil {
		return err
	}
	if err := register("clipboard.write", "Write clipboard", desktop.PermClipboardWrite, domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		text, _ := input.(string)
		return nil, clips.Write(ctx, caller, text)
	})); err != nil {
		return err
	}

	// Official plugins own dialog.* / fs.* permissions (invariant 6).
	if err := rt.RegisterPlugin(context.Background(), officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(context.Background(), officialfs.New()); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.OpenFile(ctx, caller)
	})); err != nil {
		return err
	}
	if err := rt.BindExecutor("dialog.save", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.SaveFile(ctx, caller)
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

	application, err := app.New(app.Options{
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
		if id == "app.quit" || id == "tray.quit" || id == "tray.activate" {
			application.Quit()
		}
	})

	go func() {
		// Wait until the GTK loop is up; cold WebKit on CI can exceed 500ms.
		time.Sleep(1500 * time.Millisecond)
		_ = menus.SetMenu(context.Background(), caller, []desktop.MenuItem{
			{Menu: "File", ID: "app.quit", Label: "Quit"},
			{Menu: "Help", ID: "help.about", Label: "About Vitra"},
		})
		_ = trays.SetTray(context.Background(), caller, "Vitra competitive demo", []desktop.MenuItem{
			{ID: "help.about", Label: "About Vitra"},
			{ID: "tray.quit", Label: "Quit"},
		})
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
				application.Quit()
			}()
		}
	}

	fmt.Println("starting competitive desktop runtime…")
	return application.Run(context.Background())
}
