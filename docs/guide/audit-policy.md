# Audit and policy

## Audit

Install a sink and Vitra records every security-relevant event:

```go
f, _ := os.OpenFile("audit.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
rt.SetAudit(&audit.JSONLSink{W: f})
```

| Kind | When |
|---|---|
| `command.invoke` | every call; outcome `allowed`, `denied`, or `error`. `Metadata` has `resource_path` and, for denials, `code` |
| `capability.decision` | a permission check, including re-checks by desktop services |
| `bridge.reject` | a message dropped at the bridge: no sender token, unparseable, invalid, or (macOS) from a subframe |
| `navigation.block` | a refused navigation; the URL is logged without credentials, query, or fragment |
| `policy.override` | enterprise policy turned an allow into a deny |
| `plugin.register`, `worker.lifecycle`, `update.plan` | plugin registration, worker state changes, update decisions |
| `audit.dropped` | a sink lost events before writing them (`FileSink` with a full queue); outcome `error`, `Metadata.count` is how many |

### Sinks

- `audit.NewFileSink(path, maxBytes, maxBackups)`: JSON lines to a file that rotates before it would exceed `maxBytes`, keeping `maxBackups` old files (`audit.jsonl.1` newest … `.N` oldest). See below.
- `audit.JSONLSink{W}`: one JSON object per line, for files, stdout, or a log shipper.
- `audit.CEFSink{W}`: ArcSight Common Event Format, for SIEMs.
- `audit.MultiSink{Sinks}`: several at once.
- `audit.MemorySink`: in memory, for tests.
- Your own: implement `Append(audit.Event) error` and `List() []audit.Event`.

Sinks are called synchronously, sometimes from the UI thread. Keep `Append` fast and hand slow work to a goroutine.

### Rotating file sink

```go
sink, err := audit.NewFileSink(filepath.Join(dataDir, "audit.jsonl"), 10<<20, 5) // 10 MiB, 5 backups
if err != nil {
	return err
}
defer sink.Close() // writes queued events
rt.SetAudit(sink)
```

- Files are created with mode `0600`; an existing file is tightened to `0600` and appended to.
- `Append` never waits for the disk. It queues the encoded line for a background writer and returns `audit.ErrSinkFull` if the queue is full (a stalled disk); `Dropped()` counts those events.
- Dropped events leave a visible gap in the file: before its next write, the writer records an `audit.dropped` event with `Metadata.count` set to the number lost since the last marker. `Dropped()` stays the running total.
- Write errors are returned by `Flush` and `Close`.
- `List` reads back the events still on disk, oldest first.
- With `maxBackups` 0 the file is truncated on rotation.

## Enterprise policy

A policy document lets an administrator restrict an app without rebuilding it:

```json
{
  "deny_permissions": ["clipboard.read", "path.open"],
  "allowed_update_channels": ["stable"],
  "require_update_signature": true,
  "disable_dev_privileges": true
}
```

```go
doc, err := policy.LoadDocument("/etc/notes/policy.json")
eng, err := policy.NewEngine(doc, policy.EnvProduction)
rt.SetPolicy(eng)
```

- Policy only **tightens**: it can turn an allow into a deny, never the reverse. Each override is audited as `policy.override`.
- In `production`, update signatures are always required.
- The document is plain JSON, so MDM tools can deploy it.
