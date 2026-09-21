# Phase 4 — Distribution Excellence

Phase 4 makes packaging, updates, and release inspection first-class.

## Delivered

| Area | Package |
|------|---------|
| Package targets + signing identity refs | `packaging` |
| Sign dry-run plan (no host execution) | `packaging.PlanSign` + `vitra package --sign` |
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
| `rpm-dir` | rpmbuild `_topdir` + `SPECS/*.spec` (`BuildRPMDir`) |
| `rpm` | Final `.rpm` (`BuildRPM` → `rpmbuild` or `VITRA_RPMBUILD`) |
| `snap-dir` | Snap prime dir + `meta/snap.yaml` (`BuildSnapDir`) |
| `snap` | Final `.snap` (`BuildSnap` → `snapcraft pack` or `VITRA_SNAPCRAFT`) |
| `flatpak-dir` | Flatpak stage (`files/` + `metadata` + `manifest.yml`; `BuildFlatpakDir`) |
| `flatpak` | Final `.flatpak` (`BuildFlatpak` → `VITRA_FLATPAK_BUILDER` / `flatpak-builder`) |
| `appdir` | AppImage-ready AppDir (`BuildAppDir`) |
| `appimage` | Final `.AppImage` (`BuildAppImage` → `appimagetool`) |
| `win-dir` | Staged Windows `bin/<Name>.exe` (`StageWindows`) |
| `wix` | Windows stage + `product.wxs` (`BuildWiXDir`; Start Menu + Desktop shortcuts + stable UpgradeCode) |
| `nsis-dir` | Windows stage + `installer.nsi` (`BuildNSISDir`; WriteUninstaller + ARP + Desktop/Start Menu shortcuts) |
| `msi` | Final `.msi` (`BuildMSI` → candle/light or `VITRA_CANDLE`/`VITRA_LIGHT`) |
| `nsis` | Final setup `.exe` (`BuildNSIS` → makensis or `VITRA_MAKENSIS`) |
| `app-dir` | Staged Darwin `.app` bundle (`StageDarwinApp`) |
| `dmg` | Final `.dmg` (`BuildDMG` → hdiutil or `VITRA_HDIUTIL`; includes Applications symlink) |

Optional `--icon <path>` copies a `.png` / `.svg` / `.icns` / `.ico` / `.xpm` into
Linux stages (root + `usr/share/icons/hicolor/…/apps/`, `Icon=<name>`), AppDir
(`.DirIcon` symlink + hicolor), `.deb` packages (`usr/share/pixmaps/` + hicolor +
`Icon=`), Darwin `Contents/Resources` (`CFBundleIconFile`), and Windows `bin/`
(WiX `ARPPRODUCTICON` + NSIS shortcut icon). Optional `--maintainer` sets Debian
`Maintainer` and, when provided, WiX `Manufacturer` / NSIS `PRODUCT_PUBLISHER`
(otherwise WiX/NSIS use `Name`). Optional `--description` sets the Debian
extended Description and FreeDesktop `Comment=` (default: secure Go + web
desktop blurb).

`BuildAppImage` stages an AppDir then invokes `appimagetool` (or
`VITRA_APPIMAGETOOL`). Without the tool, use `--format appdir` and fold
externally. `BuildRPM` stages an rpmbuild tree then invokes `rpmbuild` (or
`VITRA_RPMBUILD`); without it, use `--format rpm-dir`. `BuildSnap` stages a
snap prime then invokes `snapcraft pack` (or `VITRA_SNAPCRAFT`); without it,
use `--format snap-dir`. `BuildFlatpak` stages a Flatpak tree then invokes
`VITRA_FLATPAK_BUILDER` / `flatpak-builder` as `<stage> <out.flatpak>` (use a
wrapper for real `flatpak-builder` + `build-bundle`); without it, use
`--format flatpak-dir`. `BuildMSI` / `BuildNSIS` likewise stage then fold;
without WiX/NSIS tools, use `--format wix` / `nsis-dir` and fold on a Windows
host. `BuildDMG` stages a `.app` then folds with `hdiutil`; without it, use
`--format app-dir` and fold on macOS. `vitra doctor` reports whether those fold
tools are on PATH (or set via `VITRA_APPIMAGETOOL` / `VITRA_RPMBUILD` /
`VITRA_SNAPCRAFT` / `VITRA_FLATPAK_BUILDER` / `VITRA_CANDLE` / `VITRA_LIGHT` /
`VITRA_MAKENSIS` / `VITRA_HDIUTIL`).

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
  `SigningIdentityRef` validate refs; `PlanSign` / `--sign` print argv plans
  only — stage/fold never invoke `codesign` / `signtool` / notary)
- Extra Linux store polish beyond stage+fold (Flathub/Snap Store publishing,
  portal policy tuning) — `.rpm` / `.snap` / `.flatpak` generators ship via
  `BuildRPM*` / `BuildSnap*` / `BuildFlatpak*`
- Wails-class multi-framework project templates (React/Svelte/…) — `vitra new`
  ships `vanilla` + `vite` starters; richer frameworks still out

Installer **generators** (stage scripts + fold to `.deb` / `.rpm` / `.snap` /
`.flatpak` / `.AppImage` / `.msi` / NSIS setup / `.dmg`) are delivered; fold
still needs host tools (`appimagetool`, `rpmbuild`, `snapcraft`,
`flatpak-builder` / `VITRA_FLATPAK_BUILDER`, candle/light, makensis, `hdiutil`)
or env overrides.
