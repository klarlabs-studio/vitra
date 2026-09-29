package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bindings"
)

func scaffoldTypeScriptClient() (string, error) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.app"})
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	if err := rt.RegisterPlugin(ctx, officialfs.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdialog.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialclipboard.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialbrowser.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialos.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialnotification.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialpath.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialwindow.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialmenu.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialtray.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdragdrop.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialdeeplink.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialshortcut.New()); err != nil {
		return "", err
	}
	if err := rt.RegisterPlugin(ctx, officialapp.New()); err != nil {
		return "", err
	}
	greet, err := domain.NewCommandDefinition("demo.greet", "Greet", "demo.greet")
	if err != nil {
		return "", err
	}
	cmds := []*domain.CommandDefinition{greet}
	var events []domain.EventName
	for _, reg := range rt.Plugins().List() {
		cmds = append(cmds, reg.Contribution.Commands...)
		events = append(events, reg.Contribution.Events...)
	}
	return bindings.GenerateTypeScript("vitra", vitra.Version, bindings.Untyped(cmds...), events)
}

// generateFromApp runs the app in dir in code-generation mode (see
// app.EnvGenerateTypeScript) so the client covers the app's own typed
// commands, then writes it to outPath or stdout.
func generateFromApp(dir, outPath string) error {
	target := outPath
	if target == "" {
		f, err := os.CreateTemp("", "vitra-client-*.ts")
		if err != nil {
			return err
		}
		_ = f.Close()
		defer func() { _ = os.Remove(f.Name()) }()
		target = f.Name()
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), app.EnvGenerateTypeScript+"="+abs)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run app in %s: %w", dir, err)
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("app did not write a client (does it reach app.Run?): %w", err)
	}
	if outPath == "" {
		fmt.Print(string(body))
		return nil
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}

func runGenerate(args []string) error {
	if len(args) == 0 || args[0] != "typescript" {
		return fmt.Errorf("usage: vitra generate typescript [--app dir] [--out path] [--module name]")
	}
	outPath := ""
	module := "vitra"
	appDir := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--app":
			i++
			if i >= len(args) {
				return fmt.Errorf("--app requires a directory")
			}
			appDir = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return fmt.Errorf("--out requires a path")
			}
			outPath = args[i]
		case "--module":
			i++
			if i >= len(args) {
				return fmt.Errorf("--module requires a name")
			}
			module = args[i]
		default:
			return fmt.Errorf("unknown generate flag %q", args[i])
		}
	}
	if appDir != "" {
		return generateFromApp(appDir, outPath)
	}

	rt, err := vitra.New(vitra.Config{AppID: "com.vitra.generate"})
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
	var cmds []*domain.CommandDefinition
	var events []domain.EventName
	for _, reg := range rt.Plugins().List() {
		cmds = append(cmds, reg.Contribution.Commands...)
		events = append(events, reg.Contribution.Events...)
	}
	body, err := bindings.GenerateTypeScript(module, vitra.Version, bindings.Untyped(cmds...), events)
	if err != nil {
		return err
	}
	if outPath == "" {
		fmt.Print(body)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d commands)\n", outPath, len(cmds))
	return nil
}
