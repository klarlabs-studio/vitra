//go:build darwin && cgo && vitra_native

package darwin

import (
	"testing"
	"time"
)

// A call from another goroutine before Run starts the loop must still run on
// the main thread: AppKit aborts the process when a window or panel is
// created anywhere else.
func TestDispatch_BeforeRunFromAnotherGoroutineRunsOnMainThread(t *testing.T) {
	h := New()
	onMain := make(chan bool, 1)
	queued := make(chan struct{})
	go func() {
		h.dispatch(func() {
			onMain <- isMainThread()
			h.Quit()
		})
		close(queued)
	}()
	<-queued
	stop := time.AfterFunc(5*time.Second, h.Quit)
	defer stop.Stop()
	onMainThread(func() { _ = h.Run() })

	select {
	case ok := <-onMain:
		if !ok {
			t.Fatal("dispatched function ran off the main thread")
		}
	default:
		t.Fatal("dispatched function never ran")
	}
}
