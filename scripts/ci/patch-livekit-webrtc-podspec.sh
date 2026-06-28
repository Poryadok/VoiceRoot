#!/usr/bin/env bash
# livekit_client 2.8.x podspec pins WebRTC-SDK 144.7559.01; flutter_webrtc 1.5.2+ needs
# 144.7559.09 (Windows CMake fix). Patch until livekit stable ships aligned podspecs.
set -euo pipefail

ROOT="${1:-.}"
FRONTEND="$ROOT/src/frontend"
PUB_CACHE="${PUB_CACHE:-$HOME/.pub-cache}"

if [[ ! -d "$FRONTEND" ]]; then
  echo "patch-livekit-webrtc-podspec: missing $FRONTEND" >&2
  exit 1
fi

LK_VERSION="$(awk '/^  livekit_client:/{f=1} f && /^    version:/{gsub(/"/,"",$2); print $2; exit}' "$FRONTEND/pubspec.lock")"
if [[ -z "$LK_VERSION" ]]; then
  echo "patch-livekit-webrtc-podspec: livekit_client version not found in pubspec.lock" >&2
  exit 1
fi

LK_DIR="$PUB_CACHE/hosted/pub.dev/livekit_client-$LK_VERSION"
if [[ ! -d "$LK_DIR" ]]; then
  echo "patch-livekit-webrtc-podspec: package dir not found: $LK_DIR" >&2
  exit 1
fi

FROM='144.7559.01'
TO='144.7559.09'
for podspec in "$LK_DIR/ios/livekit_client.podspec" "$LK_DIR/macos/livekit_client.podspec"; do
  if [[ ! -f "$podspec" ]]; then
    continue
  fi
  if grep -q "'WebRTC-SDK', '$TO'" "$podspec"; then
    echo "patch-livekit-webrtc-podspec: already aligned in $podspec"
    continue
  fi
  if ! grep -q "'WebRTC-SDK', '$FROM'" "$podspec"; then
    echo "patch-livekit-webrtc-podspec: unexpected WebRTC-SDK pin in $podspec" >&2
    exit 1
  fi
  sed -i.bak "s/'WebRTC-SDK', '$FROM'/'WebRTC-SDK', '$TO'/" "$podspec"
  rm -f "${podspec}.bak"
  echo "patch-livekit-webrtc-podspec: $podspec WebRTC-SDK $FROM -> $TO"
done
