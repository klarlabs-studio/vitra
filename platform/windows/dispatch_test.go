//go:build windows && cgo && vitra_native

package windows

import (
	"testing"
	"time"
)

// A call from another goroutine before Run starts the loop must still run on
// the UI thread: Win32 windows belong to the thread that creates them, and a
// window created anywhere else is never pumped by Run's message loop.
func TestDispatch_BeforeRunFromAnotherGoroutineRunsOnUIThread(t *testing.T) {
	h := New()
	ranOn := make(chan uint32, 2)
	queued := make(chan struct{})
	go func() {
		h.dispatch(func() { ranOn <- currentThread() })
		h.dispatch(func() {
			ranOn <- currentThread()
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
	var loopThread uint32
	onMainThread(func() {
		loopThread = currentThread()
		_ = h.Run()
	})

	if len(ranOn) != 2 {
		t.Fatalf("%d of 2 dispatched functions ran", len(ranOn))
	}
	for range 2 {
		if tid := <-ranOn; tid != loopThread || tid != mainThread {
			t.Fatalf("dispatched function ran on thread %d, want the UI thread %d", tid, loopThread)
		}
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
