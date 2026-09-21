// Package desktop provides Phase 2 capability-gated desktop services.
//
// Menus, tray, dialogs, clipboard, shortcuts, deep links, and single-instance
// behaviour are expressed as domain services that require explicit permissions
// and platform feature support — never ambient authority.
package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Permission names for Phase 2 desktop surfaces.
const (
	PermMenuSet             domain.PermissionName = "menu.set"
	PermTraySet             domain.PermissionName = "tray.set"
	PermDialogOpen          domain.PermissionName = "dialog.open"
	PermDialogSave          domain.PermissionName = "dialog.save"
	PermDialogMessage       domain.PermissionName = "dialog.message"
	PermDialogOpenDirectory domain.PermissionName = "dialog.openDirectory"
	PermClipboardRead       domain.PermissionName = "clipboard.read"
	PermClipboardWrite      domain.PermissionName = "clipboard.write"
	PermShortcutRegister    domain.PermissionName = "shortcut.register"
	PermDeepLinkHandle      domain.PermissionName = "deeplink.handle"
	PermSingleInstance      domain.PermissionName = "app.single_instance"
	PermDragDrop            domain.PermissionName = "dragdrop.receive"
	PermWindowChrome        domain.PermissionName = "window.chrome"
	PermWindowCreate        domain.PermissionName = "window.create"
	PermWindowClose         domain.PermissionName = "window.close"
	PermOpenURL             domain.PermissionName = "browser.open"
	PermOsInfo              domain.PermissionName = "os.info"
	PermNotificationShow    domain.PermissionName = "notifications.show"
	PermPathOpen            domain.PermissionName = "path.open"
	PermAppQuit             domain.PermissionName = "app.quit"
)

// Gateway evaluates desktop permissions for a caller.
type Gateway interface {
	Authorize(caller domain.Caller, permission domain.PermissionName, resourcePath string) domain.Decision
}

// MenuItem is a portable menu entry.
type MenuItem struct {
	ID       string
	Label    string
	Shortcut string
	Menu     string // optional top-level native menu label (e.g. "File")
	Children []MenuItem
}

// MenuService applies application menus when permitted and supported.
type MenuService struct {
	Gateway Gateway
	Host    platform.Host
	OnSet   func(ctx context.Context, items []MenuItem) error // optional native hook
	OnClear func(ctx context.Context) error
}

// ParseMenuItems extracts menu entries from an invoke payload.
// Accepts a bare array or { items: [...] }. Each entry needs id + label;
// optional menu (top-level label) and shortcut are forwarded.
func ParseMenuItems(input any) ([]MenuItem, error) {
	var raw []any
	switch v := input.(type) {
	case nil:
		return nil, nil
	case []any:
		raw = v
	case map[string]any:
		if items, ok := v["items"].([]any); ok {
			raw = items
		} else {
			return nil, &domain.ErrValidation{Message: "menu items input must be an array or {items:[]}"}
		}
	default:
		return nil, &domain.ErrValidation{Message: "menu items input must be an array or {items:[]}"}
	}
	out := make([]MenuItem, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok || m == nil {
			return nil, &domain.ErrValidation{Message: "menu items must be objects"}
		}
		id, _ := m["id"].(string)
		label, _ := m["label"].(string)
		if id == "" || label == "" {
			return nil, &domain.ErrValidation{Message: "menu item requires id and label"}
		}
		item := MenuItem{ID: id, Label: label}
		if menu, ok := m["menu"].(string); ok {
			item.Menu = menu
		}
		if shortcut, ok := m["shortcut"].(string); ok {
			item.Shortcut = shortcut
		}
		out = append(out, item)
	}
	return out, nil
}

// SetMenu authorizes menu.set then applies items (or returns unsupported).
func (s *MenuService) SetMenu(ctx context.Context, caller domain.Caller, items []MenuItem) error {
	if err := authorize(s.Gateway, caller, PermMenuSet); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureMenuBar); err != nil {
		return err
	}
	if s.OnSet != nil {
		return s.OnSet(ctx, items)
	}
	return nil
}

// ClearMenu authorizes menu.set then clears the menu bar.
func (s *MenuService) ClearMenu(ctx context.Context, caller domain.Caller) error {
	if err := authorize(s.Gateway, caller, PermMenuSet); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureMenuBar); err != nil {
		return err
	}
	if s.OnClear != nil {
		return s.OnClear(ctx)
	}
	return nil
}

// TrayService manages tray icons/menus.
type TrayService struct {
	Gateway Gateway
	Host    platform.Host
	OnSet   func(ctx context.Context, tooltip string, items []MenuItem) error
	OnClear func(ctx context.Context) error
}

// ParseTraySet extracts tooltip + menu items from an invoke payload.
// Accepts { tooltip?, items: [...] } or a bare items array.
func ParseTraySet(input any) (string, []MenuItem, error) {
	switch v := input.(type) {
	case nil:
		return "", nil, nil
	case []any:
		items, err := ParseMenuItems(v)
		return "", items, err
	case map[string]any:
		tooltip, _ := v["tooltip"].(string)
		if raw, ok := v["items"]; ok {
			items, err := ParseMenuItems(raw)
			return tooltip, items, err
		}
		return tooltip, nil, nil
	default:
		return "", nil, &domain.ErrValidation{Message: "tray.set input must be an array or {tooltip?, items:[]}"}
	}
}

// SetTray authorizes tray.set then applies tray state.
func (s *TrayService) SetTray(ctx context.Context, caller domain.Caller, tooltip string, items []MenuItem) error {
	if err := authorize(s.Gateway, caller, PermTraySet); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureTray); err != nil {
		return err
	}
	if s.OnSet != nil {
		return s.OnSet(ctx, tooltip, items)
	}
	return nil
}

// ClearTray authorizes tray.set then clears the tray icon.
func (s *TrayService) ClearTray(ctx context.Context, caller domain.Caller) error {
	if err := authorize(s.Gateway, caller, PermTraySet); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureTray); err != nil {
		return err
	}
	if s.OnClear != nil {
		return s.OnClear(ctx)
	}
	return nil
}

// DialogService opens native file, directory, and message dialogs.
type DialogService struct {
	Gateway         Gateway
	Host            platform.Host
	OnOpen          func(ctx context.Context, opts platform.DialogFileOptions) ([]string, error)
	OnSave          func(ctx context.Context, opts platform.DialogFileOptions) (string, error)
	OnOpenDirectory func(ctx context.Context, opts platform.DialogFileOptions) (string, error)
	OnMessage       func(ctx context.Context, title, message, kind string) (bool, error)
}

// ParseDialogFileOptions extracts title/defaultPath/filters from an invoke payload.
// nil or unrecognized input yields zero options (legacy unfiltered dialogs).
func ParseDialogFileOptions(input any) platform.DialogFileOptions {
	var opts platform.DialogFileOptions
	m, ok := input.(map[string]any)
	if !ok || m == nil {
		return opts
	}
	if t, ok := m["title"].(string); ok {
		opts.Title = t
	}
	if d, ok := m["defaultPath"].(string); ok {
		opts.DefaultPath = d
	}
	rawFilters, ok := m["filters"]
	if !ok {
		return opts
	}
	list, ok := rawFilters.([]any)
	if !ok {
		return opts
	}
	for _, item := range list {
		fm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		f := platform.FileFilter{}
		if n, ok := fm["name"].(string); ok {
			f.Name = n
		}
		switch exts := fm["extensions"].(type) {
		case []any:
			for _, e := range exts {
				if s, ok := e.(string); ok {
					f.Extensions = append(f.Extensions, s)
				}
			}
		case []string:
			f.Extensions = append(f.Extensions, exts...)
		}
		opts.Filters = append(opts.Filters, f)
	}
	return opts
}

// OpenFile authorizes dialog.open.
func (s *DialogService) OpenFile(ctx context.Context, caller domain.Caller, opts platform.DialogFileOptions) ([]string, error) {
	if err := authorize(s.Gateway, caller, PermDialogOpen); err != nil {
		return nil, err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogOpen); err != nil {
		return nil, err
	}
	if s.OnOpen == nil {
		return nil, &platform.ErrUnsupported{Feature: platform.FeatureDialogOpen, OS: s.Host.OS(), Detail: "no dialog adapter bound"}
	}
	return s.OnOpen(ctx, opts)
}

// SaveFile authorizes dialog.save.
func (s *DialogService) SaveFile(ctx context.Context, caller domain.Caller, opts platform.DialogFileOptions) (string, error) {
	if err := authorize(s.Gateway, caller, PermDialogSave); err != nil {
		return "", err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogSave); err != nil {
		return "", err
	}
	if s.OnSave == nil {
		return "", &platform.ErrUnsupported{Feature: platform.FeatureDialogSave, OS: s.Host.OS(), Detail: "no dialog adapter bound"}
	}
	return s.OnSave(ctx, opts)
}

// OpenDirectory authorizes dialog.openDirectory.
// opts.Title and opts.DefaultPath are applied; Filters are ignored.
func (s *DialogService) OpenDirectory(ctx context.Context, caller domain.Caller, opts platform.DialogFileOptions) (string, error) {
	if err := authorize(s.Gateway, caller, PermDialogOpenDirectory); err != nil {
		return "", err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogOpenDirectory); err != nil {
		return "", err
	}
	if s.OnOpenDirectory == nil {
		return "", &platform.ErrUnsupported{Feature: platform.FeatureDialogOpenDirectory, OS: s.Host.OS(), Detail: "no directory dialog adapter bound"}
	}
	return s.OnOpenDirectory(ctx, opts)
}

// Message authorizes dialog.message. kind is "info" (OK) or "confirm" (Yes/No).
// Returns true when the user accepts (OK/Yes).
func (s *DialogService) Message(ctx context.Context, caller domain.Caller, title, message, kind string) (bool, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		kind = "info"
	}
	if kind != "info" && kind != "confirm" {
		return false, &domain.ErrValidation{Message: `kind must be "info" or "confirm"`}
	}
	if strings.TrimSpace(message) == "" {
		return false, &domain.ErrValidation{Message: "message is required"}
	}
	if err := authorize(s.Gateway, caller, PermDialogMessage); err != nil {
		return false, err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogMessage); err != nil {
		return false, err
	}
	if s.OnMessage == nil {
		return false, &platform.ErrUnsupported{Feature: platform.FeatureDialogMessage, OS: s.Host.OS(), Detail: "no message dialog adapter bound"}
	}
	return s.OnMessage(ctx, title, message, kind)
}

// ClipboardService reads/writes the system clipboard.
type ClipboardService struct {
	Gateway Gateway
	Host    platform.Host
	OnRead  func(ctx context.Context) (string, error)
	OnWrite func(ctx context.Context, text string) error
}

// Read authorizes clipboard.read.
func (s *ClipboardService) Read(ctx context.Context, caller domain.Caller) (string, error) {
	if err := authorize(s.Gateway, caller, PermClipboardRead); err != nil {
		return "", err
	}
	if err := platform.Require(s.Host, platform.FeatureClipboard); err != nil {
		return "", err
	}
	if s.OnRead == nil {
		return "", &platform.ErrUnsupported{Feature: platform.FeatureClipboard, OS: s.Host.OS(), Detail: "no clipboard adapter bound"}
	}
	return s.OnRead(ctx)
}

// Write authorizes clipboard.write.
func (s *ClipboardService) Write(ctx context.Context, caller domain.Caller, text string) error {
	if err := authorize(s.Gateway, caller, PermClipboardWrite); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureClipboard); err != nil {
		return err
	}
	if s.OnWrite == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureClipboard, OS: s.Host.OS(), Detail: "no clipboard adapter bound"}
	}
	return s.OnWrite(ctx, text)
}

// ShortcutService registers global shortcuts.
type ShortcutService struct {
	Gateway      Gateway
	Host         platform.Host
	OnRegister   func(ctx context.Context, accelerator, actionID string) error
	OnUnregister func(ctx context.Context, accelerator string) error
}

// ParseShortcutRegister extracts accelerator + action id from an invoke payload.
// Accepts { accelerator, action|actionID|id }.
func ParseShortcutRegister(input any) (accelerator, actionID string, err error) {
	m, ok := input.(map[string]any)
	if !ok || m == nil {
		return "", "", &domain.ErrValidation{Message: "shortcut.register input must be an object"}
	}
	accelerator, _ = m["accelerator"].(string)
	actionID, _ = m["action"].(string)
	if actionID == "" {
		actionID, _ = m["actionID"].(string)
	}
	if actionID == "" {
		actionID, _ = m["id"].(string)
	}
	if accelerator == "" {
		return "", "", &domain.ErrValidation{Message: "accelerator is required"}
	}
	if actionID == "" {
		return "", "", &domain.ErrValidation{Message: "action id is required"}
	}
	return accelerator, actionID, nil
}

// ParseShortcutUnregister extracts an accelerator from an invoke payload.
// Accepts a bare string or { accelerator }.
func ParseShortcutUnregister(input any) (accelerator string, err error) {
	switch v := input.(type) {
	case string:
		accelerator = v
	case map[string]any:
		accelerator, _ = v["accelerator"].(string)
	default:
		return "", &domain.ErrValidation{Message: "shortcut.unregister input must be a string or {accelerator}"}
	}
	if accelerator == "" {
		return "", &domain.ErrValidation{Message: "accelerator is required"}
	}
	return accelerator, nil
}

// Register authorizes shortcut.register then binds accelerator → actionID.
func (s *ShortcutService) Register(ctx context.Context, caller domain.Caller, accelerator, actionID string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	if actionID == "" {
		return &domain.ErrValidation{Message: "action id is required"}
	}
	if err := authorize(s.Gateway, caller, PermShortcutRegister); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureGlobalShortcut); err != nil {
		return err
	}
	if s.OnRegister == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureGlobalShortcut, OS: s.Host.OS(), Detail: "no shortcut adapter bound"}
	}
	return s.OnRegister(ctx, accelerator, actionID)
}

// Unregister authorizes shortcut.register then removes accelerator.
func (s *ShortcutService) Unregister(ctx context.Context, caller domain.Caller, accelerator string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
	}
	if err := authorize(s.Gateway, caller, PermShortcutRegister); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureGlobalShortcut); err != nil {
		return err
	}
	if s.OnUnregister == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureGlobalShortcut, OS: s.Host.OS(), Detail: "no shortcut unregister adapter bound"}
	}
	return s.OnUnregister(ctx, accelerator)
}

// AppService provides grant-gated application lifecycle commands.
type AppService struct {
	Gateway Gateway
	OnQuit  func(ctx context.Context) error
}

// Quit authorizes app.quit then requests application shutdown.
func (s *AppService) Quit(ctx context.Context, caller domain.Caller) error {
	if err := authorize(s.Gateway, caller, PermAppQuit); err != nil {
		return err
	}
	if s.OnQuit == nil {
		return &domain.ErrValidation{Message: "no app quit adapter bound"}
	}
	return s.OnQuit(ctx)
}

// SingleInstanceService enforces single-instance behaviour.
type SingleInstanceService struct {
	Gateway Gateway
	Host    platform.Host
	OnLock  func(ctx context.Context) (bool, error)
}

// Acquire authorizes app.single_instance and attempts the platform lock.
func (s *SingleInstanceService) Acquire(ctx context.Context, caller domain.Caller) (bool, error) {
	if err := authorize(s.Gateway, caller, PermSingleInstance); err != nil {
		return false, err
	}
	if err := platform.Require(s.Host, platform.FeatureSingleInstance); err != nil {
		return false, err
	}
	if s.OnLock == nil {
		return false, &platform.ErrUnsupported{Feature: platform.FeatureSingleInstance, OS: s.Host.OS(), Detail: "no single-instance adapter bound"}
	}
	return s.OnLock(ctx)
}

// DeepLinkService validates and accepts deep links.
type DeepLinkService struct {
	Gateway  Gateway
	Host     platform.Host
	Patterns []domain.DeepLinkPattern
}

// Handle authorizes deeplink.handle and matches patterns.
func (s *DeepLinkService) Handle(caller domain.Caller, raw string) (bool, error) {
	if err := authorize(s.Gateway, caller, PermDeepLinkHandle); err != nil {
		return false, err
	}
	if err := platform.Require(s.Host, platform.FeatureDeepLink); err != nil {
		return false, err
	}
	for _, p := range s.Patterns {
		if p.Match(raw) {
			return true, nil
		}
	}
	return false, &domain.ErrValidation{Message: "deep link does not match registered patterns"}
}

// DragDropService enables receiving file drops into a window.
type DragDropService struct {
	Gateway  Gateway
	Host     platform.Host
	OnEnable func(ctx context.Context, window domain.WindowID, enabled bool) error
}

// ParseDragDropEnable extracts window id + enabled flag from an invoke payload.
// Accepts a bool (window defaults to "main") or { id|window?, enabled? }.
func ParseDragDropEnable(input any) (domain.WindowID, bool, error) {
	switch v := input.(type) {
	case bool:
		return "main", v, nil
	case map[string]any:
		id, _ := v["id"].(string)
		if id == "" {
			id, _ = v["window"].(string)
		}
		if id == "" {
			id = "main"
		}
		enabled := true
		if e, ok := asBool(v["enabled"]); ok {
			enabled = e
		}
		return domain.WindowID(id), enabled, nil
	case nil:
		return "main", true, nil
	default:
		return "", false, &domain.ErrValidation{Message: "dragdrop.receive input must be a bool or object"}
	}
}

// Enable authorizes dragdrop.receive then toggles native drop targets.
func (s *DragDropService) Enable(ctx context.Context, caller domain.Caller, window domain.WindowID, enabled bool) error {
	if window == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermDragDrop); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureDragDrop); err != nil {
		return err
	}
	if s.OnEnable == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureDragDrop, OS: s.Host.OS(), Detail: "no drag-drop adapter bound"}
	}
	return s.OnEnable(ctx, window, enabled)
}

// WindowCreateOptions configures desktop.WindowService.Create.
type WindowCreateOptions struct {
	ID     domain.WindowID
	Title  string
	Path   string
	Width  int
	Height int
}

// ParseWindowCreateOptions extracts create options from an invoke payload.
func ParseWindowCreateOptions(input any) (WindowCreateOptions, error) {
	var opts WindowCreateOptions
	switch v := input.(type) {
	case string:
		opts.ID = domain.WindowID(v)
	case map[string]any:
		if id, ok := v["id"].(string); ok {
			opts.ID = domain.WindowID(id)
		}
		if t, ok := v["title"].(string); ok {
			opts.Title = t
		}
		if p, ok := v["path"].(string); ok {
			opts.Path = p
		}
		if w, ok := asPositiveInt(v["width"]); ok {
			opts.Width = w
		}
		if h, ok := asPositiveInt(v["height"]); ok {
			opts.Height = h
		}
	default:
		if input != nil {
			return opts, &domain.ErrValidation{Message: "window.create input must be a string id or object"}
		}
	}
	if opts.ID == "" {
		return opts, &domain.ErrValidation{Message: "window id is required"}
	}
	return opts, nil
}

func asPositiveInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, n > 0
	case int64:
		return int(n), n > 0
	case float64:
		i := int(n)
		return i, n == float64(i) && i > 0
	default:
		return 0, false
	}
}

// ParseWindowID extracts a window id from string or {id} payload.
func ParseWindowID(input any) (domain.WindowID, error) {
	switch v := input.(type) {
	case string:
		if v == "" {
			return "", &domain.ErrValidation{Message: "window id is required"}
		}
		return domain.WindowID(v), nil
	case map[string]any:
		id, _ := v["id"].(string)
		if id == "" {
			return "", &domain.ErrValidation{Message: "window id is required"}
		}
		return domain.WindowID(id), nil
	default:
		return "", &domain.ErrValidation{Message: "window id is required"}
	}
}

// ParseWindowAlwaysOnTop extracts window id + alwaysOnTop flag from an invoke payload.
// Requires { id, alwaysOnTop: bool }.
func ParseWindowAlwaysOnTop(input any) (domain.WindowID, bool, error) {
	m, ok := input.(map[string]any)
	if !ok || m == nil {
		return "", false, &domain.ErrValidation{Message: "window.setAlwaysOnTop input must be an object"}
	}
	id, _ := m["id"].(string)
	if id == "" {
		return "", false, &domain.ErrValidation{Message: "window id is required"}
	}
	onTop, ok := asBool(m["alwaysOnTop"])
	if !ok {
		return "", false, &domain.ErrValidation{Message: "alwaysOnTop bool is required"}
	}
	return domain.WindowID(id), onTop, nil
}

// ParseWindowChromeApply extracts window id + chrome from an invoke payload.
func ParseWindowChromeApply(input any) (domain.WindowID, platform.WindowChrome, error) {
	m, ok := input.(map[string]any)
	if !ok || m == nil {
		return "", platform.WindowChrome{}, &domain.ErrValidation{Message: "window.chrome input must be an object"}
	}
	id, _ := m["id"].(string)
	if id == "" {
		return "", platform.WindowChrome{}, &domain.ErrValidation{Message: "window id is required"}
	}
	var chrome platform.WindowChrome
	if t, ok := m["title"].(string); ok {
		chrome.Title = t
	}
	if w, ok := asPositiveInt(m["width"]); ok {
		chrome.Width = w
	}
	if h, ok := asPositiveInt(m["height"]); ok {
		chrome.Height = h
	}
	if v, ok := asBool(m["maximized"]); ok {
		chrome.Maximized = v
	}
	if v, ok := asBool(m["fullscreen"]); ok {
		chrome.Fullscreen = v
	}
	if v, ok := asBool(m["alwaysOnTop"]); ok {
		chrome.AlwaysOnTop = v
	}
	if v, ok := asBool(m["minimized"]); ok {
		chrome.Minimized = v
	}
	if v, ok := asBool(m["hidden"]); ok {
		chrome.Hidden = v
	}
	if p, ok := m["iconPath"].(string); ok {
		chrome.IconPath = p
	}
	return domain.WindowID(id), chrome, nil
}

func asBool(v any) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

// WindowService applies native window presentation and lifecycle when permitted.
type WindowService struct {
	Gateway  Gateway
	Host     platform.Host
	OnApply  func(ctx context.Context, window domain.WindowID, chrome platform.WindowChrome) error
	OnRead   func(ctx context.Context, window domain.WindowID) (platform.WindowChrome, error)
	OnFocus  func(ctx context.Context, window domain.WindowID) error
	OnBlur   func(ctx context.Context, window domain.WindowID) error
	OnCreate func(ctx context.Context, opts WindowCreateOptions) error
	OnClose  func(ctx context.Context, window domain.WindowID) error
}

// Apply authorizes window.chrome then updates title, size, and presentation hints.
func (s *WindowService) Apply(ctx context.Context, caller domain.Caller, window domain.WindowID, chrome platform.WindowChrome) error {
	if window == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if chrome.Width <= 0 || chrome.Height <= 0 {
		return &domain.ErrValidation{Message: "window width and height must be positive"}
	}
	if err := authorize(s.Gateway, caller, PermWindowChrome); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowChrome); err != nil {
		return err
	}
	if s.OnApply == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureWindowChrome, OS: s.Host.OS(), Detail: "no window chrome adapter bound"}
	}
	return s.OnApply(ctx, window, chrome)
}

// Read authorizes window.chrome then returns the current window presentation.
func (s *WindowService) Read(ctx context.Context, caller domain.Caller, window domain.WindowID) (platform.WindowChrome, error) {
	if window == "" {
		return platform.WindowChrome{}, &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermWindowChrome); err != nil {
		return platform.WindowChrome{}, err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowChrome); err != nil {
		return platform.WindowChrome{}, err
	}
	if s.OnRead == nil {
		return platform.WindowChrome{}, &platform.ErrUnsupported{Feature: platform.FeatureWindowChrome, OS: s.Host.OS(), Detail: "no window chrome reader bound"}
	}
	return s.OnRead(ctx, window)
}

// Focus authorizes window.chrome then raises the window to the foreground.
func (s *WindowService) Focus(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	if window == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermWindowChrome); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowChrome); err != nil {
		return err
	}
	if s.OnFocus == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureWindowChrome, OS: s.Host.OS(), Detail: "no window focus adapter bound"}
	}
	return s.OnFocus(ctx, window)
}

// Blur authorizes window.chrome then resigns key focus on the window.
func (s *WindowService) Blur(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	if window == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermWindowChrome); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowChrome); err != nil {
		return err
	}
	if s.OnBlur == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureWindowChrome, OS: s.Host.OS(), Detail: "no window blur adapter bound"}
	}
	return s.OnBlur(ctx, window)
}

// Hide authorizes window.chrome then hides the window via chrome.Hidden.
func (s *WindowService) Hide(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	return s.setHidden(ctx, caller, window, true)
}

// Show authorizes window.chrome then shows the window via chrome.Hidden=false.
func (s *WindowService) Show(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	return s.setHidden(ctx, caller, window, false)
}

func (s *WindowService) setHidden(ctx context.Context, caller domain.Caller, window domain.WindowID, hidden bool) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.Hidden = hidden
	return s.Apply(ctx, caller, window, chrome)
}

// Minimize authorizes window.chrome then minimizes the window via chrome.Minimized.
func (s *WindowService) Minimize(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	return s.setMinimized(ctx, caller, window, true)
}

// Maximize authorizes window.chrome then maximizes the window via chrome.Maximized.
func (s *WindowService) Maximize(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.Maximized = true
	chrome.Minimized = false
	return s.Apply(ctx, caller, window, chrome)
}

// Fullscreen authorizes window.chrome then enters fullscreen via chrome.Fullscreen.
func (s *WindowService) Fullscreen(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.Fullscreen = true
	chrome.Minimized = false
	return s.Apply(ctx, caller, window, chrome)
}

// SetAlwaysOnTop authorizes window.chrome then toggles chrome.AlwaysOnTop.
func (s *WindowService) SetAlwaysOnTop(ctx context.Context, caller domain.Caller, window domain.WindowID, onTop bool) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.AlwaysOnTop = onTop
	return s.Apply(ctx, caller, window, chrome)
}

// Restore authorizes window.chrome then clears minimized/maximized/fullscreen.
func (s *WindowService) Restore(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.Minimized = false
	chrome.Maximized = false
	chrome.Fullscreen = false
	return s.Apply(ctx, caller, window, chrome)
}

func (s *WindowService) setMinimized(ctx context.Context, caller domain.Caller, window domain.WindowID, minimized bool) error {
	chrome, err := s.Read(ctx, caller, window)
	if err != nil {
		return err
	}
	chrome.Minimized = minimized
	if minimized {
		chrome.Maximized = false
	}
	return s.Apply(ctx, caller, window, chrome)
}

// Create authorizes window.create then opens a window via the bound adapter.
func (s *WindowService) Create(ctx context.Context, caller domain.Caller, opts WindowCreateOptions) (domain.WindowID, error) {
	if opts.ID == "" {
		return "", &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermWindowCreate); err != nil {
		return "", err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowCreate); err != nil {
		return "", err
	}
	if s.OnCreate == nil {
		return "", &platform.ErrUnsupported{Feature: platform.FeatureWindowCreate, OS: s.Host.OS(), Detail: "no window create adapter bound"}
	}
	if err := s.OnCreate(ctx, opts); err != nil {
		return "", err
	}
	return opts.ID, nil
}

// Close authorizes window.close then closes a window via the bound adapter.
func (s *WindowService) Close(ctx context.Context, caller domain.Caller, window domain.WindowID) error {
	if window == "" {
		return &domain.ErrValidation{Message: "window id is required"}
	}
	if err := authorize(s.Gateway, caller, PermWindowClose); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureWindowCreate); err != nil {
		return err
	}
	if s.OnClose == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureWindowCreate, OS: s.Host.OS(), Detail: "no window close adapter bound"}
	}
	return s.OnClose(ctx, window)
}

// BrowserService opens URLs in the system default browser when permitted.
type BrowserService struct {
	Gateway Gateway
	Host    platform.Host
	OnOpen  func(ctx context.Context, rawURL string) error
}

// OpenURL authorizes browser.open then opens an http(s) or mailto URL.
func (s *BrowserService) OpenURL(ctx context.Context, caller domain.Caller, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return &domain.ErrValidation{Message: "url is required"}
	}
	if err := authorizePath(s.Gateway, caller, PermOpenURL, rawURL); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureOpenURL); err != nil {
		return err
	}
	if s.OnOpen == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureOpenURL, OS: s.Host.OS(), Detail: "no browser opener bound"}
	}
	return s.OnOpen(ctx, rawURL)
}

// OsInfo is read-only host platform metadata for os.info.
type OsInfo struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Family  string `json:"family"`
	Version string `json:"version,omitempty"`
	Locale  string `json:"locale,omitempty"`
}

// OsService returns host platform information when permitted.
// Default fill-in uses the Go runtime (no platform.Feature / CGO).
type OsService struct {
	Gateway Gateway
	OnInfo  func(ctx context.Context) (OsInfo, error)
}

// Info authorizes os.info then returns host platform metadata.
func (s *OsService) Info(ctx context.Context, caller domain.Caller) (OsInfo, error) {
	if err := authorize(s.Gateway, caller, PermOsInfo); err != nil {
		return OsInfo{}, err
	}
	if s.OnInfo != nil {
		return s.OnInfo(ctx)
	}
	return DefaultOsInfo(), nil
}

// DefaultOsInfo fills OsInfo from the Go runtime and common locale env vars.
func DefaultOsInfo() OsInfo {
	family := "unix"
	switch runtime.GOOS {
	case "windows":
		family = "windows"
	case "js", "wasip1":
		family = runtime.GOOS
	}
	locale := strings.TrimSpace(os.Getenv("LC_ALL"))
	if locale == "" {
		locale = strings.TrimSpace(os.Getenv("LANG"))
	}
	return OsInfo{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		Family: family,
		Locale: locale,
	}
}

// NotificationService shows desktop notifications when permitted.
type NotificationService struct {
	Gateway Gateway
	Host    platform.Host
	OnShow  func(ctx context.Context, title, body string) error
}

// Show authorizes notifications.show then displays a title+body notification.
func (s *NotificationService) Show(ctx context.Context, caller domain.Caller, title, body string) error {
	if strings.TrimSpace(body) == "" && strings.TrimSpace(title) == "" {
		return &domain.ErrValidation{Message: "title or body is required"}
	}
	if err := authorize(s.Gateway, caller, PermNotificationShow); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeatureNotificationShow); err != nil {
		return err
	}
	if s.OnShow == nil {
		return &platform.ErrUnsupported{Feature: platform.FeatureNotificationShow, OS: s.Host.OS(), Detail: "no notification adapter bound"}
	}
	return s.OnShow(ctx, title, body)
}

// PathService opens local filesystem paths with the OS default handler when permitted.
type PathService struct {
	Gateway Gateway
	Host    platform.Host
	OnOpen  func(ctx context.Context, path string) error
}

// Open authorizes path.open for an absolute local path then opens it.
func (s *PathService) Open(ctx context.Context, caller domain.Caller, path string) error {
	cleaned, err := validateLocalPath(path)
	if err != nil {
		return err
	}
	if err := authorizePath(s.Gateway, caller, PermPathOpen, cleaned); err != nil {
		return err
	}
	if err := platform.Require(s.Host, platform.FeaturePathOpen); err != nil {
		return err
	}
	if s.OnOpen == nil {
		return &platform.ErrUnsupported{Feature: platform.FeaturePathOpen, OS: s.Host.OS(), Detail: "no path opener bound"}
	}
	return s.OnOpen(ctx, cleaned)
}

func validateLocalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", &domain.ErrValidation{Message: "path is required"}
	}
	lower := strings.ToLower(path)
	if strings.Contains(path, "://") || strings.HasPrefix(lower, "file:") {
		return "", &domain.ErrValidation{Message: "path must be a local filesystem path, not a URL"}
	}
	if !filepath.IsAbs(path) {
		return "", &domain.ErrValidation{Message: "path must be absolute"}
	}
	return filepath.Clean(path), nil
}

func authorize(gw Gateway, caller domain.Caller, perm domain.PermissionName) error {
	return authorizePath(gw, caller, perm, "")
}

var errGatewayRequired = errors.New("desktop gateway is required")

func authorizePath(gw Gateway, caller domain.Caller, perm domain.PermissionName, resourcePath string) error {
	if gw == nil {
		return errGatewayRequired
	}
	d := gw.Authorize(caller, perm, resourcePath)
	if d.Allowed {
		return nil
	}
	return &domain.ErrDenied{
		Permission: perm,
		Window:     caller.Window,
		Origin:     caller.Origin,
		Code:       d.Code,
		Reason:     d.Reason,
	}
}
