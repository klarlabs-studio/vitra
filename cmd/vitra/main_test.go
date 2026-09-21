package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/packaging"
	"go.klarlabs.de/vitra/updater"
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
	if !strings.Contains(out, "packaging fold tools:") {
		t.Fatalf("doctor missing packaging tools: %q", out)
	}
	for _, tool := range []string{"appimagetool:", "candle:", "light:", "makensis:", "hdiutil:"} {
		if !strings.Contains(out, tool) {
			t.Fatalf("doctor missing %q: %q", tool, out)
		}
	}

	out = capture(t, func() {
		if err := run([]string{"inspect", "capabilities"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "project-files") {
		t.Fatalf("inspect output: %q", out)
	}
	if !strings.Contains(out, "vitra.fs") || !strings.Contains(out, "vitra.dialog") || !strings.Contains(out, "vitra.clipboard") {
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
	if !strings.Contains(out, "register-files") {
		t.Fatalf("help missing register-files: %q", out)
	}
	if !strings.Contains(out, "vitra package") {
		t.Fatalf("help missing package: %q", out)
	}
	if !strings.Contains(out, "generate typescript") {
		t.Fatalf("help missing generate: %q", out)
	}
	if !strings.Contains(out, "update-apply") {
		t.Fatalf("help missing update-apply: %q", out)
	}
}

func TestRun_UpdateApply(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("binary-v2")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.vitra.t", Version: "2.0.0", Channel: updater.ChannelStable,
		Artifact: "app.bin", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	body, _ := json.Marshal(m)
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(dir, "app.bin")
	if err := os.WriteFile(artifactPath, artifact, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "installed.bin")
	out := capture(t, func() {
		if err := run([]string{
			"update-apply",
			"--manifest", manifestPath,
			"--artifact", artifactPath,
			"--pubkey", hex.EncodeToString(pub),
			"--dest", dest,
			"--policy", "production",
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "installed") || !strings.Contains(out, "2.0.0") {
		t.Fatalf("stdout: %q", out)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(artifact) {
		t.Fatalf("dest=%q err=%v", got, err)
	}
	if err := run([]string{"update-apply"}); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestRun_GenerateTypeScript(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "frontend", "vitra-client.ts")
	out := capture(t, func() {
		if err := run([]string{"generate", "typescript", "--out", outPath, "--module", "demo"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "wrote") {
		t.Fatalf("generate stdout: %q", out)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"kernel:", "fsRead", "dialogOpen", "clipboardRead", `invoke("fs.read"`, "onFsChanged", `on("fs.changed"`, "createEvents"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if err := run([]string{"generate"}); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestRun_PackageStagesIcon(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	icon := filepath.Join(tmp, "logo.png")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--out", out, "--bin", bin, "--icon", icon, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(out, "T.png")); err != nil {
		t.Fatal(err)
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
	raw, err := os.ReadFile(filepath.Join(out, "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	mods, _ := doc["modules"].([]any)
	if len(mods) == 0 {
		t.Fatalf("expected modules from build info, got %s", raw)
	}
	s := string(raw)
	for _, want := range []string{"vitra.fs", "vitra.dialog", "vitra.clipboard", "fs.read", "dialog.open", "clipboard.read"} {
		if !strings.Contains(s, want) {
			t.Fatalf("provenance missing %q: %s", want, s)
		}
	}
}

func TestRun_PackageDMG(t *testing.T) {
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "fake-hdiutil")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nlast=\"\"\nfor a in \"$@\"; do last=\"$a\"; done\nprintf 'DMG' > \"$last\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_HDIUTIL", tool)
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist-dmg")
	capture(t, func() {
		if err := run([]string{"package", "--format", "dmg", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	dmg := filepath.Join(out, "t-0.1.0.dmg")
	raw, err := os.ReadFile(dmg)
	if err != nil || string(raw) != "DMG" {
		t.Fatalf("dmg=%q err=%v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(out, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PackageDarwinApp(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist-app")
	capture(t, func() {
		if err := run([]string{"package", "--format", "app-dir", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	bundle := filepath.Join(out, "T.app")
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "MacOS", "T")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "Info.plist")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PackageWiX(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("MZ"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist-wix")
	capture(t, func() {
		if err := run([]string{"package", "--format", "wix", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(out, "product.wxs")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "bin", "T.exe")); err != nil {
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

func TestRun_PackageAppImage(t *testing.T) {
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "fake-appimagetool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'IMG' > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_APPIMAGETOOL", tool)
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--format", "appimage", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "Demo", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	img := filepath.Join(out, "demo-"+packaging.DefaultArch()+".AppImage")
	raw, err := os.ReadFile(img)
	if err != nil || string(raw) != "IMG" {
		t.Fatalf("appimage=%q err=%v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(out, "provenance.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_RegisterFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("xdg file associations are Linux-only")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	bin := filepath.Join(tmp, "appbin")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := capture(t, func() {
		if err := run([]string{"register-files", "--mime", "text/plain", "--mime", "application/json", "--app-id", "com.vitra.t", "--exec", bin, "--name", "T"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "text/plain") {
		t.Fatalf("stdout: %q", out)
	}
	body, err := os.ReadFile(filepath.Join(tmp, "applications", "com.vitra.t-files.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "MimeType=text/plain;application/json;") {
		t.Fatalf("desktop:\n%s", body)
	}
	if err := run([]string{"register-files"}); err == nil {
		t.Fatal("expected usage error")
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
	for _, name := range []string{"main.go", "frontend/index.html", "frontend/vitra-client.ts", "README.md", "go.mod"} {
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
	for _, want := range []string{
		"io/fs",
		"go.klarlabs.de/vitra/desktop",
		"go.klarlabs.de/vitra/plugin/official/clipboard",
		"go.klarlabs.de/vitra/plugin/official/dialog",
		"go.klarlabs.de/vitra/plugin/official/fs",
	} {
		if !seen[want] {
			t.Fatalf("scaffold missing import %q", want)
		}
	}
	client, err := os.ReadFile(dir + "/frontend/vitra-client.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"createClient", "demo.greet", "dialog.open", "fs.read", "clipboard.read"} {
		if !strings.Contains(string(client), want) {
			t.Fatalf("vitra-client.ts missing %q", want)
		}
	}
	readme, err := os.ReadFile(dir + "/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "vitra_native") || !strings.Contains(string(readme), "vitra dev") {
		t.Fatalf("scaffold README should mention native tag and vitra dev: %s", readme)
	}
	if !strings.Contains(string(readme), "generate typescript") {
		t.Fatalf("scaffold README should mention generate typescript: %s", readme)
	}
	if !strings.Contains(out, "vitra dev") {
		t.Fatalf("new next-step should suggest vitra dev: %q", out)
	}
}

func TestSupportsNativeHostTag(t *testing.T) {
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if !supportsNativeHostTag(goos) {
			t.Fatalf("%s should use -tags vitra_native", goos)
		}
	}
	if supportsNativeHostTag("js") || supportsNativeHostTag("plan9") {
		t.Fatal("non-desktop OS must not force vitra_native")
	}
}

func TestAppendNativeHostTags(t *testing.T) {
	got := appendNativeHostTags([]string{"run"})
	if !supportsNativeHostTag(runtime.GOOS) {
		if len(got) != 1 || got[0] != "run" {
			t.Fatalf("%v", got)
		}
		return
	}
	if len(got) != 3 || got[0] != "run" || got[1] != "-tags" || got[2] != "vitra_native" {
		t.Fatalf("%v", got)
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
