package domain

import (
	"context"
	"fmt"
)

// InvocationRequest is a frontend-originated command call after the adapter
// has established caller identity at the native boundary.
type InvocationRequest struct {
	Caller       Caller
	Command      CommandName
	Input        any
	ResourcePath string // optional; used when the command's permission is path-scoped
}

// InvocationResult is a typed success or structured denial/failure.
type InvocationResult struct {
	Command    CommandName
	Output     any
	Decision   Decision
	Authorized bool
}

// InvocationService authorizes and executes commands.
// Pipeline: identify caller → resolve command → evaluate capability →
// execute → return typed result or structured denial.
type InvocationService struct {
	Commands  CommandRepository
	Grants    GrantRepository
	Windows   WindowRepository
	Executors CommandExecutorLookup
}

// Invoke runs the secure invocation pipeline.
func (s *InvocationService) Invoke(ctx context.Context, req InvocationRequest) (*InvocationResult, error) {
	if req.Command == "" {
		return nil, &ErrValidation{Message: "command is required"}
	}
	if req.Caller.Window == "" || req.Caller.Origin == "" {
		return nil, &ErrValidation{Message: "caller identity is required"}
	}

	if s.Windows != nil {
		win, err := s.Windows.Get(req.Caller.Window)
		if err != nil {
			return nil, err
		}
		if !win.IsOpen() {
			return nil, &ErrDenied{
				Permission: "",
				Window:     req.Caller.Window,
				Origin:     req.Caller.Origin,
				Code:       DenialWindowClosed,
				Reason:     "window is closed",
			}
		}
		// Native boundary wins: refuse payload-spoofed origin that disagrees
		// with the live window origin.
		if win.Origin() != req.Caller.Origin {
			return nil, &ErrDenied{
				Window: req.Caller.Window,
				Origin: req.Caller.Origin,
				Code:   DenialOriginMismatch,
				Reason: "caller origin does not match window origin",
			}
		}
	}

	cmd, err := s.Commands.Get(req.Command)
	if err != nil {
		return nil, err
	}

	grants, err := s.Grants.List()
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	gw := NewCapabilityGateway(grants...)
	decision := gw.Authorize(req.Caller, cmd.Permission(), req.ResourcePath)
	if !decision.Allowed {
		return &InvocationResult{
				Command:    req.Command,
				Decision:   decision,
				Authorized: false,
			}, &ErrDenied{
				Permission: cmd.Permission(),
				Window:     req.Caller.Window,
				Origin:     req.Caller.Origin,
				Code:       decision.Code,
				Reason:     decision.Reason,
			}
	}

	exec, ok := s.Executors.Get(req.Command)
	if !ok {
		return nil, &ErrNotFound{Entity: "command executor", ID: string(req.Command)}
	}
	out, err := exec.Execute(ctx, req.Command, req.Input)
	if err != nil {
		return nil, err
	}
	return &InvocationResult{
		Command:    req.Command,
		Output:     out,
		Decision:   decision,
		Authorized: true,
	}, nil
}
