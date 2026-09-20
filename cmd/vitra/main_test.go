package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.klarlabs.de/vitra/packaging"
)

func TestRun_VersionDoctorInspectHelp(t *testing.T) {
	out := capture(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "vitra") {
		t.Fatalf("version output: %q", out)
	}

	out = capture(t, func() {
		if err := run([]string{"doctor"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "kernel:") {
		t.Fatalf("doctor output: %q", out)
	}
	if !strings.Contains(out, "adapter:") {
		t.Fatalf("doctor missing adapter: %q", out)
	}

	out = capture(t, func() {
		if err := run([]string{"inspect", "capabilities"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "project-files") {
		t.Fatalf("inspect output: %q", out)
	}
	if !strings.Contains(out, "vitra.fs") || !strings.Contains(out, "vitra.dialog") {
		t.Fatalf("inspect missing plugin ownership: %q", out)
	}

	if err := run([]string{"inspect"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run([]string{"nope"}); err == nil {
		t.Fatal("expected unknown command")
	}
	if err := run([]string{"new"}); err == nil {
		t.Fatal("expected new usage error")
	}

	out = capture(t, func() {
		if err := run(nil); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"help"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "vitra new") {
		t.Fatalf("help output: %q", out)
	}
	if !strings.Contains(out, "register-scheme") {
		t.Fatalf("help missing register-scheme: %q", out)
	}
	if !strings.Contains(out, "vitra package") {
		t.Fatalf("help missing package: %q", out)
	}
}

func TestRun_PackageStagesLinuxDir(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(out, "bin", "T")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PackageDeb(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--format", "deb", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	deb := filepath.Join(out, "com-vitra-t_0.1.0_"+packaging.DefaultArch()+".deb")
	if _, err := os.Stat(deb); err != nil {
		entries, _ := os.ReadDir(out)
		found := false
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".deb") {
				found = true
			}
		}
		if !found {
			t.Fatalf("no deb in %v (%v)", entries, err)
		}
	}
}

func TestRun_NewScaffold(t *testing.T) {
	dir := t.TempDir() + "/app"
	out := capture(t, func() {
		if err := run([]string{"new", dir}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "created") {
		t.Fatalf("new output: %q", out)
	}
	for _, name := range []string{"main.go", "frontend/index.html", "README.md", "go.mod"} {
		if _, err := os.Stat(dir + "/" + name); err != nil {
			t.Fatal(err)
		}
	}
	src, err := os.ReadFile(dir + "/main.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, dir+"/main.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("scaffold main.go parse: %v", err)
	}
	seen := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if seen[path] {
			t.Fatalf("duplicate import %q in scaffold", path)
		}
		seen[path] = true
	}
	if !seen["io/fs"] {
		t.Fatal("scaffold missing io/fs import")
	}
}

func capture(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}
