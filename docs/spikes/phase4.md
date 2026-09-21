# Phase 4 — Distribution Excellence

Phase 4 makes packaging, updates, and release inspection first-class.

## Delivered

| Area | Package |
|------|---------|
| Package targets + signing identity refs | `packaging` |
| Linux staged app directory | `packaging.StageLinux` + `vitra package` |
| Windows staged dir + WiX/NSIS scripts | `StageWindows` / `BuildWiXDir` / `BuildNSISDir` |
| Darwin staged `.app` bundle | `StageDarwinApp` + `vitra package --format app-dir` |
| Darwin DMG fold | `BuildDMG` / `FoldDMG` (`hdiutil` or `VITRA_HDIUTIL`) |
| Signed update manifests (ed25519) | `updater` |
| Install plans only after verify | `updater.PlanInstall` |
| Atomic install after plan | `updater.ApplyInstall` / `Runtime.ApplyUpdate` |
| SBOM / plugin / capability inventory | `provenance` |

`vitra package --out dist/` stages a platform layout and `provenance.json`. Formats:

| `--format` | Output |
|------------|--------|
| `dir` (default) | Staged Linux app directory (`StageLinux`) |
| `deb` | Pure-Go `.deb` (`BuildDeb`) |
| `appdir` | AppImage-ready AppDir (`BuildAppDir`) |
| `appimage` | Final `.AppImage` (`BuildAppImage` → `appimagetool`) |
| `win-dir` | Staged Windows `bin/<Name>.exe` (`StageWindows`) |
| `wix` | Windows stage + `product.wxs` (`BuildWiXDir`) |
| `nsis-dir` | Windows stage + `installer.nsi` (`BuildNSISDir`; WriteUninstaller + ARP) |
| `msi` | Final `.msi` (`BuildMSI` → candle/light or `VITRA_CANDLE`/`VITRA_LIGHT`) |
| `nsis` | Final setup `.exe` (`BuildNSIS` → makensis or `VITRA_MAKENSIS`) |
| `app-dir` | Staged Darwin `.app` bundle (`StageDarwinApp`) |
| `dmg` | Final `.dmg` (`BuildDMG` → hdiutil or `VITRA_HDIUTIL`; includes Applications symlink) |

Optional `--icon <path>` copies a `.png` / `.svg` / `.icns` / `.ico` / `.xpm` into
Linux stages (next to the `.desktop`, `Icon=<name>`), `.deb` packages
(`usr/share/pixmaps/` + `Icon=`), Darwin `Contents/Resources`
(`CFBundleIconFile`), and Windows `bin/` (WiX `ARPPRODUCTICON` + NSIS shortcut
icon).

`BuildAppImage` stages an AppDir then invokes `appimagetool` (or
`VITRA_APPIMAGETOOL`). Without the tool, use `--format appdir` and fold
externally. `BuildMSI` / `BuildNSIS` likewise stage then fold; without WiX/NSIS
tools, use `--format wix` / `nsis-dir` and fold on a Windows host. `BuildDMG`
stages a `.app` then folds with `hdiutil`; without it, use `--format app-dir`
and fold on macOS.

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

## Honest remaining gaps

Still out of scope on main (do not claim otherwise):

- Hosted update CDN / auto-update channel hosting
- Notarization / codesign / Authenticode *execution* (`Spec.Sign` +
  `SigningIdentityRef` are validated refs only; stage/fold never invoke
  `codesign` / `signtool` / notary)
- Extra Linux targets (`.rpm`, Flatpak, Snap)
- Wails-class multi-framework project templates

Installer **generators** (stage scripts + fold to `.deb` / `.AppImage` /
`.msi` / NSIS setup / `.dmg`) are delivered; fold still needs host tools
(`appimagetool`, candle/light, makensis, `hdiutil`) or env overrides.
