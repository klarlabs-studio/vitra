package worker_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"go.klarlabs.de/vitra/worker"
)

func TestSession_RequestEcho(t *testing.T) {
	host, peer := worker.PipePair()
	t.Cleanup(func() {
		_ = host.Close()
		_ = peer.Close()
	})

	go func() {
		ctx := context.Background()
		for {
			f, err := peer.Recv(ctx)
			if err != nil {
				return
			}
			if f.Type != worker.FrameRequest || f.Method != "echo" {
				_ = peer.ReplyError(ctx, f.ID, "bad")
				continue
			}
			_ = peer.Reply(ctx, f.ID, json.RawMessage(f.Payload))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := host.Request(ctx, "echo", map[string]any{"n": 7})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["n"] != float64(7) {
		t.Fatalf("payload=%v", payload)
	}
}

func TestSession_OversizedSendRejected(t *testing.T) {
	host, peer := worker.PipePair(worker.WithMaxFrame(64))
	t.Cleanup(func() {
		_ = host.Close()
		_ = peer.Close()
	})
	big, _ := json.Marshal(strings.Repeat("x", 128))
	err := host.Send(context.Background(), worker.Frame{Type: worker.FrameMessage, Payload: big})
	if err == nil || !strings.Contains(err.Error(), "max size") {
		t.Fatalf("expected max size error, got %v", err)
	}
}

func TestSession_MalformedFailsClosed(t *testing.T) {
	pr, pw := io.Pipe()
	sess := worker.NewSession(pr, io.Discard, pr, worker.WithMaxFrame(256))
	t.Cleanup(func() { _ = sess.Close() })

	go func() {
		_, _ = pw.Write([]byte("{not-json}\n"))
		_ = pw.Close()
	}()

	_, err := sess.Recv(context.Background())
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed error, got %v", err)
	}
	if err := sess.Send(context.Background(), worker.Frame{Type: worker.FrameMessage}); err == nil {
		t.Fatal("expected closed session after malformed frame")
	}
}

func TestSession_RequestRemoteError(t *testing.T) {
	host, peer := worker.PipePair()
	t.Cleanup(func() {
		_ = host.Close()
		_ = peer.Close()
	})
	go func() {
		ctx := context.Background()
		f, err := peer.Recv(ctx)
		if err != nil {
			return
		}
		_ = peer.ReplyError(ctx, f.ID, "nope")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := host.Request(ctx, "missing", nil)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected remote error, got %v", err)
	}
}
