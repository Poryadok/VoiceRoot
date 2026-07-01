#!/usr/bin/env bash
# Prefetch sqlite3mc native asset on Linux/macOS CI and local make flutter-ci.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FRONTEND="$ROOT/src/frontend"
RELEASE_TAG="sqlite3-3.3.3"
OS_NAME="$(uname -s)"
ARCH="$(uname -m)"

case "$OS_NAME" in
  Linux)
    case "$ARCH" in
      x86_64) ASSET_NAME="libsqlite3mc.x64.linux.so" ;;
      aarch64|arm64) ASSET_NAME="libsqlite3mc.arm64.linux.so" ;;
      *) echo "unsupported Linux arch: $ARCH" >&2; exit 1 ;;
    esac
    ;;
  Darwin)
    case "$ARCH" in
      x86_64) ASSET_NAME="libsqlite3mc.x64.macos.dylib" ;;
      arm64) ASSET_NAME="libsqlite3mc.arm64.macos.dylib" ;;
      *) echo "unsupported macOS arch: $ARCH" >&2; exit 1 ;;
    esac
    ;;
  *)
    echo "flutter-linux-prefetch-sqlite3: skip unsupported OS $OS_NAME" >&2
    exit 0
    ;;
esac

case "$ASSET_NAME" in
  libsqlite3mc.x64.linux.so) EXPECTED="70a3ff34a547275edd2a752e309c57cb43a873fede4fc6fa299cca931e6d8932" ;;
  libsqlite3mc.arm64.linux.so) EXPECTED="a43373bc8d656c36377caf5159d1919b946f41d92dc62e1c325d6b2777c5c398" ;;
  libsqlite3mc.x64.macos.dylib) EXPECTED="fcc1d0d0db31920651bc7e390af55feb3ab1afdc0722e3ed0211df09ba0f018d" ;;
  libsqlite3mc.arm64.macos.dylib) EXPECTED="60cc6905a9f4b9a09441e35e55a5fb20a90687b334ae6b70cb1217efe0b39c68" ;;
  *) echo "missing hash for $ASSET_NAME" >&2; exit 1 ;;
esac

URL="https://github.com/simolus3/sqlite3.dart/releases/download/${RELEASE_TAG}/${ASSET_NAME}"
PREFETCH_DIR="$FRONTEND"
DEST="$PREFETCH_DIR/$ASSET_NAME"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

mkdir -p "$PREFETCH_DIR"
if [[ -f "$DEST" ]] && [[ "$(sha256_file "$DEST")" == "$EXPECTED" ]]; then
  echo "sqlite3 prefetch cache hit: $DEST"
else
  for i in 1 2 3 4 5; do
    echo "Downloading sqlite3mc ($i/5): $URL"
    if curl -fsSL --retry 3 --retry-delay 10 --connect-timeout 30 -o "$TMP" "$URL"; then
      ACTUAL="$(sha256_file "$TMP")"
      if [[ "$ACTUAL" == "$EXPECTED" ]]; then
        mv "$TMP" "$DEST"
        echo "Seeded $DEST"
        break
      fi
      echo "hash mismatch for $ASSET_NAME (attempt $i/5)" >&2
    else
      echo "download failed (attempt $i/5)" >&2
    fi
    if [[ $i -eq 5 ]]; then
      exit 1
    fi
    sleep 10
  done
fi

cd "$FRONTEND"
flutter pub get
