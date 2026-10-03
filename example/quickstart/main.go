// Command quickstart shows Vitra's secure runtime kernel without a window
// toolkit: a window starts with no privileges, a narrow grant gives it one
// permission on one folder, a typed command runs only within that grant,
// and navigating the window away drops its authority.
//
//	go run ./example/quickstart
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

// OpenProject is the input of project.open. Its path is the resource the
// gateway authorizes: ResourcePath binds the handler to the path that was
// checked, so it cannot be pointed anywhere else.
type OpenProject struct {
	Path string `json:"path"`
}

// ResourcePath implements vitra.ResourcePather.
func (in OpenProject) ResourcePath() string { return in.Path }

// Opened is the output of project.open.
type Opened struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "quickstart: %v\n", err)
		os.Exit(1)
	}
}

func run(out io.Writer) error {
	ctx := context.Background()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.quickstart"})
	if err != nil {
		return err
	}

	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		return err
	}
	fmt.Fprintln(out, "1) opened window main at app://local (no privileges yet)")
	surface, err := rt.InspectCapabilities("main")
	if err != nil {
		return err
	}
	fmt.Fprint(out, vitra.FormatInspect(rt.AppID(), surface))

	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"read project files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{
			Name: "fs.read",
			PathScope: &domain.PathScope{
				Allow: []string{"/project/**"},
				Deny:  []string{"/project/.secrets/**"},
			},
		}},
	)
	if err != nil {
		return err
	}
	if err := rt.RegisterGrant(grant); err != nil {
		return err
	}
	// A command names its permission: the gateway checks it, with the
	// grant's path scope, before the handler runs.
	err = vitra.Register(rt, vitra.Command[OpenProject, Opened]{
		Name:        "project.open",
		Description: "Open a project path",
		Permission:  "fs.read",
		Handler: func(_ context.Context, _ domain.Invocation, in OpenProject) (Opened, error) {
			return Opened{Path: in.Path, Status: "opened"}, nil
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "2) registered grant project-files + command project.open")

	// open invokes project.open as the window, the way the bridge does for
	// the page: the caller comes from the window, not from the input.
	open := func(path string) (*domain.InvocationResult, error) {
		caller, err := rt.CallerFor("main")
		if err != nil {
			return nil, err
		}
		return rt.Invoke(ctx, domain.InvocationRequest{
			Caller:       caller,
			Command:      "project.open",
			Input:        map[string]any{"path": path},
			ResourcePath: path,
		})
	}

	res, err := open("/project/app")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "3) invoke authorized via grant %s → %+v\n", res.Decision.Grant, res.Output)

	if err := expectDenied(open("/project/.secrets/token")); err != nil {
		return err
	}
	fmt.Fprintln(out, "4) /project/.secrets/token is denied: deny patterns win over allow")

	if err := rt.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		return err
	}
	if err := expectDenied(open("/project/app")); err != nil {
		return err
	}
	fmt.Fprintln(out, "5) after navigating to an untrusted origin, the same call is denied: authority does not follow navigation")
	return nil
}

// expectDenied returns nil when err is a capability denial.
func expectDenied(_ *domain.InvocationResult, err error) error {
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) {
		return fmt.Errorf("expected a denial, got %v", err)
	}
	return nil
}
