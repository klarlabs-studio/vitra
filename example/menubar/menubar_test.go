package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	for n, want := range map[uint64]string{
		0: "0 B", 999: "999 B", 1000: "1.0 KB", 9_500_000_000: "9.5 GB",
		87_400_000_000: "87 GB", 2_000_000_000_000: "2.0 TB",
	} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestReadUsage(t *testing.T) {
	u, err := readUsage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if u.Total == 0 || u.Free > u.Total || u.UsedPercent < 0 || u.UsedPercent > 100 {
		t.Fatalf("usage %+v", u)
	}
	if _, err := readUsage("/does/not/exist/anywhere"); err == nil {
		t.Fatal("usage of a missing path")
	}
}

// The icon is a template image: opaque black where the disk is used, a ring
// where it is free, transparent elsewhere.
func TestTrayIcon(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayIcon(25)))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("icon %v", b)
	}
	alpha := func(x, y int) uint32 { _, _, _, a := img.At(x, y).RGBA(); return a }
	c := iconSize / 2
	if alpha(c+6, c-6) == 0 { // top right quarter: used
		t.Error("used quarter is empty")
	}
	if alpha(c-6, c+6) != 0 { // bottom left, inside the ring: free
		t.Error("free area is filled")
	}
	if alpha(c-1, iconSize-3) == 0 { // the ring at the bottom
		t.Error("ring is missing")
	}
	if alpha(0, 0) != 0 {
		t.Error("corner is not transparent")
	}
	if r, g, b, _ := img.At(c+6, c-6).RGBA(); r|g|b != 0 {
		t.Error("template icon is not black")
	}
}
