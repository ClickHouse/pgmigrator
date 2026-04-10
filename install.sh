#!/bin/sh
set -eu

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

VERSION=$(curl -fsS -o /dev/null -w '%{redirect_url}' https://github.com/ClickHouse/pgmigrator/releases/latest)
VERSION="${VERSION##*/}"
if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version" >&2
  exit 1
fi

ASSET="pgmigrator-${VERSION}-${OS}-${ARCH}"
BASE_URL="https://github.com/ClickHouse/pgmigrator/releases/download/${VERSION}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading pgmigrator ${VERSION} (${OS}/${ARCH})..."
curl -fsSL -o "$TMPDIR/${ASSET}.tar.gz" "${BASE_URL}/${ASSET}.tar.gz"
curl -fsSL -o "$TMPDIR/checksums.txt" "${BASE_URL}/checksums.txt"

echo "Verifying checksum..."
EXPECTED=$(grep "${ASSET}.tar.gz" "$TMPDIR/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMPDIR/${ASSET}.tar.gz" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMPDIR/${ASSET}.tar.gz" | cut -d' ' -f1)
else
  echo "Warning: no sha256sum or shasum found, skipping verification" >&2
  ACTUAL="$EXPECTED"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "Checksum mismatch!" >&2
  echo "  expected: $EXPECTED" >&2
  echo "  actual:   $ACTUAL" >&2
  exit 1
fi

INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "$INSTALL_DIR"
tar -xzf "$TMPDIR/${ASSET}.tar.gz" -C "$TMPDIR"
cp "$TMPDIR/${ASSET}/pgmigrator" "$INSTALL_DIR/pgmigrator"
chmod +x "$INSTALL_DIR/pgmigrator"

echo "Installed pgmigrator ${VERSION} to ${INSTALL_DIR}/pgmigrator"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: add ${INSTALL_DIR} to your PATH" ;;
esac
