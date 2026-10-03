package official_test

import (
	"encoding/json"
	"testing"

	"go.klarlabs.de/vitra/internal/strictjson"
	"go.klarlabs.de/vitra/plugin/official"
)

// The input types decode page input, which Vitra treats as hostile. Each
// fuzz target below feeds one of them whatever JSON the fuzzer can produce,
// decoded the way a typed command decodes it: the IPC bridge decodes the
// payload into `any`, then strictjson.Decode converts it to the input type.
// Decoding must never panic, and anything it accepts must satisfy the type's
// documented contract.

// Shared seeds: shapes every input type should reject or tolerate.
var hostileJSONSeeds = []string{
	`null`, `true`, `false`, `0`, `-1`, `1.5`, `1e309`, `""`, `"main"`,
	`[]`, `{}`, `[null]`, `[[[[[]]]]]`, `{"":null}`,
	`{"id":null}`, `{"id":1}`, `{"id":["main"]}`, `{"id":{"id":"main"}}`,
	`{"__proto__":{"id":"main"}}`, `"\u0000"`, `{"id":"\ud800"}`,
}

// fuzzInput adds the shared seeds plus the target's own, then runs check on
// every input that decodes as JSON and as T.
func fuzzInput[T any](f *testing.F, seeds []string, check func(t *testing.T, input any, got T)) {
	f.Helper()
	for _, seed := range append(append([]string{}, hostileJSONSeeds...), seeds...) {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		var input any
		if err := json.Unmarshal(raw, &input); err != nil {
			return
		}
		got, err := strictjson.Decode[T](input)
		if err != nil {
			return
		}
		check(t, input, got)
	})
}

// maxExactJSONInt is 2^53: above it a decoded JSON number may not be the
// integer the page wrote.
const maxExactJSONInt = 1 << 53

// requireExactSize fails unless got is exactly the positive integer the page
// sent under key. A size must never be rounded, wrapped, or clamped.
func requireExactSize(t *testing.T, input any, key string, got int) {
	t.Helper()
	m, _ := input.(map[string]any)
	n, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%s=%d accepted from non-number %#v", key, got, m[key])
	}
	if got <= 0 || n > maxExactJSONInt || float64(got) != n {
		t.Fatalf("%s: page sent %v, decoded %d", key, n, got)
	}
}

func checkMenuItems(t *testing.T, items []official.MenuItem) {
	t.Helper()
	for _, it := range items {
		if it.ID == "" || it.Label == "" {
			t.Fatalf("accepted menu item without id or label: %+v", it)
		}
	}
}

var menuSeeds = []string{
	`[{"id":"open","label":"Open","menu":"File","shortcut":"CmdOrCtrl+O"}]`,
	`{"items":[{"id":"a","label":"A"},{"id":"b","label":"B","children":[{"id":"c"}]}]}`,
	`{"items":{}}`, `{"items":null}`, `[{"id":"a"}]`, `[{"id":"","label":"x"}]`,
	`[1,"a",null]`, `[{"id":"a","label":1}]`,
}

func FuzzMenuInput(f *testing.F) {
	fuzzInput(f, menuSeeds, func(t *testing.T, _ any, in official.MenuInput) {
		checkMenuItems(t, in.Items)
	})
}

func FuzzTrayInput(f *testing.F) {
	seeds := append([]string{
		`{"tooltip":"Vitra","items":[{"id":"quit","label":"Quit"}]}`,
		`{"tooltip":1,"items":"x"}`, `{"tooltip":"only"}`,
	}, menuSeeds...)
	fuzzInput(f, seeds, func(t *testing.T, _ any, in official.TrayInput) {
		checkMenuItems(t, in.Items)
	})
}

func FuzzDialogOptions(f *testing.F) {
	fuzzInput(f, []string{
		`{"title":"Open","defaultPath":"/tmp","filters":[{"name":"Images","extensions":["png","jpg"]}]}`,
		`{"filters":[{"name":"Text","extensions":"txt"}]}`,
		`{"filters":[null,1,{"extensions":[1,null,"go"]}]}`,
		`{"filters":{}}`, `{"title":["x"],"defaultPath":{}}`,
		`{"multiple":true}`, `{"multiple":"true"}`, `{"multiple":1,"filters":[]}`,
	}, func(t *testing.T, input any, opts official.DialogOptions) {
		m, isObject := input.(map[string]any)
		if !isObject {
			if input != nil {
				t.Fatalf("accepted non-object input %#v", input)
			}
			return
		}
		// Only a JSON true opts into a multi-file selection.
		if want := m["multiple"] == true; opts.Multiple != want {
			t.Fatalf("input %#v: Multiple = %v, want %v", input, opts.Multiple, want)
		}
	})
}

func FuzzShortcutInput(f *testing.F) {
	fuzzInput(f, []string{
		`{"accelerator":"CmdOrCtrl+K","action":"palette"}`,
		`{"accelerator":"Ctrl+A","actionID":"a"}`,
		`{"accelerator":"Ctrl+A","id":"a"}`,
		`{"accelerator":"Ctrl+A","action":1,"id":"b"}`,
		`{"accelerator":"Ctrl+A"}`, `{"action":"x"}`,
	}, func(t *testing.T, _ any, in official.ShortcutInput) {
		if in.Accelerator == "" || in.Action == "" {
			t.Fatalf("accepted empty accelerator %q or action %q", in.Accelerator, in.Action)
		}
	})
}

func FuzzShortcutRef(f *testing.F) {
	fuzzInput(f, []string{
		`"Ctrl+B"`, `{"accelerator":"Ctrl+Shift+Q"}`, `{"accelerator":""}`, `{"accelerator":1}`,
	}, func(t *testing.T, _ any, in official.ShortcutRef) {
		if in.Accelerator == "" {
			t.Fatal("accepted an empty accelerator")
		}
	})
}

func FuzzDragDropInput(f *testing.F) {
	fuzzInput(f, []string{
		`true`, `false`, `{"window":"aux","enabled":false}`, `{"id":"aux"}`,
		`{"id":"","window":""}`, `{"enabled":"false"}`, `{"enabled":0}`,
	}, func(t *testing.T, _ any, in official.DragDropInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
	})
}

var sizeSeeds = []string{
	`{"id":"main","width":640,"height":480}`,
	`{"id":"main","width":0,"height":-1}`,
	`{"id":"main","width":640.5,"height":480}`,
	`{"id":"main","width":1e308,"height":1e308}`,
	`{"id":"main","width":9223372036854775807,"height":9223372036854775808}`,
	`{"id":"main","width":18446744073709551616,"height":2147483648}`,
	`{"id":"main","width":"640","height":true}`,
	`{"id":"main","width":-0,"height":4.9e-324}`,
}

func FuzzWindowCreateInput(f *testing.F) {
	fuzzInput(f, append([]string{
		`"aux"`, `{"id":"aux","title":"Aux","path":"/aux.html","width":800,"height":600}`,
		`{"title":"no id"}`,
	}, sizeSeeds...), func(t *testing.T, input any, in official.WindowCreateInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if in.Width != 0 {
			requireExactSize(t, input, "width", in.Width)
		}
		if in.Height != 0 {
			requireExactSize(t, input, "height", in.Height)
		}
	})
}

func FuzzWindowRef(f *testing.F) {
	fuzzInput(f, []string{`"aux"`, `{"id":"aux"}`, `{"id":""}`, `{"window":"aux"}`},
		func(t *testing.T, _ any, in official.WindowRef) {
			if in.ID == "" {
				t.Fatal("accepted an empty window id")
			}
		})
}

func FuzzWindowAlwaysOnTopInput(f *testing.F) {
	fuzzInput(f, []string{
		`{"id":"main","alwaysOnTop":true}`, `{"id":"main"}`, `{"id":"main","alwaysOnTop":"true"}`,
	}, func(t *testing.T, input any, in official.WindowAlwaysOnTopInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["alwaysOnTop"]; sent != in.AlwaysOnTop {
			t.Fatalf("page sent alwaysOnTop=%#v, decoded %v", sent, in.AlwaysOnTop)
		}
	})
}

func FuzzWindowTitleInput(f *testing.F) {
	fuzzInput(f, []string{
		`{"id":"main","title":"Hi"}`, `{"id":"main","title":""}`, `{"id":"main"}`, `{"id":"main","title":null}`,
	}, func(t *testing.T, input any, in official.WindowTitleInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["title"]; sent != in.Title {
			t.Fatalf("page sent title=%#v, decoded %q", sent, in.Title)
		}
	})
}

func FuzzWindowSizeInput(f *testing.F) {
	fuzzInput(f, sizeSeeds, func(t *testing.T, input any, in official.WindowSizeInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		requireExactSize(t, input, "width", in.Width)
		requireExactSize(t, input, "height", in.Height)
	})
}

func FuzzWindowIconInput(f *testing.F) {
	fuzzInput(f, []string{
		`{"id":"main","iconPath":"/icons/app.png"}`, `{"id":"main","iconPath":""}`, `{"id":"main"}`,
	}, func(t *testing.T, input any, in official.WindowIconInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["iconPath"]; sent != in.IconPath {
			t.Fatalf("page sent iconPath=%#v, decoded %q", sent, in.IconPath)
		}
	})
}

func FuzzWindowChromeInput(f *testing.F) {
	fuzzInput(f, append([]string{
		`{"id":"main","title":"T","width":800,"height":600,"maximized":true,"fullscreen":false,` +
			`"alwaysOnTop":true,"minimized":false,"hidden":false,"iconPath":"/i.png"}`,
		`{"id":"main","maximized":"yes","hidden":1}`,
	}, sizeSeeds...), func(t *testing.T, input any, in official.WindowChromeInput) {
		if in.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if in.Width != 0 {
			requireExactSize(t, input, "width", in.Width)
		}
		if in.Height != 0 {
			requireExactSize(t, input, "height", in.Height)
		}
	})
}

func FuzzMessageDialogInput(f *testing.F) {
	fuzzInput(f, []string{
		`"Saved"`, `{"title":"T","message":"M","kind":"confirm"}`, `{"message":1}`, `{"text":"x"}`,
	}, func(t *testing.T, input any, in official.MessageDialogInput) {
		if s, ok := input.(string); ok && (in.Message != s || in.Title != "" || in.Kind != "") {
			t.Fatalf("string %q decoded as %+v", s, in)
		}
	})
}

func FuzzNotificationInput(f *testing.F) {
	fuzzInput(f, []string{
		`"Done"`, `{"title":"T","body":"B"}`, `{"body":1}`, `{"message":"x"}`,
	}, func(t *testing.T, input any, in official.NotificationInput) {
		if s, ok := input.(string); ok && (in.Body != s || in.Title != "") {
			t.Fatalf("string %q decoded as %+v", s, in)
		}
	})
}
