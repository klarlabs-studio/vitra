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
