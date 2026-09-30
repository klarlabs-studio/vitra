# Signed updates

Vitra verifies updates with ed25519 signatures. It does not host anything: you upload a signed manifest and the artifact to any static host, and the app checks, verifies, and installs.

## What is verified

An update is installed only if **all** of these hold:

1. The manifest's signature is valid for your public key.
2. The artifact's SHA-256 matches the manifest.
3. The manifest is for **this app** and **this channel**.
4. Its version is **strictly newer** (SemVer 2.0) than the installed one, so old signed releases cannot be replayed.
5. It has not **expired** (`expires_at`, signed; 90 days by default), so a mirror cannot keep serving a stale release forever.
6. Any installed enterprise policy allows the channel.

Channels must use HTTPS (loopback excepted, for testing).

## Release workflow

```bash
# Once: create a key pair. Keep priv.key out of the repository.
vitra update-keygen --out keys/

# Per release: sign a manifest for the built artifact.
vitra update-sign --artifact dist/notes.tar.gz --app-id com.example.notes \
  --version 1.3.0 --channel stable --privkey file:keys/priv.key --out manifest.json \
  --expires-in 30d

# Lay it out for a static host: {app}/{channel}/manifest.json + artifact
vitra update-stage --out public/updates --manifest manifest.json --artifact dist/notes.tar.gz
```

`--privkey` takes `env:NAME`, `file:path`, or `secret:…`; bare hex is rejected so keys don't end up in shell history. Re-sign before a manifest expires if you have no newer release.

## Checking and installing from the app

```go
src := updater.ChannelSource{BaseURL: "https://updates.example.com", AppID: "com.example.notes", Channel: updater.ChannelStable}
f := &updater.Fetcher{Client: http.DefaultClient}

m, err := f.FetchManifest(ctx, src)
if err != nil { return err }
artifact, err := f.FetchArtifact(ctx, src, m)
if err != nil { return err }

plan, err := rt.ApplyUpdate(m, pubKey, artifact, installPath, updater.Installed{
    Channel: updater.ChannelStable,
    Version: currentVersion, // your app's version
})
```

Downloads are capped at 64 MiB (`updater.DefaultMaxBytes`). Set `Fetcher.MaxBytes` for larger artifacts.

`ApplyUpdate` verifies everything above, then installs:

- A plain artifact replaces the file at `installPath`.
- A `.tar.gz`, `.tgz`, or `.zip` replaces the directory at `installPath` (for example a macOS `.app`), keeping file modes and internal symlinks.
- The new version is written completely next to the old one before anything is replaced. A failed install leaves the old version in place.
- Archives cannot write outside the destination: absolute paths, `..`, and symlinks pointing out are rejected, and unpacked size and entry count are capped.
- On Windows the running executable is moved aside rather than overwritten. Call `updater.CleanupStale(installPath)` early at startup to remove leftovers.

Restart the app yourself after a successful install.

## From the command line

```bash
vitra update-check --base-url https://updates.example.com --app-id com.example.notes --channel stable --pubkey <hex>
vitra update-apply --base-url https://updates.example.com --app-id com.example.notes --channel stable \
  --current-version 1.2.0 --pubkey <hex> --dest /Applications/Notes.app
```
