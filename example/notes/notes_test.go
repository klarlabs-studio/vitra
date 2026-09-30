package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

// setup returns a runtime wired like the app, a seeded vault, and a path
// outside it holding a "secret".
func setup(t *testing.T) (rt *vitra.Runtime, root, secret string) {
	t.Helper()
	root, err := openVault(filepath.Join(t.TempDir(), "vault"))
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	secret = filepath.ToSlash(filepath.Join(outside, "secret.md"))
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt, err = vitra.New(vitra.Config{AppID: appID})
	if err != nil {
		t.Fatal(err)
	}
	log := &liveAudit{}
	rt.SetAudit(log)
	if err := grantVault(rt, root); err != nil {
		t.Fatal(err)
	}
	if err := registerCommands(rt, root, log); err != nil {
		t.Fatal(err)
	}
	for _, w := range []domain.WindowID{mainWindow, "other"} {
		if _, err := rt.OpenWindow(context.Background(), w, domain.OriginPackagedLocal); err != nil {
			t.Fatal(err)
		}
	}
	return rt, root, secret
}

func invoke(t *testing.T, rt *vitra.Runtime, window domain.WindowID, cmd domain.CommandName, input map[string]any, resourcePath string) (any, error) {
	t.Helper()
	caller, err := rt.CallerFor(window)
	if err != nil {
		t.Fatal(err)
	}
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{
		Caller: caller, Command: cmd, Input: input, ResourcePath: resourcePath,
	})
	if err != nil {
		return nil, err
	}
	return res.Output, nil
}

func denialCode(err error) domain.DenialCode {
	var denied *domain.ErrDenied
	if errors.As(err, &denied) {
		return denied.Code
	}
	return ""
}

func TestVault_AllowsReadingAndWritingNotes(t *testing.T) {
	rt, root, _ := setup(t)
	welcome := root + "/Welcome.md"
	out, err := invoke(t, rt, mainWindow, "notes.read", map[string]any{"path": welcome}, welcome)
	if err != nil || !strings.Contains(out.(string), "Welcome to Vitra Notes") {
		t.Fatalf("read Welcome.md = %v, %v", out, err)
	}
	note := root + "/projects/Ideas.md"
	if _, err := invoke(t, rt, mainWindow, "notes.write", map[string]any{"path": note, "content": "# Ideas\n"}, note); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if b, _ := os.ReadFile(note); string(b) != "# Ideas\n" {
		t.Fatalf("note on disk = %q", b)
	}
	out, err = invoke(t, rt, mainWindow, "notes.list", map[string]any{"dir": root}, root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range out.([]Entry) {
		names = append(names, e.Name)
	}
	if got := strings.Join(names, ","); got != ".private,projects,Shortcuts.md,Welcome.md" {
		t.Fatalf("vault listing = %s", got)
	}
}

func TestVault_RefusesEverythingElse(t *testing.T) {
	rt, root, secret := setup(t)
	private := root + "/.private/credentials.md"
	if err := os.Symlink(filepath.FromSlash(secret), filepath.FromSlash(root+"/link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(filepath.FromSlash(secret)), filepath.FromSlash(root+"/linkdir")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		window domain.WindowID
		cmd    domain.CommandName
		input  map[string]any
		rp     string
		code   domain.DenialCode // "" means any refusal
	}{
		{"file outside the vault", mainWindow, "notes.read", map[string]any{"path": secret}, secret, domain.DenialPathOutOfScope},
		{"denied .private folder", mainWindow, "notes.read", map[string]any{"path": private}, private, domain.DenialPathDenied},
		{"denied .private, other case", mainWindow, "notes.read", map[string]any{"path": root + "/.PRIVATE/credentials.md"}, root + "/.PRIVATE/credentials.md", domain.DenialPathDenied},
		{"list .private", mainWindow, "notes.list", map[string]any{"dir": root + "/.private"}, root + "/.private", domain.DenialPathDenied},
		{"write .private", mainWindow, "notes.write", map[string]any{"path": private, "content": "x"}, private, domain.DenialPathDenied},
		{"traversal", mainWindow, "notes.read", map[string]any{"path": root + "/../x.md"}, root + "/../x.md", ""},
		{"write a script", mainWindow, "notes.write", map[string]any{"path": root + "/payload.sh", "content": "x"}, root + "/payload.sh", domain.DenialPathOutOfScope},
		{"authorize one path, read another", mainWindow, "notes.read", map[string]any{"path": secret}, root + "/Welcome.md", ""},
		{"symlinked file leaving the vault", mainWindow, "notes.read", map[string]any{"path": root + "/link.md"}, root + "/link.md", ""},
		{"symlinked folder leaving the vault", mainWindow, "notes.list", map[string]any{"dir": root + "/linkdir"}, root + "/linkdir", ""},
		{"another window", "other", "notes.read", map[string]any{"path": root + "/Welcome.md"}, root + "/Welcome.md", domain.DenialWindowMismatch},
		{"unregistered command", mainWindow, "shell.exec", map[string]any{"command": "rm -rf ~"}, "", domain.DenialCommandMissing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := invoke(t, rt, tc.window, tc.cmd, tc.input, tc.rp)
			if err == nil {
				t.Fatalf("allowed, returned %v", out)
			}
			if tc.code != "" && denialCode(err) != tc.code {
				t.Fatalf("err = %v (code %q), want code %q", err, denialCode(err), tc.code)
			}
		})
	}
	if b, _ := os.ReadFile(filepath.FromSlash(private)); strings.TrimSpace(string(b)) == "x" {
		t.Fatal(".private/credentials.md was overwritten")
	}
	if _, err := os.Stat(filepath.FromSlash(root + "/payload.sh")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("payload.sh was written")
	}
}

func TestOpenVault_SeedsOnlyAnEmptyFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Mine.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := openVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(root), "Welcome.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("seeded a vault that already had notes")
	}
	fresh, err := openVault(filepath.Join(t.TempDir(), "new"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Welcome.md", ".private/credentials.md", "projects/Roadmap.md"} {
		info, err := os.Stat(filepath.Join(filepath.FromSlash(fresh), name))
		if err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		if info.Mode().Perm()&0o200 == 0 {
			t.Fatalf("seed %s is read-only", name)
		}
	}
}

// A fresh app has an empty audit log; the page iterates the result, so it
// must be a JSON array, not null.
func TestAuditFollow_EmptyLogIsAnArray(t *testing.T) {
	rt, _, _ := setup(t)
	out, err := invoke(t, rt, mainWindow, "audit.follow", map[string]any{}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "[]" {
		t.Fatalf("audit.follow on an empty log = %s, want []", b)
	}
}
