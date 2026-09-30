package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	mode             os.FileMode
	dir              bool
}

func tarGz(t *testing.T, entries []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: int64(e.mode)}
		switch {
		case e.dir:
			h.Typeflag = tar.TypeDir
		case e.link != "":
			h.Typeflag, h.Linkname = tar.TypeSymlink, e.link
		default:
			h.Typeflag, h.Size = tar.TypeReg, int64(len(e.body))
		}
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			_, _ = tw.Write([]byte(e.body))
		}
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	return buf.Bytes()
}

func planFor(name string, artifact []byte) InstallPlan {
	return InstallPlan{AppID: "com.example.app", Version: "2.0.0", Artifact: name, SHA256: DigestArtifact(artifact)}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A macOS .app is a directory; the update must replace the whole bundle and
// keep executable bits and internal symlinks.
func TestApplyInstall_ReplacesBundleFromTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and exec bits")
	}
	dest := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.MkdirAll(filepath.Join(dest, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dest, "Contents", "MacOS", "demo"), []byte("v1"), 0o755)
	_ = os.WriteFile(filepath.Join(dest, "Contents", "stale.txt"), []byte("old"), 0o644)

	artifact := tarGz(t, []entry{
		{name: "Demo.app/", dir: true, mode: 0o755},
		{name: "Demo.app/Contents/MacOS/demo", body: "v2", mode: 0o755},
		{name: "Demo.app/Contents/Info.plist", body: "<plist/>"},
		{name: "Demo.app/Contents/Current", link: "MacOS"},
	})
	if err := ApplyInstall(planFor("Demo.app.tar.gz", artifact), artifact, dest); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dest, "Contents", "MacOS", "demo")); got != "v2" {
		t.Fatalf("binary = %q", got)
	}
	if fi, _ := os.Stat(filepath.Join(dest, "Contents", "MacOS", "demo")); fi.Mode()&0o100 == 0 {
		t.Fatal("exec bit lost")
	}
	if _, err := os.Stat(filepath.Join(dest, "Contents", "stale.txt")); !os.IsNotExist(err) {
		t.Fatal("files from the old bundle survived")
	}
	if target, err := os.Readlink(filepath.Join(dest, "Contents", "Current")); err != nil || target != "MacOS" {
		t.Fatalf("symlink = %q %v", target, err)
	}
	assertNoLeftovers(t, dest)
}

func TestApplyInstall_ZipWithLooseFilesBecomesDest(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "app")
	artifact := zipOf(t, map[string]string{"app.exe": "v2", "resources/data.bin": "d"})
	if err := ApplyInstall(planFor("app.zip", artifact), artifact, dest); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dest, "app.exe")) != "v2" || read(t, filepath.Join(dest, "resources", "data.bin")) != "d" {
		t.Fatal("zip contents not installed")
	}
	assertNoLeftovers(t, dest)
}

// Archive paths come from the (signed) artifact, but extraction must still
// never write outside the destination (zip-slip) or unpack without bound.
func TestApplyInstall_RejectsUnsafeArchives(t *testing.T) {
	cases := map[string][]byte{
		"zip traversal":     zipOf(t, map[string]string{"../evil": "x"}),
		"zip absolute":      zipOf(t, map[string]string{"/abs/evil": "x"}),
		"tar traversal":     tarGz(t, []entry{{name: "app/../../evil", body: "x"}}),
		"tar symlink out":   tarGz(t, []entry{{name: "app/x", link: "../../../etc/passwd"}}),
		"tar abs symlink":   tarGz(t, []entry{{name: "app/x", link: "/etc/passwd"}}),
		"zip backslash dir": zipOf(t, map[string]string{`..\evil`: "x"}),
	}
	for name, artifact := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dest := filepath.Join(root, "sub", "app")
			if err := os.MkdirAll(dest, 0o755); err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(filepath.Join(dest, "keep"), []byte("v1"), 0o644)
			ext := ".zip"
			if strings.HasPrefix(name, "tar") {
				ext = ".tar.gz"
			}
			if err := ApplyInstall(planFor("app"+ext, artifact), artifact, dest); err == nil {
				t.Fatal("unsafe archive installed")
			}
			if read(t, filepath.Join(dest, "keep")) != "v1" {
				t.Fatal("failed install changed the existing app")
			}
			if _, err := os.Stat(filepath.Join(root, "evil")); !os.IsNotExist(err) {
				t.Fatal("wrote outside the destination")
			}
			assertNoLeftovers(t, dest)
		})
	}
}

func TestApplyInstall_EnforcesExtractionLimit(t *testing.T) {
	prev := maxExtractedBytes
	maxExtractedBytes = 1024
	t.Cleanup(func() { maxExtractedBytes = prev })
	dest := filepath.Join(t.TempDir(), "app")
	artifact := zipOf(t, map[string]string{"big": strings.Repeat("a", 4096)})
	if err := ApplyInstall(planFor("app.zip", artifact), artifact, dest); err == nil {
		t.Fatal("archive over the extraction limit installed")
	}
}

// Windows cannot overwrite a running executable but can rename it, so the
// old binary is moved aside first and removed when no longer in use.
func TestApplyInstall_MovesRunningBinaryAside(t *testing.T) {
	prev := moveAsideFirst
	moveAsideFirst = true
	t.Cleanup(func() { moveAsideFirst = prev })
	dir := t.TempDir()
	dest := filepath.Join(dir, "app.exe")
	_ = os.WriteFile(dest, []byte("v1"), 0o755)
	artifact := []byte("v2")
	if err := ApplyInstall(planFor("app.exe", artifact), artifact, dest); err != nil {
		t.Fatal(err)
	}
	if read(t, dest) != "v2" {
		t.Fatal("binary not replaced")
	}
	// Simulate an old binary that could not be deleted while it ran.
	_ = os.WriteFile(dest+staleSuffix+"123", []byte("v1"), 0o755)
	if err := CleanupStale(dest); err != nil {
		t.Fatal(err)
	}
	assertNoLeftovers(t, dest)
}

func assertNoLeftovers(t *testing.T, dest string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Dir(dest))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".vitra-update-") || strings.Contains(e.Name(), staleSuffix) {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}
