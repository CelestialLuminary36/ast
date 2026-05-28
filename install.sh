#!/usr/bin/env bash
set -euo pipefail

# install.sh — one-liner installer for ast
#
# curl -fsSL https://raw.githubusercontent.com/CelestialLuminary36/ast/main/install.sh | bash
#
# Overrides:
#   VERSION=0.2.0 bash install.sh     — pin a specific version
#   PREFIX=/usr/local bash install.sh  — choose install dir (default ~/.local/bin)

REPO="CelestialLuminary36/ast"
PREFIX="${PREFIX:-"$HOME/.local/bin"}"
VERSION="${VERSION:-latest}"

# ---------- detect OS & arch ----------
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
	x86_64|amd64)   ARCH="amd64" ;;
	aarch64|arm64)  ARCH="arm64" ;;
	*)
		echo "Unsupported architecture: $ARCH" >&2
		exit 1
		;;
esac

case "$OS" in
	linux)   EXT="tar.gz" ;;
	darwin)  EXT="tar.gz" ;;
	mingw*|msys*|cygwin*|windows*)
		echo "Windows detected — grab the zip from https://github.com/$REPO/releases" >&2
		exit 1
		;;
	*)
		echo "Unsupported OS: $OS" >&2
		exit 1
		;;
esac

# ---------- resolve version ----------
if [ "$VERSION" = "latest" ]; then
	if ! command -v curl >/dev/null 2>&1; then
		echo "curl is required to fetch the latest version." >&2
		exit 1
	fi
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
		| grep -o '"tag_name": *"[^"]*"' \
		| cut -d'"' -f4) || {
		echo "Failed to fetch latest version from GitHub." >&2
		exit 1
	}
fi

# Strip leading 'v' if present (goreleaser archive name uses the plain version).
ARCHIVE="ast_${VERSION}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/${VERSION}/${ARCHIVE}"

# ---------- download ----------
echo "Downloading ast ${VERSION} (${OS}/${ARCH}) ..."
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/$ARCHIVE" || {
	echo "Download failed: $DOWNLOAD_URL" >&2
	exit 1
}

# ---------- extract ----------
cd "$TMPDIR"
case "$EXT" in
	tar.gz) tar xzf "$ARCHIVE" ;;
	zip)    unzip -q "$ARCHIVE" ;;
esac

# ---------- install ----------
mkdir -p "$PREFIX"

ACTION="Installed"
if [ -f "$PREFIX/ast" ]; then
	OLD_VERSION=$("$PREFIX/ast" version 2>/dev/null | awk '{print $2}' || echo "unknown")
	ACTION="Upgraded from ${OLD_VERSION} to"
fi

mv ast "$PREFIX/ast"
chmod +x "$PREFIX/ast"

echo ""
echo "ast ${VERSION} ${ACTION} → $PREFIX/ast"
if ! echo "$PATH" | grep -q "$PREFIX"; then
	echo "  NOTE: $PREFIX is not on your PATH. Add it, or re-run with:"
	echo "    PREFIX=/usr/local/bin bash install.sh"
fi
echo ""
echo "Verify: $PREFIX/ast version"
