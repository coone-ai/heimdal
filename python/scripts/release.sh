#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/release.sh <version> [--no-push]

Examples:
  ./scripts/release.sh 0.0.2
  ./scripts/release.sh v0.0.2
  ./scripts/release.sh 0.1.0rc1 --no-push

What it does:
  1) Bumps version in pyproject.toml and src/ailab_eval/__init__.py
  2) Commits the bump
  3) Creates tag: v<version>
  4) Pushes commit + tag (unless --no-push)
EOF
}

if [[ $# -lt 1 || $# -gt 2 ]]; then
  usage
  exit 1
fi

RAW_VERSION="$1"
PUSH=1
if [[ $# -eq 2 ]]; then
  if [[ "$2" == "--no-push" ]]; then
    PUSH=0
  else
    echo "Unknown flag: $2" >&2
    usage
    exit 1
  fi
fi

VERSION="${RAW_VERSION#v}"
if [[ ! "$VERSION" =~ ^[0-9]+(\.[0-9]+){2}([-.][0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "Invalid version: ${RAW_VERSION}" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PYPROJECT="${ROOT_DIR}/pyproject.toml"
INIT_FILE="${ROOT_DIR}/src/ailab_eval/__init__.py"
TAG="v${VERSION}"

if ! git -C "${ROOT_DIR}" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Not inside a git repository: ${ROOT_DIR}" >&2
  exit 1
fi

CURRENT_VERSION="$(sed -n 's/^version = "\(.*\)"/\1/p' "${PYPROJECT}" | head -n 1)"
if [[ -z "${CURRENT_VERSION}" ]]; then
  echo "Could not read current version from pyproject.toml" >&2
  exit 1
fi

if [[ "${CURRENT_VERSION}" == "${VERSION}" ]]; then
  echo "Version is already ${VERSION}; nothing to bump." >&2
  exit 1
fi

if git -C "${ROOT_DIR}" rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "Tag already exists locally: ${TAG}" >&2
  exit 1
fi

if git -C "${ROOT_DIR}" ls-remote --tags origin "refs/tags/${TAG}" | grep -q "${TAG}"; then
  echo "Tag already exists on origin: ${TAG}" >&2
  exit 1
fi

python3 - "$PYPROJECT" "$INIT_FILE" "$VERSION" <<'PY'
import pathlib
import re
import sys

pyproject = pathlib.Path(sys.argv[1])
init_file = pathlib.Path(sys.argv[2])
version = sys.argv[3]

py_text = pyproject.read_text(encoding="utf-8")
py_text_new, py_count = re.subn(
    r'(?m)^version = ".*"$',
    f'version = "{version}"',
    py_text,
    count=1,
)
if py_count != 1:
    raise SystemExit("Failed to update version in pyproject.toml")
pyproject.write_text(py_text_new, encoding="utf-8")

init_text = init_file.read_text(encoding="utf-8")
init_text_new, init_count = re.subn(
    r'(?m)^__version__ = ".*"$',
    f'__version__ = "{version}"',
    init_text,
    count=1,
)
if init_count != 1:
    raise SystemExit("Failed to update __version__ in __init__.py")
init_file.write_text(init_text_new, encoding="utf-8")
PY

git -C "${ROOT_DIR}" add -f "${PYPROJECT}" "${INIT_FILE}"
git -C "${ROOT_DIR}" commit -m "chore(python): release coone-ailab-cli v${VERSION}"
git -C "${ROOT_DIR}" tag -a "${TAG}" -m "coone-ailab-cli ${VERSION}"

if [[ "${PUSH}" -eq 1 ]]; then
  BRANCH="$(git -C "${ROOT_DIR}" branch --show-current)"
  if [[ -z "${BRANCH}" ]]; then
    echo "Detached HEAD: cannot auto-push branch. Push manually." >&2
    exit 1
  fi
  git -C "${ROOT_DIR}" push origin "${BRANCH}"
  git -C "${ROOT_DIR}" push origin "${TAG}"
  echo "Released ${TAG} and pushed to origin."
else
  echo "Prepared ${TAG} locally (commit + tag)."
fi
