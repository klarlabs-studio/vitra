// Command quickstart demonstrates the Vitra Phase 1 secure runtime kernel:
// open a window with no ambient privileges, register a narrow capability
// grant, invoke an explicit command, then observe denial after navigation.
package main

import (
	"context"
	"fmt"
	"os"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

type openProject struct{}

func (openProject) Execute(_ context.Context, _ domain.CommandName, input any) (any, error) {
	return map[string]any{"path": input, "status": "opened"}, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "quickstart: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.quickstart"})
	if err != nil {
		return err
	}

	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		return err
	}
	fmt.Println("1) opened window main at app://local (no privileges yet)")
	fmt.Print(vitra.FormatInspect(rt.AppID(), mustSurface(rt, "main")))

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
	cmd, err := domain.NewCommandDefinition("project.open", "Open a project path", "fs.read")
	if err != nil {
		return err
	}
	if err := rt.RegisterCommand(cmd, openProject{}); err != nil {
		return err
	}
	fmt.Println("2) registered grant project-files + command project.open")

	caller, err := rt.CallerFor("main")
	if err != nil {
		return err
	}
	res, err := rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		Input:        "/project/app",
		ResourcePath: "/project/app",
	})
	if err != nil {
		return err
	}
	fmt.Printf("3) invoke authorized via grant %s → %v\n", res.Decision.Grant, res.Output)

	if err := rt.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		return err
	}
	caller, err = rt.CallerFor("main")
	if err != nil {
		return err
	}
	_, err = rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		ResourcePath: "/project/app",
	})
	if err == nil {
		return fmt.Errorf("expected denial after navigation")
	}
	fmt.Printf("4) after navigate to untrusted origin: %v\n", err)
	return nil
}

func mustSurface(rt *vitra.Runtime, id domain.WindowID) domain.EffectiveSurface {
	s, err := rt.InspectCapabilities(id)
	if err != nil {
		panic(err)
	}
	return s
}
