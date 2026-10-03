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

### Event JSON

`JSONLSink` and `FileSink` write one object per line, led by its format version:

```json
{"schema":"1","at":"2026-10-03T12:00:00Z","kind":"command.invoke","window":"main","origin":"vitra://app","action":"notes.save","outcome":"allowed"}
```

- `schema` is `audit.EventSchema`. A custom sink that writes JSON should encode with `audit.MarshalEvent` so its lines match.
- `audit.ParseEvent` reads a line back. It reads every schema written by an earlier release of the same major version and refuses newer ones with `audit.ErrUnsupportedSchema`; `FileSink.List` skips such lines. A line without `schema` (Vitra 0.9 and earlier) is read as schema `"1"`.
- CEF output has no `schema` field: CEF lines are versioned by their own header (`CEF:0|…`).

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
  "schema": "1",
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
- `schema` is the format version (`policy.DocumentSchema`); `Document.Save` and `Encode` always write it. A runtime reads every schema written for an earlier release of the same major version. A newer or unknown schema is refused with `policy.ErrUnsupportedSchema`, before any field is checked, so a policy is never half-applied. Update the app before deploying a policy in a newer schema.
- A document without `schema` (written for Vitra 0.9 and earlier) is read as schema `"1"`. **Deprecated:** support for unversioned documents is removed before 1.0. Add `"schema": "1"`.
- Unknown fields are rejected.
