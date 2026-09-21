package darwin

import (
	"context"
	"testing"
)

func TestOpenURL_ValidatesAndInvokes(t *testing.T) {
	h := New()
	prev := openURLRunner
	t.Cleanup(func() { openURLRunner = prev })

	var gotBin string
	var gotArgs []string
	openURLRunner = func(_ context.Context, bin string, args ...string) error {
		gotBin = bin
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := h.OpenURL(context.Background(), "https://example.com/path?q=1"); err != nil {
		t.Fatal(err)
	}
	if gotBin != "open" || len(gotArgs) != 1 || gotArgs[0] != "https://example.com/path?q=1" {
		t.Fatalf("bin=%q args=%v", gotBin, gotArgs)
	}

	for _, bad := range []string{
		"",
		"ftp://example.com",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"https://",
	} {
		if err := h.OpenURL(context.Background(), bad); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		}
	}
	if err := h.OpenURL(context.Background(), "mailto:dev@example.com"); err != nil {
		t.Fatal(err)
	}
}
