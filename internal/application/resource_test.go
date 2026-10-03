package application_test

import (
	"testing"

	"go.klarlabs.de/vitra/internal/application"
)

func TestResourceHandle_Ownership(t *testing.T) {
	h, err := application.NewResourceHandle("res-1", "file", "main")
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

func TestNewResourceID(t *testing.T) {
	if _, err := application.NewResourceID(""); err == nil {
		t.Fatal("empty resource id accepted")
	}
	id, err := application.NewResourceID("res-1")
	if err != nil || id != "res-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}
