package inmemory_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/inmemory"
)

func TestRepos_RoundTrip(t *testing.T) {
	grants := inmemory.NewGrantRepo()
	g, err := domain.NewCapabilityGrant(
		"g1", "d",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{{Name: "fs.read"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grants.Save(g); err != nil {
		t.Fatal(err)
	}
	got, err := grants.Get("g1")
	if err != nil || got.Name() != "g1" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	list, err := grants.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if _, err := grants.Get("missing"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatalf("expected not found, got %v", err)
	}

	cmds := inmemory.NewCommandRepo()
	cmd, _ := domain.NewCommandDefinition("c1", "d", "fs.read")
	if err := cmds.Save(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmds.Save(cmd); err == nil {
		t.Fatal("expected conflict on duplicate command")
	}
	gotCmd, err := cmds.Get("c1")
	if err != nil || gotCmd.Name() != "c1" {
		t.Fatalf("get cmd: %v %v", gotCmd, err)
	}
	if _, err := cmds.Get("nope"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatal(err)
	}
	cmdList, err := cmds.List()
	if err != nil || len(cmdList) != 1 {
		t.Fatalf("list cmds: %v %v", cmdList, err)
	}

	windows := inmemory.NewWindowRepo()
	win, _ := domain.NewWindow("main", domain.OriginPackagedLocal)
	if err := windows.Save(win); err != nil {
		t.Fatal(err)
	}
	gotWin, err := windows.Get("main")
	if err != nil || gotWin.ID() != "main" {
		t.Fatal(err)
	}
	winList, err := windows.List()
	if err != nil || len(winList) != 1 {
		t.Fatal(err)
	}
	if err := windows.Delete("main"); err != nil {
		t.Fatal(err)
	}
	if _, err := windows.Get("main"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatal(err)
	}

	resources := inmemory.NewResourceRepo()
	h, _ := domain.NewResourceHandle("r1", "file", "main")
	h2, _ := domain.NewResourceHandle("r2", "file", "other")
	_ = resources.Save(h)
	_ = resources.Save(h2)
	gotH, err := resources.Get("r1")
	if err != nil || gotH.ID() != "r1" {
		t.Fatal(err)
	}
	owned, err := resources.ListByOwner("main")
	if err != nil || len(owned) != 1 {
		t.Fatalf("owned=%v err=%v", owned, err)
	}
	if err := resources.Delete("r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := resources.Get("r1"); !errors.Is(err, &domain.ErrNotFound{}) {
		t.Fatal(err)
	}

	execs := inmemory.NewExecutorRegistry()
	fn := commandFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		return "ok", nil
	})
	if err := execs.Register("c1", fn); err != nil {
		t.Fatal(err)
	}
	if err := execs.Register("c1", fn); err == nil {
		t.Fatal("expected conflict")
	}
	e, ok := execs.Get("c1")
	if !ok || e == nil {
		t.Fatal("missing executor")
	}
	if _, ok := execs.Get("missing"); ok {
		t.Fatal("expected miss")
	}
}

type commandFunc func(ctx context.Context, name domain.CommandName, input any) (any, error)

func (f commandFunc) Execute(ctx context.Context, name domain.CommandName, input any) (any, error) {
	return f(ctx, name, input)
}
