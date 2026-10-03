#!/usr/bin/env bash
# Authenticode-signs a Windows binary in place. GoReleaser runs it as a build
# post-hook for every target, so the archives, checksums, SBOMs, and the
# cosign signature all cover the signed .exe.
#
# Does nothing (exit 0) unless the target is windows and WINDOWS_SIGN_PFX is
# set, so releases without the certificate still succeed unchanged.
#
#   scripts/sign-windows.sh <binary> <goos>
#
# Environment:
#   WINDOWS_SIGN_PFX            base64-encoded PFX (certificate + private key)
#   WINDOWS_SIGN_PASSWORD       PFX password
#   WINDOWS_SIGN_TIMESTAMP_URL  RFC 3161 timestamp server
#                               (default http://timestamp.digicert.com)
#
# Needs osslsigncode (apt-get install osslsigncode).
set -euo pipefail

bin=${1:?usage: sign-windows.sh <binary> <goos>}
goos=${2:?usage: sign-windows.sh <binary> <goos>}

if [ "$goos" != "windows" ] || [ -z "${WINDOWS_SIGN_PFX:-}" ]; then
    exit 0
fi

if ! command -v osslsigncode >/dev/null 2>&1; then
    echo "sign-windows: WINDOWS_SIGN_PFX is set but osslsigncode is not installed" >&2
    exit 1
fi

timestamp_url=${WINDOWS_SIGN_TIMESTAMP_URL:-http://timestamp.digicert.com}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
chmod 700 "$work"

pfx="$work/cert.pfx"
pass="$work/pass"
signed="$work/signed.exe"
(umask 077 && printf '%s' "$WINDOWS_SIGN_PFX" | base64 -d > "$pfx")
(umask 077 && printf '%s' "${WINDOWS_SIGN_PASSWORD:-}" > "$pass")

osslsigncode sign \
    -pkcs12 "$pfx" \
    -readpass "$pass" \
    -n "Vitra" \
    -i "https://github.com/klarlabs-studio/vitra" \
    -h sha256 \
    -ts "$timestamp_url" \
    -in "$bin" \
    -out "$signed"

# Fail if no signature was embedded. Full chain verification is reported but
# not enforced: it depends on the runner's CA bundle, not on the signature.
osslsigncode extract-signature -in "$signed" -out "$work/sig.der" >/dev/null
osslsigncode verify -in "$signed" ||
    echo "sign-windows: chain verification not confirmed on this runner" >&2

# Overwrite in place so the file keeps its mode, then restore GoReleaser's
# reproducible mod_timestamp.
touch -r "$bin" "$work/mtime"
cat "$signed" > "$bin"
touch -r "$work/mtime" "$bin"
echo "sign-windows: signed $bin"
