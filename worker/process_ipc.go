package worker

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// StdioIPC returns a Runner that execs cmd with stdin/stdout wired to a Session.
// The returned Session is the host side; the child process speaks JSON-lines on
// its stdin/stdout. Cancel stops the process and closes the session.
func StdioIPC(cmd Command) (Runner, *Session, error) {
	if cmd.Path == "" {
		return nil, nil, errors.New("command path is required")
	}
	hostR, childW := io.Pipe()
	childR, hostW := io.Pipe()
	host := NewSession(hostR, hostW, multiCloser{hostW, hostR})

	run := func(ctx context.Context) error {
		c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
		if cmd.Dir != "" {
			c.Dir = cmd.Dir
		}
		if cmd.Env != nil {
			c.Env = cmd.Env
		}
		c.Stdin = childR
		c.Stdout = childW

		go func() {
			<-ctx.Done()
			_ = hostW.Close()
			_ = childW.Close()
			_ = childR.Close()
			_ = hostR.Close()
		}()

		err := c.Run()
		_ = host.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return run, host, nil
}
