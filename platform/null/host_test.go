package null_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
	"go.klarlabs.de/vitra/platform/null"
)

func TestNullHost_ExplicitUnsupportedDialog(t *testing.T) {
	// Security invariant 14: unsupported platform behaviour is explicit.
	h := null.New(platform.OSLinux)
	err := h.DialogOpen(context.Background())
	var unsupp *platform.ErrUnsupported
	if !errors.As(err, &unsupp) || unsupp.Feature != platform.FeatureDialogOpen {
		t.Fatalf("expected explicit unsupported dialog, got %v", err)
	}
	if h.Features().Available(platform.FeatureDialogOpen) {
		t.Fatal("dialog must not be available on null host")
	}
}

func TestNullHost_WindowAndMessageRoundTrip(t *testing.T) {
	h := null.New(platform.OSDarwin)
	ctx := context.Background()
	if err := h.CreateWindow(ctx, platform.WindowSpec{
		ID:     "main",
		Title:  "Demo",
		Origin: domain.OriginPackagedLocal,
	}); err != nil {
		t.Fatal(err)
	}
	msg := []byte(`{"protocol":"1","kind":"invoke"}`)
	if err := h.PostMessage(ctx, "main", msg); err != nil {
		t.Fatal(err)
	}
	got := h.Messages()
	if len(got) != 1 || string(got[0].Message) != string(msg) {
		t.Fatalf("messages=%v", got)
	}
	if err := h.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		t.Fatal(err)
	}
	if err := h.CloseWindow(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if err := h.PostMessage(ctx, "main", msg); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatalf("expected not found after close, got %v", err)
	}
}
