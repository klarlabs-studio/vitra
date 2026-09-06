package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRun_VersionDoctorInspectHelp(t *testing.T) {
	out := capture(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "vitra") {
		t.Fatalf("version output: %q", out)
	}

	out = capture(t, func() {
		if err := run([]string{"doctor"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "kernel:") {
		t.Fatalf("doctor output: %q", out)
	}

	out = capture(t, func() {
		if err := run([]string{"inspect", "capabilities"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "project-files") {
		t.Fatalf("inspect output: %q", out)
	}

	if err := run([]string{"inspect"}); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run([]string{"nope"}); err == nil {
		t.Fatal("expected unknown command")
	}

	out = capture(t, func() {
		if err := run(nil); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{"help"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("help output: %q", out)
	}
}

func capture(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}
