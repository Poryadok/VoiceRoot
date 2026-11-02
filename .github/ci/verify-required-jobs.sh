#!/usr/bin/env bash
# Verify path-filtered CI jobs that should have run did not skip unexpectedly.
set -euo pipefail

EVENT_NAME="${1:?event name required}"
CODE="${2:-false}"
PROFILE="${3:-auto}"

fail() {
  echo "ci-gate: $*" >&2
  exit 1
}

require_job() {
  local name="$1"
  local result="$2"
  if [[ "${result}" == "skipped" ]]; then
    fail "expected job ${name} to run (result=skipped)"
  fi
  if [[ "${result}" != "success" ]]; then
    fail "job ${name} result=${result}"
  fi
}

if [[ "${EVENT_NAME}" != "pull_request" && "${EVENT_NAME}" != "push" ]]; then
  echo "ci-gate: event outside PR/master-push policy; nothing to verify"
  exit 0
fi

RUN_COMPOSE_E2E="${RUN_COMPOSE_E2E:-false}"
compose_result="${JOB_COMPOSE_E2E:-skipped}"
if [[ "${RUN_COMPOSE_E2E}" == "true" ]]; then
  require_job compose-e2e "${compose_result}"
elif [[ "${compose_result}" != "success" && "${compose_result}" != "skipped" ]]; then
  fail "unexpected compose-e2e result=${compose_result}"
fi

# Job results passed via env JOB_<name_with_underscore>=result
# e.g. JOB_PROTOBUF=success

check_if() {
  local cond="$1"
  local job="$2"
  if [[ "${cond}" == "true" ]]; then
    local env_name="JOB_${job}"
    env_name="${env_name//-/_}"
    env_name="${env_name^^}"
    require_job "${job}" "${!env_name:-skipped}"
  fi
}

or_true() {
  for v in "$@"; do
    [[ "${v}" == "true" ]] && echo true && return 0
  done
  echo false
}

RUN_GO="${RUN_GO:-false}"
RUN_GO_INTEGRATION="${RUN_GO_INTEGRATION:-false}"
RUN_PKG="${RUN_PKG:-false}"
RUN_AUTH="${RUN_AUTH:-false}"
RUN_FLUTTER="${RUN_FLUTTER:-false}"
RUN_WINDOWS_DESKTOP="${RUN_WINDOWS_DESKTOP:-false}"
RUN_WEB="${RUN_WEB:-false}"
RUN_ADMIN="${RUN_ADMIN:-false}"
RUN_PORTAL="${RUN_PORTAL:-false}"
FILTER_AUTH="${FILTER_AUTH:-false}"
FILTER_ADMIN="${FILTER_ADMIN:-false}"
FILTER_PORTAL="${FILTER_PORTAL:-false}"
PROTOS="${PROTOS:-false}"
COMPOSE="${COMPOSE:-false}"
GLOBAL="${GLOBAL:-false}"

if [[ "${PROFILE}" == "full" ]]; then
  RUN_GO=true
  RUN_PKG=true
  RUN_AUTH=true
  RUN_FLUTTER=true
  RUN_WEB=true
fi

# Some acceptance and broad Go jobs are selected even when CODE=false (for
# example ci_global-only workflow edits). Validate them before the code guard.
if [[ "${EVENT_NAME}" == pull_request ]]; then
  require_job current-diff-contract "${JOB_CURRENT_DIFF_CONTRACT:-skipped}"
fi
check_if "${RUN_F13_HISTORICAL_REPLAY:-false}" f13-historical-replay
check_if "${RUN_CHAT_AUTH_JAVA:-false}" chat-auth-java-integration
check_if "$(or_true "${GLOBAL}" "${RUN_CI_SCRIPTS:-false}")" ci-script-tests
check_if "$(or_true "${RUN_GO}" "${GLOBAL}")" golangci
check_if "$(or_true "${RUN_PKG}" "${GLOBAL}")" backend-go-pkg
check_if "$(or_true "${RUN_GO}" "${GLOBAL}")" backend-go
check_if "${RUN_NATS_HUB_CONFIG_RENDERER_IMAGE:-$(or_true "${RUN_PKG}" "${COMPOSE}" "${GLOBAL}")}" nats-hub-config-renderer-image
for job in flutter-key-backup-goldens flutter-android-smoke flutter-web-integration flutter-ios \
  a1-e2e a1-flutter-profile-handoff a1-attachment-restart-proof \
  analytics-clickhouse-integration grafana-analytics-smoke; do
  selector="RUN_${job//-/_}"
  selector="${selector^^}"
  check_if "${!selector:-false}" "${job}"
done

if [[ "${CODE}" != "true" ]]; then
  echo "ci-gate: selected non-code jobs passed; skip code-only jobs"
  exit 0
fi

check_if "$(or_true "${PROTOS}" "${GLOBAL}")" protobuf
check_if "$(or_true "${COMPOSE}" "${GLOBAL}")" compose-config
check_if "${RUN_MATCH_SQUAD_OVERLAY:-false}" match-squad-overlay
check_if "$(or_true "${RUN_FLUTTER}" "${PROTOS}" "${GLOBAL}")" flutter
check_if "$(or_true "${RUN_FLUTTER_MUSIC:-false}" "${PROTOS}" "${GLOBAL}")" flutter-music-proof
check_if "$(or_true "${RUN_FLUTTER_HELP:-false}" "${PROTOS}" "${GLOBAL}")" flutter-help-proof
check_if "$(or_true "${RUN_FLUTTER_STARTUP:-false}" "${PROTOS}" "${GLOBAL}")" flutter-startup-proof
check_if "$(or_true "${RUN_FLUTTER}" "${PROTOS}" "${GLOBAL}")" flutter-unit-tests
check_if "$(or_true "${RUN_FLUTTER}" "${PROTOS}" "${GLOBAL}")" flutter-device-driver
check_if "${RUN_WINDOWS_DESKTOP}" flutter-windows
check_if "$(or_true "${RUN_WEB}" "${RUN_FLUTTER}" "${GLOBAL}")" web
check_if "${RUN_GO_INTEGRATION}" backend-go-integration-pr
check_if "$(or_true "${RUN_AUTH}" "${FILTER_AUTH}" "${GLOBAL}" "${RUN_AUTH_TEST_IMPACT:-false}")" backend-auth
if [[ "${EVENT_NAME}" == pull_request ]]; then
  check_if "$(or_true "${RUN_AUTH}" "${FILTER_AUTH}" "${GLOBAL}")" backend-auth-image
fi
check_if "$(or_true "${RUN_PORTAL}" "${FILTER_PORTAL}" "${GLOBAL}")" developer-portal
check_if "$(or_true "${RUN_ADMIN}" "${FILTER_ADMIN}" "${GLOBAL}")" admin

echo "ci-gate: all required jobs present"
