package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bindings"
	"go.klarlabs.de/vitra/plugin/official"
)

// GreetRequest and Greeting mirror the starter's command types in
// templates/main.go.tmpl; TestScaffold_ClientMatchesGeneratedApp keeps them
// in sync.
type GreetRequest struct {
	Name string `json:"name"`
}

type Greeting struct {
	Message string `json:"message"`
}

// DataDirRequest and DataDir mirror the dataDir command a `--with fs`
// starter registers.
type DataDirRequest struct{}

type DataDir struct {
	Path string `json:"path"`
}

// scaffoldTypeScriptClient returns the client for the starter app with the
// given plugins, the same one `vitra generate typescript --app` writes for
// it, so a new app's frontend builds before Go is ever run.
func scaffoldTypeScriptClient(plugins []scaffoldPlugin) (string, error) {
	const appID = "com.example.app"
	rt, err := vitra.New(vitra.Config{AppID: appID})
	if err != nil {
		return "", err
	}
	if err := vitra.Register(rt, vitra.Command[GreetRequest, Greeting]{
		Name:        "greet",
		Description: "Greet someone by name",
		Permission:  "greet",
		Handler: func(context.Context, domain.Invocation, GreetRequest) (Greeting, error) {
			return Greeting{}, nil
		},
	}); err != nil {
		return "", err
	}
	for _, p := range plugins {
		if p.name != "fs" {
			continue
		}
		if err := vitra.Register(rt, vitra.Command[DataDirRequest, DataDir]{
			Name:        "dataDir",
			Description: "The folder the page may read and write",
			Permission:  "dataDir",
			Handler: func(context.Context, domain.Invocation, DataDirRequest) (DataDir, error) {
				return DataDir{}, nil
			},
		}); err != nil {
			return "", err
		}
	}
	for _, p := range plugins {
		if err := rt.RegisterPlugin(context.Background(), p.new()); err != nil {
			return "", err
		}
	}
	return rt.TypeScript(appID)
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
	for _, p := range official.All() {
		if err := rt.RegisterPlugin(ctx, p); err != nil {
			return err
		}
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
