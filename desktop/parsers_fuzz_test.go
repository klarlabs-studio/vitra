package desktop_test

import (
	"encoding/json"
	"testing"

	"go.klarlabs.de/vitra/desktop"
)

// The Parse* functions decode page input, which Vitra treats as hostile.
// Each fuzz target below feeds them whatever JSON the fuzzer can produce,
// decoded the way the IPC bridge decodes an invoke payload: into `any`.
// A parser must never panic, and anything it accepts must satisfy its
// documented contract.

// Shared seeds: shapes every parser should reject or tolerate.
var hostileJSONSeeds = []string{
	`null`, `true`, `false`, `0`, `-1`, `1.5`, `1e309`, `""`, `"main"`,
	`[]`, `{}`, `[null]`, `[[[[[]]]]]`, `{"":null}`,
	`{"id":null}`, `{"id":1}`, `{"id":["main"]}`, `{"id":{"id":"main"}}`,
	`{"__proto__":{"id":"main"}}`, `"\u0000"`, `{"id":"\ud800"}`,
}

// fuzzJSON adds the shared seeds plus the target's own, then runs check on
// every input that decodes as JSON.
func fuzzJSON(f *testing.F, seeds []string, check func(t *testing.T, input any)) {
	f.Helper()
	for _, seed := range append(append([]string{}, hostileJSONSeeds...), seeds...) {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		var input any
		if err := json.Unmarshal(raw, &input); err != nil {
			return
		}
		check(t, input)
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
		t.Fatalf("%s: page sent %v, parser produced %d", key, n, got)
	}
}

func checkMenuItems(t *testing.T, items []desktop.MenuItem) {
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

func FuzzParseMenuItems(f *testing.F) {
	fuzzJSON(f, menuSeeds, func(t *testing.T, input any) {
		items, err := desktop.ParseMenuItems(input)
		if err != nil {
			return
		}
		checkMenuItems(t, items)
	})
}

func FuzzParseTraySet(f *testing.F) {
	seeds := append([]string{
		`{"tooltip":"Vitra","items":[{"id":"quit","label":"Quit"}]}`,
		`{"tooltip":1,"items":"x"}`, `{"tooltip":"only"}`,
	}, menuSeeds...)
	fuzzJSON(f, seeds, func(t *testing.T, input any) {
		_, items, err := desktop.ParseTraySet(input)
		if err != nil {
			return
		}
		checkMenuItems(t, items)
	})
}

func FuzzParseDialogFileOptions(f *testing.F) {
	fuzzJSON(f, []string{
		`{"title":"Open","defaultPath":"/tmp","filters":[{"name":"Images","extensions":["png","jpg"]}]}`,
		`{"filters":[{"name":"Text","extensions":"txt"}]}`,
		`{"filters":[null,1,{"extensions":[1,null,"go"]}]}`,
		`{"filters":{}}`, `{"title":["x"],"defaultPath":{}}`,
		`{"multiple":true}`, `{"multiple":"true"}`, `{"multiple":1,"filters":[]}`,
	}, func(t *testing.T, input any) {
		opts := desktop.ParseDialogFileOptions(input)
		m, isObject := input.(map[string]any)
		if !isObject {
			if opts.Title != "" || opts.DefaultPath != "" || len(opts.Filters) != 0 || opts.Multiple {
				t.Fatalf("non-object input %#v produced options %+v", input, opts)
			}
			return
		}
		// Only a JSON true opts into a multi-file selection.
		if want := m["multiple"] == true; opts.Multiple != want {
			t.Fatalf("input %#v: Multiple = %v, want %v", input, opts.Multiple, want)
		}
	})
}

func FuzzParseShortcutRegister(f *testing.F) {
	fuzzJSON(f, []string{
		`{"accelerator":"CmdOrCtrl+K","action":"palette"}`,
		`{"accelerator":"Ctrl+A","actionID":"a"}`,
		`{"accelerator":"Ctrl+A","id":"a"}`,
		`{"accelerator":"Ctrl+A","action":1,"id":"b"}`,
		`{"accelerator":"Ctrl+A"}`, `{"action":"x"}`,
	}, func(t *testing.T, input any) {
		accel, action, err := desktop.ParseShortcutRegister(input)
		if err != nil {
			return
		}
		if accel == "" || action == "" {
			t.Fatalf("accepted empty accelerator %q or action %q", accel, action)
		}
	})
}

func FuzzParseShortcutUnregister(f *testing.F) {
	fuzzJSON(f, []string{
		`"Ctrl+B"`, `{"accelerator":"Ctrl+Shift+Q"}`, `{"accelerator":""}`, `{"accelerator":1}`,
	}, func(t *testing.T, input any) {
		accel, err := desktop.ParseShortcutUnregister(input)
		if err == nil && accel == "" {
			t.Fatal("accepted an empty accelerator")
		}
	})
}

func FuzzParseDragDropEnable(f *testing.F) {
	fuzzJSON(f, []string{
		`true`, `false`, `{"window":"aux","enabled":false}`, `{"id":"aux"}`,
		`{"id":"","window":""}`, `{"enabled":"false"}`, `{"enabled":0}`,
	}, func(t *testing.T, input any) {
		win, _, err := desktop.ParseDragDropEnable(input)
		if err == nil && win == "" {
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

func FuzzParseWindowCreateOptions(f *testing.F) {
	fuzzJSON(f, append([]string{
		`"aux"`, `{"id":"aux","title":"Aux","path":"/aux.html","width":800,"height":600}`,
		`{"title":"no id"}`,
	}, sizeSeeds...), func(t *testing.T, input any) {
		opts, err := desktop.ParseWindowCreateOptions(input)
		if err != nil {
			return
		}
		if opts.ID == "" {
			t.Fatal("accepted an empty window id")
		}
		if opts.Width != 0 {
			requireExactSize(t, input, "width", opts.Width)
		}
		if opts.Height != 0 {
			requireExactSize(t, input, "height", opts.Height)
		}
	})
}

func FuzzParseWindowID(f *testing.F) {
	fuzzJSON(f, []string{`"aux"`, `{"id":"aux"}`, `{"id":""}`, `{"window":"aux"}`},
		func(t *testing.T, input any) {
			id, err := desktop.ParseWindowID(input)
			if err == nil && id == "" {
				t.Fatal("accepted an empty window id")
			}
		})
}

func FuzzParseWindowAlwaysOnTop(f *testing.F) {
	fuzzJSON(f, []string{
		`{"id":"main","alwaysOnTop":true}`, `{"id":"main"}`, `{"id":"main","alwaysOnTop":"true"}`,
	}, func(t *testing.T, input any) {
		id, onTop, err := desktop.ParseWindowAlwaysOnTop(input)
		if err != nil {
			return
		}
		if id == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["alwaysOnTop"]; sent != onTop {
			t.Fatalf("page sent alwaysOnTop=%#v, parser produced %v", sent, onTop)
		}
	})
}

func FuzzParseWindowSetTitle(f *testing.F) {
	fuzzJSON(f, []string{
		`{"id":"main","title":"Hi"}`, `{"id":"main","title":""}`, `{"id":"main"}`, `{"id":"main","title":null}`,
	}, func(t *testing.T, input any) {
		id, title, err := desktop.ParseWindowSetTitle(input)
		if err != nil {
			return
		}
		if id == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["title"]; sent != title {
			t.Fatalf("page sent title=%#v, parser produced %q", sent, title)
		}
	})
}

func FuzzParseWindowSetSize(f *testing.F) {
	fuzzJSON(f, sizeSeeds, func(t *testing.T, input any) {
		id, w, h, err := desktop.ParseWindowSetSize(input)
		if err != nil {
			return
		}
		if id == "" {
			t.Fatal("accepted an empty window id")
		}
		requireExactSize(t, input, "width", w)
		requireExactSize(t, input, "height", h)
	})
}

func FuzzParseWindowSetIcon(f *testing.F) {
	fuzzJSON(f, []string{
		`{"id":"main","iconPath":"/icons/app.png"}`, `{"id":"main","iconPath":""}`, `{"id":"main"}`,
	}, func(t *testing.T, input any) {
		id, iconPath, err := desktop.ParseWindowSetIcon(input)
		if err != nil {
			return
		}
		if id == "" {
			t.Fatal("accepted an empty window id")
		}
		if sent := input.(map[string]any)["iconPath"]; sent != iconPath {
			t.Fatalf("page sent iconPath=%#v, parser produced %q", sent, iconPath)
		}
	})
}

func FuzzParseWindowChromeApply(f *testing.F) {
	fuzzJSON(f, append([]string{
		`{"id":"main","title":"T","width":800,"height":600,"maximized":true,"fullscreen":false,` +
			`"alwaysOnTop":true,"minimized":false,"hidden":false,"iconPath":"/i.png"}`,
		`{"id":"main","maximized":"yes","hidden":1}`,
	}, sizeSeeds...), func(t *testing.T, input any) {
		id, chrome, err := desktop.ParseWindowChromeApply(input)
		if err != nil {
			return
		}
		if id == "" {
			t.Fatal("accepted an empty window id")
		}
		if chrome.Width != 0 {
			requireExactSize(t, input, "width", chrome.Width)
		}
		if chrome.Height != 0 {
			requireExactSize(t, input, "height", chrome.Height)
		}
	})
}
