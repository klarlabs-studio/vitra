package windows

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenPath_ValidatesAndInvokes(t *testing.T) {
	h := New()
	prev := openPathRunner
	t.Cleanup(func() { openPathRunner = prev })

	var gotBin string
	var gotArgs []string
	openPathRunner = func(_ context.Context, bin string, args ...string) error {
		gotBin = bin
		gotArgs = append([]string(nil), args...)
		return nil
	}

	want := filepath.Clean("/tmp/vitra-open-path")
	if runtime.GOOS == "windows" {
		want = filepath.Clean(`C:\Temp\vitra-open-path`)
	}
	if err := h.OpenPath(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if gotBin != "explorer" || len(gotArgs) != 1 || gotArgs[0] != want {
		t.Fatalf("bin=%q args=%v", gotBin, gotArgs)
	}

	for _, bad := range []string{"", "relative\\path", "https://example.com", "file:///C:/Windows"} {
		if err := h.OpenPath(context.Background(), bad); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		}
	}
}
