package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"go.klarlabs.de/vitra/worker"
)

func buildEchoWorker(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "echo")
	cmd := exec.Command("go", "build", "-o", out, "./testdata/echo")
	cmd.Dir = ".."
	// package tests run with cwd = worker/; testdata is worker/testdata
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build echo worker: %v\n%s", err, outBytes)
	}
	return out
}

func TestStdioIPC_EchoRoundTrip(t *testing.T) {
	bin := buildEchoWorker(t)
	run, host, err := worker.StdioIPC(worker.Command{Path: bin})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })

	sup := worker.NewSupervisor()
	if err := sup.Start(context.Background(), worker.Spec{ID: "echo", Name: "echo", MaxRestarts: 0}, run); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rec, _ := sup.Get("echo"); rec.State == worker.StateRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := host.Request(ctx, "echo", map[string]any{"hello": "vitra"})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["hello"] != "vitra" {
		t.Fatalf("payload=%v", payload)
	}

	_ = host.Close()
	if err := sup.Stop("echo"); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rec, _ := sup.Get("echo"); rec.State == worker.StateStopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("echo worker did not stop")
}
