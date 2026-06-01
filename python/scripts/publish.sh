#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "${ROOT_DIR}"

echo "Building coone-ailab-cli package..."
python3 -m pip install --upgrade build twine
python3 -m build

echo "Uploading to PyPI..."
python3 -m twine upload dist/*
