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
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/linux"
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

	var e2eOK atomic.Bool

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

	clipRead, _ := domain.NewCommandDefinition("clipboard.read", "Read clipboard", "clipboard.read")
	_ = rt.RegisterCommand(clipRead, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		if err := platform.Require(host, platform.FeatureClipboard); err != nil {
			return nil, err
		}
		return host.ClipboardGet()
	}))
	clipWrite, _ := domain.NewCommandDefinition("clipboard.write", "Write clipboard", "clipboard.write")
	_ = rt.RegisterCommand(clipWrite, domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		if err := platform.Require(host, platform.FeatureClipboard); err != nil {
			return nil, err
		}
		text, _ := input.(string)
		return nil, host.ClipboardSet(text)
	}))
	clipGrant, _ := domain.NewCapabilityGrant(
		"clipboard",
		"clipboard access",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "clipboard.read"}, {Name: "clipboard.write"}},
	)
	_ = rt.RegisterGrant(clipGrant)

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
		_ = host.SetMenuBar("main", []linux.MenuItem{
			{Menu: "File", ID: "app.quit", Label: "Quit"},
			{Menu: "Help", ID: "help.about", Label: "About Vitra"},
		})
		_ = host.SetTray("Vitra competitive demo")
		if os.Getenv("VITRA_E2E") == "1" {
			js := `window.vitra.invoke("demo.greet","E2E").then(function(r){document.getElementById("out").textContent=JSON.stringify(r,null,2);}).catch(function(e){document.getElementById("out").textContent=String(e);});`
			_ = host.Eval("main", js)
		}
	}()

	if secs := os.Getenv("VITRA_DEMO_SECONDS"); secs != "" {
		var n int
		fmt.Sscanf(secs, "%d", &n)
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
