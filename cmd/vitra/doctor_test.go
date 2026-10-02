package main

import (
	"encoding/json"
	"strings"
	"testing"
)

type doctorJSON struct {
	Checks []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"checks"`
}

func TestDoctorJSON_ShapeAndCoversTextChecks(t *testing.T) {
	var jsonErr, textErr error
	jsonOut := capture(t, func() { jsonErr = run([]string{"doctor", "--json"}) })
	textOut := capture(t, func() { textErr = run([]string{"doctor"}) })

	if (jsonErr == nil) != (textErr == nil) {
		t.Fatalf("exit status differs: json err=%v, text err=%v", jsonErr, textErr)
	}

	dec := json.NewDecoder(strings.NewReader(jsonOut))
	dec.DisallowUnknownFields()
	var report doctorJSON
	if err := dec.Decode(&report); err != nil {
		t.Fatalf("doctor --json is not valid JSON: %v\n%s", err, jsonOut)
	}
	if dec.More() {
		t.Fatalf("doctor --json printed more than one JSON value:\n%s", jsonOut)
	}
	if len(report.Checks) == 0 {
		t.Fatal("doctor --json reported no checks")
	}

	names := map[string]bool{}
	for _, c := range report.Checks {
		if c.Name == "" {
			t.Fatalf("check without name: %+v", c)
		}
		switch c.Status {
		case "ok", "warn", "fail":
		default:
			t.Fatalf("check %q has status %q, want ok|warn|fail", c.Name, c.Status)
		}
		if c.Detail == "" {
			t.Fatalf("check %q has empty detail", c.Name)
		}
		names[c.Name] = true
	}

	for _, label := range doctorTextLabels(textOut) {
		if !names[label] {
			t.Errorf("text check %q missing from --json output", label)
		}
	}
	for _, want := range []string{"go", "os/arch", "cgo", "kernel", "adapter", "window.create", "appimagetool", "codesign"} {
		if !names[want] {
			t.Errorf("--json missing check %q", want)
		}
	}
}

func TestDoctor_RejectsUnknownFlag(t *testing.T) {
	if err := run([]string{"doctor", "--bogus"}); err == nil {
		t.Fatal("doctor accepted an unknown flag")
	}
}

// doctorTextLabels returns the label of every check line in the text report:
// indented "label: value" lines, excluding the title and section headings
// (lines ending in ':').
func doctorTextLabels(out string) []string {
	var labels []string
	for i, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if i == 0 || trimmed == "" || strings.HasSuffix(trimmed, ":") {
			continue
		}
		label, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		labels = append(labels, label)
	}
	return labels
}

func TestDoctorTextLabels(t *testing.T) {
	got := doctorTextLabels("vitra doctor\n  go:      go1\n  packaging fold tools:\n    hdiutil:     ok (/x)\n")
	if len(got) != 2 || got[0] != "go" || got[1] != "hdiutil" {
		t.Fatalf("labels = %v", got)
	}
}
