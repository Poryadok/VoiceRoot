#!/usr/bin/env bash
# Full compose E2E: all gateway live tests + full Flutter live suite (feature catalog).
# Prerequisites: generated Phase0 fixture + docker-compose.voice-space-media.yml
# over the base app profile, with healthy Gateway and dependencies (docs/DEV_SETUP.md).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_LIVEKIT_PUBLIC_URL="${VOICE_LIVEKIT_PUBLIC_URL:-ws://127.0.0.1:7880}"

# Validate both phases before dependency installation or executing either suite.
GATEWAY_RUN="$(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" full_gateway_run)" || exit 1
flutter_test_list="$(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" full_flutter)" || exit 1
mapfile -t FLUTTER_TESTS <<<"${flutter_test_list}"

if [ "$(uname -s)" = "Linux" ] && ! pkg-config --exists opus 2>/dev/null; then
  echo "Installing LiveKit CGO deps for gateway voice media live tests..."
  sudo apt-get update -qq
  DEBIAN_FRONTEND=noninteractive sudo apt-get install -y -qq libopus-dev libopusfile-dev libsoxr-dev pkg-config
fi

cd "${ROOT}/src/backend/gateway"
gateway_status=0
if go test -count=1 -parallel 1 -timeout 20m -tags live -run "${GATEWAY_RUN}" ./...; then
  echo "Gateway live phase: PASSED"
else
  gateway_status=$?
  echo "Gateway live phase: FAILED (exit code ${gateway_status})" >&2
fi

cd "${ROOT}/src/frontend"
ARGS=()
for f in "${FLUTTER_TESTS[@]}"; do
  ARGS+=("${f}")
done
if [ "${#ARGS[@]}" -eq 0 ] || [ -z "${ARGS[0]}" ]; then
  echo "no full_flutter tests in ${MANIFEST}" >&2
  echo "Flutter live phase: FAILED (empty full_flutter manifest)" >&2
  exit 1
fi
flutter_status=0
if flutter test --concurrency=1 "${ARGS[@]}" \
    --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
    --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}"; then
  echo "Flutter live phase: PASSED"
else
  flutter_status=$?
  echo "Flutter live phase: FAILED (exit code ${flutter_status})" >&2
fi

if (( gateway_status != 0 || flutter_status != 0 )); then
  exit 1
fi
