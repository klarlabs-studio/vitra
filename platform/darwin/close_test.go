//go:build darwin && cgo && vitra_native

package darwin

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/vitra/platform"
)

// Closing a window frees it once (AppKit released it on close as well, and
// the next event crashed the process), and the host keeps working after its
// last window is gone: a menu bar app lives on in its tray.
func TestCloseWindow_HostKeepsWorking(t *testing.T) {
	h := New()
	ranAfterClose := make(chan struct{})
	go func() {
		defer h.Quit()
		if err := h.Open(platform.WindowSpec{ID: "main", Title: "Main", Width: 300, Height: 200}, "about:blank", ""); err != nil {
			t.Error(err)
			return
		}
		if err := h.CloseWindow(context.Background(), "main"); err != nil {
			t.Error(err)
			return
		}
		h.dispatch(func() { close(ranAfterClose) })
	}()
	stop := time.AfterFunc(10*time.Second, h.Quit)
	defer stop.Stop()
	onMainThread(func() { _ = h.Run() })
	select {
	case <-ranAfterClose:
	default:
		t.Fatal("the host stopped running calls after its last window closed")
	}
}
