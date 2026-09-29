package windows

import (
	"context"
	"strings"
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
	if gotBin != "rundll32" || len(gotArgs) != 2 || gotArgs[0] != "url.dll,FileProtocolHandler" {
		t.Fatalf("bin=%q args=%v", gotBin, gotArgs)
	}
	if gotArgs[1] != "https://example.com/path?q=1" {
		t.Fatalf("url arg=%v", gotArgs)
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

// cmd.exe interprets &, |, ^, <, > in its command line even inside a quoted
// argv element built by os/exec, so a URL must never reach a shell.
func TestOpenURL_NeverInvokesShell(t *testing.T) {
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

	for _, raw := range []string{
		"https://example.com/?x&calc",
		"https://example.com/?a|whoami",
		"https://example.com/?a^b>out.txt",
		"mailto:dev@example.com?subject=a&body=calc",
	} {
		gotBin, gotArgs = "", nil
		if err := h.OpenURL(context.Background(), raw); err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		switch strings.ToLower(gotBin) {
		case "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh":
			t.Fatalf("%q: opener %q interprets shell metacharacters", raw, gotBin)
		}
		for _, a := range gotArgs {
			if strings.EqualFold(a, "/c") || strings.EqualFold(a, "start") {
				t.Fatalf("%q: shell-style argv %v", raw, gotArgs)
			}
		}
	}
}
