package darwin

import (
	"context"
	"path/filepath"
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
	if err := h.OpenPath(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	if gotBin != "open" || len(gotArgs) != 1 || gotArgs[0] != want {
		t.Fatalf("bin=%q args=%v", gotBin, gotArgs)
	}

	for _, bad := range []string{"", "relative/path", "https://example.com", "file:///etc/passwd"} {
		if err := h.OpenPath(context.Background(), bad); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		}
	}
}
