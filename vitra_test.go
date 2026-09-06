package vitra_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

type echoExec struct{}

func (echoExec) Execute(_ context.Context, _ domain.CommandName, input any) (any, error) {
	return input, nil
}

func TestRuntime_SecureByDefault(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}

	cmd, _ := domain.NewCommandDefinition("notify.show", "Show notification", "notifications.show")
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}

	caller, err := rt.CallerFor("main")
	if err != nil {
		t.Fatal(err)
	}
	_, err = rt.Invoke(context.Background(), domain.InvocationRequest{
		Caller:  caller,
		Command: "notify.show",
		Input:   "hi",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialNoGrant {
		t.Fatalf("expected no_grant, got %v", err)
	}

	report := mustInspect(t, rt, "main")
	if !strings.Contains(report, "(none)") {
		t.Fatalf("expected empty surface report:\n%s", report)
	}
}

func TestRuntime_GrantThenInvokeAndInspect(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}

	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"read project files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{
			Name:      "fs.read",
			PathScope: &domain.PathScope{Allow: []string{"/project/**"}, Deny: []string{"/project/.secrets/**"}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("project.open", "Open project", "fs.read")
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}

	caller, _ := rt.CallerFor("main")
	res, err := rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		Input:        "/project/README.md",
		ResourcePath: "/project/README.md",
	})
	if err != nil || !res.Authorized {
		t.Fatalf("invoke failed: %+v %v", res, err)
	}

	// Navigation drops authority.
	if err := rt.NavigateWindow(ctx, "main", "https://untrusted.example"); err != nil {
		t.Fatal(err)
	}
	caller, _ = rt.CallerFor("main")
	_, err = rt.Invoke(ctx, domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		ResourcePath: "/project/README.md",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialOriginMismatch {
		t.Fatalf("expected origin denial after navigate, got %v", err)
	}

	// Restore and inspect.
	if err := rt.NavigateWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	report := mustInspect(t, rt, "main")
	if !strings.Contains(report, "project-files") || !strings.Contains(report, "fs.read") {
		t.Fatalf("inspect report missing grants:\n%s", report)
	}
}

func TestRuntime_RequiresAppID(t *testing.T) {
	if _, err := vitra.New(vitra.Config{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRuntime_CloseWindowAndNilExecutor(t *testing.T) {
	rt, err := vitra.New(vitra.Config{AppID: "com.example.demo"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := rt.OpenWindow(ctx, "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("x", "x", "notifications.show")
	if err := rt.RegisterCommand(cmd, nil); err == nil {
		t.Fatal("expected nil executor error")
	}
	// Nil executor must not leave a registered command.
	if err := rt.RegisterCommand(cmd, echoExec{}); err != nil {
		t.Fatal(err)
	}
	if err := rt.CloseWindow(ctx, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.CallerFor("main"); err == nil {
		t.Fatal("closed window should not yield caller")
	}
	if _, err := rt.CallerFor("missing"); err == nil {
		t.Fatal("expected missing window error")
	}
}

func mustInspect(t *testing.T, rt *vitra.Runtime, window domain.WindowID) string {
	t.Helper()
	surface, err := rt.InspectCapabilities(window)
	if err != nil {
		t.Fatal(err)
	}
	return vitra.FormatInspect(rt.AppID(), surface)
}
