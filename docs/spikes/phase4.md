# Phase 4 — Distribution Excellence

Phase 4 makes packaging, updates, and release inspection first-class.

## Delivered

| Area | Package |
|------|---------|
| Package targets + signing identity refs | `packaging` |
| Linux staged app directory | `packaging.StageLinux` + `vitra package` |
| Signed update manifests (ed25519) | `updater` |
| Install plans only after verify | `updater.PlanInstall` |
| Atomic install after plan | `updater.ApplyInstall` / `Runtime.ApplyUpdate` |
| SBOM / plugin / capability inventory | `provenance` |

`vitra package --out dist/` stages `bin/<name>`, a `.desktop` launcher, and
`provenance.json`. Formats:

| `--format` | Output |
|------------|--------|
| `dir` (default) | Staged Linux app directory (`StageLinux`) |
| `deb` | Pure-Go `.deb` (`BuildDeb`) |
| `appdir` | AppImage-ready AppDir (`BuildAppDir`) |
| `appimage` | Final `.AppImage` (`BuildAppImage` → `appimagetool`) |

`BuildAppImage` stages an AppDir then invokes `appimagetool` (or
`VITRA_APPIMAGETOOL`). Without the tool, use `--format appdir` and fold
externally.

## Update apply

```go
plan, err := rt.ApplyUpdate(manifest, pubKey, artifactBytes, destPath)
```

Pipeline: optional `policy.AuthorizeUpdate` (channel + unsigned reject) →
`PlanInstall` (signature + digest) → `ApplyInstall` (re-check digest, atomic
rename into place). No filesystem mutation occurs unless verification succeeds.

CLI:

```bash
vitra update-apply --manifest update.json --artifact app.bin --pubkey <hex> --dest ./vitra-app --policy production
```

## Security invariants

- **9**: `PlanInstall` / `ApplyUpdate` refuse unsigned or digest-mismatched artifacts
- **10**: `packaging.Spec` accepts only `SigningIdentityRef` (e.g. `env:…` /
  keychain refs), never inline private key material

## Non-goals in this PR

Hosted update CDN, notarization API clients, and full installer generators —
those are adapters that consume these contracts.
