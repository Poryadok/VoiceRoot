#!/usr/bin/env bash
# Full compose E2E: all gateway live tests + full Flutter live suite (feature catalog).
# Prerequisites: docker compose --profile app up (healthy gateway and dependencies).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_LIVEKIT_PUBLIC_URL="${VOICE_LIVEKIT_PUBLIC_URL:-ws://127.0.0.1:7880}"

if [ "$(uname -s)" = "Linux" ] && ! pkg-config --exists opus 2>/dev/null; then
  echo "Installing LiveKit CGO deps for gateway voice media live tests..."
  sudo apt-get update -qq
  DEBIAN_FRONTEND=noninteractive sudo apt-get install -y -qq libopus-dev libopusfile-dev libsoxr-dev pkg-config
fi

GATEWAY_RUN="$(awk -F"'" '/^full_gateway_run:/ {print $2}' "${MANIFEST}")"
GATEWAY_RUN="${GATEWAY_RUN:-TestCompose.*_live}"

cd "${ROOT}/src/backend/gateway"
go test -count=1 -parallel 1 -timeout 20m -tags live -run "${GATEWAY_RUN}" ./...

mapfile -t FLUTTER_TESTS < <(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" full_flutter)

cd "${ROOT}/src/frontend"
ARGS=()
for f in "${FLUTTER_TESTS[@]}"; do
  ARGS+=("${f}")
done
flutter test --concurrency=1 "${ARGS[@]}" \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}"
