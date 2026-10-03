# Contributing to vitra

Thanks for considering a contribution. Vitra aims to stay small, principled,
and secure-by-construction — please keep that in mind when proposing changes.

## Core principles

1. **Zero external dependencies in the kernel.** The `domain/` package must not
   import anything outside the standard library. The default adapters (`internal/inmemory/`)
   also use stdlib only. Platform WebView bindings (`platform/*`,
   `-tags vitra_native`) are adapters outside the kernel.
2. **DDD boundaries.** Respect the dependency direction:
   `domain` ← `internal/application` ← `internal/inmemory` ← `vitra` (root facade) ← consumer code.
   The domain owns its port interfaces.
3. **Least privilege by default.** New windows have no ambient authority.
   Commands are explicitly registered. Grants must name windows, origins, and
   permissions.
4. **Aggregates enforce invariants.** Unexported fields, constructor validation,
   defensive copies.
5. **TDD.** Start with failing tests — especially for security invariants.

## Development setup

```bash
git clone https://github.com/klarlabs-studio/vitra.git
cd vitra
make install-hooks
make check
```

You need Go 1.26+, `golangci-lint` v2, and optionally `nox` + `coverctl`.

## Commit style

Use [Conventional Commits](https://www.conventionalcommits.org/) and sign off
with `git commit -s` (DCO).

## Pull request checklist

- [ ] `make check` passes locally
- [ ] Tests added for new behavior (TDD preferred)
- [ ] Race detector clean: `go test ./... -race`
- [ ] Security invariants still hold
- [ ] Docs updated if public API or architecture changed

## Releasing

1. Move the `CHANGELOG.md` Unreleased entries under the new version and set
   `vitra.Version` (in `vitra.go`) to match; `vitra new` requires that tag.
2. Merge to `main`, then push a signed tag: `git tag -s vX.Y.Z && git push origin vX.Y.Z`.
3. `.github/workflows/release.yml` publishes the CLI archives with SBOMs, a
   cosign-signed checksums file, and SLSA provenance. The release notes explain
   how to verify them.

### Code-signing certificates

The release pipeline can also sign the `vitra` binaries for macOS and Windows,
so downloads do not trigger Gatekeeper or SmartScreen warnings. Each half turns
on when its repository secrets exist (*Settings → Secrets and variables →
Actions*). Without them it is skipped and the release succeeds unsigned at the
OS level. Signing runs right after the build, so the archives, checksums, SBOMs,
cosign signature, and SLSA provenance all cover the signed binaries.

**macOS** (GoReleaser `notarize`, which uses quill on the Linux runner):

| Secret | Value |
|---|---|
| `MACOS_SIGN_P12` | Base64 of a *Developer ID Application* certificate and private key exported as `.p12` |
| `MACOS_SIGN_PASSWORD` | Password of that `.p12` |
| `MACOS_NOTARY_ISSUER_ID` | App Store Connect API issuer ID (a UUID) |
| `MACOS_NOTARY_KEY_ID` | App Store Connect API key ID |
| `MACOS_NOTARY_KEY` | Base64 of the API key's `.p8` file |

1. In an Apple Developer Program account, create a *Developer ID Application*
   certificate (Certificates, Identifiers & Profiles; only the Account Holder
   can). Import it into Keychain Access, then export the certificate together
   with its private key as `.p12` with a password.
2. In App Store Connect, under *Users and Access → Integrations → App Store
   Connect API*, create a team key with the *Developer* role. Note the issuer
   ID and key ID and download the `.p8` (it can be downloaded only once).
3. `base64 -i cert.p12 | pbcopy` and `base64 -i AuthKey_XXXX.p8 | pbcopy` give
   the secret values.

The darwin binaries are signed and notarized before archiving. A bare
binary cannot be stapled, so Gatekeeper checks the notarization ticket online
on first run.

**Windows** (`scripts/sign-windows.sh`, a GoReleaser build hook using
osslsigncode):

| Secret | Value |
|---|---|
| `WINDOWS_SIGN_PFX` | Base64 of an OV code-signing certificate and private key as `.pfx` |
| `WINDOWS_SIGN_PASSWORD` | Password of that `.pfx` |

Optionally set the repository *variable* `WINDOWS_SIGN_TIMESTAMP_URL` to the
RFC 3161 timestamp server of your CA; the default is
`http://timestamp.digicert.com`. Timestamping keeps signatures valid after the
certificate expires.

Get the certificate from a CA that sells Authenticode certificates (DigiCert,
Sectigo, SSL.com, …) and export it with its key as `.pfx`, then
`base64 -w0 cert.pfx` (Linux) or `base64 -i cert.pfx` (macOS).

Since June 2023, CAs issue new code-signing keys, OV and EV alike, only on a
hardware token or in a cloud HSM, so most new certificates cannot be exported
as a `.pfx`. For those, replace osslsigncode in `scripts/sign-windows.sh` with
[jsign](https://ebourg.github.io/jsign/), which signs through a cloud key
service: for example `jsign --storetype AZUREKEYVAULT` (Azure Key Vault or
Azure Trusted Signing), `GOOGLECLOUD`, `AWS`, or the CA's own service such as
DigiCert KeyLocker or SSL.com eSigner, with the matching credentials as
secrets. A USB token cannot be used from a hosted runner.

## Documentation site

The site at https://klarlabs-studio.github.io/vitra/ is built with VitePress from `docs/` and deployed by the *Docs* workflow on every push to main. To preview locally:

```bash
cd docs
npm install
npm run dev
```

`docs/spikes/` holds working notes and is not published.
