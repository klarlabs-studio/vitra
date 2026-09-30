package app_test

import (
	"context"
	"embed"
	"io/fs"
	"log"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/desktop"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform/linux"
)

var frontend embed.FS

// Turn on the official desktop plugins, then grant the main window only what
// it needs: writing to the clipboard and opening files. Every other official
// command exists but is refused.
func ExampleApp_UseOfficialPlugins() {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.notes"})
	if err != nil {
		log.Fatal(err)
	}
	grant, err := domain.NewCapabilityGrant("main-window", "clipboard and open dialog",
		[]domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: desktop.PermClipboardWrite},
			{Name: desktop.PermDialogOpen},
		})
	if err != nil {
		log.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		log.Fatal(err)
	}

	assets, _ := fs.Sub(frontend, "frontend")
	a, err := app.New(app.Options{
		AppID:   "com.example.notes",
		Assets:  assets,
		Host:    linux.New(), // darwin.New() or windows.New() on those systems
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main", Title: "Notes"},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := a.UseOfficialPlugins(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := a.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
