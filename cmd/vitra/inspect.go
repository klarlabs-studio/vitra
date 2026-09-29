package main

import (
	"context"
	"fmt"

	officialapp "go.klarlabs.de/vitra/plugin/official/app"
	officialbrowser "go.klarlabs.de/vitra/plugin/official/browser"
	officialclipboard "go.klarlabs.de/vitra/plugin/official/clipboard"
	officialdeeplink "go.klarlabs.de/vitra/plugin/official/deeplink"
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

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

func inspectDemo(args []string) error {
	if len(args) == 0 || args[0] != "capabilities" {
		return fmt.Errorf("usage: vitra inspect capabilities")
	}
	rt, err := vitra.New(vitra.Config{AppID: "com.example.myapp"})
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialclipboard.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialbrowser.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialos.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialwindow.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialmenu.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialtray.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdragdrop.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialdeeplink.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialshortcut.New()); err != nil {
		return err
	}
	if err := rt.RegisterPlugin(ctx, officialapp.New()); err != nil {
		return err
	}
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		return err
	}
	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"project file access",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{
				Name: "fs.read",
				PathScope: &domain.PathScope{
					Allow: []string{"/project/**"},
					Deny:  []string{"/project/.secrets/**"},
				},
			},
			{Name: "dialog.open"},
			{Name: "clipboard.read"},
			{Name: "browser.open"},
			{Name: "os.info"},
			{Name: "notifications.show"},
			{Name: "path.open", PathScope: &domain.PathScope{Allow: []string{"/project/**"}}},
			{Name: "window.create"},
			{Name: "window.close"},
			{Name: "window.chrome"},
			{Name: "menu.set"},
			{Name: "tray.set"},
			{Name: "dragdrop.receive"},
			{Name: "shortcut.register"},
			{Name: "app.quit"},
		},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(grant); err != nil {
		return err
	}
	surface, err := rt.InspectCapabilities("main")
	if err != nil {
		return err
	}
	fmt.Print(vitra.FormatInspectFull(rt.AppID(), rt.Plugins().InspectSurface(), surface))
	return nil
}
