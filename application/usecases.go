// Package application contains Vitra use cases that orchestrate the domain.
package application

import (
	"context"
	"fmt"

	"go.klarlabs.de/vitra/domain"
)

// RegisterGrantUseCase persists a capability grant.
type RegisterGrantUseCase struct {
	Grants domain.GrantRepository
}

// Execute saves the grant.
func (uc *RegisterGrantUseCase) Execute(grant *domain.CapabilityGrant) error {
	if grant == nil {
		return &domain.ErrValidation{Message: "grant is required"}
	}
	return uc.Grants.Save(grant)
}

// OpenWindowUseCase creates and persists a window.
type OpenWindowUseCase struct {
	Windows domain.WindowRepository
}

// Execute opens a window at origin.
func (uc *OpenWindowUseCase) Execute(id domain.WindowID, origin domain.Origin) (*domain.Window, error) {
	if existing, err := uc.Windows.Get(id); err == nil && existing != nil {
		return nil, &domain.ErrConflict{Message: "window already open: " + string(id)}
	}
	win, err := domain.NewWindow(id, origin)
	if err != nil {
		return nil, err
	}
	if err := uc.Windows.Save(win); err != nil {
		return nil, err
	}
	return win, nil
}

// NavigateWindowUseCase changes a window's origin.
type NavigateWindowUseCase struct {
	Windows domain.WindowRepository
}

// Execute navigates the window.
func (uc *NavigateWindowUseCase) Execute(id domain.WindowID, origin domain.Origin) error {
	win, err := uc.Windows.Get(id)
	if err != nil {
		return err
	}
	if err := win.Navigate(origin); err != nil {
		return err
	}
	return uc.Windows.Save(win)
}

// CloseWindowUseCase closes a window and releases owned resources.
type CloseWindowUseCase struct {
	Windows   domain.WindowRepository
	Resources domain.ResourceRepository
}

// Execute closes the window and owned handles.
func (uc *CloseWindowUseCase) Execute(id domain.WindowID) error {
	win, err := uc.Windows.Get(id)
	if err != nil {
		return err
	}
	if err := win.Close(); err != nil {
		return err
	}
	if uc.Resources != nil {
		handles, err := uc.Resources.ListByOwner(id)
		if err != nil {
			return fmt.Errorf("list resources: %w", err)
		}
		for _, h := range handles {
			h.Close()
			if err := uc.Resources.Delete(h.ID()); err != nil {
				return err
			}
		}
	}
	return uc.Windows.Save(win)
}

// RegisterCommandUseCase registers a command definition.
type RegisterCommandUseCase struct {
	Commands domain.CommandRepository
}

// Execute saves the command definition.
func (uc *RegisterCommandUseCase) Execute(cmd *domain.CommandDefinition) error {
	if cmd == nil {
		return &domain.ErrValidation{Message: "command is required"}
	}
	return uc.Commands.Save(cmd)
}

// InvokeCommandUseCase is the frontend→Go invocation entry.
type InvokeCommandUseCase struct {
	Invoker *domain.InvocationService
}

// Execute runs the secure invocation pipeline.
func (uc *InvokeCommandUseCase) Execute(ctx context.Context, req domain.InvocationRequest) (*domain.InvocationResult, error) {
	return uc.Invoker.Invoke(ctx, req)
}

// InspectCapabilitiesUseCase projects the effective privileged surface.
type InspectCapabilitiesUseCase struct {
	Grants  domain.GrantRepository
	Windows domain.WindowRepository
}

// Execute returns the inspectable surface for a window.
func (uc *InspectCapabilitiesUseCase) Execute(windowID domain.WindowID) (domain.EffectiveSurface, error) {
	win, err := uc.Windows.Get(windowID)
	if err != nil {
		return domain.EffectiveSurface{}, err
	}
	grants, err := uc.Grants.List()
	if err != nil {
		return domain.EffectiveSurface{}, err
	}
	gw := domain.NewCapabilityGateway(grants...)
	return gw.Inspect(win.ID(), win.Origin()), nil
}
