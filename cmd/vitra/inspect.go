package main

import (
	"context"
	"fmt"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin/official"
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
	for _, p := range official.All() {
		if err := rt.RegisterPlugin(ctx, p); err != nil {
			return err
		}
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
