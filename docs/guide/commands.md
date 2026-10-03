# Commands

A command is a Go function the page may call by name, if a grant allows the command's permission.

## Typed commands

```go
type RenameRequest struct {
    Path    string `json:"path"`
    NewName string `json:"newName"`
}

type RenameResult struct {
    Path string `json:"path"`
}

err := vitra.Register(rt, vitra.Command[RenameRequest, RenameResult]{
    Name:        "notes.rename",
    Description: "Rename a note",
    Permission:  "fs.write",
    Handler: func(ctx context.Context, inv domain.Invocation, req RenameRequest) (RenameResult, error) {
        // inv.Caller.Window and inv.Caller.Origin: who called.
        // inv.ResourcePath: the path the gateway authorized.
        ...
    },
})
```

- **Name** is what the page calls. Use a `domain.subject` style such as `notes.rename`.
- **Permission** is what a grant must include. Several commands can share one.
- **Input** is decoded strictly: unknown fields and type mismatches are rejected before your handler runs.
- **Output** is sent back as JSON.

Use `struct{}` for a command with no input, and `vitra.Void` for one that returns nothing: the page receives `null`, and the client types the call as `Promise<void>`.

### Typing a plugin's commands

A plugin contributes command definitions without handlers. `vitra.Bind` attaches a typed handler to one, keeping the plugin's name, description, and permission, with the same strict decoding and client types as `vitra.Register`:

```go
err := vitra.Bind(rt, "notes.rename",
    func(ctx context.Context, inv domain.Invocation, req RenameRequest) (RenameResult, error) {
        ...
    })
```

`app.UseOfficialPlugins` binds every [official command](/reference/plugins#inputs-and-outputs) this way.

### Binding the input to the authorized path

For path-scoped permissions the page passes a `resourcePath` alongside the input, and the gateway checks that path against the grant. If your input type has a `ResourcePath() string` method, its value must equal the authorized path, or the call is refused with `path_out_of_scope`:

```go
func (r RenameRequest) ResourcePath() string { return r.Path }
```

Without this, a page could get one path approved and put another in the input. Implement it on every input that names a file.

### Errors

A handler's error message is sent to the page with the code `error`. Denials from the gateway carry their [denial code](/reference/denials). Don't put secrets in error messages.

## The TypeScript client

```bash
vitra generate typescript --app . --out frontend/vitra-client.ts
```

This builds and runs your app in generation mode (it sets `VITRA_GENERATE_TYPESCRIPT` and exits before opening a window), so the client covers exactly the commands your app registers:

```ts
export interface RenameRequest {
  path: string;
  newName: string;
}

export interface VitraClient {
  /** Rename a note — requires permission `fs.write` */
  notesRename(input: RenameRequest, resourcePath?: string): Promise<RenameResult>;
}
```

```ts
import { createClient } from "./vitra-client";

const client = createClient(window.vitra.invoke);
await client.notesRename({ path, newName }, path);
```

Types follow `encoding/json`: `json` tags, `omitempty` makes a field optional, pointers and slices may be `null`. An input whose fields are all optional may be left out.

Without `--app`, `vitra generate typescript` writes a client for the official plugins only, typed as `app.UseOfficialPlugins` binds them. Commands registered with `Runtime.RegisterCommand` or bound with `Runtime.BindExecutor` are typed `unknown`.

## Untyped commands

`Runtime.RegisterCommand` takes a `domain.CommandDefinition` and any `domain.CommandExecutor`. The input arrives as decoded JSON (`map[string]any`, `string`, `float64`, …). Use `domain.CallerExecutorFunc` to receive the calling window:

```go
def, _ := domain.NewCommandDefinition("app.ping", "Ping", "app.ping")
rt.RegisterCommand(def, domain.CallerExecutorFunc(
    func(ctx context.Context, caller domain.Caller, input any) (any, error) {
        return "pong from " + string(caller.Window), nil
    }))
```

Prefer typed commands: strict decoding and the path binding are easy to forget by hand.

## Acting on behalf of the caller

Handlers that call [desktop services](/guide/desktop) pass `inv.Caller`, so the service checks the same window's grants again, including for the real target of a symlink:

```go
files := &desktop.FileService{Gateway: rt}
b, err := files.Read(ctx, inv.Caller, req.Path)
```
