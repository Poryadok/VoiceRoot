#!/usr/bin/env bash
# Compose E2E smoke: one representative live test per product feature (tier 2 / master push).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "${ROOT}/scripts/ci/compose-fcm-diagnostic-lib.sh"
bash "${ROOT}/scripts/ci/compose-fcm-diagnostic-lib-test.sh"
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_REPO_ROOT="${ROOT}"

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

(cd "${ROOT}/src/backend/notification" && go test -count=1 -run '^TestComposeFcmObserverIsAbsentWithoutPrivateBuildTag$' .)
(cd "${ROOT}/src/backend/notification" && go test -tags voice_compose_fcm_diagnostic -count=1 -run '^TestComposeFcmObserver' .)

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
  local original_status=$?
  trap - EXIT
  if [[ -n "${TRACE_FILE}" ]]; then
    if ! voice_fcm_diag_cleanup "${TRACE_FILE}" "${TRACE_DIR}"; then
      echo 'compose_fcm_cleanup=failed' >&2
      if ((original_status == 0)); then
        original_status=1
      fi
    fi
  fi
  exit "${original_status}"
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

# Rebuild only the disposable Notification service with its private diagnostic
# tag. The numeric runner identity must read the private control file but cannot
# write through the read-only bind mount. Abort before the FCM test if it fails.
if [[ -z "${TRACE_FILE}" ]]; then
  echo 'compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
VOICE_FCM_DIAG_UID="$(id -u)"
VOICE_FCM_DIAG_GID="$(id -g)"
if ! voice_fcm_diag_identity_valid "${VOICE_FCM_DIAG_UID}" "${VOICE_FCM_DIAG_GID}"; then
  echo 'compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
export VOICE_FCM_DIAG_UID VOICE_FCM_DIAG_GID VOICE_FCM_DIAGNOSTIC_DIR="${TRACE_DIR}"
chmod 700 "${TRACE_DIR}"
chmod 600 "${TRACE_FILE}"
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" build notification >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" run --rm --no-deps --entrypoint /bin/sh notification -ec 'test -r /run/voice-fcm/correlation.json; ! (printf x >>/run/voice-fcm/correlation.json) 2>/dev/null; ! (touch /run/voice-fcm/write-probe) 2>/dev/null' >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
echo 'compose_fcm_preflight=pass'
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" up -d --no-deps --force-recreate --wait --wait-timeout 60 notification >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi

set +e
flutter test --concurrency=1 "${ARGS[@]}" \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}" \
  --dart-define=VOICE_FCM_DIAGNOSTIC_FILE="${TRACE_FILE}"
flutter_status=$?
set -e

if ((flutter_status != 0)) && [[ -n "${TRACE_FILE}" && -s "${TRACE_FILE}" ]]; then
  diagnostic_logs="$(timeout 5s docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" logs --no-color --no-log-prefix --tail 100 notification 2>/dev/null | head -c 65537 || true)"
  diagnostic_line="$(voice_fcm_diag_parse_log "${diagnostic_logs}")"
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
      if [[ "${line}" =~ ^compose_fcm_probe\ available=(true|false)\ stage=[a-z_]+\ event_match_count=[0-9]+\ event_scan=(complete_retained|bounded|unknown)\ consumer_info_available=(true|false|unknown)\ delivered_seq_ge_event=(true|false|unknown)\ ack_floor_seq_ge_event=(true|false|unknown)$ ]]; then
        probe_line="${line}"
      fi
    done <<<"${probe_output}"
    if [[ -n "${probe_line}" ]]; then
      printf '%s\n' "${probe_line}"
    elif ((probe_status != 0)); then
      echo 'compose_fcm_probe available=false stage=probe_failed event_match_count=0 event_scan=unknown consumer_info_available=unknown delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
    else
      echo 'compose_fcm_probe available=false stage=probe_output_unavailable event_match_count=0 event_scan=unknown consumer_info_available=unknown delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
    fi
  else
    echo 'compose_fcm_probe available=false stage=port_lookup event_match_count=0 event_scan=unknown consumer_info_available=unknown delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown'
  fi
fi

cleanup_failed=false
if [[ -n "${TRACE_FILE}" ]]; then
  if voice_fcm_diag_cleanup "${TRACE_FILE}" "${TRACE_DIR}"; then
    TRACE_FILE=''
    TRACE_DIR=''
  else
    cleanup_failed=true
    echo 'compose_fcm_cleanup=failed' >&2
  fi
fi
trap - EXIT
if [[ -n "${diagnostic_line:-}" ]]; then
  if [[ "${cleanup_failed}" == true ]]; then
    echo "${VOICE_FCM_DIAG_UNKNOWN}"
  else
    printf '%s\n' "${diagnostic_line}"
  fi
fi

if [[ "${cleanup_failed}" == true && "${flutter_status}" == 0 ]]; then
  exit 1
fi

if voice_fcm_diag_preserve_status "${flutter_status}"; then
  exit 0
else
  preserved_status=$?
  exit "${preserved_status}"
fi
