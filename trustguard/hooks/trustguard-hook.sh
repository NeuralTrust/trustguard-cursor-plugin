#!/bin/sh
# Bootstrap for the TrustGuard Cursor plugin (macOS/Linux).
#
# Cursor invokes this script on each hook event. It executes trustguard-cursor
# from the PATH when present (manual/MDM installs win); otherwise it installs
# the pinned release for this OS/arch into ~/.trustguard/bin in the background,
# verifying its SHA-256 against the table below, and evaluates from the next
# event on. Every bootstrap failure fails open (the editor must never brick)
# with a warning on stderr.
#
# The VERSION and SHA256_* table are updated automatically by the Release
# workflow on every push to main.
set -u

VERSION="0.1.4"
BASE_URL="${TRUSTGUARD_CURSOR_DOWNLOAD_BASE:-https://github.com/NeuralTrust/trustguard-cursor-plugin/releases/download}"
BIN_DIR="${TRUSTGUARD_CURSOR_BIN_DIR:-$HOME/.trustguard/bin}"

# Per-platform SHA-256 of the release binaries (filled per release).
SHA256_darwin_amd64="172e8ae62fdb0acc418e0832aeab4776e4b8a96bcd8b5db3d31a3c1c87327f23"
SHA256_darwin_arm64="fdce772424364314cf3d67e08527eee0e12a801e279aa6dc0752db22d19b473d"
SHA256_linux_amd64="632f676d880fe3dcc51d1a4581add3f7dab6e0f8ef542c666ff001c7a5eb00ae"
SHA256_linux_arm64="336dfaf88ffebaebc3ab576d353eae35a85e45bd6e2f12648353df1c4603fa9c"
SHA256_windows_amd64="a843a49300fde06f56918e5cb91562093a643684da1f781dc95b852afb3eb4d0"
SHA256_windows_arm64="5c0d98bc44d3a91a7f6e4c72f61c347640c5ffaeba39c7417a6f735f3be32df9"

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

# 3. Not cached: install in the background and fail open right away. Downloading
# inline would freeze the editor on every event, and Cursor kills the hook long
# before curl gives up, so a killed download never lands and the next event pays
# the same stall again. Evaluation starts once the binary is in place.
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
mkdir -p "$BIN_DIR" 2>/dev/null || fail_open "cannot create $BIN_DIR"

install_binary() {
    TMP="$BIN.download.$$"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 5 --max-time 300 -o "$TMP" "$URL" || { rm -f "$TMP"; return 1; }
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 300 -O "$TMP" "$URL" || { rm -f "$TMP"; return 1; }
    else
        return 1
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        GOT_SHA=$(sha256sum "$TMP" | cut -d' ' -f1)
    elif command -v shasum >/dev/null 2>&1; then
        GOT_SHA=$(shasum -a 256 "$TMP" | cut -d' ' -f1)
    else
        rm -f "$TMP"
        return 1
    fi
    if [ "$GOT_SHA" != "$WANT_SHA" ]; then
        rm -f "$TMP"
        return 1
    fi

    chmod 0755 "$TMP" || { rm -f "$TMP"; return 1; }
    mv -f "$TMP" "$BIN" || { rm -f "$TMP"; return 1; }
}

# mkdir is atomic, so the events racing on the first prompt elect a single
# downloader instead of each fetching its own copy. A lock left behind by a
# killed process is reclaimed after ten minutes.
LOCK="$BIN_DIR/install-$VERSION.lock"
if [ -d "$LOCK" ] && [ -n "$(find "$LOCK" -maxdepth 0 -mmin +10 2>/dev/null)" ]; then
    rmdir "$LOCK" 2>/dev/null || :
fi
if mkdir "$LOCK" 2>/dev/null; then
    # stdout must be detached: Cursor reads the hook's pipe until every writer
    # closes it, so an inherited stdout would keep the editor waiting.
    ( install_binary; rmdir "$LOCK" 2>/dev/null ) >/dev/null 2>&1 &
fi
fail_open "trustguard-cursor $VERSION not installed yet; fetching it in the background"
