package windows

import (
	"errors"
	"testing"
)

func TestClipboard_GetSet(t *testing.T) {
	h := New()
	prevGet, prevSet := clipboardGetRunner, clipboardSetRunner
	t.Cleanup(func() {
		clipboardGetRunner = prevGet
		clipboardSetRunner = prevSet
	})

	var stored string
	clipboardGetRunner = func() (string, error) { return stored, nil }
	clipboardSetRunner = func(text string) error {
		stored = text
		return nil
	}

	if err := h.ClipboardSet("hello vitra"); err != nil {
		t.Fatal(err)
	}
	got, err := h.ClipboardGet()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello vitra" {
		t.Fatalf("got %q", got)
	}

	clipboardGetRunner = func() (string, error) { return "", errors.New("boom") }
	if _, err := h.ClipboardGet(); err == nil {
		t.Fatal("expected get error")
	}
	clipboardSetRunner = func(string) error { return errors.New("boom") }
	if err := h.ClipboardSet("x"); err == nil {
		t.Fatal("expected set error")
	}
}
