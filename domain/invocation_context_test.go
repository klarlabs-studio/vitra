package domain_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

func twoWindowService(t *testing.T, exec domain.CommandExecutor) (*domain.InvocationService, *domain.Window, *domain.Window) {
	t.Helper()
	grant, err := domain.NewCapabilityGrant("project-files", "files",
		[]domain.WindowID{"main", "aux"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.read", PathScope: &domain.PathScope{Allow: []string{"/project/**"}}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	cmd, _ := domain.NewCommandDefinition("project.open", "Open", "fs.read")
	main, _ := domain.NewWindow("main", domain.OriginPackagedLocal)
	aux, _ := domain.NewWindow("aux", domain.OriginPackagedLocal)
	svc := &domain.InvocationService{
		Commands:  &memCommands{byName: map[domain.CommandName]*domain.CommandDefinition{cmd.Name(): cmd}},
		Grants:    &memGrants{items: []*domain.CapabilityGrant{grant}},
		Windows:   &memWindows{byID: map[domain.WindowID]*domain.Window{main.ID(): main, aux.ID(): aux}},
		Executors: memExecLookup{"project.open": exec},
	}
	return svc, main, aux
}

// Executors must act as the window that actually invoked them, on the
// resource the gateway actually authorized.
func TestInvocationService_ExecutorSeesAuthorizedInvocation(t *testing.T) {
	var got domain.Invocation
	var ok bool
	svc, _, aux := twoWindowService(t, domain.CommandExecutorFunc(func(ctx context.Context, _ domain.CommandName, _ any) (any, error) {
		got, ok = domain.InvocationFrom(ctx)
		return nil, nil
	}))
	caller, _ := aux.Caller()
	if _, err := svc.Invoke(context.Background(), domain.InvocationRequest{
		Caller: caller, Command: "project.open", ResourcePath: "/project/a",
	}); err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("executor context carries no invocation")
	}
	if got.Caller != caller || got.Command != "project.open" || got.ResourcePath != "/project/a" || got.Grant != "project-files" {
		t.Fatalf("invocation = %+v", got)
	}
}

func TestCallerExecutorFunc_ReceivesInvokingWindow(t *testing.T) {
	var gotWindow domain.WindowID
	svc, _, aux := twoWindowService(t, domain.CallerExecutorFunc(func(_ context.Context, caller domain.Caller, _ any) (any, error) {
		gotWindow = caller.Window
		return nil, nil
	}))
	caller, _ := aux.Caller()
	if _, err := svc.Invoke(context.Background(), domain.InvocationRequest{
		Caller: caller, Command: "project.open", ResourcePath: "/project/a",
	}); err != nil {
		t.Fatal(err)
	}
	if gotWindow != "aux" {
		t.Fatalf("executor ran as %q, want aux", gotWindow)
	}
}

// Called outside the gateway (no authorized invocation), a caller-bound
// executor must refuse rather than guess an identity.
func TestCallerExecutorFunc_FailsClosedWithoutInvocation(t *testing.T) {
	called := false
	exec := domain.CallerExecutorFunc(func(context.Context, domain.Caller, any) (any, error) {
		called = true
		return nil, nil
	})
	_, err := exec.Execute(context.Background(), "project.open", nil)
	if !errors.Is(err, domain.ErrNoInvocation) || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestInvocationFrom_EmptyContext(t *testing.T) {
	if _, ok := domain.InvocationFrom(context.Background()); ok {
		t.Fatal("empty context reported an invocation")
	}
}
