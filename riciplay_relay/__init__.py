"""proxy-relay — lightweight peer-to-peer IP relay.

On first run, downloads the correct Go binary for this platform from GitHub
releases, caches it in ~/.proxy-relay/bin/, and delegates all CLI arguments
to it.

Usage via pip:  pip install proxy-relay
Usage direct:   python -m proxy_relay relay|serve|connect [...]
"""

import os
import platform
import shutil
import subprocess
import sys
import urllib.request
from pathlib import Path

VERSION = "0.1.0"
REPO = "Zaidux/proxy-relay"
BINARY_NAME = "proxy-relay"

_BIN_DIR = Path.home() / ".proxy-relay" / "bin"
_BIN_DIR.mkdir(parents=True, exist_ok=True)

_ARCH_MAP = {
    ("linux", "x86_64"): "linux-amd64",
    ("linux", "amd64"): "linux-amd64",
    ("linux", "aarch64"): "linux-arm64",
    ("linux", "arm64"): "linux-arm64",
    ("darwin", "x86_64"): "darwin-amd64",
    ("darwin", "amd64"): "darwin-amd64",
    ("darwin", "arm64"): "darwin-arm64",
    ("darwin", "aarch64"): "darwin-arm64",
    ("win32", "amd64"): "windows-amd64",
    ("win32", "x86_64"): "windows-amd64",
    ("android", "arm64"): "linux-arm64",
    ("android", "aarch64"): "linux-arm64",
}


def _get_arch() -> str:
    return _ARCH_MAP.get((sys.platform, platform.machine().lower()), "linux-amd64")


def _binary_path() -> Path:
    return _BIN_DIR / BINARY_NAME


def _download_url() -> str:
    arch = _get_arch()
    ext = ".exe" if sys.platform == "win32" else ""
    return (
        f"https://github.com/{REPO}/releases/latest/download/"
        f"{BINARY_NAME}-{arch}{ext}"
    )


def _ensure_binary() -> Path:
    dest = _binary_path()
    if dest.exists():
        return dest

    url = _download_url()
    sys.stderr.write(f"proxy-relay: downloading {url}\n")

    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": f"proxy-relay-python/{VERSION}",
            "Accept": "application/octet-stream",
        })
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = resp.read()
    except Exception as exc:
        sys.stderr.write(
            f"proxy-relay: failed to download binary: {exc}\n"
            f"Build from source: go install github.com/{REPO}@latest\n"
        )
        sys.exit(1)

    dest.write_bytes(data)
    dest.chmod(0o755)
    sys.stderr.write(f"proxy-relay: installed to {dest}\n")
    return dest


def main():
    binary = _ensure_binary()

    # Handle `python -m proxy_relay` with no args
    args = sys.argv[1:]
    if not args:
        args = ["--help"]

    os.execv(str(binary), [BINARY_NAME] + args)
