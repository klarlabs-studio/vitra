package vitra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"go.klarlabs.de/vitra/bindings"
	"go.klarlabs.de/vitra/domain"
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
// Out is encoded to the frontend as JSON. Register records In and Out so
// Runtime.TypeScript can generate a typed client.
type Command[In, Out any] struct {
	Name        domain.CommandName
	Description string
	Permission  domain.PermissionName
	Handler     func(ctx context.Context, inv domain.Invocation, in In) (Out, error)
}

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
	handler := c.Handler
	exec := domain.CommandExecutorFunc(func(ctx context.Context, name domain.CommandName, input any) (any, error) {
		inv, ok := domain.InvocationFrom(ctx)
		if !ok {
			return nil, domain.ErrNoInvocation
		}
		in, err := decodeInput[In](input)
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
		return handler(ctx, inv, in)
	})
	if err := rt.RegisterCommand(def, exec); err != nil {
		return err
	}
	rt.typedMu.Lock()
	defer rt.typedMu.Unlock()
	if rt.typed == nil {
		rt.typed = map[domain.CommandName]bindings.Command{}
	}
	rt.typed[def.Name()] = bindings.Command{
		Name:        def.Name(),
		Description: def.Description(),
		Permission:  def.Permission(),
		Input:       reflect.TypeFor[In](),
		Output:      reflect.TypeFor[Out](),
	}
	return nil
}

// decodeInput converts the frontend's decoded JSON value into In, rejecting
// unknown object fields and type mismatches.
func decodeInput[In any](input any) (In, error) {
	var in In
	raw, err := json.Marshal(input)
	if err != nil {
		return in, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	return in, nil
}

// TypeScript returns a generated TypeScript client for every registered
// command: commands registered with Register get typed inputs and outputs,
// others are typed as unknown. Plugin events get subscription helpers.
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
