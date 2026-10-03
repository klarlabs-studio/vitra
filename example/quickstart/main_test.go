package main

import (
	"strings"
	"testing"
)

// The walkthrough runs end to end: allowed, denied by a deny pattern, and
// denied after navigation.
func TestQuickstart(t *testing.T) {
	var out strings.Builder
	if err := run(&out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	for _, want := range []string{"3) invoke authorized via grant project-files", "4) ", "5) "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
