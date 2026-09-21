package worker_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/vitra/worker"
)

func TestCommandRunner_CrashAndStop(t *testing.T) {
	run, err := worker.CommandRunner(worker.Command{Path: "/bin/false"})
	if err != nil {
		t.Fatal(err)
	}
	sup := worker.NewSupervisor()
	if err := sup.Start(context.Background(), worker.Spec{ID: "fail", Name: "fail", MaxRestarts: 0}, run); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec, err := sup.Get("fail")
		if err != nil {
			t.Fatal(err)
		}
		if rec.State == worker.StateCrashed {
			if rec.LastError == "" {
				t.Fatal("expected last error")
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if rec, _ := sup.Get("fail"); rec.State != worker.StateCrashed {
		t.Fatalf("state=%s", rec.State)
	}

	sleep, err := worker.CommandRunner(worker.Command{Path: "/bin/sleep", Args: []string{"30"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := sup.Start(context.Background(), worker.Spec{ID: "sleep", Name: "sleep", MaxRestarts: 0}, sleep); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := sup.Stop("sleep"); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ := sup.Get("sleep")
		if rec.State == worker.StateStopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("sleep worker did not stop")
}

func TestCommandRunner_RequiresPath(t *testing.T) {
	if _, err := worker.CommandRunner(worker.Command{}); err == nil {
		t.Fatal("expected path error")
	}
}
