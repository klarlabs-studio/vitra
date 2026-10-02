package main

import (
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// allScaffoldPlugins is every plugin `vitra new --with` supports.
const allScaffoldPlugins = "fs,dialog,clipboard,notification,os"

func TestParseWith(t *testing.T) {
	for in, want := range map[string][]string{
		"fs":                            {"fs"},
		"fs,dialog,clipboard":           {"fs", "dialog", "clipboard"},
		"clipboard,fs":                  {"fs", "clipboard"}, // canonical order
		" os , fs ":                     {"fs", "os"},
		"fs,fs":                         {"fs"},
		"notification,os,dialog":        {"dialog", "notification", "os"},
		allScaffoldPlugins:              {"fs", "dialog", "clipboard", "notification", "os"},
		"dialog,,clipboard":             {"dialog", "clipboard"},
		"Clipboard":                     nil, // names are exact
		"path":                          nil,
		"":                              nil,
		",":                             nil,
		"fs,menu":                       nil,
		"fs,dialog,clipboard,shortcut ": nil,
	} {
		got, err := parseWith(in)
		if want == nil {
			if err == nil {
				t.Errorf("parseWith(%q) accepted: %v", in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseWith(%q): %v", in, err)
			continue
		}
		names := make([]string, len(got))
		for i, p := range got {
			names[i] = p.name
		}
		if !slices.Equal(names, want) {
			t.Errorf("parseWith(%q) = %v, want %v", in, names, want)
		}
	}
}

func TestScaffold_UnknownPluginListsValidNames(t *testing.T) {
	err := run([]string{"new", filepath.Join(t.TempDir(), "x"), "--with", "fs,path"})
	if err == nil {
		t.Fatal("unknown plugin accepted")
	}
	for _, want := range []string{`"path"`, "fs, dialog, clipboard, notification, os"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if err := run([]string{"new", filepath.Join(t.TempDir(), "x"), "--with"}); err == nil || !strings.Contains(err.Error(), "fs, dialog") {
		t.Errorf("--with without a value: %v", err)
	}
}

// scaffoldWith generates an app with args after the directory and returns
// its directory.
func scaffoldWith(t *testing.T, args ...string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	capture(t, func() {
		if err := run(append([]string{"new", dir}, args...)); err != nil {
			t.Fatal(err)
		}
	})
	return dir
}

func readScaffold(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var grantedPerm = regexp.MustCompile(`\{Name:\s*([^,}]+)`)

// grantedPermissions returns the permission names in src's grants.
func grantedPermissions(src string) []string {
	var out []string
	for _, m := range grantedPerm.FindAllStringSubmatch(src, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	slices.Sort(out)
	return out
}

// Each --with choice adds exactly its least-privilege permissions and
// nothing else, and the defaults stay what TestScaffold_GrantsLeastPrivilegeByDefault
// checks.
func TestScaffold_WithGrantsLeastPrivilege(t *testing.T) {
	fsPerms := []string{`"dataDir"`, "desktop.PermFSRead", "desktop.PermFSWrite"}
	cases := map[string][]string{
		"fs":           fsPerms,
		"dialog":       {"desktop.PermDialogOpen", "desktop.PermDialogSave"},
		"clipboard":    {"desktop.PermClipboardWrite"},
		"notification": {"desktop.PermNotificationShow"},
		"os":           {"desktop.PermOsInfo"},
	}
	var all []string
	for _, perms := range cases {
		all = append(all, perms...)
	}
	cases[allScaffoldPlugins] = all

	ctor := map[string]string{
		"fs": "official.FS()", "dialog": "official.Dialog()", "clipboard": "official.Clipboard()",
		"notification": "official.Notification()", "os": "official.OS()",
	}
	for with, perms := range cases {
		t.Run(with, func(t *testing.T) {
			src := scaffoldGrant(t, "--with", with)
			want := append([]string{`"greet"`}, perms...)
			slices.Sort(want)
			if got := grantedPermissions(src); !slices.Equal(got, want) {
				t.Errorf("granted %v, want %v", got, want)
			}
			// fs.write and path.open must never share a folder; the
			// scaffold never grants path.open at all.
			for _, never := range []string{"PermPathOpen", "PermOpenURL", "PermClipboardRead", "PermDialogOpenDirectory", "PermDialogMessage"} {
				if strings.Contains(src, never) {
					t.Errorf("scaffold grants or uses %s", never)
				}
			}
			// UseOfficialPlugins gets exactly the chosen plugins.
			var calls []string
			for _, name := range strings.Split(with, ",") {
				calls = append(calls, ctor[name])
			}
			if !strings.Contains(src, "a.UseOfficialPlugins(context.Background(), "+strings.Join(calls, ", ")+")") {
				t.Errorf("UseOfficialPlugins call lacks exactly %v:\n%s", calls, src)
			}
			if strings.Contains(src, "UseOfficialPlugins(context.Background())") {
				t.Error("scaffold registers every official plugin")
			}
			if !strings.Contains(with, "fs") {
				if strings.Contains(src, "appDataDir") {
					t.Error("data folder created without fs")
				}
				return
			}
			// fs.read and fs.write are scoped to the app's data folder, a
			// real (symlink-free) path in slash form.
			for _, perm := range []string{"desktop.PermFSRead", "desktop.PermFSWrite"} {
				if !regexp.MustCompile(`\{Name:\s*` + regexp.QuoteMeta(perm) + `,\s*PathScope:\s*inDataDir\}`).MatchString(src) {
					t.Errorf("%s is not scoped to the data folder", perm)
				}
			}
			for _, want := range []string{
				`inDataDir := &domain.PathScope{Allow: []string{dataDir + "/**"}}`,
				"filepath.EvalSymlinks(dir)",
				"filepath.ToSlash(real)",
				"os.UserConfigDir()",
			} {
				if !strings.Contains(src, want) {
					t.Errorf("main.go lacks %q", want)
				}
			}
		})
	}
}

// Optional permissions are documented next to the grant, never granted.
func TestScaffold_WithDocumentsOptIns(t *testing.T) {
	src := readScaffold(t, scaffoldWith(t, "--with", "fs,clipboard"), "main.go")
	for _, want := range []string{"desktop.PermClipboardRead", `dataDir + "/**/*.md"`, "path.open"} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go does not mention opt-in %q", want)
		}
	}
}

// Every combination renders gofmt-clean Go.
func TestScaffold_WithMainIsFormatted(t *testing.T) {
	plugins := strings.Split(allScaffoldPlugins, ",")
	for mask := 0; mask < 1<<len(plugins); mask++ {
		var with []string
		for i, p := range plugins {
			if mask&(1<<i) != 0 {
				with = append(with, p)
			}
		}
		chosen, err := parseWith(strings.Join(with, ","))
		if len(with) == 0 {
			chosen, err = nil, nil
		}
		if err != nil {
			t.Fatal(err)
		}
		files, err := scaffoldFiles(scaffoldTemplates[0], "example.com/app", "", chosen)
		if err != nil {
			t.Fatal(err)
		}
		src := files["main.go"]
		formatted, err := format.Source([]byte(src))
		if err != nil {
			t.Fatalf("--with %v: %v\n%s", with, err, src)
		}
		if string(formatted) != src {
			t.Errorf("--with %v: main.go is not gofmt-clean", with)
		}
	}
}

// The page shows one working call per chosen plugin, and nothing for the
// others.
func TestScaffold_WithFrontendCallsEachPlugin(t *testing.T) {
	calls := map[string][]string{
		"fs":           {`"dataDir"`, `"fs.write"`, `"fs.read"`},
		"dialog":       {`"dialog.open"`, `"dialog.save"`},
		"clipboard":    {`"clipboard.write"`},
		"notification": {`"notifications.show"`},
		"os":           {`"os.info"`},
	}
	tsCalls := map[string][]string{
		"fs":           {"client.dataDir(", "client.fsWrite(", "client.fsRead("},
		"dialog":       {"client.dialogOpen(", "client.dialogSave("},
		"clipboard":    {"client.clipboardWrite("},
		"notification": {"client.notificationsShow("},
		"os":           {"client.osInfo("},
	}
	for _, with := range []string{"fs", "dialog,clipboard", allScaffoldPlugins} {
		chosen := strings.Split(with, ",")
		t.Run("vanilla/"+with, func(t *testing.T) {
			page := readScaffold(t, scaffoldWith(t, "--with", with), "frontend/index.html")
			for name, want := range calls {
				for _, call := range want {
					if got := strings.Contains(page, "invoke("+call); got != slices.Contains(chosen, name) {
						t.Errorf("page calls %s = %v", call, got)
					}
				}
			}
		})
		for _, tmpl := range scaffoldTemplates[1:] {
			t.Run(tmpl.name+"/"+with, func(t *testing.T) {
				dir := scaffoldWith(t, "--template", tmpl.name, "--with", with)
				dist := readScaffold(t, dir, "frontend/dist/index.html")
				if !strings.Contains(dist, "invoke("+calls[chosen[0]][0]) {
					t.Error("starter dist page lacks the plugin demos")
				}
				demos := readScaffold(t, dir, "frontend/src/plugins.ts")
				for name, want := range tsCalls {
					for _, call := range want {
						if got := strings.Contains(demos, call); got != slices.Contains(chosen, name) {
							t.Errorf("plugins.ts calls %s = %v", call, got)
						}
					}
				}
				entry := "frontend/src/main.ts"
				if tmpl.name == "react" {
					entry = "frontend/src/main.tsx"
				}
				if !strings.Contains(readScaffold(t, dir, entry), `import "./plugins";`) {
					t.Errorf("%s does not load the plugin demos", entry)
				}
			})
		}
	}
}

// Without --with nothing plugin-related is generated.
func TestScaffold_WithoutPluginsHasNoDemos(t *testing.T) {
	for _, tmpl := range scaffoldTemplates {
		dir := scaffoldWith(t, "--template", tmpl.name)
		if _, err := os.Stat(filepath.Join(dir, "frontend", "src", "plugins.ts")); err == nil {
			t.Errorf("%s: plugins.ts generated without --with", tmpl.name)
		}
		if src := readScaffold(t, dir, "main.go"); strings.Contains(src, "UseOfficialPlugins(") && !strings.Contains(src, "// more, call a.UseOfficialPlugins(ctx)") {
			t.Errorf("%s: main.go uses plugins without --with", tmpl.name)
		}
	}
}

// isolateHome points the user's home and config directories at a temporary
// folder, so running a generated app does not create its data folder in the
// real home. Go's caches stay where they are.
func isolateHome(t *testing.T) string {
	t.Helper()
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOENV"} {
		out, err := exec.Command("go", "env", key).Output()
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, strings.TrimSpace(string(out)))
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	return home
}

// A --with app builds and vets as generated.
func TestScaffold_WithAppBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a Go program")
	}
	_, self, _, _ := runtime.Caller(0)
	t.Setenv("VITRA_MODULE_PATH", filepath.Clean(filepath.Join(filepath.Dir(self), "..", "..")))
	dir := scaffoldWith(t, "--with", allScaffoldPlugins)
	for _, args := range [][]string{{"vet", "."}, {"build", "-o", os.DevNull, "."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v: %v\n%s", args, err, out)
		}
	}
}
