package domain_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
)

type memCommands struct {
	byName map[domain.CommandName]*domain.CommandDefinition
}

func (m *memCommands) Save(cmd *domain.CommandDefinition) error {
	m.byName[cmd.Name()] = cmd
	return nil
}
func (m *memCommands) Get(name domain.CommandName) (*domain.CommandDefinition, error) {
	c, ok := m.byName[name]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "command", ID: string(name)}
	}
	return c, nil
}
func (m *memCommands) List() ([]*domain.CommandDefinition, error) {
	out := make([]*domain.CommandDefinition, 0, len(m.byName))
	for _, c := range m.byName {
		out = append(out, c)
	}
	return out, nil
}

type memGrants struct {
	items []*domain.CapabilityGrant
}

func (m *memGrants) Save(g *domain.CapabilityGrant) error {
	m.items = append(m.items, g)
	return nil
}
func (m *memGrants) Get(name domain.GrantName) (*domain.CapabilityGrant, error) {
	for _, g := range m.items {
		if g.Name() == name {
			return g, nil
		}
	}
	return nil, &domain.ErrNotFound{Entity: "grant", ID: string(name)}
}
func (m *memGrants) List() ([]*domain.CapabilityGrant, error) { return m.items, nil }

type memWindows struct {
	byID map[domain.WindowID]*domain.Window
}

func (m *memWindows) Save(w *domain.Window) error {
	m.byID[w.ID()] = w
	return nil
}
func (m *memWindows) Get(id domain.WindowID) (*domain.Window, error) {
	w, ok := m.byID[id]
	if !ok {
		return nil, &domain.ErrNotFound{Entity: "window", ID: string(id)}
	}
	return w, nil
}
func (m *memWindows) Delete(id domain.WindowID) error {
	delete(m.byID, id)
	return nil
}
func (m *memWindows) List() ([]*domain.Window, error) {
	out := make([]*domain.Window, 0, len(m.byID))
	for _, w := range m.byID {
		out = append(out, w)
	}
	return out, nil
}

type memExec struct {
	fn func(ctx context.Context, name domain.CommandName, input any) (any, error)
}

func (m memExec) Execute(ctx context.Context, name domain.CommandName, input any) (any, error) {
	return m.fn(ctx, name, input)
}

type memExecLookup map[domain.CommandName]domain.CommandExecutor

func (m memExecLookup) Get(name domain.CommandName) (domain.CommandExecutor, bool) {
	e, ok := m[name]
	return e, ok
}

func TestInvocationService_AuthorizeThenExecute(t *testing.T) {
	grant := mustGrant(t)
	cmd, err := domain.NewCommandDefinition("project.open", "Open project", "fs.read")
	if err != nil {
		t.Fatal(err)
	}
	win, _ := domain.NewWindow("main", domain.OriginPackagedLocal)

	svc := &domain.InvocationService{
		Commands: &memCommands{byName: map[domain.CommandName]*domain.CommandDefinition{cmd.Name(): cmd}},
		Grants:   &memGrants{items: []*domain.CapabilityGrant{grant}},
		Windows:  &memWindows{byID: map[domain.WindowID]*domain.Window{win.ID(): win}},
		Executors: memExecLookup{"project.open": memExec{fn: func(ctx context.Context, name domain.CommandName, input any) (any, error) {
			return map[string]any{"opened": input}, nil
		}}},
	}

	caller, _ := win.Caller()
	res, err := svc.Invoke(context.Background(), domain.InvocationRequest{
		Caller:       caller,
		Command:      "project.open",
		Input:        "/project/app",
		ResourcePath: "/project/app",
	})
	if err != nil || !res.Authorized {
		t.Fatalf("invoke: res=%+v err=%v", res, err)
	}
}

func TestInvocationService_DeniesWithoutGrant(t *testing.T) {
	cmd, _ := domain.NewCommandDefinition("project.open", "Open project", "fs.read")
	win, _ := domain.NewWindow("main", domain.OriginPackagedLocal)
	svc := &domain.InvocationService{
		Commands:  &memCommands{byName: map[domain.CommandName]*domain.CommandDefinition{cmd.Name(): cmd}},
		Grants:    &memGrants{},
		Windows:   &memWindows{byID: map[domain.WindowID]*domain.Window{win.ID(): win}},
		Executors: memExecLookup{},
	}
	caller, _ := win.Caller()
	_, err := svc.Invoke(context.Background(), domain.InvocationRequest{
		Caller:  caller,
		Command: "project.open",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialNoGrant {
		t.Fatalf("expected denial, got %v", err)
	}
}

func TestInvocationService_RejectsSpoofedOrigin(t *testing.T) {
	// Security invariant 4: IPC sender identity is validated at the native boundary.
	grant := mustGrant(t)
	cmd, _ := domain.NewCommandDefinition("project.open", "Open", "fs.read")
	win, _ := domain.NewWindow("main", domain.OriginPackagedLocal)
	svc := &domain.InvocationService{
		Commands:  &memCommands{byName: map[domain.CommandName]*domain.CommandDefinition{cmd.Name(): cmd}},
		Grants:    &memGrants{items: []*domain.CapabilityGrant{grant}},
		Windows:   &memWindows{byID: map[domain.WindowID]*domain.Window{win.ID(): win}},
		Executors: memExecLookup{},
	}
	spoofed, _ := domain.NewCaller("main", "https://evil.example")
	_, err := svc.Invoke(context.Background(), domain.InvocationRequest{
		Caller:       spoofed,
		Command:      "project.open",
		ResourcePath: "/project/a.go",
	})
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || denied.Code != domain.DenialOriginMismatch {
		t.Fatalf("expected spoof denial, got %v", err)
	}
}

func TestCommandDefinition_RequiresPermission(t *testing.T) {
	_, err := domain.NewCommandDefinition("x", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	cmd, err := domain.NewCommandDefinition("project.open", "Open", "fs.read")
	if err != nil {
		t.Fatal(err)
	}
	cmd.WithPlugin("fs")
	if cmd.Plugin() != "fs" || cmd.Permission() != "fs.read" {
		t.Fatalf("bad cmd %+v", cmd)
	}
}
