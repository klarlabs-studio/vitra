package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func runDev(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	fmt.Println("vitra dev: watching for .go/.html/.css/.js changes (ctrl-c to stop)")
	var (
		cmd   *exec.Cmd
		stamp = map[string]time.Time{}
		first = true
	)
	restart := func() error {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		argsGo := appendNativeHostTags([]string{"run"})
		argsGo = append(argsGo, ".")
		cmd = exec.Command("go", argsGo...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("vitra dev: starting…")
		return cmd.Start()
	}
	for {
		changed := first
		first = false
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && (d.Name() == ".git" || d.Name() == "node_modules") {
					return fs.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".go", ".html", ".css", ".js", ".json":
			default:
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			prev, ok := stamp[path]
			if !ok || info.ModTime().After(prev) {
				stamp[path] = info.ModTime()
				if ok {
					changed = true
				}
			}
			return nil
		})
		if changed {
			if err := restart(); err != nil {
				fmt.Fprintf(os.Stderr, "vitra dev: start failed: %v\n", err)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func runBuild(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	argsGo := appendNativeHostTags([]string{"build"})
	argsGo = append(argsGo, "-o", "vitra-app", ".")
	return execGo(dir, argsGo...)
}

// appendNativeHostTags adds -tags vitra_native on desktop OSes that ship a
// DesktopHost adapter (Linux WebKitGTK, Darwin WKWebView, Windows WebView2).
func appendNativeHostTags(args []string) []string {
	if supportsNativeHostTag(runtime.GOOS) {
		return append(args, "-tags", "vitra_native")
	}
	return args
}

func supportsNativeHostTag(goos string) bool {
	switch goos {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

func execGo(dir string, goArgs ...string) error {
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
