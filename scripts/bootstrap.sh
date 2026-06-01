#!/usr/bin/env bash
set -euo pipefail

# Lightweight bootstrap script that forwards to the canonical installer.
INSTALL_URL="${HEIMDAL_INSTALL_URL:-https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.sh}"
echo "Running heimdal bootstrap installer"
echo "Source: ${INSTALL_URL}"
curl -fsSL "${INSTALL_URL}" | bash
