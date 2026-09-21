# Phase 4 — Distribution Excellence

Phase 4 makes packaging, updates, and release inspection first-class.

## Delivered

| Area | Package |
|------|---------|
| Package targets + signing identity refs | `packaging` |
| AppStream metainfo | `AppStreamMetainfoXML` staged under `usr/share/metainfo/` (Flatpak `files/share/metainfo/`); Homepage / Categories / License / Description via Spec (also RPM/snap, Debian `copyright`, Windows ARP URL/Comments/LegalCopyright, Darwin `NSHumanReadableCopyright`) |
| Sign dry-run plan | `packaging.PlanSign` + `vitra package --sign` (Darwin codesign + notarytool/stapler; Windows signtool; Linux dpkg-sig/rpmsign/snapcraft/gpg) |
| Notary credential bootstrap plan | `PlanNotaryCredentials` + `vitra notary-setup` (store-credentials argv; not executed) |
| Sign execution | `packaging.ExecuteSign` + `vitra package --sign-execute` [--sign-follow-ups] (expands `${ENV}` and `<secret:…>`→`VITRA_SECRET_*`; sets `Artifact.Signed`) |
| Store publish dry-run | `packaging.PlanPublish` + `vitra package --publish` (Snap Store / Flathub step plans) |
| Store publish execution | `packaging.ExecutePublish` + `vitra package --publish-execute` (snapcraft upload / flatpak-builder only; never login or Flathub PR) |
| Linux Flatpak stage + fold | `BuildFlatpakDir` / `BuildFlatpak` (portal-oriented finish-args + metadata Context) |
| Windows staged dir + WiX/NSIS scripts | `StageWindows` / `BuildWiXDir` / `BuildNSISDir` |
| Darwin staged `.app` bundle | `StageDarwinApp` + `vitra package --format app-dir` |
| Darwin DMG fold | `BuildDMG` / `FoldDMG` (`hdiutil` or `VITRA_HDIUTIL`) |
| Signed update manifests (ed25519) | `updater` |
| Update keygen + sign CLI | `updater.GenerateKeyPair` / `BuildSignedManifest` / `LoadPrivateKeyRef` + `vitra update-keygen` / `update-sign` |
| HTTP(S) channel fetch (client only) | `updater.ChannelSource` / `Fetcher` + `vitra update-check` |
| Channel tree for static CDN upload | `updater.StageChannel` + `vitra update-stage` |
| Install plans only after verify | `updater.PlanInstall` |
| Atomic install after plan | `updater.ApplyInstall` / `Runtime.ApplyUpdate` |
| SBOM / plugin / capability inventory | `provenance` |

`vitra package --out dist/` stages a platform layout and `provenance.json`. Formats:

| `--format` | Output |
|------------|--------|
| `dir` (default) | Staged Linux app directory (`StageLinux`) |
| `deb` | Pure-Go `.deb` (`BuildDeb`; DEP-5 copyright + Section from Categories) |
| `rpm-dir` | rpmbuild `_topdir` + `SPECS/*.spec` (`BuildRPMDir`; Group from Categories) |
| `rpm` | Final `.rpm` (`BuildRPM` → `rpmbuild` or `VITRA_RPMBUILD`) |
| `snap-dir` | Snap prime dir + `meta/snap.yaml` (`BuildSnapDir`; license/website/contact/keywords) |
| `snap` | Final `.snap` (`BuildSnap` → `snapcraft pack` or `VITRA_SNAPCRAFT`) |
| `flatpak-dir` | Flatpak stage (`files/` + `metadata` + `manifest.yml`; `BuildFlatpakDir`) |
| `flatpak` | Final `.flatpak` (`BuildFlatpak` → `VITRA_FLATPAK_BUILDER` / `flatpak-builder`) |
| `appdir` | AppImage-ready AppDir (`BuildAppDir`) |
| `appimage` | Final `.AppImage` (`BuildAppImage` → `appimagetool`) |
| `win-dir` | Staged Windows `bin/<Name>.exe` (`StageWindows`) |
| `wix` | Windows stage + `product.wxs` (`BuildWiXDir`; Start Menu + Desktop shortcuts + stable UpgradeCode + ARP URL/comments/copyright) |
| `nsis-dir` | Windows stage + `installer.nsi` (`BuildNSISDir`; WriteUninstaller + ARP + Desktop/Start Menu shortcuts + URLInfoAbout/Comments/LegalCopyright) |
| `msi` | Final `.msi` (`BuildMSI` → candle/light or `VITRA_CANDLE`/`VITRA_LIGHT`) |
| `nsis` | Final setup `.exe` (`BuildNSIS` → makensis or `VITRA_MAKENSIS`) |
| `app-dir` | Staged Darwin `.app` bundle (`StageDarwinApp`; Info.plist + `NSHumanReadableCopyright`) |
| `dmg` | Final `.dmg` (`BuildDMG` → hdiutil or `VITRA_HDIUTIL`; includes Applications symlink) |

Optional `--icon <path>` copies a `.png` / `.svg` / `.icns` / `.ico` / `.xpm` into
Linux stages (root + `usr/share/icons/hicolor/…/apps/`, `Icon=<name>`), AppDir
(`.DirIcon` symlink + hicolor), `.deb` packages (`usr/share/pixmaps/` + hicolor +
`Icon=`), Darwin `Contents/Resources` (`CFBundleIconFile`), and Windows `bin/`
(WiX `ARPPRODUCTICON` + NSIS shortcut icon). Optional `--homepage` sets Debian
`Homepage`, AppStream/RPM/snap URLs, and Windows ARP support URL
(`URLInfoAbout` / `ARPURLINFOABOUT`) when provided. Optional `--maintainer` sets Debian
`Maintainer`, snap `contact:`, DEP-5 Upstream-Contact, and, when provided, WiX
`Manufacturer` / NSIS `PRODUCT_PUBLISHER` (otherwise WiX/NSIS use `Name`). Optional `--description` sets the Debian
extended Description, FreeDesktop `Comment=`, and Windows ARP Comments /
`ARPCOMMENTS` (default: secure Go + web desktop blurb). Optional `--license`
sets AppStream/RPM/snap license fields, Debian DEP-5
`usr/share/doc/<pkg>/copyright`, Windows ARP `LegalCopyright` /
`ARPCOPYRIGHT`, and Darwin Info.plist `NSHumanReadableCopyright`
(default: `LicenseRef-proprietary`). Optional
`--categories` sets FreeDesktop/AppStream categories, Debian control
`Section:` via `DebianSection`, and RPM `Group:` via `RPMGroup`
(default Utility→utils / Applications/System).

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

Channel fetch (client only — Vitra does not host a CDN):

```go
src := updater.ChannelSource{BaseURL: "https://updates.example/", AppID: "com.example.app", Channel: updater.ChannelStable}
m, err := (&updater.Fetcher{}).FetchManifest(ctx, src)
// VerifyManifest / PlanInstall / ApplyUpdate as usual
```

Stage a tree for static hosting:

```go
stage, err := updater.StageChannel("dist/updates", signedManifest, artifactBytes)
```

CLI:

```bash
vitra update-keygen --out keys/
vitra update-sign --artifact app.bin --app-id com.example.app --version 1.2.0 \
  --privkey file:keys/priv.key --out update.json
vitra update-stage --out dist/updates --manifest update.json --artifact app.bin
vitra update-check --base-url https://updates.example/ --app-id com.example.app --channel stable --pubkey <hex>
vitra update-apply --base-url https://updates.example/ --app-id com.example.app --channel stable --pubkey <hex> --dest ./vitra-app
vitra update-apply --manifest update.json --artifact app.bin --pubkey <hex> --dest ./vitra-app --policy production
```

## Security invariants

- **9**: `PlanInstall` / `ApplyUpdate` refuse unsigned or digest-mismatched artifacts
- **10**: `packaging.Spec` accepts only `SigningIdentityRef` (e.g. `env:…` /
  keychain refs), never inline private key material

## Honest remaining gaps

Still out of scope on main (do not claim otherwise):

- Hosted update CDN / auto-update channel *hosting* (HTTP(S) *client* fetch
  via `ChannelSource` / `Fetcher` / `vitra update-check` ships; `StageChannel`
  / `vitra update-stage` writes a static tree for upload to any host;
  `update-keygen` / `update-sign` complete the publish-side sign loop)
- Interactive notarization credential entry beyond the
  `PlanNotaryCredentials` / `vitra notary-setup` dry-run (`store-credentials`
  argv with `${APPLE_ID}` / `${APPLE_TEAM_ID}` / `${APP_SPECIFIC_PASSWORD}`;
  operator still runs notarytool)
- Extra Linux store polish beyond publish *plans* / non-interactive *execute*,
  AppStream metainfo, and portal-oriented Flatpak finish-args (Flathub PR
  automation, interactive store login) — `PlanPublish` / `--publish` prints Snap
  Store / Flathub argv guidance; `--publish-execute` runs Executable steps only
  (`snapcraft upload`, `flatpak-builder`); stages emit AppStream
  `<appid>.metainfo.xml`; Flatpak stage uses xdg-desktop-portal talk-names;
  `.rpm` / `.snap` / `.flatpak` generators ship via `BuildRPM*` / `BuildSnap*` /
  `BuildFlatpak*`
- Wails-class template breadth beyond Vitra’s `vanilla` / `vite` / `react` /
  `svelte` / `vue` / `solid` / `preact` / `lit` / `alpine` / `htmx` / `angular` / `qwik` / `mithril` / `riot` starters (additional frameworks, richer presets)

Installer **generators** (stage scripts + fold to `.deb` / `.rpm` / `.snap` /
`.flatpak` / `.AppImage` / `.msi` / NSIS setup / `.dmg`) are delivered; fold
still needs host tools (`appimagetool`, `rpmbuild`, `snapcraft`,
`flatpak-builder` / `VITRA_FLATPAK_BUILDER`, candle/light, makensis, `hdiutil`)
or env overrides.
