# Packaging

```bash
vitra build                        # go build -tags vitra_native
vitra package --out dist/ --format deb --app-id com.example.notes --name Notes --version 1.2.0
```

`vitra package` stages or builds an installable package from your app binary and writes `provenance.json` next to it: the app ID and version, the Go toolchain, module versions, and registered plugins.

## Formats

| Format | Output | Host tool |
|---|---|---|
| `dir` | Linux staging folder (binary, `.desktop`, icon) | none |
| `deb` | `.deb` | `dpkg-deb` |
| `rpm-dir` / `rpm` | `rpmbuild` tree / `.rpm` | `rpmbuild` |
| `snap-dir` / `snap` | `snapcraft.yaml` project / `.snap` | `snapcraft` |
| `flatpak-dir` / `flatpak` | manifest / bundle | `flatpak-builder` |
| `appdir` / `appimage` | AppDir / `.AppImage` | `appimagetool` |
| `win-dir` | Windows staging folder | none |
| `wix` / `msi` | WiX project / `.msi` | `wix` (v4) or `candle` + `light` |
| `nsis-dir` / `nsis` | NSIS script / installer `.exe` | `makensis` |
| `app-dir` / `dmg` | `.app` bundle / `.dmg` | `hdiutil` |

The `*-dir` formats need no extra tools, so you can inspect or customize the project before building it yourself.

Metadata flags: `--bin`, `--app-id`, `--name`, `--version`, `--icon`, `--maintainer`, `--description`, `--homepage`, `--categories`, `--keywords`, `--license` (SPDX), and `--accessory`, which marks a menu bar app (`LSUIElement` in the macOS `Info.plist`, so it starts without a Dock icon; pair it with `app.PresentationAccessory`).

## Signing

```bash
vitra package --out dist/ --format dmg ... --sign --signing-identity keychain:"Developer ID Application: Example"
```

- `--sign` prints a signing plan (codesign, signtool, rpmsign, gpg, notarytool) and checks that the identity reference resolves. It runs nothing.
- `--sign-execute` runs the plan. `--sign-follow-ups` also runs follow-up steps such as notarization and stapling.
- Identities are references: `env:NAME`, `keychain:…`, `file:path`, or `secret:…`. Vitra never puts secret values in plans, logs, or project files.

`vitra notary-setup` prints the `notarytool store-credentials` command for a one-time macOS notarization setup.

## Publishing

`--publish` prints store submission steps for snap and Flatpak. `--publish-execute` runs the non-interactive ones. Store logins and Flathub pull requests stay with you.

## Other integration

```bash
vitra register-scheme myapp com.example.notes /path/to/app      # myapp:// deep links
vitra register-files --mime text/markdown --app-id com.example.notes --exec /path/to/app
```

These register URL-scheme and file-type handlers with the OS: xdg on Linux, a helper `.app` on macOS, `.reg` files on Windows.
