package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra"
)

const genAppMain = `package main

import (
	"context"
	"log"
	"testing/fstest"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform/linux"
)

type Note struct {
	Title string   ` + "`json:\"title\"`" + `
	Tags  []string ` + "`json:\"tags,omitempty\"`" + `
}

func main() {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.notes"})
	if err != nil {
		log.Fatal(err)
	}
	err = vitra.Register(rt, vitra.Command[Note, Note]{
		Name: "notes.save", Description: "Save a note", Permission: "notes.write",
		Handler: func(_ context.Context, _ domain.Invocation, n Note) (Note, error) { return n, nil },
	})
	if err != nil {
		log.Fatal(err)
	}
	a, err := app.New(app.Options{
		AppID: "com.example.notes", Runtime: rt, Host: linux.New(), // stub host without -tags vitra_native
		Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Window: app.WindowOptions{ID: "main"},
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := a.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
`

func TestRun_GenerateTypeScriptFromApp(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a Go program")
	}
	_, self, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(self), "..", ".."))
	dir := t.TempDir()
	repoMod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	goLine := "go 1.26"
	for _, line := range strings.Split(string(repoMod), "\n") {
		if strings.HasPrefix(line, "go ") {
			goLine = line
		}
	}
	gomod := "module example.com/notes\n\n" + goLine + "\n\nrequire go.klarlabs.de/vitra v0.0.0\n\nreplace go.klarlabs.de/vitra => " + repo + "\n"
	for name, body := range map[string]string{"go.mod": gomod, "main.go": genAppMain} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "frontend", "vitra-client.ts")
	capture(t, func() {
		if err := run([]string{"generate", "typescript", "--app", dir, "--out", out}); err != nil {
			t.Fatal(err)
		}
	})
	ts, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export interface Note {\n  title: string;\n  tags?: string[] | null;\n}",
		"notesSave(input: Note, resourcePath?: string): Promise<Note>;",
		"Save a note — requires permission `notes.write`",
	} {
		if !strings.Contains(string(ts), want) {
			t.Errorf("missing %q in:\n%s", want, ts)
		}
	}
}

// A fresh `vitra new` app must build as generated: its go directive can't be
// older than vitra's, and it must require a released vitra version.
func TestScaffoldGoMod_MatchesVitraModule(t *testing.T) {
	t.Setenv("VITRA_MODULE_PATH", "")
	_, self, _, _ := runtime.Caller(0)
	repoMod, err := os.ReadFile(filepath.Join(filepath.Dir(self), "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var goLine string
	for _, line := range strings.Split(string(repoMod), "\n") {
		if strings.HasPrefix(line, "go ") {
			goLine = line
		}
	}
	got := scaffoldGoMod("example.com/app")
	if !strings.Contains(got, "\n"+goLine+"\n") {
		t.Errorf("scaffold go.mod %q lacks vitra's %q", got, goLine)
	}
	if !strings.Contains(got, "require go.klarlabs.de/vitra v"+vitra.Version+"\n") {
		t.Errorf("scaffold go.mod %q does not require v%s", got, vitra.Version)
	}
}

func TestParseTTL(t *testing.T) {
	for in, want := range map[string]time.Duration{"30d": 30 * 24 * time.Hour, "72h": 72 * time.Hour, "90m": 90 * time.Minute} {
		if got, err := parseTTL(in); err != nil || got != want {
			t.Errorf("parseTTL(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "0d", "-1d", "xd", "-5h", "soon"} {
		if _, err := parseTTL(bad); err == nil {
			t.Errorf("parseTTL(%q) accepted", bad)
		}
	}
}
