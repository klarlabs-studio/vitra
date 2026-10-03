package application

import (
	"context"
	"errors"
	"fmt"

	"go.klarlabs.de/vitra/domain"
)

// InvocationService authorizes and executes commands.
// Pipeline: identify caller → resolve command → evaluate capability →
// optional enterprise overlay → execute → return typed result or structured denial.
type InvocationService struct {
	Commands  CommandRepository
	Grants    GrantRepository
	Windows   WindowRepository
	Executors CommandExecutorLookup
	// Overlay optionally tightens an allow decision (enterprise policy).
	// Never used to loosen a denial.
	Overlay func(permission domain.PermissionName, d domain.Decision) domain.Decision
}

// Invoke runs the secure invocation pipeline.
func (s *InvocationService) Invoke(ctx context.Context, req domain.InvocationRequest) (*domain.InvocationResult, error) {
	if req.Command == "" {
		return nil, &domain.ErrValidation{Message: "command is required"}
	}
	if req.Caller.Window == "" || req.Caller.Origin == "" {
		return nil, &domain.ErrValidation{Message: "caller identity is required"}
	}

	if s.Windows != nil {
		win, err := s.Windows.Get(req.Caller.Window)
		if err != nil {
			return nil, err
		}
		if !win.IsOpen() {
			return nil, &domain.ErrDenied{
				Permission: "",
				Window:     req.Caller.Window,
				Origin:     req.Caller.Origin,
				Code:       domain.DenialWindowClosed,
				Reason:     "window is closed",
			}
		}
		// Native boundary wins: refuse payload-spoofed origin that disagrees
		// with the live window origin.
		if win.Origin() != req.Caller.Origin {
			return nil, &domain.ErrDenied{
				Window: req.Caller.Window,
				Origin: req.Caller.Origin,
				Code:   domain.DenialOriginMismatch,
				Reason: "caller origin does not match window origin",
			}
		}
	}

	cmd, err := s.Commands.Get(req.Command)
	var missing *domain.ErrNotFound
	if errors.As(err, &missing) {
		// Only registered commands exist; calling anything else is refused
		// like any other denial, so probes show up in the audit log.
		return nil, &domain.ErrDenied{
			Window: req.Caller.Window,
			Origin: req.Caller.Origin,
			Code:   domain.DenialCommandMissing,
			Reason: "command is not registered",
		}
	}
	if err != nil {
		return nil, err
	}

	grants, err := s.Grants.List()
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	gw := NewCapabilityGateway(grants...)
	decision := gw.Authorize(req.Caller, cmd.Permission(), req.ResourcePath)
	if s.Overlay != nil {
		decision = s.Overlay(cmd.Permission(), decision)
	}
	if !decision.Allowed {
		denied := &domain.ErrDenied{
			Permission: cmd.Permission(),
			Window:     req.Caller.Window,
			Origin:     req.Caller.Origin,
			Code:       decision.Code,
			Reason:     decision.Reason,
		}
		result := &domain.InvocationResult{
			Command:    req.Command,
			Decision:   decision,
			Authorized: false,
		}
		return result, denied
	}

	exec, ok := s.Executors.Get(req.Command)
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "command executor", ID: string(req.Command)}
	}
	ctx = domain.WithInvocation(ctx, domain.Invocation{
		Caller:       req.Caller,
		Command:      req.Command,
		ResourcePath: req.ResourcePath,
		Grant:        decision.Grant,
	})
	out, err := exec.Execute(ctx, req.Command, req.Input)
	if err != nil {
		return nil, err
	}
	return &domain.InvocationResult{
		Command:    req.Command,
		Output:     out,
		Decision:   decision,
		Authorized: true,
	}, nil
}
