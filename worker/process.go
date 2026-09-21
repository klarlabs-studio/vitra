package worker

import (
	"context"
	"errors"
	"os/exec"
)

// Command is an OS process supervised outside the WebView host.
// Path is executed directly (no shell).
type Command struct {
	Path string
	Args []string
	Dir  string
	// Env replaces the process environment when non-nil.
	// Nil inherits the supervisor environment.
	Env []string
}

// CommandRunner returns a Runner that execs cmd until it exits or ctx is cancelled.
func CommandRunner(cmd Command) (Runner, error) {
	if cmd.Path == "" {
		return nil, errors.New("command path is required")
	}
	return func(ctx context.Context) error {
		c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
		if cmd.Dir != "" {
			c.Dir = cmd.Dir
		}
		if cmd.Env != nil {
			c.Env = cmd.Env
		}
		err := c.Run()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}, nil
}
