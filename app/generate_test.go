package app_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/app"
	"go.klarlabs.de/vitra/domain"
)

type echoIn struct {
	Text string `json:"text"`
}

// With VITRA_GENERATE_TYPESCRIPT set, Run writes the app's own typed client
// and returns without serving assets or opening a window.
func TestRun_GenerateTypeScriptMode(t *testing.T) {
	out := filepath.Join(t.TempDir(), "client.ts")
	t.Setenv(app.EnvGenerateTypeScript, out)

	rt, err := vitra.New(vitra.Config{AppID: "com.example.gen"})
	if err != nil {
		t.Fatal(err)
	}
	if err := vitra.Register(rt, vitra.Command[echoIn, string]{
		Name: "echo", Permission: "echo",
		Handler: func(_ context.Context, _ domain.Invocation, in echoIn) (string, error) { return in.Text, nil },
	}); err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{quit: make(chan struct{}), ran: make(chan struct{})}
	application, err := app.New(app.Options{
		AppID:   "com.example.gen",
		Assets:  fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}},
		Host:    host,
		Runtime: rt,
		Window:  app.WindowOptions{ID: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if host.opened {
		t.Fatal("generate mode opened a window")
	}
	ts, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ts), "echo(input: EchoIn, resourcePath?: string): Promise<string>;") {
		t.Fatalf("client:\n%s", ts)
	}
}
