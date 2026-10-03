//go:build linux && cgo && vitra_native

package linux

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// A call from another goroutine before Run starts the loop must still run on
// the UI thread: GTK is not thread-safe, so gtk_init and widget calls made
// anywhere else are undefined behaviour.
func TestDispatch_BeforeRunFromAnotherGoroutineRunsOnUIThread(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is required for the GTK main loop")
	}
	h := New()
	ranOn := make(chan int, 1)
	queued := make(chan struct{})
	go func() {
		h.dispatch(func() {
			ranOn <- syscall.Gettid()
			h.Quit()
		})
		close(queued)
	}()
	<-queued
	select {
	case <-ranOn:
		t.Fatal("dispatched function ran before Run started the loop")
	default:
	}
	stop := time.AfterFunc(5*time.Second, h.Quit)
	defer stop.Stop()
	loopThread := 0
	onMainThread(func() {
		loopThread = syscall.Gettid()
		_ = h.Run()
	})

	select {
	case tid := <-ranOn:
		if tid != loopThread || tid != mainThread {
			t.Fatalf("dispatched function ran on thread %d, want the UI thread %d", tid, loopThread)
		}
	default:
		t.Fatal("dispatched function never ran")
	}
}

// Before Run, a call on the UI thread itself still runs inline.
func TestDispatch_BeforeRunOnUIThreadRunsInline(t *testing.T) {
	h := New()
	ran := false
	onMainThread(func() { h.dispatch(func() { ran = true }) })
	if !ran {
		t.Fatal("a call on the UI thread before Run did not run inline")
	}
}
