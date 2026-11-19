#!/bin/sh
# Install semaphore-cli from GitHub releases.
# Usage: sh install.sh [-b DIR] [VERSION]
#   -b DIR    install directory (default: /usr/local/bin, or ~/.local/bin without root)
#   VERSION   release tag like v0.1.0 (default: latest)
set -eu

REPO="Alg0rix/semaphore-cli"
BIN="semaphore-cli"

while getopts "b:h" opt; do
    case "$opt" in
        b) INSTALL_DIR="$OPTARG" ;;
        h) sed -n '2,4p' "$0"; exit 0 ;;
        *) exit 2 ;;
    esac
done
shift $((OPTIND - 1))
VERSION="${1:-latest}"

case "$(uname -s)" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64)   ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
    [ -n "$VERSION" ] || { echo "could not resolve latest release" >&2; exit 1; }
fi
echo "installing ${BIN} ${VERSION} (${OS}/${ARCH})"

if [ -z "${INSTALL_DIR:-}" ]; then
    if [ -w /usr/local/bin ]; then
        INSTALL_DIR=/usr/local/bin
    else
        INSTALL_DIR="$HOME/.local/bin"
    fi
fi
mkdir -p "$INSTALL_DIR"

EXT="tar.gz"; [ "$OS" = "windows" ] && EXT="zip"
ASSET="${BIN}_${OS}_${ARCH}.${EXT}"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "${BASE}/${ASSET}" -o "${TMP}/${ASSET}"
curl -fsSL "${BASE}/checksums.txt" -o "${TMP}/checksums.txt"

cd "$TMP"
if command -v sha256sum >/dev/null 2>&1; then
    grep " ${ASSET}\$" checksums.txt | sha256sum -c - >/dev/null
elif command -v shasum >/dev/null 2>&1; then
    grep " ${ASSET}\$" checksums.txt | shasum -a 256 -c - >/dev/null
else
    echo "warning: no sha256 tool found, skipping checksum verification" >&2
fi

if [ "$EXT" = "zip" ]; then
    unzip -q -o "$ASSET"
else
    tar -xzf "$ASSET"
fi

BINFILE="$BIN"; [ "$OS" = "windows" ] && BINFILE="${BIN}.exe"
chmod +x "$BINFILE" 2>/dev/null || true

if [ -w "$INSTALL_DIR" ]; then
    mv "$BINFILE" "$INSTALL_DIR/$BINFILE"
else
    sudo mv "$BINFILE" "$INSTALL_DIR/$BINFILE"
fi

echo "installed: $INSTALL_DIR/$BINFILE"
