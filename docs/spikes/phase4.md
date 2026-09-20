# Phase 4 — Distribution Excellence

Phase 4 makes packaging, updates, and release inspection first-class.

## Delivered

| Area | Package |
|------|---------|
| Package targets + signing identity refs | `packaging` |
| Linux staged app directory | `packaging.StageLinux` + `vitra package` |
| Signed update manifests (ed25519) | `updater` |
| Install plans only after verify | `updater.PlanInstall` |
| SBOM / plugin / capability inventory | `provenance` |

`vitra package --out dist/` stages `bin/<name>`, a `.desktop` launcher, and
`provenance.json` (artifact digest + official plugin inventory). AppImage/deb
generators remain future adapters on top of `TargetLinuxDir`.

## Security invariants

- **9**: `PlanInstall` refuses unsigned or digest-mismatched artifacts
- **10**: `packaging.Spec` accepts only `SigningIdentityRef` (e.g. `env:…` /
  keychain refs), never inline private key material

## Non-goals in this PR

Hosted update CDN, notarization API clients, and full installer generators —
those are adapters that consume these contracts.
