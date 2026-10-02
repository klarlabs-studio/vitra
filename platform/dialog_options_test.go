package platform_test

import (
	"testing"

	"go.klarlabs.de/vitra/platform"
)

func TestEncodeFileFilters(t *testing.T) {
	if got := platform.EncodeFileFilters(nil); got != "" {
		t.Fatalf("nil: %q", got)
	}
	got := platform.EncodeFileFilters([]platform.FileFilter{
		{Name: "Images", Extensions: []string{"png", ".jpg", " gif "}},
		{Name: " Text ", Extensions: []string{"txt"}},
		{Name: "", Extensions: []string{"md"}},
		{Name: "Empty", Extensions: nil},
	})
	want := "Images:png,jpg,gif;Text:txt;md:md;Empty:"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDecodePathList(t *testing.T) {
	if got := platform.DecodePathList(nil); got != nil {
		t.Fatalf("nil buffer = %#v, want nil", got)
	}
	// Newlines are legal in Linux file names, so they must survive.
	got := platform.DecodePathList([]byte("/tmp/a b.txt\x00/tmp/line\nbreak.md\x00"))
	if len(got) != 2 || got[0] != "/tmp/a b.txt" || got[1] != "/tmp/line\nbreak.md" {
		t.Fatalf("decoded = %#v", got)
	}
	// A missing trailing NUL and empty entries are tolerated, never returned.
	got = platform.DecodePathList([]byte("\x00/a\x00\x00/b"))
	if len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Fatalf("decoded = %#v", got)
	}
}
