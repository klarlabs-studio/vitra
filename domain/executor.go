package domain

import "context"

// CommandExecutorFunc adapts a function to CommandExecutor.
type CommandExecutorFunc func(ctx context.Context, name CommandName, input any) (any, error)

// Execute implements CommandExecutor.
func (f CommandExecutorFunc) Execute(ctx context.Context, name CommandName, input any) (any, error) {
	return f(ctx, name, input)
}
