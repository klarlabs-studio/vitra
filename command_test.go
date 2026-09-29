package vitra_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
)

type greetIn struct {
	Name  string `json:"name"`
	Times int    `json:"times,omitempty"`
}

type greetOut struct {
	Message string `json:"message"`
}

type readIn struct {
	Path string `json:"path"`
}

func (r readIn) ResourcePath() string { return r.Path }

func typedRuntime(t *testing.T) *vitra.Runtime {
	t.Helper()
	rt, err := vitra.New(vitra.Config{AppID: "com.example.typed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.OpenWindow(context.Background(), "main", domain.OriginPackagedLocal); err != nil {
		t.Fatal(err)
	}
	grant, err := domain.NewCapabilityGrant("g", "d",
		[]domain.WindowID{"main"},
		[]domain.Origin{domain.OriginPackagedLocal},
		[]domain.PermissionSpec{
			{Name: "greet"},
			{Name: "fs.read", PathScope: &domain.PathScope{Allow: []string{"/project/**"}}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RegisterGrant(grant); err != nil {
		t.Fatal(err)
	}
	return rt
}

func invokeAs(t *testing.T, rt *vitra.Runtime, cmd domain.CommandName, input any, resource string) (any, error) {
	t.Helper()
	caller, err := rt.CallerFor("main")
	if err != nil {
		t.Fatal(err)
	}
	res, err := rt.Invoke(context.Background(), domain.InvocationRequest{
		Caller: caller, Command: cmd, Input: input, ResourcePath: resource,
	})
	if err != nil {
		return nil, err
	}
	return res.Output, nil
}

func TestRegister_DecodesTypedInputAndPassesInvocation(t *testing.T) {
	rt := typedRuntime(t)
	var gotWindow domain.WindowID
	err := vitra.Register(rt, vitra.Command[greetIn, greetOut]{
		Name:        "greet",
		Description: "Greet someone",
		Permission:  "greet",
		Handler: func(_ context.Context, inv domain.Invocation, in greetIn) (greetOut, error) {
			gotWindow = inv.Caller.Window
			return greetOut{Message: strings.Repeat("hi "+in.Name+" ", in.Times)}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Frontend input arrives as decoded JSON values.
	out, err := invokeAs(t, rt, "greet", map[string]any{"name": "Klar", "times": float64(2)}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := out.(greetOut).Message; got != "hi Klar hi Klar " || gotWindow != "main" {
		t.Fatalf("out=%q window=%q", got, gotWindow)
	}
}

func TestRegister_RejectsMalformedInput(t *testing.T) {
	rt := typedRuntime(t)
	called := false
	if err := vitra.Register(rt, vitra.Command[greetIn, greetOut]{
		Name: "greet", Permission: "greet",
		Handler: func(context.Context, domain.Invocation, greetIn) (greetOut, error) {
			called = true
			return greetOut{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []any{
		map[string]any{"name": "x", "admin": true}, // unknown field
		map[string]any{"name": 42},                 // wrong type
		"just a string",
	} {
		_, err := invokeAs(t, rt, "greet", input, "")
		if !errors.Is(err, &domain.ErrValidation{}) || called {
			t.Fatalf("input %v: err=%v called=%v", input, err, called)
		}
	}
}

// When the input names the resource it acts on, it must be the resource the
// gateway authorized; otherwise a frontend could get "/project/a" checked
// and then read "/etc/passwd".
func TestRegister_BindsInputResourcePathToAuthorizedPath(t *testing.T) {
	rt := typedRuntime(t)
	var read string
	if err := vitra.Register(rt, vitra.Command[readIn, string]{
		Name: "file.read", Permission: "fs.read",
		Handler: func(_ context.Context, _ domain.Invocation, in readIn) (string, error) {
			read = in.Path
			return "ok", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := invokeAs(t, rt, "file.read", map[string]any{"path": "/project/a"}, "/project/a"); err != nil {
		t.Fatal(err)
	}
	read = ""
	_, err := invokeAs(t, rt, "file.read", map[string]any{"path": "/etc/passwd"}, "/project/a")
	var denied *domain.ErrDenied
	if !errors.As(err, &denied) || read != "" {
		t.Fatalf("mismatched path: err=%v read=%q", err, read)
	}
}

func TestRegister_ValidatesDefinition(t *testing.T) {
	rt := typedRuntime(t)
	h := func(context.Context, domain.Invocation, greetIn) (greetOut, error) { return greetOut{}, nil }
	for _, c := range []vitra.Command[greetIn, greetOut]{
		{Permission: "greet", Handler: h},    // no name
		{Name: "greet", Handler: h},          // no permission
		{Name: "greet", Permission: "greet"}, // no handler
	} {
		if err := vitra.Register(rt, c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}

func TestRuntime_TypeScriptIncludesTypedCommands(t *testing.T) {
	rt := typedRuntime(t)
	if err := vitra.Register(rt, vitra.Command[greetIn, greetOut]{
		Name: "greet", Description: "Greet someone", Permission: "greet",
		Handler: func(context.Context, domain.Invocation, greetIn) (greetOut, error) { return greetOut{}, nil },
	}); err != nil {
		t.Fatal(err)
	}
	ts, err := rt.TypeScript("app")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export interface GreetIn {",
		"  name: string;",
		"  times?: number;",
		"export interface GreetOut {",
		"greet(input: GreetIn, resourcePath?: string): Promise<GreetOut>;",
		"requires permission `greet`",
	} {
		if !strings.Contains(ts, want) {
			t.Errorf("missing %q in:\n%s", want, ts)
		}
	}
}
