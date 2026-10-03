//go:build windows && cgo && vitra_native

package windows

import (
	"os"
	"testing"
)

// The host's windows belong to the process's main thread. Tests run on other
// goroutines, so TestMain keeps the main goroutine on the main thread (init
// locks it) and runs whatever onMainThread hands it.

var mainThreadJobs = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case f := <-mainThreadJobs:
			f()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMainThread runs f on the main thread and waits for it to return.
func onMainThread(f func()) {
	finished := make(chan struct{})
	mainThreadJobs <- func() {
		defer close(finished)
		f()
	}
	<-finished
}
