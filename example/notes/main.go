// Command notes is a Markdown notes app that shows Vitra's security model at
// work: the page can read the vault and write Markdown notes in it, and
// nothing else. The "Try to break it" panel attempts real attacks, and the
// audit panel shows each one being refused as it happens.
//
//	CGO_ENABLED=1 go run -tags vitra_native ./example/notes
//	CGO_ENABLED=1 go run -tags vitra_native ./example/notes -vault /path/to/folder
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/platform/darwin"
	"go.klarlabs.de/vitra/platform/linux"
	"go.klarlabs.de/vitra/platform/windows"
)

const appID = "de.klarlabs.vitra.notes"

var (
	//go:embed frontend
	frontendFS embed.FS
	// seedFS holds the notes a new, empty vault starts with.
	//
	//go:embed all:seed
	seedFS embed.FS
)

func main() {
	home, _ := os.UserHomeDir()
	vault := flag.String("vault", filepath.Join(home, "VitraNotes"), "folder the app may read and write notes in")
	flag.Parse()
	if err := run(*vault); err != nil {
		fmt.Fprintln(os.Stderr, "notes:", err)
		os.Exit(1)
	}
}

func run(vault string) error {
	root, err := openVault(vault)
	if err != nil {
		return err
	}
	a, _, err := newApp(root, newHost())
	if err != nil {
		return err
	}
	fmt.Println("vault:", root)
	return a.Run(context.Background())
}

// newApp wires the runtime, grants, commands, and live audit log for a vault
// at root (absolute, symlinks resolved).
func newApp(root string, host app.DesktopHost) (*app.App, *liveAudit, error) {
	rt, err := vitra.New(vitra.Config{AppID: appID})
	if err != nil {
		return nil, nil, err
	}
	log := &liveAudit{}
	rt.SetAudit(log)
	if err := grantVault(rt, root); err != nil {
		return nil, nil, err
	}
	if err := registerCommands(rt, root, log); err != nil {
		return nil, nil, err
	}
	assets, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		return nil, nil, err
	}
	a, err := app.New(app.Options{
		AppID:   appID,
		Title:   "Vitra Notes",
		Assets:  assets,
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: mainWindow, Width: 1180, Height: 760},
	})
	if err != nil {
		return nil, nil, err
	}
	log.Subscribe(func(e audit.Event) {
		_ = a.Emit(context.Background(), auditEvent, e)
	})
	return a, log, nil
}

// openVault creates the vault if needed, seeds an empty one with the welcome
// notes, and returns its real path in slash form. Path scopes compare
// strings, so the grant must name the path the files really live at.
func openVault(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		seed, _ := fs.Sub(seedFS, "seed")
		if err := os.CopyFS(real, seed); err != nil {
			return "", fmt.Errorf("seed vault: %w", err)
		}
	}
	return filepath.ToSlash(real), nil
}

func newHost() app.DesktopHost {
	switch runtime.GOOS {
	case "darwin":
		h := darwin.New()
		h.SetProgramName("vitra-notes")
		return h
	case "windows":
		h := windows.New()
		h.SetProgramName("vitra-notes")
		return h
	default:
		h := linux.New()
		h.SetProgramName("vitra-notes")
		return h
	}
}
