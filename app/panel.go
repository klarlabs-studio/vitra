package app

import (
	"fmt"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/platform"
)

// checkWindowKind accepts a normal window, and a panel on hosts with panels.
func (a *App) checkWindowKind(kind WindowKind) error {
	switch kind {
	case WindowKindNormal:
		return nil
	case WindowKindPanel:
		_, err := a.panels()
		return err
	default:
		return &domain.ErrValidation{Message: fmt.Sprintf("unknown window kind %q", kind)}
	}
}

func (a *App) panels() (platform.Panels, error) {
	if err := platform.Require(a.host, platform.FeatureWindowPanel); err != nil {
		return nil, err
	}
	p, ok := a.host.(platform.Panels)
	if !ok {
		return nil, &platform.ErrUnsupported{Feature: platform.FeatureWindowPanel, OS: a.host.OS(), Detail: "host cannot show panels"}
	}
	return p, nil
}

// useTrayPanel makes primary tray clicks toggle panel (none when empty).
func (a *App) useTrayPanel(panel domain.WindowID) {
	a.mu.Lock()
	first := a.trayPanel == "" && panel != ""
	a.trayPanel = panel
	a.mu.Unlock()
	if first {
		a.trayClicks.onEach(a.toggleTrayPanel)
		a.installTrayClicks()
	}
}

func (a *App) currentTrayPanel() domain.WindowID {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.trayPanel
}

// toggleTrayPanel hides the tray panel when it is shown (or just hid itself
// because this very click took its focus) and shows it under the icon
// otherwise.
func (a *App) toggleTrayPanel(c platform.TrayClick) {
	id := a.currentTrayPanel()
	if id == "" {
		return
	}
	p, err := a.panels()
	if err != nil {
		return
	}
	if shown, err := p.PanelShown(id); err != nil {
		return
	} else if shown {
		_ = p.HidePanel(id)
		return
	}
	_ = a.showPanel(p, id, c)
}

func (a *App) showPanel(p platform.Panels, id domain.WindowID, c platform.TrayClick) error {
	anchor, has := c.Anchor, c.HasAnchor
	if !has {
		if r, err := a.TrayAnchor(); err == nil {
			anchor, has = r, true
		}
	}
	return p.ShowPanel(id, anchor, has)
}

// ShowTrayPanel shows the tray panel (TraySpec.Panel) under the tray icon,
// for example from a global shortcut. Where the host cannot tell where the
// icon is, the panel is centered.
func (a *App) ShowTrayPanel() error {
	id, p, err := a.trayPanelHost()
	if err != nil {
		return err
	}
	return a.showPanel(p, id, platform.TrayClick{})
}

// HideTrayPanel hides the tray panel.
func (a *App) HideTrayPanel() error {
	id, p, err := a.trayPanelHost()
	if err != nil {
		return err
	}
	return p.HidePanel(id)
}

func (a *App) trayPanelHost() (domain.WindowID, platform.Panels, error) {
	id := a.currentTrayPanel()
	if id == "" {
		return "", nil, &domain.ErrValidation{Message: "the tray has no panel: set TraySpec.Panel"}
	}
	p, err := a.panels()
	return id, p, err
}
