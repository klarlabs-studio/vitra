package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
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
	for _, tool := range []string{"appimagetool:", "rpmbuild:", "snapcraft:", "flatpak-builder:", "candle:", "light:", "makensis:", "hdiutil:"} {
		if !strings.Contains(out, tool) {
			t.Fatalf("doctor missing %q: %q", tool, out)
		}
	}
	if !strings.Contains(out, "packaging sign tools") {
		t.Fatalf("doctor missing sign tools: %q", out)
	}
	for _, tool := range []string{"codesign:", "signtool:", "notarytool:", "gpg:", "dpkg-sig:", "rpmsign:"} {
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
	if !strings.Contains(out, "update-check") {
		t.Fatalf("help missing update-check: %q", out)
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

func TestRun_UpdateCheck(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("channel-check")
	sum := sha256.Sum256(artifact)
	m := updater.Manifest{
		AppID: "com.vitra.channel", Version: "9.1.0", Channel: updater.ChannelStable,
		Artifact: "app.bin", SHA256: hex.EncodeToString(sum[:]),
		CreatedAt: time.Now().UTC(),
	}
	m, err = updater.SignManifest(m, priv)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/com.vitra.channel/stable/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	out := capture(t, func() {
		if err := run([]string{
			"update-check",
			"--base-url", srv.URL,
			"--app-id", "com.vitra.channel",
			"--channel", "stable",
			"--pubkey", hex.EncodeToString(pub),
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "9.1.0") || !strings.Contains(out, "update available") {
		t.Fatalf("stdout: %q", out)
	}
	if err := run([]string{"update-check"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run([]string{
		"update-check",
		"--base-url", srv.URL,
		"--app-id", "com.vitra.channel",
		"--pubkey", hex.EncodeToString(make([]byte, ed25519.PublicKeySize)),
	}); err == nil {
		t.Fatal("expected signature verification failure")
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

func TestRun_PackageSignPlan(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("mach-o"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_TEST_SIGN_ID", "not-leaked-value")
	out := filepath.Join(tmp, "Demo.app")
	printed := capture(t, func() {
		if err := run([]string{
			"package", "--format", "app-dir", "--out", out, "--bin", bin,
			"--app-id", "com.vitra.t", "--name", "Demo", "--version", "0.1.0",
			"--sign", "--signing-identity", "env:VITRA_TEST_SIGN_ID",
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(printed, "sign plan") || !strings.Contains(printed, "codesign") {
		t.Fatalf("expected sign plan, got %q", printed)
	}
	if !strings.Contains(printed, "plan only until ExecuteSign") {
		t.Fatalf("expected plan-only note: %q", printed)
	}
	if !strings.Contains(printed, "notarytool") || !strings.Contains(printed, "stapler") {
		t.Fatalf("expected notarize/staple follow-ups: %q", printed)
	}
	if strings.Contains(printed, "not-leaked-value") {
		t.Fatalf("leaked identity value: %q", printed)
	}
	if err := run([]string{
		"package", "--format", "dir", "--out", filepath.Join(tmp, "bad"), "--bin", bin,
		"--sign",
	}); err == nil {
		t.Fatal("expected --sign without identity to fail")
	}
}

func TestRun_PackageSignExecute(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("elf"), 0o755); err != nil {
		t.Fatal(err)
	}
	tool := filepath.Join(tmp, "fake-gpg")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_GPG", tool)
	t.Setenv("GPG_KEY_ID", "TESTKEY")
	out := filepath.Join(tmp, "stage")
	printed := capture(t, func() {
		if err := run([]string{
			"package", "--format", "appdir", "--out", out, "--bin", bin,
			"--app-id", "com.vitra.t", "--name", "Demo", "--version", "0.1.0",
			"--sign-execute", "--signing-identity", "env:GPG_KEY_ID",
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(printed, "signed") || !strings.Contains(printed, "gpg") {
		t.Fatalf("expected execute confirmation: %q", printed)
	}
	if err := run([]string{
		"package", "--format", "dir", "--out", filepath.Join(tmp, "nofu"), "--bin", bin,
		"--sign-follow-ups", "--signing-identity", "env:GPG_KEY_ID",
	}); err == nil {
		t.Fatal("expected --sign-follow-ups without --sign-execute to fail")
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

func TestRun_PackageRPM(t *testing.T) {
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "fake-rpmbuild")
	if err := os.WriteFile(tool, []byte(`#!/bin/sh
top=""
prev=""
for a in "$@"; do
  if [ "$prev" = "--define" ]; then
    case "$a" in
      _topdir\ *) top="${a#_topdir }" ;;
    esac
  fi
  prev="$a"
done
mkdir -p "$top/RPMS/x86_64"
printf 'RPM' > "$top/RPMS/x86_64/out.rpm"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_RPMBUILD", tool)
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--format", "rpm", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	rpm := filepath.Join(out, fmt.Sprintf("com-vitra-t-0.1.0.%s.rpm", packaging.DefaultArch()))
	raw, err := os.ReadFile(rpm)
	if err != nil || string(raw) != "RPM" {
		entries, _ := os.ReadDir(out)
		t.Fatalf("rpm=%v raw=%q entries=%v", err, raw, entries)
	}
	stage := filepath.Join(tmp, "rpm-stage")
	capture(t, func() {
		if err := run([]string{"package", "--format", "rpm-dir", "--out", stage, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(stage, "SPECS", "com-vitra-t.spec")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PackageSnap(t *testing.T) {
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "fake-snapcraft")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nout=\"\"\nprev=\"\"\nfor a in \"$@\"; do\n  if [ \"$prev\" = \"--output\" ]; then out=\"$a\"; fi\n  prev=\"$a\"\ndone\nprintf 'SNAP' > \"$out\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_SNAPCRAFT", tool)
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--format", "snap", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	snap := filepath.Join(out, fmt.Sprintf("com-vitra-t_0.1.0_%s.snap", packaging.DefaultArch()))
	raw, err := os.ReadFile(snap)
	if err != nil || string(raw) != "SNAP" {
		entries, _ := os.ReadDir(out)
		t.Fatalf("snap=%v raw=%q entries=%v", err, raw, entries)
	}
	stage := filepath.Join(tmp, "snap-stage")
	capture(t, func() {
		if err := run([]string{"package", "--format", "snap-dir", "--out", stage, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(stage, "meta", "snap.yaml")); err != nil {
		t.Fatal(err)
	}
}

func TestRun_PackageFlatpak(t *testing.T) {
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "fake-flatpak")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf 'FLATPAK' > \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VITRA_FLATPAK_BUILDER", tool)
	bin := filepath.Join(tmp, "vitra-app")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "dist")
	capture(t, func() {
		if err := run([]string{"package", "--format", "flatpak", "--out", out, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	fp := filepath.Join(out, "com-vitra-t-0.1.0.flatpak")
	raw, err := os.ReadFile(fp)
	if err != nil || string(raw) != "FLATPAK" {
		entries, _ := os.ReadDir(out)
		t.Fatalf("flatpak=%v raw=%q entries=%v", err, raw, entries)
	}
	stage := filepath.Join(tmp, "flatpak-stage")
	capture(t, func() {
		if err := run([]string{"package", "--format", "flatpak-dir", "--out", stage, "--bin", bin, "--app-id", "com.vitra.t", "--name", "T", "--version", "0.1.0"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(stage, "metadata")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "manifest.yml")); err != nil {
		t.Fatal(err)
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
	if !strings.Contains(out, "created") || !strings.Contains(out, "template=vanilla") {
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
	if !strings.Contains(string(src), "//go:embed frontend/*") {
		t.Fatalf("vanilla embed missing: %s", src)
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

func TestRun_NewScaffoldVite(t *testing.T) {
	dir := t.TempDir() + "/vite-app"
	out := capture(t, func() {
		if err := run([]string{"new", dir, "--template", "vite"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "template=vite") {
		t.Fatalf("new output: %q", out)
	}
	for _, name := range []string{
		"main.go", "frontend/package.json", "frontend/vite.config.js",
		"frontend/src/main.ts", "frontend/index.html", "frontend/dist/index.html",
		"frontend/vitra-client.ts", ".gitignore",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	src, err := os.ReadFile(dir + "/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "//go:embed all:frontend/dist") {
		t.Fatalf("vite embed missing: %s", src)
	}
	if !strings.Contains(string(src), `fs.Sub(frontendRoot, "frontend/dist")`) {
		t.Fatalf("vite Sub path missing: %s", src)
	}
	mainTS, err := os.ReadFile(dir + "/frontend/src/main.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"createClient", "demoGreet", "dialogOpen", "clipboardRead"} {
		if !strings.Contains(string(mainTS), want) {
			t.Fatalf("main.ts missing %q: %s", want, mainTS)
		}
	}
	readme, err := os.ReadFile(dir + "/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "npm run build") {
		t.Fatalf("vite README should mention npm build: %s", readme)
	}
	if err := run([]string{"new", dir, "--template", "nope"}); err == nil {
		t.Fatal("expected unknown template error")
	}
	if err := run([]string{"new", "--template", "vite"}); err == nil {
		t.Fatal("expected missing dir error")
	}
}

func TestRun_NewScaffoldReact(t *testing.T) {
	dir := t.TempDir() + "/react-app"
	out := capture(t, func() {
		if err := run([]string{"new", "--template", "react", dir}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "template=react") {
		t.Fatalf("new output: %q", out)
	}
	for _, name := range []string{
		"main.go", "frontend/package.json", "frontend/vite.config.js",
		"frontend/src/main.tsx", "frontend/src/App.tsx", "frontend/index.html",
		"frontend/dist/index.html", "frontend/vitra-client.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := os.ReadFile(dir + "/frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"react"`, `"@vitejs/plugin-react"`, `"vite"`} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("package.json missing %s: %s", want, pkg)
		}
	}
	cfg, err := os.ReadFile(dir + "/frontend/vite.config.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "plugin-react") && !strings.Contains(string(cfg), "plugins: [react()]") {
		t.Fatalf("vite config missing react plugin: %s", cfg)
	}
	app, err := os.ReadFile(dir + "/frontend/src/App.tsx")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"createClient", "demoGreet", "dialogOpen", "clipboardRead", "useState"} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("App.tsx missing %q: %s", want, app)
		}
	}
	src, err := os.ReadFile(dir + "/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "//go:embed all:frontend/dist") {
		t.Fatalf("react embed missing: %s", src)
	}
}

func TestRun_NewScaffoldSvelte(t *testing.T) {
	dir := t.TempDir() + "/svelte-app"
	out := capture(t, func() {
		if err := run([]string{"new", dir, "--template", "svelte"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "template=svelte") {
		t.Fatalf("new output: %q", out)
	}
	for _, name := range []string{
		"main.go", "frontend/package.json", "frontend/vite.config.js",
		"frontend/svelte.config.js", "frontend/src/main.ts", "frontend/src/App.svelte",
		"frontend/dist/index.html", "frontend/vitra-client.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := os.ReadFile(dir + "/frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"svelte"`, `"@sveltejs/vite-plugin-svelte"`, `"vite"`} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("package.json missing %s: %s", want, pkg)
		}
	}
	app, err := os.ReadFile(dir + "/frontend/src/App.svelte")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"createClient", "demoGreet", "dialogOpen", "clipboardRead", "$state"} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("App.svelte missing %q: %s", want, app)
		}
	}
	cfg, err := os.ReadFile(dir + "/frontend/vite.config.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "vite-plugin-svelte") {
		t.Fatalf("vite config missing svelte plugin: %s", cfg)
	}
}

func TestRun_NewScaffoldVue(t *testing.T) {
	dir := t.TempDir() + "/vue-app"
	out := capture(t, func() {
		if err := run([]string{"new", dir, "--template", "vue"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "template=vue") {
		t.Fatalf("new output: %q", out)
	}
	for _, name := range []string{
		"main.go", "frontend/package.json", "frontend/vite.config.js",
		"frontend/src/main.ts", "frontend/src/App.vue", "frontend/index.html",
		"frontend/dist/index.html", "frontend/vitra-client.ts",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := os.ReadFile(dir + "/frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"vue"`, `"@vitejs/plugin-vue"`, `"vite"`} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("package.json missing %s: %s", want, pkg)
		}
	}
	app, err := os.ReadFile(dir + "/frontend/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"createClient", "demoGreet", "dialogOpen", "clipboardRead", "ref(", "<script setup"} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("App.vue missing %q: %s", want, app)
		}
	}
	cfg, err := os.ReadFile(dir + "/frontend/vite.config.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "plugin-vue") && !strings.Contains(string(cfg), "plugins: [vue()]") {
		t.Fatalf("vite config missing vue plugin: %s", cfg)
	}
	src, err := os.ReadFile(dir + "/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "//go:embed all:frontend/dist") {
		t.Fatalf("vue embed missing: %s", src)
	}
	readme, err := os.ReadFile(dir + "/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "Vite + Vue") {
		t.Fatalf("vue README should mention Vite + Vue: %s", readme)
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
