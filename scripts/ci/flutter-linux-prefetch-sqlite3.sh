#!/usr/bin/env bash
# Prefetch sqlite3mc native assets for test-sqlite3mc hooks (pubspec directory: .).
# Profiles: host (default), android, ios-sim. Used by CI and make flutter-linux-prefetch-sqlite3.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FRONTEND="$ROOT/src/frontend"
RELEASE_TAG="sqlite3-3.3.3"
OS_NAME="$(uname -s)"
ARCH="$(uname -m)"

asset_hash() {
  case "$1" in
    libsqlite3mc.arm.android.so) echo "fba142468870f7cc761ef1d71cfb026f7cdbb01fc9fdb38dd61aade0f391be8f" ;;
    libsqlite3mc.arm64.android.so) echo "321408b02ff8b1977b9619329b18b0540d7c86961bba340a46f16665ab3aa956" ;;
    libsqlite3mc.ia32.android.so) echo "62f5b4fdc983a8f70c6e2c265cc6c03705dbba8496c81bc4a08462f8850a7801" ;;
    libsqlite3mc.x64.android.so) echo "8b8d3350191dcecf290edf1984d8abdf79d0a259112af870d8a41dc62f2c3e7d" ;;
    libsqlite3mc.arm64.ios_sim.dylib) echo "d4cc2818e07f2adecc470a3ecb6c9a1f22e441f5d990dfff3180f49bca17279d" ;;
    libsqlite3mc.x64.ios_sim.dylib) echo "f79b66c91064ed7018ef4fa7ace0ece30088780af07c04deaa1ba9d86eb68379" ;;
    libsqlite3mc.arm64.linux.so) echo "a43373bc8d656c36377caf5159d1919b946f41d92dc62e1c325d6b2777c5c398" ;;
    libsqlite3mc.x64.linux.so) echo "70a3ff34a547275edd2a752e309c57cb43a873fede4fc6fa299cca931e6d8932" ;;
    libsqlite3mc.arm64.macos.dylib) echo "60cc6905a9f4b9a09441e35e55a5fb20a90687b334ae6b70cb1217efe0b39c68" ;;
    libsqlite3mc.x64.macos.dylib) echo "fcc1d0d0db31920651bc7e390af55feb3ab1afdc0722e3ed0211df09ba0f018d" ;;
    *) return 1 ;;
  esac
}

host_asset() {
  case "$OS_NAME" in
    Linux)
      case "$ARCH" in
        x86_64) echo "libsqlite3mc.x64.linux.so" ;;
        aarch64|arm64) echo "libsqlite3mc.arm64.linux.so" ;;
        *) echo "unsupported Linux arch: $ARCH" >&2; return 1 ;;
      esac
      ;;
    Darwin)
      case "$ARCH" in
        arm64) echo "libsqlite3mc.arm64.macos.dylib" ;;
        x86_64) echo "libsqlite3mc.x64.macos.dylib" ;;
        *) echo "unsupported macOS arch: $ARCH" >&2; return 1 ;;
      esac
      ;;
    *)
      echo "flutter-linux-prefetch-sqlite3: skip unsupported OS $OS_NAME" >&2
      return 0
      ;;
  esac
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

download_asset() {
  local asset_name="$1"
  local expected url dest tmp actual
  expected="$(asset_hash "$asset_name")" || {
    echo "missing hash for $asset_name" >&2
    return 1
  }
  url="https://github.com/simolus3/sqlite3.dart/releases/download/${RELEASE_TAG}/${asset_name}"
  dest="$FRONTEND/$asset_name"
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' RETURN

  mkdir -p "$FRONTEND"
  if [[ -f "$dest" ]] && [[ "$(sha256_file "$dest")" == "$expected" ]]; then
    echo "sqlite3 prefetch cache hit: $dest"
    return 0
  fi

  for i in 1 2 3 4 5; do
    echo "Downloading sqlite3mc ($i/5): $url"
    if curl -fsSL --retry 3 --retry-delay 10 --connect-timeout 30 -o "$tmp" "$url"; then
      actual="$(sha256_file "$tmp")"
      if [[ "$actual" == "$expected" ]]; then
        mv "$tmp" "$dest"
        echo "Seeded $dest"
        trap - RETURN
        return 0
      fi
      echo "hash mismatch for $asset_name (attempt $i/5)" >&2
    else
      echo "download failed (attempt $i/5)" >&2
    fi
    if [[ $i -eq 5 ]]; then
      return 1
    fi
    sleep 10
  done
}

collect_profiles() {
  local profiles=("$@")
  if [[ ${#profiles[@]} -eq 0 ]]; then
    profiles=(host)
  fi
  local profile asset assets=()
  for profile in "${profiles[@]}"; do
    case "$profile" in
      host)
        asset="$(host_asset)" || return 1
        [[ -n "$asset" ]] && assets+=("$asset")
        ;;
      android)
        assets+=(
          libsqlite3mc.arm.android.so
          libsqlite3mc.arm64.android.so
          libsqlite3mc.ia32.android.so
          libsqlite3mc.x64.android.so
        )
        ;;
      ios-sim)
        assets+=(
          libsqlite3mc.arm64.ios_sim.dylib
          libsqlite3mc.x64.ios_sim.dylib
        )
        ;;
      *)
        echo "unknown sqlite3 prefetch profile: $profile" >&2
        return 1
        ;;
    esac
  done

  local seen=() name
  for name in "${assets[@]}"; do
    local duplicate=0
    for seen_name in "${seen[@]:-}"; do
      if [[ "$seen_name" == "$name" ]]; then
        duplicate=1
        break
      fi
    done
    if [[ $duplicate -eq 0 ]]; then
      seen+=("$name")
      download_asset "$name"
    fi
  done
}

collect_profiles "$@"

cd "$FRONTEND"
flutter pub get
