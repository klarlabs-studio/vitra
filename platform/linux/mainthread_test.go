//go:build linux && cgo && vitra_native

package linux

import (
	"runtime"
	"syscall"
)

// The host owns GTK from the process's main thread. Tests run on other
// goroutines: TestMain keeps the main goroutine on the main thread and runs
// whatever onMainThread hands it, for tests that need Run.

var mainThreadJobs = make(chan func())

// onMainThread runs f on the main thread and waits for it to return.
func onMainThread(f func()) {
	finished := make(chan struct{})
	mainThreadJobs <- func() {
		defer close(finished)
		f()
	}
	<-finished
}

// newHostOnThisThread returns a host that owns GTK from the calling test's
// own thread. Tests that never run the loop drive GTK inline from there; the
// calling goroutine stays locked to its thread for the rest of the test.
func newHostOnThisThread() *Host {
	runtime.LockOSThread()
	h := New()
	h.uiThread = syscall.Gettid()
	return h
}
