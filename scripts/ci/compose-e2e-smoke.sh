#!/usr/bin/env bash
# Compose E2E smoke: one representative live test per product feature (tier 2 / master push).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"

mapfile -t GATEWAY_TESTS < <(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" smoke_gateway)
mapfile -t FLUTTER_TESTS < <(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" smoke_flutter)

if ((${#GATEWAY_TESTS[@]} == 0)); then
  echo "no smoke gateway tests in ${MANIFEST}" >&2
  exit 1
fi

GATEWAY_RUN=''
for t in "${GATEWAY_TESTS[@]}"; do
  if [ -n "${GATEWAY_RUN}" ]; then
    GATEWAY_RUN+='|'
  fi
  GATEWAY_RUN+="${t}"
done

echo "Smoke gateway tests (${#GATEWAY_TESTS[@]}): ${GATEWAY_RUN}"

cd "${ROOT}/src/backend/gateway"
go test -count=1 -parallel 1 -timeout 20m -run "${GATEWAY_RUN}" ./...

echo "Clearing compose Redis rate limits before Flutter smoke..."
for pattern in \
  "ratelimit:AuthLogin:*" \
  "ratelimit:AuthRegister:*" \
  "ratelimit:Auth:*" \
  "ratelimit:FileUpload:*"; do
  mapfile -t _rl_keys < <(
    docker compose -f "${ROOT}/docker-compose.yml" exec -T redis redis-cli --scan --pattern "${pattern}" 2>/dev/null || true
  )
  for key in "${_rl_keys[@]}"; do
    if [ -n "${key}" ]; then
      docker compose -f "${ROOT}/docker-compose.yml" exec -T redis redis-cli DEL "${key}" >/dev/null 2>&1 || true
    fi
  done
done

cd "${ROOT}/src/frontend"
ARGS=()
for f in "${FLUTTER_TESTS[@]}"; do
  ARGS+=("${f}")
done
flutter test --concurrency=1 "${ARGS[@]}" \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}"
