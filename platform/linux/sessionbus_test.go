//go:build linux && cgo && vitra_native

package linux

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D-Bus test harness shared by the portal and tray tests.
//
// TestMain gives the whole test binary a private session bus (when
// dbus-daemon is installed) before anything connects to one: GLib caches the
// session bus per process, so it must be in place first. Fake D-Bus services
// are small python3-gi scripts in testdata, run on that bus.

// privateBus is the private session bus address, or empty when none runs.
var privateBus string

func TestMain(m *testing.M) {
	stop := startPrivateSessionBus()
	code := m.Run()
	stop()
	os.Exit(code)
}

func startPrivateSessionBus() func() {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		return func() {}
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address=1")
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return func() {}
	}
	if err := cmd.Start(); err != nil {
		return func() {}
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	line := make(chan string, 1)
	go func() {
		addr, _ := bufio.NewReader(out).ReadString('\n')
		line <- strings.TrimSpace(addr)
		_, _ = io.Copy(io.Discard, out)
	}()
	select {
	case addr := <-line:
		if addr == "" {
			stop()
			return func() {}
		}
		privateBus = addr
		_ = os.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
		return stop
	case <-time.After(10 * time.Second):
		stop()
		return func() {}
	}
}

// requireDBusFakes skips (or, in CI, fails) when the private bus or the
// python3-gi runtime for the fake services is missing.
func requireDBusFakes(t *testing.T) {
	t.Helper()
	missing := ""
	if privateBus == "" {
		missing = "dbus-daemon (package dbus)"
	} else if err := exec.Command("python3", "-c", "from gi.repository import Gio").Run(); err != nil {
		missing = "python3 with GObject introspection (python3-gi)"
	}
	if missing == "" {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("D-Bus fake services need %s", missing)
	}
	t.Skipf("D-Bus fake services need %s", missing)
}

// fakeService is a fake D-Bus service script; it prints one JSON event per
// line and takes commands on stdin.
type fakeService struct {
	t      *testing.T
	stdin  io.WriteCloser
	events chan map[string]any
}

// startFake runs testdata/<script> with args and waits until it owns its bus
// name. It is stopped when the test ends.
func startFake(t *testing.T, script string, args ...string) *fakeService {
	t.Helper()
	requireDBusFakes(t)
	cmd := exec.Command("python3", append([]string{filepath.Join("testdata", script)}, args...)...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	f := &fakeService{t: t, stdin: stdin, events: make(chan map[string]any, 64)}
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			var ev map[string]any
			if json.Unmarshal(sc.Bytes(), &ev) == nil {
				f.events <- ev
			}
		}
		close(f.events)
	}()
	t.Cleanup(func() {
		_ = stdin.Close() // the fake exits on EOF, releasing its name
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})
	f.waitFor("ready", func(ev map[string]any) bool { return ev["ready"] == true })
	return f
}

// send writes one command line to the fake.
func (f *fakeService) send(format string, args ...any) {
	f.t.Helper()
	if _, err := fmt.Fprintf(f.stdin, format+"\n", args...); err != nil {
		f.t.Fatal(err)
	}
}

// waitFor returns the next event matching ok, failing after 10s.
func (f *fakeService) waitFor(what string, ok func(map[string]any) bool) map[string]any {
	f.t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, open := <-f.events:
			if !open {
				f.t.Fatalf("fake service exited while waiting for %s", what)
			}
			if ok(ev) {
				return ev
			}
		case <-deadline:
			f.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// waitCall returns the next event recording a call to method.
func (f *fakeService) waitCall(method string) map[string]any {
	f.t.Helper()
	return f.waitFor(method, func(ev map[string]any) bool { return ev["call"] == method })
}
