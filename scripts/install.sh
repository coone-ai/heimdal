#!/usr/bin/env bash
set -euo pipefail

# Heimdal CLI installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.sh | bash
#
# Optional environment variables:
#   HEIMDAL_REPO      GitHub repo in owner/name format (default: coone-ai/heimdal)
#   HEIMDAL_VERSION   Release tag to install, e.g. v0.0.1 (default: latest)
#   HEIMDAL_INSTALL   Install directory (default: /usr/local/bin)

REPO="${HEIMDAL_REPO:-coone-ai/heimdal}"
BINARY="heimdal"
ALIAS_BINARY="coval"
INSTALL_DIR="${HEIMDAL_INSTALL:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported operating system for install.sh: $OS" >&2
    echo "Use install.ps1 or install.cmd on Windows." >&2
    exit 1
    ;;
esac

if [[ -z "${HEIMDAL_VERSION:-}" ]]; then
  echo "Resolving latest release tag..."
  HEIMDAL_VERSION="$(
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
      | head -n 1
  )"
fi

if [[ -z "${HEIMDAL_VERSION}" ]]; then
  echo "Failed to resolve release tag from GitHub." >&2
  exit 1
fi

ASSET_VERSION="${HEIMDAL_VERSION#v}"
ARCHIVE="${BINARY}_${ASSET_VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${HEIMDAL_VERSION}/${ARCHIVE}"

echo "Downloading ${URL}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

EXTRACTED_BINARY="${TMP_DIR}/${BINARY}"
if [[ ! -f "${EXTRACTED_BINARY}" ]]; then
  echo "Binary not found in archive: ${ARCHIVE}" >&2
  exit 1
fi
chmod +x "${EXTRACTED_BINARY}"

mkdir -p "${INSTALL_DIR}"
TARGET="${INSTALL_DIR}/${BINARY}"
ALIAS_TARGET="${INSTALL_DIR}/${ALIAS_BINARY}"
if [[ -w "${INSTALL_DIR}" ]]; then
  mv "${EXTRACTED_BINARY}" "${TARGET}"
  ln -sf "${TARGET}" "${ALIAS_TARGET}"
else
  echo "Installing to ${INSTALL_DIR} requires sudo..."
  sudo mv "${EXTRACTED_BINARY}" "${TARGET}"
  sudo ln -sf "${TARGET}" "${ALIAS_TARGET}"
fi

echo ""
echo "Installed: ${TARGET}"
echo "Alias: ${ALIAS_TARGET}"
if command -v "${BINARY}" >/dev/null 2>&1; then
  "${BINARY}" version || true
else
  echo "Add this to your shell profile:"
  echo "export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
echo ""
