package vitra_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/audit"
	"go.klarlabs.de/vitra/domain"
)

type OpenRequest struct {
	Path string `json:"path"`
}

// ResourcePath binds the input to the path the gateway authorized.
func (r OpenRequest) ResourcePath() string { return r.Path }

type OpenResult struct {
	Title string `json:"title"`
}

// newExampleRuntime returns a runtime with a main window allowed to read
// /notes, except /notes/.private.
func newExampleRuntime() *vitra.Runtime {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.notes"})
	if err != nil {
		panic(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		panic(err)
	}
	grant, err := domain.NewCapabilityGrant("notes", "read notes",
		[]domain.WindowID{"main"}, []domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.read", PathScope: &domain.PathScope{
			Allow: []string{"/notes/**"},
			Deny:  []string{"/notes/.private/**"},
		}}})
	if err != nil {
		panic(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		panic(err)
	}
	return rt
}

// A typed command: the input is decoded strictly into OpenRequest, the
// handler learns who called, and a caller cannot get one path authorized and
// name another in the input.
func ExampleRegister() {
	rt := newExampleRuntime()
	err := vitra.Register(rt, vitra.Command[OpenRequest, OpenResult]{
		Name:        "notes.open",
		Description: "Open a note",
		Permission:  "fs.read",
		Handler: func(_ context.Context, inv domain.Invocation, req OpenRequest) (OpenResult, error) {
			return OpenResult{Title: req.Path + " for " + string(inv.Caller.Window)}, nil
		},
	})
	if err != nil {
		panic(err)
	}

	call := func(input map[string]any, resourcePath string) {
		caller, _ := rt.CallerFor("main")
		res, err := rt.Invoke(context.Background(), domain.InvocationRequest{
			Caller: caller, Command: "notes.open", Input: input, ResourcePath: resourcePath,
		})
		var denied *domain.ErrDenied
		switch {
		case errors.As(err, &denied):
			fmt.Println("denied:", denied.Code)
		case err != nil:
			fmt.Println("error:", err)
		default:
			fmt.Printf("%+v\n", res.Output)
		}
	}
	call(map[string]any{"path": "/notes/todo.md"}, "/notes/todo.md")
	call(map[string]any{"path": "/notes/.private/keys.md"}, "/notes/.private/keys.md")
	call(map[string]any{"path": "/etc/passwd"}, "/notes/todo.md") // path swapped after authorization
	call(map[string]any{"path": "/notes/a.md", "admin": true}, "/notes/a.md")
	// Output:
	// {Title:/notes/todo.md for main}
	// denied: path_denied
	// denied: path_out_of_scope
	// error: command notes.open input: json: unknown field "admin"
}

// The TypeScript client follows the Go types of registered commands.
func ExampleRuntime_TypeScript() {
	rt := newExampleRuntime()
	_ = vitra.Register(rt, vitra.Command[OpenRequest, OpenResult]{
		Name: "notes.open", Description: "Open a note", Permission: "fs.read",
		Handler: func(context.Context, domain.Invocation, OpenRequest) (OpenResult, error) {
			return OpenResult{}, nil
		},
	})
	ts, err := rt.TypeScript("com.example.notes")
	if err != nil {
		panic(err)
	}
	for _, line := range strings.Split(ts, "\n") {
		if strings.Contains(line, "notesOpen(") || strings.HasPrefix(line, "export interface Open") {
			fmt.Println(strings.TrimSpace(line))
		}
	}
	// Output:
	// export interface OpenRequest {
	// export interface OpenResult {
	// notesOpen(input: OpenRequest, resourcePath?: string): Promise<OpenResult>;
}

// Every call is audited with the path that was attempted and, for refusals,
// the denial code.
func ExampleRuntime_SetAudit() {
	rt := newExampleRuntime()
	sink := &audit.MemorySink{}
	rt.SetAudit(sink)
	def, _ := domain.NewCommandDefinition("notes.read", "Read a note", "fs.read")
	_ = rt.RegisterCommand(def, domain.CommandExecutorFunc(
		func(context.Context, domain.CommandName, any) (any, error) { return "ok", nil }))

	caller, _ := rt.CallerFor("main")
	for _, p := range []string{"/notes/a.md", "/home/me/.ssh/id_ed25519"} {
		_, _ = rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "notes.read", ResourcePath: p})
	}
	_, _ = rt.Invoke(context.Background(), domain.InvocationRequest{Caller: caller, Command: "shell.exec"})

	for _, e := range sink.List() {
		if e.Kind == audit.KindCommandInvoke {
			fmt.Println(e.Action, e.Outcome, e.Metadata["resource_path"], e.Metadata["code"])
		}
	}
	// Output:
	// notes.read allowed /notes/a.md <nil>
	// notes.read denied /home/me/.ssh/id_ed25519 path_out_of_scope
	// shell.exec denied <nil> command_missing
}
