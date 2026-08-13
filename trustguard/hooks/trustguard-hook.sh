#!/bin/sh
# Bootstrap for the TrustGuard Cursor plugin (macOS/Linux).
#
# Cursor invokes this script on each hook event. It executes trustguard-cursor
# from the PATH when present (manual/MDM installs win); otherwise it downloads
# the pinned release for this OS/arch into ~/.trustguard/bin, verifies its
# SHA-256 against the table below, and executes it. Every bootstrap failure
# fails open (the editor must never brick) with a warning on stderr.
#
# The VERSION and SHA256_* table are updated on each cursor-v* release; the
# release workflow prints the exact block to paste here.
set -u

VERSION="0.1.0"
BASE_URL="${TRUSTGUARD_CURSOR_DOWNLOAD_BASE:-https://github.com/NeuralTrust/trustguard-cursor-plugin/releases/download}"
BIN_DIR="${TRUSTGUARD_CURSOR_BIN_DIR:-$HOME/.trustguard/bin}"

# Per-platform SHA-256 of the release binaries (filled per release).
SHA256_darwin_amd64="ee5c09b8de148bfca721b2ea3677816c9a27f1e9b9277ff6d7e81c73f0584b8a"
SHA256_darwin_arm64="83b31f2bd15bf4dce75b55f3d8b99b01be9ce7dfd30d733fbcdc62ad6c88db16"
SHA256_linux_amd64="827b533cf43bcd270ffb16bb5d054b08c8bc3079537cab661e416a948474fab6"
SHA256_linux_arm64="5303bf241d1fabc6e7aef7da45d9092d9683157c5c840a6809184ed01b11f129"
SHA256_windows_amd64="7abb7ae3ba0a45cc4c19c3382a47d05948967fcb228b77c9040b19ff5247951f"
SHA256_windows_arm64="4363b5a61cd0d0cbe8c84b0ea803748063a3141adb6af2ee0e9e4b0d1c8004bf"

fail_open() {
    echo "trustguard-cursor bootstrap: $1 — allowing without evaluation" >&2
    # `continue` answers beforeSubmitPrompt; `permission` answers the rest.
    printf '{"continue":true,"permission":"allow"}\n'
    exit 0
}

# 1. A binary on the PATH always wins (manual, MDM or package-manager install).
if command -v trustguard-cursor >/dev/null 2>&1; then
    exec trustguard-cursor hook
fi

# Git Bash / MSYS on Windows runs this script too; it needs the .exe artifact.
EXT=""
case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux) OS="linux" ;;
    MINGW* | MSYS* | CYGWIN*) OS="windows" EXT=".exe" ;;
    *) OS="" ;;
esac

# 2. Cached pinned version.
BIN="$BIN_DIR/trustguard-cursor-$VERSION$EXT"
if [ -x "$BIN" ]; then
    exec "$BIN" hook
fi

# 3. Download, verify, install, exec.
if [ -z "$OS" ]; then
    fail_open "unsupported OS $(uname -s); install trustguard-cursor manually"
fi
case "$(uname -m)" in
    x86_64 | amd64) ARCH="amd64" ;;
    arm64 | aarch64) ARCH="arm64" ;;
    *) fail_open "unsupported arch $(uname -m); install trustguard-cursor manually" ;;
esac

WANT_SHA=$(eval "printf '%s' \"\${SHA256_${OS}_${ARCH}:-}\"")
if [ -z "$WANT_SHA" ]; then
    fail_open "no pinned checksum for ${OS}/${ARCH} (release ${VERSION} not published yet?); install trustguard-cursor manually"
fi

URL="$BASE_URL/v$VERSION/trustguard-cursor_${VERSION}_${OS}_${ARCH}${EXT}"
TMP="$BIN.download.$$"
mkdir -p "$BIN_DIR" 2>/dev/null || fail_open "cannot create $BIN_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 5 --max-time 60 -o "$TMP" "$URL" || { rm -f "$TMP"; fail_open "download failed: $URL"; }
elif command -v wget >/dev/null 2>&1; then
    wget -q -T 60 -O "$TMP" "$URL" || { rm -f "$TMP"; fail_open "download failed: $URL"; }
else
    fail_open "neither curl nor wget available"
fi

if command -v sha256sum >/dev/null 2>&1; then
    GOT_SHA=$(sha256sum "$TMP" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
    GOT_SHA=$(shasum -a 256 "$TMP" | cut -d' ' -f1)
else
    rm -f "$TMP"
    fail_open "no sha256 tool available to verify the download"
fi
if [ "$GOT_SHA" != "$WANT_SHA" ]; then
    rm -f "$TMP"
    fail_open "checksum mismatch for $URL (got $GOT_SHA, want $WANT_SHA)"
fi

chmod 0755 "$TMP" || { rm -f "$TMP"; fail_open "cannot mark binary executable"; }
mv -f "$TMP" "$BIN" || { rm -f "$TMP"; fail_open "cannot install binary to $BIN"; }
echo "trustguard-cursor bootstrap: installed $BIN" >&2
exec "$BIN" hook
