# Events

Go can push events to windows. The page listens with `window.vitra.on`.

```go
// Deliver "sync.progress" to the main window.
if _, err := rt.SubscribeEvent("main-sync", "sync.progress", "main"); err != nil {
    return err
}

// Later, from anywhere in Go:
_ = a.Emit(ctx, "sync.progress", map[string]any{"done": 3, "total": 10})
```

```ts
const off = window.vitra.on("sync.progress", (p) => {
  const { done, total } = p as { done: number; total: number };
  progress.value = done / total;
});
// off() to stop listening
```

- A subscription names one event and one window. Only subscribed, open windows receive the event.
- Re-subscribing with the same ID replaces the subscription, so it is safe to repeat.
- The window must be open when you subscribe. Subscribing from a command the page calls on startup guarantees that; the [notes example](/guide/examples#notes) subscribes its window to the live audit log this way:

```go
vitra.Register(rt, vitra.Command[struct{}, []audit.Event]{
    Name: "audit.follow", Permission: "audit.read",
    Handler: func(_ context.Context, inv domain.Invocation, _ struct{}) ([]audit.Event, error) {
        id := domain.SubscriptionID("audit-" + string(inv.Caller.Window))
        if _, err := rt.SubscribeEvent(id, "audit.event", inv.Caller.Window); err != nil {
            return nil, err
        }
        return log.List(), nil
    },
})
```

## Built-in events

Some [official plugins](/reference/plugins) emit events: `menu.action`, `tray.action`, `shortcut.action`, `dragdrop.drop`, `deeplink.open`, and `fs.changed`. Subscribe a window to the ones it should see.

## Payloads

Payloads are encoded as JSON. Don't send more than the window needs: events go to whatever page is loaded in the window.
