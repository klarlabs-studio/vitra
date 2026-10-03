package domain

import (
	"context"
	"errors"
)

// CommandExecutor executes a registered command after authorization.
type CommandExecutor interface {
	Execute(ctx context.Context, name CommandName, input any) (any, error)
}

// CommandExecutorFunc adapts a function to CommandExecutor.
type CommandExecutorFunc func(ctx context.Context, name CommandName, input any) (any, error)

// Execute implements CommandExecutor.
func (f CommandExecutorFunc) Execute(ctx context.Context, name CommandName, input any) (any, error) {
	return f(ctx, name, input)
}

// ErrNoInvocation is returned by a caller-bound executor that runs outside an
// authorized invocation, so it has no caller identity to act as.
var ErrNoInvocation = errors.New("no authorized invocation in context")

// Invocation describes a command call the gateway has authorized. The
// runtime puts it on the executor's context; read it with InvocationFrom.
type Invocation struct {
	// Caller is the identity established at the native boundary: the window
	// that sent the call and its live origin.
	Caller Caller
	// Command is the invoked command.
	Command CommandName
	// ResourcePath is the path the gateway checked against the grant's path
	// scope, if any. An executor that touches a path-scoped resource must act
	// on this path, or authorize the path it does use.
	ResourcePath string
	// Grant names the grant that allowed the call.
	Grant GrantName
}

type invocationKey struct{}

// WithInvocation returns a context carrying inv. The runtime calls it before
// running an executor; tests call it to run an executor or a
// CallerExecutorFunc as a given caller without a runtime.
func WithInvocation(ctx context.Context, inv Invocation) context.Context {
	return context.WithValue(ctx, invocationKey{}, inv)
}

// InvocationFrom returns the authorized invocation an executor is running
// for, if any.
func InvocationFrom(ctx context.Context) (Invocation, bool) {
	inv, ok := ctx.Value(invocationKey{}).(Invocation)
	return inv, ok
}

// CallerExecutorFunc adapts a function that acts on behalf of the invoking
// caller to CommandExecutor. It receives the caller the gateway authorized,
// never one chosen by the executor, and fails with ErrNoInvocation if run
// outside an authorized invocation.
type CallerExecutorFunc func(ctx context.Context, caller Caller, input any) (any, error)

// Execute implements CommandExecutor.
func (f CallerExecutorFunc) Execute(ctx context.Context, _ CommandName, input any) (any, error) {
	inv, ok := InvocationFrom(ctx)
	if !ok {
		return nil, ErrNoInvocation
	}
	return f(ctx, inv.Caller, input)
}
