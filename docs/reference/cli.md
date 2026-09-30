# CLI

```bash
go install go.klarlabs.de/vitra/cmd/vitra@latest
```

## Develop

| Command | What it does |
|---|---|
| `vitra version` | Print the Vitra version |
| `vitra doctor` | Check Go, cgo, and the WebView toolchain for this OS |
| `vitra new <dir> [--template vanilla\|vite\|react\|svelte\|vue]` | Create a starter app |
| `vitra dev [dir]` | Run the app with the native host and the inspector; restart when `.go`, `.html`, `.css`, or `.js` files change |
| `vitra build [dir]` | Build the app binary with the native host |
| `vitra generate typescript [--app dir] [--out path] [--module name]` | Write a TypeScript client. With `--app`, the client covers that app's own commands; without it, the official plugin commands |
| `vitra inspect capabilities` | Print the capability surface of a demo runtime |

## Package and integrate

| Command | What it does |
|---|---|
| `vitra package --out <dir> [--format …] [metadata] [--sign …] [--publish …]` | Build a package; see [Packaging](/guide/packaging) |
| `vitra notary-setup [--profile name]` | Print the macOS `notarytool store-credentials` setup (not executed) |
| `vitra register-scheme <scheme> [app-id] [exec]` | Register a URL-scheme handler |
| `vitra register-files --mime <type> [--mime …] [--app-id id] [--exec path] [--name name]` | Register file-type handlers |

## Updates

| Command | What it does |
|---|---|
| `vitra update-keygen [--out dir]` | Create an ed25519 key pair (`priv.key`, `pub.key`, hex) |
| `vitra update-sign --artifact <path> --app-id <id> --version <ver> --privkey <ref> --out <manifest.json> [--channel stable\|beta] [--artifact-name name] [--expires-in 90d]` | Sign an update manifest. `--privkey` takes `env:`, `file:`, or `secret:` |
| `vitra update-stage --out <dir> --manifest <json> --artifact <path>` | Lay out `{app}/{channel}/manifest.json` and the artifact for a static host |
| `vitra update-check --base-url <url> --app-id <id> --channel <name> --pubkey <hex>` | Fetch and verify the channel's manifest without installing |
| `vitra update-apply (--manifest <json> --artifact <path> \| --base-url <url>) --app-id <id> [--channel name] --current-version <semver> --pubkey <hex> --dest <path> [--policy production\|development]` | Verify and install an update |

See [Signed updates](/guide/updates).

## Environment

| Variable | Effect |
|---|---|
| `VITRA_DEVTOOLS=1` | Enable the WebView inspector (`vitra dev` sets it) |
| `VITRA_GENERATE_TYPESCRIPT=<path>` | Make `app.Run` write the TypeScript client and exit (`vitra generate typescript --app` sets it) |
| `VITRA_MODULE_PATH=<dir>` | Make `vitra new` add a `replace` directive pointing at a local Vitra checkout |
