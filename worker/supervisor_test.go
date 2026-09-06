package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/vitra/worker"
)

func TestSupervisor_CrashDoesNotCorruptBookkeeping(t *testing.T) {
	// Reliability invariant 4.
	sup := worker.NewSupervisor()
	ctx := context.Background()
	errBoom := errors.New("boom")
	if err := sup.Start(ctx, worker.Spec{ID: "w1", Name: "crashy", MaxRestarts: 0, Elevated: true}, func(context.Context) error {
		return errBoom
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec, err := sup.Get("w1")
		if err != nil {
			t.Fatal(err)
		}
		if rec.State == worker.StateCrashed {
			if rec.LastError == "" || rec.Spec.ID != "w1" {
				t.Fatalf("bookkeeping corrupted: %+v", rec)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("worker did not reach crashed state")
}

func TestSupervisor_Stop(t *testing.T) {
	sup := worker.NewSupervisor()
	ctx := context.Background()
	started := make(chan struct{})
	if err := sup.Start(ctx, worker.Spec{ID: "w2", Name: "ok", MaxRestarts: 0}, func(c context.Context) error {
		close(started)
		<-c.Done()
		return c.Err()
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not started")
	}
	if err := sup.Stop("w2"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ := sup.Get("w2")
		if rec.State == worker.StateStopped {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("not stopped")
}
