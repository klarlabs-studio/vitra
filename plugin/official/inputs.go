package official

import (
	"bytes"
	"io/fs"
	"strings"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/strictjson"
)

// The types below are the inputs and outputs of the official commands, as
// app.UseOfficialPlugins binds them with vitra.Bind. Input is decoded
// strictly: an unknown field, a wrong type, or a missing required value is a
// *domain.ErrValidation, and the host is never called.
//
// The generated TypeScript client uses the object form of each input. The
// page may still send the older shorthands (a bare window id, a bare menu
// array, the actionID alias, ...) through window.vitra.invoke; each type
// documents the ones it accepts.

// maxExactJSONInt is 2^53, the largest integer below which every JSON number
// decodes to exactly the integer the page wrote (JavaScript's
// Number.MAX_SAFE_INTEGER + 1). The bridge decodes numbers as float64, so a
// larger size may already be rounded and is rejected.
const maxExactJSONInt int64 = 1 << 53

// WriteFileInput is the input of fs.write: the file to write and its text.
type WriteFileInput struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

// FileFilter is a named set of file extensions for an open or save dialog.
type FileFilter struct {
	Name       string   `json:"name,omitempty"`
	Extensions []string `json:"extensions,omitempty"`
}

// DialogOptions is the input of dialog.open, dialog.save, and
// dialog.openDirectory. All fields are optional, and null means none.
// Multiple asks dialog.open for several files.
type DialogOptions struct {
	Title       string       `json:"title,omitempty"`
	DefaultPath string       `json:"defaultPath,omitempty"`
	Filters     []FileFilter `json:"filters,omitempty"`
	Multiple    bool         `json:"multiple,omitempty"`
}

// MessageDialogInput is the input of dialog.message. Kind is "info" (the
// default) or "confirm". A bare string is the message.
type MessageDialogInput struct {
	Title   string `json:"title,omitempty"`
	Message string `json:"message"`
	Kind    string `json:"kind,omitempty"`
}

// UnmarshalJSON accepts {title?, message, kind?} or a bare message string.
func (in *MessageDialogInput) UnmarshalJSON(data []byte) error {
	type plain MessageDialogInput
	if jsonKind(data) == '"' {
		*in = MessageDialogInput{}
		return strictjson.Unmarshal(data, &in.Message)
	}
	return decodeObject(data, "dialog.message", (*plain)(in))
}

// NotificationInput is the input of notifications.show. A bare string is
// the body.
type NotificationInput struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body"`
}

// UnmarshalJSON accepts {title?, body} or a bare body string.
func (in *NotificationInput) UnmarshalJSON(data []byte) error {
	type plain NotificationInput
	if jsonKind(data) == '"' {
		*in = NotificationInput{}
		return strictjson.Unmarshal(data, &in.Body)
	}
	return decodeObject(data, "notifications.show", (*plain)(in))
}

// WindowRef names the window a window command acts on (window.close,
// window.focus, window.getChrome, ...). A bare string is the id.
type WindowRef struct {
	ID domain.WindowID `json:"id"`
}

// UnmarshalJSON accepts {id} or a bare id string. The id is required.
func (in *WindowRef) UnmarshalJSON(data []byte) error {
	type plain WindowRef
	*in = WindowRef{}
	if jsonKind(data) == '"' {
		if err := strictjson.Unmarshal(data, &in.ID); err != nil {
			return err
		}
	} else if err := decodeObject(data, "window", (*plain)(in)); err != nil {
		return err
	}
	return requireWindowID(in.ID)
}

// WindowCreateInput is the input of window.create. Width and Height are
// optional; when given they must be positive integers. A bare string is
// the id.
type WindowCreateInput struct {
	ID     domain.WindowID `json:"id"`
	Title  string          `json:"title,omitempty"`
	Path   string          `json:"path,omitempty"`
	Width  int             `json:"width,omitempty"`
	Height int             `json:"height,omitempty"`
}

// UnmarshalJSON accepts {id, title?, path?, width?, height?} or a bare id
// string.
func (in *WindowCreateInput) UnmarshalJSON(data []byte) error {
	type plain WindowCreateInput
	*in = WindowCreateInput{}
	if jsonKind(data) == '"' {
		if err := strictjson.Unmarshal(data, &in.ID); err != nil {
			return err
		}
	} else if err := decodeObject(data, "window.create", (*plain)(in)); err != nil {
		return err
	}
	if err := requireWindowID(in.ID); err != nil {
		return err
	}
	return optionalSizes(in.Width, in.Height)
}

// WindowCreated is the output of window.create.
type WindowCreated struct {
	ID domain.WindowID `json:"id"`
}

// WindowChromeInput is the input of window.chrome: the presentation to
// apply to a window. Every field but ID is optional; a zero width or height
// leaves the size unchanged.
type WindowChromeInput struct {
	ID          domain.WindowID `json:"id"`
	Title       string          `json:"title,omitempty"`
	Width       int             `json:"width,omitempty"`
	Height      int             `json:"height,omitempty"`
	Maximized   bool            `json:"maximized,omitempty"`
	Fullscreen  bool            `json:"fullscreen,omitempty"`
	AlwaysOnTop bool            `json:"alwaysOnTop,omitempty"`
	Minimized   bool            `json:"minimized,omitempty"`
	Hidden      bool            `json:"hidden,omitempty"`
	IconPath    string          `json:"iconPath,omitempty"`
}

// UnmarshalJSON accepts the object form only.
func (in *WindowChromeInput) UnmarshalJSON(data []byte) error {
	type plain WindowChromeInput
	*in = WindowChromeInput{}
	if err := decodeObject(data, "window.chrome", (*plain)(in)); err != nil {
		return err
	}
	if err := requireWindowID(in.ID); err != nil {
		return err
	}
	return optionalSizes(in.Width, in.Height)
}

// WindowAlwaysOnTopInput is the input of window.setAlwaysOnTop. Both fields
// are required.
type WindowAlwaysOnTopInput struct {
	ID          domain.WindowID `json:"id"`
	AlwaysOnTop bool            `json:"alwaysOnTop"`
}

// UnmarshalJSON requires {id, alwaysOnTop}.
func (in *WindowAlwaysOnTopInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          domain.WindowID `json:"id"`
		AlwaysOnTop *bool           `json:"alwaysOnTop"`
	}
	if err := decodeObject(data, "window.setAlwaysOnTop", &raw); err != nil {
		return err
	}
	if err := requireWindowID(raw.ID); err != nil {
		return err
	}
	if raw.AlwaysOnTop == nil {
		return &domain.ErrValidation{Message: "alwaysOnTop bool is required"}
	}
	*in = WindowAlwaysOnTopInput{ID: raw.ID, AlwaysOnTop: *raw.AlwaysOnTop}
	return nil
}

// WindowTitleInput is the input of window.setTitle. The title is required
// and may be empty.
type WindowTitleInput struct {
	ID    domain.WindowID `json:"id"`
	Title string          `json:"title"`
}

// UnmarshalJSON requires {id, title}.
func (in *WindowTitleInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID    domain.WindowID `json:"id"`
		Title *string         `json:"title"`
	}
	if err := decodeObject(data, "window.setTitle", &raw); err != nil {
		return err
	}
	if err := requireWindowID(raw.ID); err != nil {
		return err
	}
	if raw.Title == nil {
		return &domain.ErrValidation{Message: "title string is required"}
	}
	*in = WindowTitleInput{ID: raw.ID, Title: *raw.Title}
	return nil
}

// WindowSizeInput is the input of window.setSize. Width and height must be
// positive integers.
type WindowSizeInput struct {
	ID     domain.WindowID `json:"id"`
	Width  int             `json:"width"`
	Height int             `json:"height"`
}

// UnmarshalJSON requires {id, width, height}.
func (in *WindowSizeInput) UnmarshalJSON(data []byte) error {
	type plain WindowSizeInput
	*in = WindowSizeInput{}
	if err := decodeObject(data, "window.setSize", (*plain)(in)); err != nil {
		return err
	}
	if err := requireWindowID(in.ID); err != nil {
		return err
	}
	if !validSize(in.Width) || in.Width == 0 {
		return &domain.ErrValidation{Message: "width must be a positive integer"}
	}
	if !validSize(in.Height) || in.Height == 0 {
		return &domain.ErrValidation{Message: "height must be a positive integer"}
	}
	return nil
}

// WindowIconInput is the input of window.setIcon. The icon path is required;
// an empty path leaves the icon unchanged on hosts that treat it so.
type WindowIconInput struct {
	ID       domain.WindowID `json:"id"`
	IconPath string          `json:"iconPath"`
}

// UnmarshalJSON requires {id, iconPath}.
func (in *WindowIconInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID       domain.WindowID `json:"id"`
		IconPath *string         `json:"iconPath"`
	}
	if err := decodeObject(data, "window.setIcon", &raw); err != nil {
		return err
	}
	if err := requireWindowID(raw.ID); err != nil {
		return err
	}
	if raw.IconPath == nil {
		return &domain.ErrValidation{Message: "iconPath string is required"}
	}
	*in = WindowIconInput{ID: raw.ID, IconPath: *raw.IconPath}
	return nil
}

// MenuItem is one entry of an application or tray menu. ID and Label are
// required unless Separator is set, in which case only Menu may be. Menu
// names the top-level menu it goes in (for example "File"); application menu
// items without one go in an "App" menu. Disabled items are shown greyed out
// and never fire; Checked shows a checkmark.
type MenuItem struct {
	ID        string `json:"id,omitempty"`
	Label     string `json:"label,omitempty"`
	Menu      string `json:"menu,omitempty"`
	Shortcut  string `json:"shortcut,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
	Checked   bool   `json:"checked,omitempty"`
}

// MenuInput is the input of menu.set.
type MenuInput struct {
	Items []MenuItem `json:"items"`
}

// UnmarshalJSON accepts {items}, a bare items array, or null (no items).
func (in *MenuInput) UnmarshalJSON(data []byte) error {
	*in = MenuInput{}
	errShape := &domain.ErrValidation{Message: "menu items input must be an array or {items:[]}"}
	switch jsonKind(data) {
	case 'n':
		return nil
	case '[':
		if err := strictjson.Unmarshal(data, &in.Items); err != nil {
			return err
		}
	case '{':
		var raw struct {
			Items *[]MenuItem `json:"items"`
		}
		if err := strictjson.Unmarshal(data, &raw); err != nil {
			return err
		}
		if raw.Items == nil {
			return errShape
		}
		in.Items = *raw.Items
	default:
		return errShape
	}
	return validateMenuItems(in.Items)
}

// TrayInput is the input of tray.set: an optional tooltip, the status shown
// in the menu bar or tray, and the tray menu. Title is text shown next to
// the icon where the platform supports it. Icon is the path of a PNG inside
// the app's assets (a leading "/" is allowed, as in page URLs); it can never
// name a file outside them. Template marks the icon as a macOS template
// image, which the system tints to match the menu bar. ClickActivates
// makes a left click on the icon emit tray.click; the menu then opens on a
// right click. Panel names a panel window the left click shows under the
// icon and hides again (it implies ClickActivates).
type TrayInput struct {
	Tooltip        string          `json:"tooltip,omitempty"`
	Title          string          `json:"title,omitempty"`
	Icon           string          `json:"icon,omitempty"`
	Template       bool            `json:"template,omitempty"`
	ClickActivates bool            `json:"clickActivates,omitempty"`
	Panel          domain.WindowID `json:"panel,omitempty"`
	Items          []MenuItem      `json:"items,omitempty"`
}

// UnmarshalJSON accepts {tooltip?, items?}, a bare items array, or null.
func (in *TrayInput) UnmarshalJSON(data []byte) error {
	type plain TrayInput
	*in = TrayInput{}
	switch jsonKind(data) {
	case 'n':
		return nil
	case '[':
		if err := strictjson.Unmarshal(data, &in.Items); err != nil {
			return err
		}
	case '{':
		if err := strictjson.Unmarshal(data, (*plain)(in)); err != nil {
			return err
		}
	default:
		return &domain.ErrValidation{Message: "tray.set input must be an array or {tooltip?, title?, icon?, template?, clickActivates?, panel?, items?}"}
	}
	if in.Icon != "" {
		icon, err := assetPath(in.Icon)
		if err != nil {
			return err
		}
		in.Icon = icon
	}
	return validateMenuItems(in.Items)
}

// assetPath validates p as a file path inside the app's assets and returns
// it in io/fs form. One leading "/" is allowed; "..", ".", empty elements,
// backslashes and drive letters are not.
func assetPath(p string) (string, error) {
	name := strings.TrimPrefix(p, "/")
	if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, `\:`) {
		return "", &domain.ErrValidation{Message: "icon must be a file path inside the app's assets"}
	}
	return name, nil
}

// LoginItem is the input of app.setLoginItem and the output of
// app.loginItem: whether the app starts with the user's session.
type LoginItem struct {
	Enabled bool `json:"enabled"`
}

// UnmarshalJSON accepts {enabled}; enabled is required.
func (in *LoginItem) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeObject(data, "app.setLoginItem", &raw); err != nil {
		return err
	}
	if raw.Enabled == nil {
		return &domain.ErrValidation{Message: "app.setLoginItem requires enabled"}
	}
	in.Enabled = *raw.Enabled
	return nil
}

// DragDropInput is the input of dragdrop.receive. ID defaults to "main" and
// Enabled to true.
type DragDropInput struct {
	ID      domain.WindowID `json:"id,omitempty"`
	Enabled bool            `json:"enabled,omitempty"`
}

// UnmarshalJSON accepts {id?, enabled?}, a bare bool (enabled, for the
// "main" window), or null. "window" is accepted as an alias of "id".
func (in *DragDropInput) UnmarshalJSON(data []byte) error {
	*in = DragDropInput{ID: "main", Enabled: true}
	switch jsonKind(data) {
	case 'n':
		return nil
	case 't', 'f':
		return strictjson.Unmarshal(data, &in.Enabled)
	case '{':
	default:
		return &domain.ErrValidation{Message: "dragdrop.receive input must be a bool or object"}
	}
	var raw struct {
		ID      domain.WindowID `json:"id"`
		Window  domain.WindowID `json:"window"`
		Enabled *bool           `json:"enabled"`
	}
	if err := strictjson.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.ID != "" {
		in.ID = raw.ID
	} else if raw.Window != "" {
		in.ID = raw.Window
	}
	if raw.Enabled != nil {
		in.Enabled = *raw.Enabled
	}
	return nil
}

// ShortcutInput is the input of shortcut.register: the accelerator to
// register and the action id its activation emits as shortcut.action.
type ShortcutInput struct {
	Accelerator string `json:"accelerator"`
	Action      string `json:"action"`
}

// UnmarshalJSON requires {accelerator, action}; "actionID" and "id" are
// accepted as aliases of "action".
func (in *ShortcutInput) UnmarshalJSON(data []byte) error {
	var raw struct {
		Accelerator string `json:"accelerator"`
		Action      string `json:"action"`
		ActionID    string `json:"actionID"`
		ID          string `json:"id"`
	}
	if err := decodeObject(data, "shortcut.register", &raw); err != nil {
		return err
	}
	action := raw.Action
	if action == "" {
		action = raw.ActionID
	}
	if action == "" {
		action = raw.ID
	}
	if raw.Accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	if action == "" {
		return &domain.ErrValidation{Message: "action id is required"}
	}
	*in = ShortcutInput{Accelerator: raw.Accelerator, Action: action}
	return nil
}

// ShortcutRef is the input of shortcut.unregister. A bare string is the
// accelerator.
type ShortcutRef struct {
	Accelerator string `json:"accelerator"`
}

// UnmarshalJSON accepts {accelerator} or a bare accelerator string.
func (in *ShortcutRef) UnmarshalJSON(data []byte) error {
	type plain ShortcutRef
	*in = ShortcutRef{}
	switch jsonKind(data) {
	case '"':
		if err := strictjson.Unmarshal(data, &in.Accelerator); err != nil {
			return err
		}
	case '{':
		if err := strictjson.Unmarshal(data, (*plain)(in)); err != nil {
			return err
		}
	default:
		return &domain.ErrValidation{Message: "shortcut.unregister input must be a string or {accelerator}"}
	}
	if in.Accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	return nil
}

// jsonKind returns the first byte of the JSON value in data: '{', '[', '"',
// 't', 'f', 'n', or a digit or '-' for a number.
func jsonKind(data []byte) byte {
	data = bytes.TrimLeft(data, " \t\r\n")
	if len(data) == 0 {
		return 0
	}
	return data[0]
}

// decodeObject strictly decodes data, which must be a JSON object, into v.
func decodeObject(data []byte, command string, v any) error {
	if jsonKind(data) != '{' {
		return &domain.ErrValidation{Message: command + " input must be an object"}
	}
	return strictjson.Unmarshal(data, v)
}

func requireWindowID(id domain.WindowID) error {
	if id == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	return nil
}

// validSize reports whether n is zero or a positive integer the page sent
// exactly.
func validSize(n int) bool {
	return n >= 0 && int64(n) <= maxExactJSONInt
}

// optionalSizes checks sizes where zero means "unchanged".
func optionalSizes(width, height int) error {
	if !validSize(width) {
		return &domain.ErrValidation{Message: "width must be a positive integer"}
	}
	if !validSize(height) {
		return &domain.ErrValidation{Message: "height must be a positive integer"}
	}
	return nil
}

func validateMenuItems(items []MenuItem) error {
	for _, it := range items {
		if it.Separator {
			if it.ID != "" || it.Label != "" || it.Shortcut != "" || it.Disabled || it.Checked {
				return &domain.ErrValidation{Message: "a menu separator takes no id, label, shortcut, disabled or checked"}
			}
			continue
		}
		if it.ID == "" || it.Label == "" {
			return &domain.ErrValidation{Message: "menu item requires id and label"}
		}
	}
	return nil
}
