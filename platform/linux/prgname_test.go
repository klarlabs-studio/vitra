//go:build linux && cgo && vitra_native

package linux

import (
	"os"
	"runtime"
	"testing"
)

func TestNativeProgramName(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for GTK init")
	}
	runtime.LockOSThread()
	h := New()
	h.SetProgramName("Vitra-Demo")
	got := h.ProgramName()
	if got != "Vitra-Demo" {
		t.Fatalf("prgname=%q want Vitra-Demo", got)
	}
}
