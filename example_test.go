package vitra_test

import (
	"context"
	"errors"
	"fmt"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

// A window starts with no authority. A grant gives it one permission, scoped
// to a path; a command is reachable only through that permission; and the
// authority does not follow the window to another origin.
func Example() {
	ctx := context.Background()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		panic(err)
	}
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		panic(err)
	}

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
		panic(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		panic(err)
	}

	cmd, err := domain.NewCommandDefinition("project.open", "Open a project path", "fs.read")
	if err != nil {
		panic(err)
	}
	open := domain.CommandExecutorFunc(func(_ context.Context, _ domain.CommandName, input any) (any, error) {
		return fmt.Sprintf("opened %v", input), nil
	})
	if err := rt.RegisterCommand(cmd, open); err != nil {
		panic(err)
	}

	invoke := func(path string) {
		// The caller comes from the runtime's window record, never from the
		// frontend payload.
		caller, err := rt.CallerFor("main")
		if err != nil {
			panic(err)
		}
		res, err := rt.Invoke(ctx, domain.InvocationRequest{
			Caller:       caller,
			Command:      "project.open",
			Input:        path,
			ResourcePath: path,
		})
		var denied *domain.ErrDenied
		switch {
		case errors.As(err, &denied):
			fmt.Printf("%s: denied (%s)\n", path, denied.Code)
		case err != nil:
			panic(err)
		default:
			fmt.Printf("%s: %v\n", path, res.Output)
		}
	}

	invoke("/project/app")
	invoke("/project/.secrets/token")
	invoke("/etc/passwd")

	if err := rt.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		panic(err)
	}
	invoke("/project/app")

	// Output:
	// /project/app: opened /project/app
	// /project/.secrets/token: denied (path_denied)
	// /etc/passwd: denied (path_out_of_scope)
	// /project/app: denied (origin_mismatch)
}
