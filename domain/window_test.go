package domain_test

import (
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func TestWindow_LifecycleAndCaller(t *testing.T) {
	win, err := domain.NewWindow("main", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}
	if !win.IsOpen() || win.Origin() != domain.OriginPackagedLocal {
		t.Fatalf("bad initial state")
	}
	caller, err := win.Caller()
	if err != nil || caller.Window != "main" {
		t.Fatalf("caller=%v err=%v", caller, err)
	}
	if err := win.Close(); err != nil {
		t.Fatal(err)
	}
	if win.IsOpen() {
		t.Fatal("expected closed")
	}
	if _, err := win.Caller(); err == nil {
		t.Fatal("closed window must not yield caller")
	}
}

func TestResourceHandle_Ownership(t *testing.T) {
	h, err := domain.NewResourceHandle("res-1", "file", "main")
	if err != nil {
		t.Fatal(err)
	}
	if h.Owner() != "main" || h.IsClosed() {
		t.Fatal("bad handle")
	}
	h.Close()
	if !h.IsClosed() {
		t.Fatal("expected closed")
	}
	h.Close() // idempotent
}
