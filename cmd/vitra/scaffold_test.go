package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffold_EveryTemplateGeneratesACompleteApp(t *testing.T) {
	for _, tmpl := range scaffoldTemplates {
		t.Run(tmpl.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "app")
			capture(t, func() {
				if err := run([]string{"new", dir, "--template", tmpl.name}); err != nil {
					t.Fatal(err)
				}
			})
			want := []string{"go.mod", "main.go", "README.md", "frontend/vitra-client.ts"}
			embed := "//go:embed frontend/*"
			if tmpl.vite {
				want = append(want, "frontend/package.json", "frontend/index.html", "frontend/dist/index.html", ".gitignore")
				embed = "//go:embed all:frontend/dist"
			} else {
				want = append(want, "frontend/index.html")
			}
			for _, f := range want {
				if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
					t.Errorf("missing %s", f)
				}
			}
			main, _ := os.ReadFile(filepath.Join(dir, "main.go"))
			if !strings.Contains(string(main), embed) || strings.Contains(string(main), "__VITRA_") {
				t.Errorf("main.go embed directive or placeholders wrong")
			}
		})
	}
}

func TestScaffold_UnknownTemplatePointsToVite(t *testing.T) {
	err := run([]string{"new", filepath.Join(t.TempDir(), "x"), "--template", "ember"})
	if err == nil || !strings.Contains(err.Error(), "vanilla|vite|react|svelte|vue") || !strings.Contains(err.Error(), "start from vite") {
		t.Fatalf("err = %v", err)
	}
}

// Every embedded template directory must be reachable from scaffoldTemplates,
// so dead templates cannot accumulate again.
func TestScaffold_NoOrphanTemplateDirectories(t *testing.T) {
	listed := map[string]bool{}
	for _, tmpl := range scaffoldTemplates {
		listed[tmpl.name] = true
	}
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() && !listed[e.Name()] {
			t.Errorf("templates/%s is not listed in scaffoldTemplates", e.Name())
		}
	}
}

// The starter is the first code people read: it should be small enough to
// take in at once.
func TestScaffold_StarterIsSmall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	capture(t, func() {
		if err := run([]string{"new", dir}); err != nil {
			t.Fatal(err)
		}
	})
	src, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "\n"); n > 120 {
		t.Fatalf("starter main.go has %d lines, want at most 120", n)
	}
}
