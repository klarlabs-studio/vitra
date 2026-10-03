package vitra

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"go.klarlabs.de/vitra/domain"
	"go.klarlabs.de/vitra/internal/bindings"
	"go.klarlabs.de/vitra/internal/strictjson"
)

// Command is a typed frontend-callable command.
//
// The frontend reaches Handler only if the calling window holds Permission
// under a registered grant. Input is decoded strictly into In: unknown fields
// and mismatched types are rejected before Handler runs. Handler receives the
// authorized invocation (the calling window and origin, and the resource path
// the gateway checked). If In has a ResourcePath() string method, its value
// must equal that checked path, so a handler cannot be pointed at a
// different resource than the one that was authorized.
//
// Out is encoded to the frontend as JSON; use [Void] for a command that
// returns nothing. Register records In and Out so Runtime.TypeScript can
// generate a typed client.
type Command[In, Out any] struct {
	Name        domain.CommandName
	Description string
	Permission  domain.PermissionName
	Handler     func(ctx context.Context, inv domain.Invocation, in In) (Out, error)
}

// Void is the output type of a command that returns nothing. The frontend
// receives null, and the generated client types the call as Promise<void>.
type Void struct{}

// MarshalJSON encodes Void as null.
func (Void) MarshalJSON() ([]byte, error) { return []byte("null"), nil }

var voidType = reflect.TypeFor[Void]()

// ResourcePather is implemented by command inputs that name the resource the
// command acts on. See Command.
type ResourcePather interface {
	ResourcePath() string
}

// Register validates c and registers it on rt as an explicit command.
func Register[In, Out any](rt *Runtime, c Command[In, Out]) error {
	if c.Handler == nil {
		return &domain.ErrValidation{Message: "command handler is required"}
	}
	def, err := domain.NewCommandDefinition(c.Name, c.Description, c.Permission)
	if err != nil {
		return err
	}
	if err := rt.RegisterCommand(def, typedExecutor(def, c.Handler)); err != nil {
		return err
	}
	rt.recordTyped(def, reflect.TypeFor[In](), reflect.TypeFor[Out]())
	return nil
}

// Bind attaches a typed handler to a command that is already registered,
// typically one a plugin contributed through Runtime.RegisterPlugin. It is
// the typed counterpart of Runtime.BindExecutor: the command keeps the name,
// description, and permission its plugin declared, its input is decoded
// strictly into In as for [Command], and Runtime.TypeScript types it with In
// and Out instead of unknown.
func Bind[In, Out any](rt *Runtime, name domain.CommandName, handler func(ctx context.Context, inv domain.Invocation, in In) (Out, error)) error {
	if handler == nil {
		return &domain.ErrValidation{Message: "command handler is required"}
	}
	def, err := rt.commands.Get(name)
	if err != nil {
		return err
	}
	if err := rt.BindExecutor(name, typedExecutor(def, handler)); err != nil {
		return err
	}
	rt.recordTyped(def, reflect.TypeFor[In](), reflect.TypeFor[Out]())
	return nil
}

// typedExecutor wraps handler so it runs only for an authorized invocation,
// on strictly decoded input bound to the authorized resource path.
func typedExecutor[In, Out any](def *domain.CommandDefinition, handler func(context.Context, domain.Invocation, In) (Out, error)) domain.CommandExecutor {
	return domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		inv, ok := domain.InvocationFrom(ctx)
		if !ok {
			return nil, domain.ErrNoInvocation
		}
		in, err := strictjson.Decode[In](input)
		if err != nil {
			return nil, &domain.ErrValidation{Message: fmt.Sprintf("command %s input: %v", name, err)}
		}
		if rp, ok := any(in).(ResourcePather); ok && rp.ResourcePath() != inv.ResourcePath {
			return nil, &domain.ErrDenied{
				Permission: def.Permission(),
				Window:     inv.Caller.Window,
				Origin:     inv.Caller.Origin,
				Code:       domain.DenialPathOutOfScope,
				Reason:     "input resource path differs from the authorized resource path",
			}
		}
		out, err := handler(ctx, inv, in)
		if _, void := any(out).(Void); void {
			return nil, err
		}
		return out, err
	})
}

// recordTyped remembers a command's input and output types for TypeScript.
func (rt *Runtime) recordTyped(def *domain.CommandDefinition, in, out reflect.Type) {
	cmd := bindings.Command{
		Name:        def.Name(),
		Description: def.Description(),
		Permission:  def.Permission(),
		Input:       in,
		Output:      out,
	}
	if out == voidType {
		cmd.Output, cmd.Void = nil, true
	}
	rt.typedMu.Lock()
	defer rt.typedMu.Unlock()
	if rt.typed == nil {
		rt.typed = map[domain.CommandName]bindings.Command{}
	}
	rt.typed[def.Name()] = cmd
}

// TypeScript returns a generated TypeScript client for every registered
// command: commands registered with Register or bound with Bind get typed
// inputs and outputs, others are typed as unknown. Plugin events get
// subscription helpers.
func (rt *Runtime) TypeScript(module string) (string, error) {
	defs, err := rt.commands.List()
	if err != nil {
		return "", err
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name() < defs[j].Name() })
	rt.typedMu.Lock()
	cmds := make([]bindings.Command, 0, len(defs))
	for _, d := range defs {
		if c, ok := rt.typed[d.Name()]; ok {
			cmds = append(cmds, c)
			continue
		}
		cmds = append(cmds, bindings.Untyped(d)...)
	}
	rt.typedMu.Unlock()
	var events []domain.EventName
	for _, reg := range rt.plugins.List() {
		events = append(events, reg.Contribution.Events...)
	}
	return bindings.GenerateTypeScript(module, Version, cmds, events)
}
