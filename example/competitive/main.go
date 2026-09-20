package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/linux"
)

//go:embed frontend/*
var frontendRoot embed.FS

type allowGateway map[domain.PermissionName]struct{}

func (g allowGateway) Authorize(caller domain.Caller, permission domain.PermissionName, _ string) domain.Decision {
	if caller.Origin != domain.OriginPackagedLocal {
		return domain.Decision{Permission: permission, Code: domain.DenialOriginMismatch, Reason: "origin not trusted"}
	}
	if _, ok := g[permission]; ok {
		return domain.Decision{Allowed: true, Permission: permission}
	}
	return domain.Decision{Permission: permission, Code: domain.DenialPermissionAbsent, Reason: "permission not granted to desktop gateway"}
}

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
	gw := allowGateway{
		desktop.PermMenuSet:        {},
		desktop.PermTraySet:        {},
		desktop.PermDialogOpen:     {},
		desktop.PermDialogSave:     {},
		desktop.PermClipboardRead:  {},
		desktop.PermClipboardWrite: {},
		desktop.PermSingleInstance: {},
		desktop.PermDeepLinkHandle: {},
	}

	menus := &desktop.MenuService{
		Gateway: gw,
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
		Gateway: gw,
		Host:    host,
		OnSet: func(ctx context.Context, tooltip string, _ []desktop.MenuItem) error {
			return host.SetTray(tooltip)
		},
	}
	dialogs := &desktop.DialogService{
		Gateway: gw,
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
		Gateway: gw,
		Host:    host,
		OnRead:  func(ctx context.Context) (string, error) { return host.ClipboardGet() },
		OnWrite: func(ctx context.Context, text string) error { return host.ClipboardSet(text) },
	}
	single := &desktop.SingleInstanceService{
		Gateway: gw,
		Host:    host,
		OnLock:  func(ctx context.Context) (bool, error) { return true, nil },
	}
	if ok, err := single.Acquire(context.Background(), caller); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("single-instance acquire failed")
	}

	deepLinks := &desktop.DeepLinkService{
		Gateway:  gw,
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
		// Surface to the UI when the window is up.
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
	if err := register("clipboard.read", "Read clipboard", "clipboard.read", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return clips.Read(ctx, caller)
	})); err != nil {
		return err
	}
	if err := register("clipboard.write", "Write clipboard", "clipboard.write", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, input any) (any, error) {
		text, _ := input.(string)
		return nil, clips.Write(ctx, caller, text)
	})); err != nil {
		return err
	}
	if err := register("dialog.open", "Open file dialog", "dialog.open", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.OpenFile(ctx, caller)
	})); err != nil {
		return err
	}
	if err := register("dialog.save", "Save file dialog", "dialog.save", domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		return dialogs.SaveFile(ctx, caller)
	})); err != nil {
		return err
	}
	chromeGrant, err := domain.NewCapabilityGrant(
		"desktop-chrome",
		"clipboard and dialogs",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "clipboard.read"},
			{Name: "clipboard.write"},
			{Name: "dialog.open"},
			{Name: "dialog.save"},
		},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(chromeGrant); err != nil {
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
		if id == "app.quit" || id == "tray.activate" {
			application.Quit()
		}
	})

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = menus.SetMenu(context.Background(), caller, []desktop.MenuItem{
			{Menu: "File", ID: "app.quit", Label: "Quit"},
			{Menu: "Help", ID: "help.about", Label: "About Vitra"},
		})
		_ = trays.SetTray(context.Background(), caller, "Vitra competitive demo", nil)
		if os.Getenv("VITRA_E2E") == "1" {
			js := `window.vitra.invoke("demo.greet","E2E").then(function(r){document.getElementById("out").textContent=JSON.stringify(r,null,2);}).catch(function(e){document.getElementById("out").textContent=String(e);});`
			_ = host.Eval("main", js)
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
