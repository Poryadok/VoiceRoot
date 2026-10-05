#!/usr/bin/env bash
# Compose E2E smoke: one representative live test per product feature (tier 2 / master push).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "${ROOT}/scripts/ci/compose-fcm-diagnostic-lib.sh"
if bash "${ROOT}/scripts/ci/compose-fcm-diagnostic-lib-test.sh"; then
  printf 'compose_e2e_pre_echo_stage=diagnostic_contract status=passed exit=0\n'
else
  contract_exit=$?
  printf 'compose_e2e_pre_echo_stage=diagnostic_contract status=failed exit=%d\n' "${contract_exit}"
  exit "${contract_exit}"
fi
MANIFEST="${ROOT}/.github/ci/e2e-features.yml"
export VOICE_RUN_LIVE_COMPOSE="${VOICE_RUN_LIVE_COMPOSE:-true}"
export VOICE_API_BASE_URL="${VOICE_API_BASE_URL:-http://127.0.0.1:18080}"
export VOICE_REPO_ROOT="${ROOT}"

mapfile -t GATEWAY_TESTS < <(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" smoke_gateway)
gateway_manifest_pid=$!
if wait "${gateway_manifest_pid}"; then
  printf 'compose_e2e_pre_echo_stage=gateway_manifest status=completed exit=0\n'
else
  gateway_manifest_exit=$?
  printf 'compose_e2e_pre_echo_stage=gateway_manifest status=failed exit=%d\n' "${gateway_manifest_exit}"
fi

mapfile -t FLUTTER_TESTS < <(bash "${ROOT}/scripts/ci/e2e-manifest.sh" "${MANIFEST}" smoke_flutter)
flutter_manifest_pid=$!
if wait "${flutter_manifest_pid}"; then
  printf 'compose_e2e_pre_echo_stage=flutter_manifest status=completed exit=0\n'
else
  flutter_manifest_exit=$?
  printf 'compose_e2e_pre_echo_stage=flutter_manifest status=failed exit=%d\n' "${flutter_manifest_exit}"
fi

if ((${#GATEWAY_TESTS[@]} == 0)); then
  printf 'compose_e2e_pre_echo_stage=gateway_empty_list status=failed exit=1\n'
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
STATUS_FILE=''
notification_container=''
cleanup_fcm_diagnostic() {
  local original_status=$?
  trap - EXIT
  if [[ -n "${TRACE_FILE}" ]]; then
    if ! voice_fcm_diag_cleanup "${TRACE_FILE}" "${TRACE_DIR}" "${STATUS_FILE}"; then
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
  STATUS_FILE="${TRACE_DIR}/status.txt"
  if ! (umask 077 && : >"${TRACE_FILE}" && printf 'pre=unknown\npost=unknown\n' >"${STATUS_FILE}"); then
    echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
    exit 1
  fi
fi

# Rebuild only the disposable Notification service with its private diagnostic
# tag. The numeric runner identity must read the private control file but cannot
# write through the read-only bind mount. Abort before the FCM test if it fails.
if [[ -z "${TRACE_FILE}" ]]; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
VOICE_FCM_DIAG_UID="$(id -u)"
VOICE_FCM_DIAG_GID="$(id -g)"
if ! voice_fcm_diag_identity_valid "${VOICE_FCM_DIAG_UID}" "${VOICE_FCM_DIAG_GID}"; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
export VOICE_FCM_DIAG_UID VOICE_FCM_DIAG_GID VOICE_FCM_DIAGNOSTIC_DIR="${TRACE_DIR}"
chmod 700 "${TRACE_DIR}"
chmod 600 "${TRACE_FILE}"
chmod 600 "${STATUS_FILE}"
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" build notification >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" run --rm --no-deps --entrypoint /bin/sh notification -ec 'test -r /run/voice-fcm/correlation.json; ! (printf x >>/run/voice-fcm/correlation.json) 2>/dev/null; ! (touch /run/voice-fcm/write-probe) 2>/dev/null' >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
echo 'compose_fcm_preflight=pass'
set +e
previous_notification_containers="$(docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" ps -q notification 2>/dev/null)"
previous_lookup_status=$?
set -e
if ((previous_lookup_status != 0)); then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
while IFS= read -r previous_container; do
  [[ -z "${previous_container}" ]] && continue
  if [[ ! "${previous_container}" =~ ^[a-f0-9]{64}$ ]]; then
    echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
    exit 1
  fi
done <<<"${previous_notification_containers}"
if ! docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" up -d --no-deps --force-recreate --wait --wait-timeout 60 notification >/dev/null 2>&1; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi

# Bind this run to one freshly recreated disposable service instance. Keep the
# opaque identity in memory only; never include it in diagnostics.
set +e
notification_container="$(docker compose -f "${ROOT}/docker-compose.yml" -f "${ROOT}/scripts/ci/compose-fcm-diagnostic.yml" ps -q notification 2>/dev/null)"
container_lookup_status=$?
set -e
if ((container_lookup_status != 0)) || [[ ! "${notification_container}" =~ ^[a-f0-9]{64}$ ]] || \
    { [[ -n "${previous_notification_containers}" ]] && printf '%s\n' "${previous_notification_containers}" | grep -Fxq -- "${notification_container}"; }; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi

set +e
diagnostic_since="$(date -u +%Y-%m-%dT%H:%M:%S.%NZ)"
date_status=$?
set -e
if ((date_status != 0)) || ! voice_fcm_diag_since_valid "${diagnostic_since}"; then
  echo 'compose_fcm_diag valid=false reason=unknown admission=none candidates=0 attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown'
  exit 1
fi
set +e
flutter test --concurrency=1 "${ARGS[@]}" \
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true \
  --dart-define=VOICE_API_BASE_URL="${VOICE_API_BASE_URL}" \
  --dart-define=VOICE_FCM_DIAGNOSTIC_FILE="${TRACE_FILE}" \
  --dart-define=VOICE_FCM_DIAGNOSTIC_STATUS_FILE="${STATUS_FILE}"
flutter_status=$?
set -e

if ((flutter_status != 0)) && [[ -n "${TRACE_FILE}" ]]; then
  set +e
  set -o pipefail
  diagnostic_logs="$(timeout 5s docker logs --since "${diagnostic_since}" "${notification_container}" 2>/dev/null | head -c 65537)"
  diagnostic_logs_status=$?
  status_read_status=1
  status_data=''
  diagnostic_summary=''
  if [[ -f "${STATUS_FILE}" && ! -L "${STATUS_FILE}" ]] && [[ "$(stat -c '%a' -- "${STATUS_FILE}" 2>/dev/null)" == 600 ]]; then
    status_data="$(head -c 66 -- "${STATUS_FILE}" 2>/dev/null | { cat; printf '\001'; })"
    status_read_status=$?
    status_data="${status_data%$'\001'}"
  fi
  set -e
  voice_fcm_diag_parse_collected_log "${diagnostic_logs_status}" "${diagnostic_logs}" diagnostic_line
  if ! voice_fcm_diag_normalize_collected \
      "${status_read_status}" "${status_data}" \
      "${diagnostic_logs_status}" "${diagnostic_logs}" \
      "${VOICE_FCM_DIAG_PARSE_RESULT}" diagnostic_summary; then
    if [[ "${VOICE_FCM_DIAG_PARSE_RESULT}" == accepted || "${VOICE_FCM_DIAG_PARSE_RESULT}" == missing ]]; then
      VOICE_FCM_DIAG_PARSE_RESULT=unknown
    fi
    voice_fcm_diag_emit "${VOICE_FCM_DIAG_UNKNOWN}" diagnostic_line
  fi
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
  if voice_fcm_diag_cleanup "${TRACE_FILE}" "${TRACE_DIR}" "${STATUS_FILE}"; then
    TRACE_FILE=''
    TRACE_DIR=''
    STATUS_FILE=''
  else
    cleanup_failed=true
    echo 'compose_fcm_cleanup=failed' >&2
  fi
fi
trap - EXIT
if [[ -n "${diagnostic_line:-}" ]]; then
  if [[ "${cleanup_failed}" == true ]]; then
    VOICE_FCM_DIAG_PARSE_RESULT=unknown
    diagnostic_summary=''
    diagnostic_line="${VOICE_FCM_DIAG_UNKNOWN}"
  fi
  echo "compose_fcm_parse_result=${VOICE_FCM_DIAG_PARSE_RESULT}"
  if [[ "${cleanup_failed}" == true ]]; then
    echo "${VOICE_FCM_DIAG_UNKNOWN}"
  else
    if [[ -n "${diagnostic_summary:-}" ]]; then
      printf '%s\n' "${diagnostic_summary}"
    fi
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
