#!/usr/bin/env sh
set -eu

REPO="${LOXA_REPO:-astraive/loxa}"
INSTALL_DIR="${LOXA_INSTALL_DIR:-$HOME/.local/bin}"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "loxa installer requires $1" >&2
    exit 1
  }
}

need curl
need tar

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  linux) os="linux" ;;
  darwin) os="darwin" ;;
  *) echo "unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

api="https://api.github.com/repos/$REPO/releases"
releases="$TMP_DIR/releases.json"
curl -fsSL "$api" -o "$releases"
cli_line="$(grep -n '"tag_name": "cli/v' "$releases" | head -n 1 | cut -d: -f1)"

if [ -z "$cli_line" ]; then
  echo "could not find a LOXA CLI release" >&2
  exit 1
fi

asset_url="$(
  tail -n +"$cli_line" "$releases" |
    sed -n 's/.*"browser_download_url": "\(.*loxa_.*_'"$os"'_'"$arch"'\.tar\.gz\)".*/\1/p' |
    head -n 1
)"

if [ -z "$asset_url" ]; then
  echo "could not find a LOXA CLI release asset for $os/$arch" >&2
  exit 1
fi

mkdir -p "$INSTALL_DIR"
curl -fsSL "$asset_url" -o "$TMP_DIR/loxa.tar.gz"
tar -xzf "$TMP_DIR/loxa.tar.gz" -C "$TMP_DIR"
install -m 0755 "$TMP_DIR/loxa" "$INSTALL_DIR/loxa"

echo "Installed loxa to $INSTALL_DIR/loxa"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH to run loxa from any shell." ;;
esac
