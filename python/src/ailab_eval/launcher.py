from __future__ import annotations

import hashlib
import json
import os
import platform
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
import zipfile
from pathlib import Path
from typing import Optional

DEFAULT_REPO = "coone-ai/heimdal"
API_TIMEOUT_SECONDS = 20
DOWNLOAD_TIMEOUT_SECONDS = 120


def _normalize_os() -> str:
    raw = platform.system().lower()
    if raw == "darwin":
        return "darwin"
    if raw == "linux":
        return "linux"
    if raw == "windows":
        return "windows"
    raise RuntimeError(f"Unsupported operating system: {raw}")


def _normalize_arch() -> str:
    raw = platform.machine().lower()
    if raw in {"x86_64", "amd64"}:
        return "amd64"
    if raw in {"arm64", "aarch64"}:
        return "arm64"
    raise RuntimeError(f"Unsupported architecture: {raw}")


def _default_cache_dir() -> Path:
    custom = os.environ.get("HEIMDAL_INSTALL_CACHE_DIR", "").strip()
    if custom:
        return Path(custom).expanduser()

    if os.name == "nt":
        base = os.environ.get("LOCALAPPDATA") or str(Path.home() / "AppData" / "Local")
        return Path(base) / "coone-ailab-cli" / "cache"

    xdg = os.environ.get("XDG_CACHE_HOME")
    if xdg:
        return Path(xdg) / "coone-ailab-cli"
    return Path.home() / ".cache" / "coone-ailab-cli"


def _ensure_tag_prefix(version: str) -> str:
    version = version.strip()
    if not version:
        raise RuntimeError("Empty version value")
    if version.startswith("v"):
        return version
    return f"v{version}"


def _fetch_latest_tag(repo: str) -> str:
    url = f"https://api.github.com/repos/{repo}/releases/latest"
    req = urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "coone-ailab-cli-bootstrap",
        },
    )
    with urllib.request.urlopen(req, timeout=API_TIMEOUT_SECONDS) as resp:
        payload = json.loads(resp.read().decode("utf-8"))
    tag = str(payload.get("tag_name", "")).strip()
    if not tag:
        raise RuntimeError("GitHub latest release payload does not contain tag_name")
    return _ensure_tag_prefix(tag)


def _resolve_tag(repo: str) -> str:
    env_version = os.environ.get("HEIMDAL_VERSION", "").strip()
    if env_version:
        return _ensure_tag_prefix(env_version)
    return _fetch_latest_tag(repo)


def _archive_name(tag: str, os_name: str, arch: str) -> str:
    ext = "zip" if os_name == "windows" else "tar.gz"
    asset_version = tag[1:] if tag.startswith("v") else tag
    return f"heimdal_{asset_version}_{os_name}_{arch}.{ext}"


def _download(url: str, dest: Path, timeout: int) -> None:
    req = urllib.request.Request(url, headers={"User-Agent": "coone-ailab-cli-bootstrap"})
    with urllib.request.urlopen(req, timeout=timeout) as resp, dest.open("wb") as out:
        shutil.copyfileobj(resp, out)


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _parse_checksum_file(checksum_file: Path, archive_name: str) -> Optional[str]:
    for line in checksum_file.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line or "  " not in line:
            continue
        checksum, filename = line.split("  ", 1)
        if Path(filename.strip()).name == archive_name:
            return checksum.strip().lower()
    return None


def _verify_checksum_if_available(repo: str, tag: str, archive_name: str, archive_path: Path) -> None:
    asset_version = tag[1:] if tag.startswith("v") else tag
    checksum_name = f"heimdal_{asset_version}_checksums.txt"
    checksum_url = f"https://github.com/{repo}/releases/download/{tag}/{checksum_name}"
    with tempfile.TemporaryDirectory(prefix="coone-ailab-cli-checksum-") as td:
        checksum_path = Path(td) / checksum_name
        try:
            _download(checksum_url, checksum_path, timeout=API_TIMEOUT_SECONDS)
        except Exception:
            return

        expected = _parse_checksum_file(checksum_path, archive_name)
        if not expected:
            return

        actual = _sha256_file(archive_path).lower()
        if actual != expected:
            raise RuntimeError(
                f"Checksum verification failed for {archive_name}: expected {expected}, got {actual}"
            )


def _extract_binary(archive_path: Path, target_path: Path, os_name: str) -> None:
    binary_name = "heimdal.exe" if os_name == "windows" else "heimdal"
    with tempfile.TemporaryDirectory(prefix="coone-ailab-cli-extract-") as td:
        temp_dir = Path(td)
        if archive_path.suffix == ".zip":
            with zipfile.ZipFile(archive_path, "r") as zf:
                zf.extractall(temp_dir)
        else:
            with tarfile.open(archive_path, "r:gz") as tf:
                tf.extractall(temp_dir)

        candidates = list(temp_dir.rglob(binary_name))
        if not candidates:
            raise RuntimeError(f"Could not find {binary_name} in downloaded archive")

        source = candidates[0]
        target_path.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(source), str(target_path))

    if os_name != "windows":
        current_mode = target_path.stat().st_mode
        target_path.chmod(current_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)


def _ensure_binary(cache_dir: Path, repo: str, tag: str, os_name: str, arch: str) -> Path:
    binary_name = "heimdal.exe" if os_name == "windows" else "heimdal"
    bin_dir = cache_dir / "binaries" / tag / f"{os_name}-{arch}"
    binary_path = bin_dir / binary_name
    if binary_path.exists():
        return binary_path

    archive = _archive_name(tag, os_name, arch)
    download_url = f"https://github.com/{repo}/releases/download/{tag}/{archive}"
    bin_dir.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix="coone-ailab-cli-download-") as td:
        archive_path = Path(td) / archive
        try:
            _download(download_url, archive_path, timeout=DOWNLOAD_TIMEOUT_SECONDS)
        except urllib.error.HTTPError as exc:
            raise RuntimeError(
                f"Failed to download release asset ({exc.code}): {download_url}"
            ) from exc
        except urllib.error.URLError as exc:
            raise RuntimeError(f"Failed to connect while downloading: {download_url}") from exc

        _verify_checksum_if_available(repo, tag, archive, archive_path)
        _extract_binary(archive_path, binary_path, os_name)

    return binary_path


def _run_binary(binary_path: Path, argv: list[str]) -> int:
    cmd = [str(binary_path), *argv]
    proc = subprocess.run(cmd)
    return int(proc.returncode)


def main() -> None:
    repo = os.environ.get("HEIMDAL_REPO", DEFAULT_REPO).strip() or DEFAULT_REPO
    cache_dir = _default_cache_dir()
    try:
        os_name = _normalize_os()
        arch = _normalize_arch()
        tag = _resolve_tag(repo)
        binary = _ensure_binary(cache_dir, repo, tag, os_name, arch)
        rc = _run_binary(binary, sys.argv[1:])
    except Exception as exc:
        print(f"heimdal bootstrap error: {exc}", file=sys.stderr)
        sys.exit(1)
    sys.exit(rc)
