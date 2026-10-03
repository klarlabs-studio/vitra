//go:build darwin && cgo && vitra_native

package darwin

import (
	"os"
	"runtime"
	"testing"
)

// AppKit insists on the process's main thread. Tests run on other
// goroutines, so TestMain keeps the main goroutine on the main thread and
// runs whatever onMainThread hands it.
func init() { runtime.LockOSThread() }

var mainThread = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case f := <-mainThread:
			f()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMainThread runs f on the main thread and waits for it to return.
func onMainThread(f func()) {
	finished := make(chan struct{})
	mainThread <- func() {
		defer close(finished)
		f()
	}
	<-finished
}
