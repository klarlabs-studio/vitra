package application_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/application"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/inmemory"
)

func TestOpenWindow_LeastPrivilege(t *testing.T) {
	windows := inmemory.NewWindowRepo()
	grants := inmemory.NewGrantRepo()
	open := &application.OpenWindowUseCase{Windows: windows}
	inspect := &application.InspectCapabilitiesUseCase{Grants: grants, Windows: windows}

	win, err := open.Execute("main", domain.OriginPackagedLocal)
	if err != nil {
		t.Fatal(err)
	}
	surface, err := inspect.Execute(win.ID())
	if err != nil {
		t.Fatal(err)
	}
	if len(surface.Permissions) != 0 {
		t.Fatalf("new window must have empty surface: %+v", surface)
	}
}

func TestCloseWindow_ReleasesResources(t *testing.T) {
	windows := inmemory.NewWindowRepo()
	resources := inmemory.NewResourceRepo()
	open := &application.OpenWindowUseCase{Windows: windows}
	closeUC := &application.CloseWindowUseCase{Windows: windows, Resources: resources}

	if _, err := open.Execute("main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	h, err := domain.NewResourceHandle("r1", "stream", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := resources.Save(h); err != nil {
		t.Fatal(err)
	}
	if err := closeUC.Execute("main"); err != nil {
		t.Fatal(err)
	}
	if !h.IsClosed() {
		t.Fatal("resource must be closed")
	}
	if _, err := resources.Get("r1"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatalf("resource must be deleted: %v", err)
	}
}

func TestRegisterAndNavigateValidation(t *testing.T) {
	windows := inmemory.NewWindowRepo()
	grants := inmemory.NewGrantRepo()
	commands := inmemory.NewCommandRepo()

	if err := (&application.RegisterGrantUseCase{Grants: grants}).Execute(nil); err == nil {
		t.Fatal("expected nil grant error")
	}
	if err := (&application.RegisterCommandUseCase{Commands: commands}).Execute(nil); err == nil {
		t.Fatal("expected nil command error")
	}

	open := &application.OpenWindowUseCase{Windows: windows}
	if _, err := open.Execute("main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	if _, err := open.Execute("main", domain.OriginPackagedLocal); err == nil {
		t.Fatal("expected conflict")
	}
	nav := &application.NavigateWindowUseCase{Windows: windows}
	if err := nav.Execute("main", "https://trusted.example"); err != nil {
		t.Fatal(err)
	}
	win, _ := windows.Get("main")
	if win.Origin() != "https://trusted.example" {
		t.Fatalf("origin=%s", win.Origin())
	}
	inspect := &application.InspectCapabilitiesUseCase{Grants: grants, Windows: windows}
	if _, err := inspect.Execute("missing"); err == nil {
		t.Fatal("expected missing window")
	}
}

func TestInvokeCommand_EndToEnd(t *testing.T) {
	windows := inmemory.NewWindowRepo()
	grants := inmemory.NewGrantRepo()
	commands := inmemory.NewCommandRepo()
	execs := inmemory.NewExecutorRegistry()

	open := &application.OpenWindowUseCase{Windows: windows}
	if _, err := open.Execute("main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}

	grant, err := domain.NewCapabilityGrant(
		"project-files",
		"files",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{
			Name:      "fs.read",
			PathScope: &domain.PathScope{Allow: []string{"/project/**"}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&application.RegisterGrantUseCase{Grants: grants}).Execute(grant); err != nil {
		t.Fatal(err)
	}

	cmd, err := domain.NewCommandDefinition("project.open", "Open", "fs.read")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&application.RegisterCommandUseCase{Commands: commands}).Execute(cmd); err != nil {
		t.Fatal(err)
	}
	_ = execs.Register("project.open", commandFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		return "ok:" + input.(string), nil
	}))

	invoker := &domain.InvocationService{
		Commands:  commands,
		Grants:    grants,
		Windows:   windows,
		Executors: execs,
	}
	uc := &application.InvokeCommandUseCase{Invoker: invoker}
	win, _ := windows.Get("main")
	caller, _ := win.Caller()
	res, err := uc.Execute(context.Background(), domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		Input:        "/project/app",
		ResourcePath: "/project/app",
	})
	if err != nil || res.Output != "ok:/project/app" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

type commandFunc func(ctx context.Context, name domain.CommandName, input any) (any, error)

func (f commandFunc) Execute(ctx context.Context, name domain.CommandName, input any) (any, error) {
	return f(ctx, name, input)
}
