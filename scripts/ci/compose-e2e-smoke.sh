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
  "ratelimit:OTP:*" \
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

TRACE_DIR=''
TRACE_FILE=''
cleanup_fcm_diagnostic() {
  if [[ -n "${TRACE_FILE}" ]]; then
    rm -f -- "${TRACE_FILE}"
    rmdir -- "${TRACE_DIR}" 2>/dev/null || true
  fi
}
trap cleanup_fcm_diagnostic EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if TRACE_DIR="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/voice-fcm-diagnostic.XXXXXX" 2>/dev/null)"; then
  TRACE_FILE="${TRACE_DIR}/correlation.json"
  if ! (umask 077 && : >"${TRACE_FILE}"); then
    rm -f -- "${TRACE_FILE}"
    rmdir -- "${TRACE_DIR}" 2>/dev/null || true
    TRACE_DIR=''
    TRACE_FILE=''
  fi
fi

set +e
flutter test --concurrency=1 "${ARGS[@]}" \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}" \
  --dart-define=VOICE_FCM_DIAGNOSTIC_FILE="${TRACE_FILE}"
flutter_status=$?
set -e

if ((flutter_status != 0)) && [[ -n "${TRACE_FILE}" && -s "${TRACE_FILE}" ]]; then
  port_mapping="$(docker compose -f "${ROOT}/docker-compose.yml" port nats 4222 2>/dev/null || true)"
  mapped_port=''
  if [[ "${port_mapping}" =~ :([0-9]+)$ ]]; then
    mapped_port="${BASH_REMATCH[1]}"
  fi
  if [[ "${mapped_port}" =~ ^[0-9]{1,5}$ ]] && ((10#${mapped_port} > 0 && 10#${mapped_port} <= 65535)); then
    set +e
    probe_output="$(cd "${ROOT}/src/backend/notification" && \
      NATS_URL="nats://127.0.0.1:${mapped_port}" \
      VOICE_FCM_DIAGNOSTIC_FILE="${TRACE_FILE}" \
      go test -v -count=1 -run '^TestComposeFcmEventCorrelationProbe$' . 2>&1)"
    probe_status=$?
    set -e
    probe_line=''
    while IFS= read -r line; do
      if [[ "${line}" =~ ^compose_fcm_probe\ available=(true|false)\ stage=[a-z_]+\ event_match_count=[0-9]+\ event_scan=(complete_retained|bounded|unknown)\ consumer_info_available=(true|false)\ delivered_seq_ge_event=(true|false|unknown)\ ack_floor_seq_ge_event=(true|false|unknown)$ ]]; then
        probe_line="${line}"
      fi
    done <<<"${probe_output}"
    if [[ -n "${probe_line}" ]]; then
      printf '%s\n' "${probe_line}"
    elif ((probe_status != 0)); then
      echo 'compose_fcm_probe available=false stage=probe_failed event_match_count=0 event_scan=unknown consumer_info_available=false delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
    else
      echo 'compose_fcm_probe available=false stage=probe_output_unavailable event_match_count=0 event_scan=unknown consumer_info_available=false delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
    fi
  else
    echo 'compose_fcm_probe available=false stage=port_lookup event_match_count=0 event_scan=unknown consumer_info_available=false delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
  fi
fi

exit "${flutter_status}"
