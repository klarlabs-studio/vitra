package strictjson_test

import (
	"testing"

	"go.klarlabs.de/vitra/internal/strictjson"
)

type point struct {
	X int `json:"x"`
}

func TestDecode_RejectsUnknownFieldsAndMismatchedTypes(t *testing.T) {
	if p, err := strictjson.Decode[point](map[string]any{"x": 1.0}); err != nil || p.X != 1 {
		t.Fatalf("Decode = %+v, %v", p, err)
	}
	for _, input := range []any{
		map[string]any{"x": 1.0, "y": 2.0},
		map[string]any{"x": "1"},
		map[string]any{"x": 1.5},
		"x",
	} {
		if p, err := strictjson.Decode[point](input); err == nil {
			t.Errorf("Decode(%v) = %+v, want an error", input, p)
		}
	}
}

func TestUnmarshal_RejectsTrailingData(t *testing.T) {
	var p point
	if err := strictjson.Unmarshal([]byte(`{"x":1} {"x":2}`), &p); err == nil {
		t.Fatal("accepted trailing data")
	}
	if err := strictjson.Unmarshal([]byte(` {"x":1} `), &p); err != nil || p.X != 1 {
		t.Fatalf("Unmarshal = %+v, %v", p, err)
	}
}
