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

### Sinks

- `audit.JSONLSink{W}`: one JSON object per line, for files, stdout, or a log shipper.
- `audit.CEFSink{W}`: ArcSight Common Event Format, for SIEMs.
- `audit.MultiSink{Sinks}`: several at once.
- `audit.MemorySink`: in memory, for tests.
- Your own: implement `Append(audit.Event) error` and `List() []audit.Event`.

Sinks are called synchronously, sometimes from the UI thread. Keep `Append` fast and hand slow work to a goroutine.

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
