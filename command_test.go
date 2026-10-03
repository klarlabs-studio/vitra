package vitra_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/vitra"
	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/plugin"
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

// greetPlugin contributes the greet command without an executor, as
// official and third-party plugins do.
type greetPlugin struct{}

func (greetPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{ID: "example.greet", Name: "Greet", Version: plugin.SemVer{Major: 1},
		Permissions: []domain.PermissionName{"greet"}}
}

func (greetPlugin) Contribute() (plugin.Contribution, error) {
	cmd, err := domain.NewCommandDefinition("greet", "Greet someone", "greet")
	if err != nil {
		return plugin.Contribution{}, err
	}
	cmd.WithPlugin("example.greet")
	return plugin.Contribution{Commands: []*domain.CommandDefinition{cmd}}, nil
}

func pluginRuntime(t *testing.T) *vitra.Runtime {
	t.Helper()
	rt := typedRuntime(t)
	if err := rt.RegisterPlugin(context.Background(), greetPlugin{}); err != nil {
		t.Fatal(err)
	}
	return rt
}

// Bind gives a plugin-contributed command a typed handler: strict decoding,
// the authorized invocation, and a typed client.
func TestBind_TypesAPluginCommand(t *testing.T) {
	rt := pluginRuntime(t)
	var gotWindow domain.WindowID
	if err := vitra.Bind(rt, "greet", func(_ context.Context, inv domain.Invocation, in greetIn) (greetOut, error) {
		gotWindow = inv.Caller.Window
		return greetOut{Message: "hi " + in.Name}, nil
	}); err != nil {
		t.Fatal(err)
	}
	out, err := invokeAs(t, rt, "greet", map[string]any{"name": "Ada"}, "")
	if got, _ := out.(greetOut); err != nil || got.Message != "hi Ada" || gotWindow != "main" {
		t.Fatalf("greet = %v, %v (window %q)", out, err, gotWindow)
	}
	if _, err := invokeAs(t, rt, "greet", map[string]any{"name": "Ada", "admin": true}, ""); !errors.Is(err, &domain.ErrValidation{}) {
		t.Fatalf("unknown field: %v, want a validation error", err)
	}
	ts, err := rt.TypeScript("app")
	if err != nil {
		t.Fatal(err)
	}
	if want := "greet(input: GreetIn, resourcePath?: string): Promise<GreetOut>;"; !strings.Contains(ts, want) {
		t.Fatalf("missing %q in:\n%s", want, ts)
	}
	// The plugin's description and permission are kept.
	if !strings.Contains(ts, "/** Greet someone — requires permission `greet` */") {
		t.Fatalf("plugin definition not kept:\n%s", ts)
	}
}

func TestBind_RequiresARegisteredCommandAndHandler(t *testing.T) {
	rt := pluginRuntime(t)
	h := func(context.Context, domain.Invocation, greetIn) (greetOut, error) { return greetOut{}, nil }
	var nf *domain.ErrNotFound
	if err := vitra.Bind(rt, "missing", h); !errors.As(err, &nf) {
		t.Fatalf("Bind on an unknown command: %v, want not found", err)
	}
	if err := vitra.Bind[greetIn, greetOut](rt, "greet", nil); err == nil {
		t.Fatal("Bind accepted a nil handler")
	}
	if _, err := invokeAs(t, rt, "greet", nil, ""); err == nil {
		t.Fatal("a failed Bind left an executor behind")
	}
}

// A Void command returns nothing: the frontend gets null, and the generated
// client says Promise<void>.
func TestVoid_IsNullOnTheWireAndVoidInTheClient(t *testing.T) {
	rt := typedRuntime(t)
	called := false
	if err := vitra.Register(rt, vitra.Command[struct{}, vitra.Void]{
		Name: "ping", Permission: "greet",
		Handler: func(context.Context, domain.Invocation, struct{}) (vitra.Void, error) {
			called = true
			return vitra.Void{}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	out, err := invokeAs(t, rt, "ping", nil, "")
	if err != nil || out != nil || !called {
		t.Fatalf("ping = %#v, %v (called %v); want nil output", out, err, called)
	}
	if b, err := json.Marshal(vitra.Void{}); err != nil || string(b) != "null" {
		t.Fatalf("Void encodes as %s, %v", b, err)
	}
	ts, err := rt.TypeScript("app")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ping(input?: {}, resourcePath?: string): Promise<void>;"; !strings.Contains(ts, want) {
		t.Fatalf("missing %q in:\n%s", want, ts)
	}
}
