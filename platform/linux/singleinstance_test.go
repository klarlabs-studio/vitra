package linux

import "testing"

func TestTrySingleInstance_Exclusive(t *testing.T) {
	h := New()
	held, release, err := h.TrySingleInstance("vitra-si-test")
	if err != nil || !held {
		t.Fatalf("first lock: held=%v err=%v", held, err)
	}
	defer release()

	h2 := New()
	held2, release2, err := h2.TrySingleInstance("vitra-si-test")
	if err != nil {
		t.Fatalf("second lock err: %v", err)
	}
	if held2 {
		release2()
		t.Fatal("second lock should not be held")
	}
}
