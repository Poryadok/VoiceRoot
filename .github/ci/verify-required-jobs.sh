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
  if [[ "${result}" == "failure" || "${result}" == "cancelled" ]]; then
    fail "job ${name} result=${result}"
  fi
}

if [[ "${EVENT_NAME}" != "pull_request" ]]; then
  echo "ci-gate: non-PR event; nothing to verify"
  exit 0
fi

if [[ "${CODE}" != "true" ]]; then
  echo "ci-gate: docs-only PR; skip gate"
  exit 0
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
RUN_PKG="${RUN_PKG:-false}"
RUN_AUTH="${RUN_AUTH:-false}"
RUN_FLUTTER="${RUN_FLUTTER:-false}"
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

check_if "$(or_true "${PROTOS}" "${GLOBAL}")" protobuf
check_if "$(or_true "${COMPOSE}" "${GLOBAL}")" compose-config
check_if "$(or_true "${RUN_FLUTTER}" "${PROTOS}" "${GLOBAL}")" flutter
check_if "$(or_true "${RUN_WEB}" "${RUN_FLUTTER}" "${GLOBAL}")" web
check_if "${RUN_GO}" golangci
check_if "${RUN_PKG}" backend-go-pkg
check_if "${RUN_GO}" backend-go
check_if "${RUN_GO}" backend-go-integration-pr
check_if "$(or_true "${RUN_AUTH}" "${FILTER_AUTH}" "${GLOBAL}")" backend-auth
check_if "$(or_true "${RUN_PORTAL}" "${FILTER_PORTAL}" "${GLOBAL}")" developer-portal
check_if "$(or_true "${RUN_ADMIN}" "${FILTER_ADMIN}" "${GLOBAL}")" admin

echo "ci-gate: all required jobs present"
