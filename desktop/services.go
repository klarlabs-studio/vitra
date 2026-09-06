// Package desktop provides Phase 2 capability-gated desktop services.
//
// Menus, tray, dialogs, clipboard, shortcuts, deep links, and single-instance
// behaviour are expressed as domain services that require explicit permissions
// and platform feature support — never ambient authority.
package desktop

import (
	"context"
	"errors"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// Permission names for Phase 2 desktop surfaces.
const (
	PermMenuSet          domain.PermissionName = "menu.set"
	PermTraySet          domain.PermissionName = "tray.set"
	PermDialogOpen       domain.PermissionName = "dialog.open"
	PermDialogSave       domain.PermissionName = "dialog.save"
	PermClipboardRead    domain.PermissionName = "clipboard.read"
	PermClipboardWrite   domain.PermissionName = "clipboard.write"
	PermShortcutRegister domain.PermissionName = "shortcut.register"
	PermDeepLinkHandle   domain.PermissionName = "deeplink.handle"
	PermSingleInstance   domain.PermissionName = "app.single_instance"
	PermDragDrop         domain.PermissionName = "dragdrop.receive"
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
	Children []MenuItem
}

// MenuService applies application menus when permitted and supported.
type MenuService struct {
	Gateway Gateway
	Host    platform.Host
	OnSet   func(ctx context.Context, items []MenuItem) error // optional native hook
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

// TrayService manages tray icons/menus.
type TrayService struct {
	Gateway Gateway
	Host    platform.Host
	OnSet   func(ctx context.Context, tooltip string, items []MenuItem) error
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

// DialogService opens native file dialogs.
type DialogService struct {
	Gateway Gateway
	Host    platform.Host
	OnOpen  func(ctx context.Context) ([]string, error)
	OnSave  func(ctx context.Context) (string, error)
}

// OpenFile authorizes dialog.open.
func (s *DialogService) OpenFile(ctx context.Context, caller domain.Caller) ([]string, error) {
	if err := authorize(s.Gateway, caller, PermDialogOpen); err != nil {
		return nil, err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogOpen); err != nil {
		return nil, err
	}
	if s.OnOpen == nil {
		return nil, &platform.ErrUnsupported{Feature: platform.FeatureDialogOpen, OS: s.Host.OS(), Detail: "no dialog adapter bound"}
	}
	return s.OnOpen(ctx)
}

// SaveFile authorizes dialog.save.
func (s *DialogService) SaveFile(ctx context.Context, caller domain.Caller) (string, error) {
	if err := authorize(s.Gateway, caller, PermDialogSave); err != nil {
		return "", err
	}
	if err := platform.Require(s.Host, platform.FeatureDialogSave); err != nil {
		return "", err
	}
	if s.OnSave == nil {
		return "", &platform.ErrUnsupported{Feature: platform.FeatureDialogSave, OS: s.Host.OS(), Detail: "no dialog adapter bound"}
	}
	return s.OnSave(ctx)
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
	Gateway    Gateway
	Host       platform.Host
	OnRegister func(ctx context.Context, accelerator string) error
}

// Register authorizes shortcut.register.
func (s *ShortcutService) Register(ctx context.Context, caller domain.Caller, accelerator string) error {
	if accelerator == "" {
		return &domain.ErrValidation{Message: "accelerator is required"}
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
	return s.OnRegister(ctx, accelerator)
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

func authorize(gw Gateway, caller domain.Caller, perm domain.PermissionName) error {
	if gw == nil {
		return errors.New("desktop gateway is required")
	}
	d := gw.Authorize(caller, perm, "")
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
