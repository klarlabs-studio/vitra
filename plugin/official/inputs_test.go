package official_test

import (
	"errors"
	"reflect"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/strictjson"
	"go.klarlabs.de/vitra/plugin/official"
)

func decode[T any](t *testing.T, input any) T {
	t.Helper()
	got, err := strictjson.Decode[T](input)
	if err != nil {
		t.Fatalf("decode %T from %#v: %v", got, input, err)
	}
	return got
}

func rejects[T any](t *testing.T, input any) {
	t.Helper()
	if got, err := strictjson.Decode[T](input); err == nil {
		t.Errorf("decode %T accepted %#v as %+v", got, input, got)
	}
}

// Every shape the official commands accepted before they were typed still
// decodes to the same values.
func TestInputs_AcceptEveryDocumentedShape(t *testing.T) {
	obj := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	item := func(id, label string) map[string]any { return obj("id", id, "label", label) }
	for _, tc := range []struct {
		name string
		got  func() any
		want any
	}{
		{"window id string", func() any { return decode[official.WindowRef](t, "aux") }, official.WindowRef{ID: "aux"}},
		{"window id object", func() any { return decode[official.WindowRef](t, obj("id", "aux")) }, official.WindowRef{ID: "aux"}},
		{"create string", func() any { return decode[official.WindowCreateInput](t, "aux") }, official.WindowCreateInput{ID: "aux"}},
		{"create object", func() any {
			return decode[official.WindowCreateInput](t, obj("id", "aux", "title", "Aux", "path", "/a.html", "width", 800.0, "height", 600.0))
		}, official.WindowCreateInput{ID: "aux", Title: "Aux", Path: "/a.html", Width: 800, Height: 600}},
		{"chrome", func() any {
			return decode[official.WindowChromeInput](t, obj("id", "main", "title", "T", "width", 900.0, "maximized", true, "iconPath", "/i.png"))
		}, official.WindowChromeInput{ID: "main", Title: "T", Width: 900, Maximized: true, IconPath: "/i.png"}},
		{"always on top", func() any {
			return decode[official.WindowAlwaysOnTopInput](t, obj("id", "main", "alwaysOnTop", false))
		}, official.WindowAlwaysOnTopInput{ID: "main", AlwaysOnTop: false}},
		{"empty title", func() any { return decode[official.WindowTitleInput](t, obj("id", "main", "title", "")) },
			official.WindowTitleInput{ID: "main", Title: ""}},
		{"size", func() any {
			return decode[official.WindowSizeInput](t, obj("id", "main", "width", 640.0, "height", 480.0))
		},
			official.WindowSizeInput{ID: "main", Width: 640, Height: 480}},
		{"icon", func() any { return decode[official.WindowIconInput](t, obj("id", "main", "iconPath", "")) },
			official.WindowIconInput{ID: "main"}},
		{"message string", func() any { return decode[official.MessageDialogInput](t, "Saved") },
			official.MessageDialogInput{Message: "Saved"}},
		{"message object", func() any {
			return decode[official.MessageDialogInput](t, obj("title", "T", "message", "M", "kind", "confirm"))
		}, official.MessageDialogInput{Title: "T", Message: "M", Kind: "confirm"}},
		{"notification string", func() any { return decode[official.NotificationInput](t, "Done") },
			official.NotificationInput{Body: "Done"}},
		{"notification object", func() any { return decode[official.NotificationInput](t, obj("title", "T", "body", "B")) },
			official.NotificationInput{Title: "T", Body: "B"}},
		{"dialog options", func() any {
			return decode[official.DialogOptions](t, obj("title", "T", "defaultPath", "/home", "multiple", true,
				"filters", []any{obj("name", "Docs", "extensions", []any{"pdf", "txt"})}))
		}, official.DialogOptions{Title: "T", DefaultPath: "/home", Multiple: true,
			Filters: []official.FileFilter{{Name: "Docs", Extensions: []string{"pdf", "txt"}}}}},
		{"dialog nil", func() any { return decode[official.DialogOptions](t, nil) }, official.DialogOptions{}},
		{"menu array", func() any {
			return decode[official.MenuInput](t, []any{obj("id", "app.quit", "label", "Quit", "menu", "File", "shortcut", "Ctrl+Q")})
		}, official.MenuInput{Items: []official.MenuItem{{ID: "app.quit", Label: "Quit", Menu: "File", Shortcut: "Ctrl+Q"}}}},
		{"menu object", func() any { return decode[official.MenuInput](t, obj("items", []any{item("x", "X")})) },
			official.MenuInput{Items: []official.MenuItem{{ID: "x", Label: "X"}}}},
		{"menu nil", func() any { return decode[official.MenuInput](t, nil) }, official.MenuInput{}},
		{"tray object", func() any {
			return decode[official.TrayInput](t, obj("tooltip", "Demo", "items", []any{item("tray.quit", "Quit")}))
		}, official.TrayInput{Tooltip: "Demo", Items: []official.MenuItem{{ID: "tray.quit", Label: "Quit"}}}},
		{"tray array", func() any { return decode[official.TrayInput](t, []any{item("a", "A")}) },
			official.TrayInput{Items: []official.MenuItem{{ID: "a", Label: "A"}}}},
		{"tray tooltip only", func() any { return decode[official.TrayInput](t, obj("tooltip", "only")) },
			official.TrayInput{Tooltip: "only"}},
		{"drag drop bool", func() any { return decode[official.DragDropInput](t, false) },
			official.DragDropInput{ID: "main", Enabled: false}},
		{"drag drop nil", func() any { return decode[official.DragDropInput](t, nil) },
			official.DragDropInput{ID: "main", Enabled: true}},
		{"drag drop window alias", func() any { return decode[official.DragDropInput](t, obj("window", "aux", "enabled", false)) },
			official.DragDropInput{ID: "aux", Enabled: false}},
		{"drag drop default enabled", func() any { return decode[official.DragDropInput](t, obj("id", "aux")) },
			official.DragDropInput{ID: "aux", Enabled: true}},
		{"shortcut action", func() any {
			return decode[official.ShortcutInput](t, obj("accelerator", "CmdOrCtrl+K", "action", "palette"))
		}, official.ShortcutInput{Accelerator: "CmdOrCtrl+K", Action: "palette"}},
		{"shortcut actionID alias", func() any { return decode[official.ShortcutInput](t, obj("accelerator", "Ctrl+A", "actionID", "a")) },
			official.ShortcutInput{Accelerator: "Ctrl+A", Action: "a"}},
		{"shortcut id alias", func() any { return decode[official.ShortcutInput](t, obj("accelerator", "Ctrl+A", "id", "a")) },
			official.ShortcutInput{Accelerator: "Ctrl+A", Action: "a"}},
		{"unregister string", func() any { return decode[official.ShortcutRef](t, "Ctrl+B") },
			official.ShortcutRef{Accelerator: "Ctrl+B"}},
		{"unregister object", func() any { return decode[official.ShortcutRef](t, obj("accelerator", "Ctrl+B")) },
			official.ShortcutRef{Accelerator: "Ctrl+B"}},
		{"write file", func() any { return decode[official.WriteFileInput](t, obj("path", "/tmp/a", "data", "x")) },
			official.WriteFileInput{Path: "/tmp/a", Data: "x"}},
	} {
		if got := tc.got(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// Inputs are decoded strictly: unknown fields, wrong types, and missing
// required values are rejected, never ignored.
func TestInputs_RejectUnknownFieldsWrongTypesAndMissingValues(t *testing.T) {
	rejects[official.WriteFileInput](t, map[string]any{"path": "/tmp/a", "data": "x", "mode": 0o777})
	rejects[official.WriteFileInput](t, map[string]any{"path": 1})
	rejects[official.DialogOptions](t, map[string]any{"title": "x", "bogus": true})
	rejects[official.DialogOptions](t, "x")
	rejects[official.DialogOptions](t, map[string]any{"filters": []any{map[string]any{"name": "T", "extensions": "txt"}}})
	rejects[official.DialogOptions](t, map[string]any{"multiple": "true"})
	rejects[official.MessageDialogInput](t, map[string]any{"message": "m", "buttons": []any{}})
	rejects[official.MessageDialogInput](t, 1.0)
	rejects[official.NotificationInput](t, map[string]any{"body": "b", "icon": "x"})
	rejects[official.WindowRef](t, nil)
	rejects[official.WindowRef](t, "")
	rejects[official.WindowRef](t, map[string]any{"window": "aux"})
	rejects[official.WindowRef](t, map[string]any{"id": "aux", "extra": 1})
	rejects[official.WindowCreateInput](t, nil)
	rejects[official.WindowCreateInput](t, map[string]any{"title": "no id"})
	rejects[official.WindowCreateInput](t, map[string]any{"id": "aux", "url": "https://example.com"})
	rejects[official.WindowChromeInput](t, map[string]any{"id": "main", "maximized": "yes"})
	rejects[official.WindowChromeInput](t, map[string]any{"title": "no id"})
	rejects[official.WindowAlwaysOnTopInput](t, map[string]any{"id": "main"})
	rejects[official.WindowAlwaysOnTopInput](t, map[string]any{"id": "main", "alwaysOnTop": "true"})
	rejects[official.WindowTitleInput](t, map[string]any{"id": "main"})
	rejects[official.WindowTitleInput](t, map[string]any{"id": "main", "title": nil})
	rejects[official.WindowSizeInput](t, map[string]any{"id": "main", "width": 640.0})
	rejects[official.WindowSizeInput](t, map[string]any{"id": "main", "width": 0.0, "height": 480.0})
	rejects[official.WindowIconInput](t, map[string]any{"id": "main"})
	rejects[official.MenuInput](t, map[string]any{"id": "x"})
	rejects[official.MenuInput](t, []any{map[string]any{"id": "a"}})
	rejects[official.MenuInput](t, []any{map[string]any{"id": "a", "label": "A", "children": []any{}}})
	rejects[official.MenuInput](t, "x")
	rejects[official.TrayInput](t, map[string]any{"tooltip": 1.0})
	rejects[official.TrayInput](t, true)
	rejects[official.DragDropInput](t, map[string]any{"enabled": "false"})
	rejects[official.DragDropInput](t, "main")
	rejects[official.ShortcutInput](t, map[string]any{"accelerator": "Ctrl+A"})
	rejects[official.ShortcutInput](t, map[string]any{"action": "x"})
	rejects[official.ShortcutInput](t, "Ctrl+A")
	rejects[official.ShortcutRef](t, map[string]any{})
	rejects[official.ShortcutRef](t, "")
}

// Validation failures are domain validation errors, as before.
func TestInputs_FailWithValidationErrors(t *testing.T) {
	_, err := strictjson.Decode[official.WindowSizeInput](map[string]any{"id": "main", "width": 640.0})
	if !errors.Is(err, &domain.ErrValidation{}) {
		t.Fatalf("missing height: %v, want a validation error", err)
	}
}

// A size must be exactly the integer the page sent. Numbers above 2^53 are
// not exact once decoded from JSON (2e16+1 arrives as 2e16), and numbers
// beyond the int range would be saturated or wrapped by the float-to-int
// conversion (which differs by CPU), so both are rejected.
func TestWindowSizes_RejectInexactOrOutOfRangeNumbers(t *testing.T) {
	in := decode[official.WindowSizeInput](t, map[string]any{"id": "main", "width": float64(1 << 53), "height": 1.0})
	if in.Width != 1<<53 {
		t.Fatalf("2^53 must stay accepted: w=%d", in.Width)
	}
	for _, n := range []float64{1<<53 + 2, 2.000000000000001e16, 1 << 63, 1 << 64, 1e19, 1e308, 640.5, -1} {
		in := map[string]any{"id": "main", "width": n, "height": n}
		rejects[official.WindowSizeInput](t, in)
		rejects[official.WindowCreateInput](t, in)
		rejects[official.WindowChromeInput](t, in)
	}
}

// tray.set carries the menu bar status: title, an icon from the app's assets,
// and menu items with separators, disabled and checked states.
func TestInputs_TrayStatus(t *testing.T) {
	got := decode[official.TrayInput](t, map[string]any{
		"tooltip": "Usage", "title": "42%", "icon": "icons/tray.png", "template": true,
		"items": []any{
			map[string]any{"id": "refresh", "label": "Refresh", "disabled": true},
			map[string]any{"separator": true},
			map[string]any{"id": "pin", "label": "Pin", "checked": true},
		},
	})
	want := official.TrayInput{
		Tooltip: "Usage", Title: "42%", Icon: "icons/tray.png", Template: true,
		Items: []official.MenuItem{
			{ID: "refresh", Label: "Refresh", Disabled: true},
			{Separator: true},
			{ID: "pin", Label: "Pin", Checked: true},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	menu := decode[official.MenuInput](t, []any{
		map[string]any{"id": "a", "label": "A", "menu": "File"},
		map[string]any{"separator": true, "menu": "File"},
	})
	if !menu.Items[1].Separator || menu.Items[1].Menu != "File" {
		t.Fatalf("menu separator: %+v", menu.Items)
	}
}

// The tray icon names a file inside the app's assets; anything that could
// reach outside them is rejected before the host sees it.
func TestInputs_TrayIconMustStayInAssets(t *testing.T) {
	for _, icon := range []string{
		"../secret.png", "//etc/passwd", "icons/../../x.png", "icons/./tray.png",
		`icons\tray.png`, "icons//tray.png", "icons/", "C:/x.png", "./tray.png",
	} {
		rejects[official.TrayInput](t, map[string]any{"icon": icon})
	}
	// A leading slash is the URL path the page already uses for assets.
	if got := decode[official.TrayInput](t, map[string]any{"icon": "/icons/tray.png"}); got.Icon != "icons/tray.png" {
		t.Fatalf("icon %q, want icons/tray.png", got.Icon)
	}
}

func TestInputs_MenuSeparatorsCarryNothingElse(t *testing.T) {
	for _, it := range []map[string]any{
		{"separator": true, "id": "x"},
		{"separator": true, "label": "X"},
		{"separator": true, "shortcut": "Ctrl+X"},
		{"separator": true, "checked": true},
		{"separator": true, "disabled": true},
		{"id": "x"},
		{"label": "X"},
	} {
		rejects[official.TrayInput](t, []any{it})
		rejects[official.MenuInput](t, []any{it})
	}
}
