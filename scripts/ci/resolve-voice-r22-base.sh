#!/usr/bin/env bash
# Resolve the event commit used by the R22.2 source-scope oracle.
set -euo pipefail

event_name="${VOICE_CI_EVENT_NAME:-}"
pr_base_sha="${VOICE_CI_PR_BASE_SHA:-}"
push_before_sha="${VOICE_CI_PUSH_BEFORE_SHA:-}"
zero_sha='0000000000000000000000000000000000000000'

is_nonzero_sha() {
  local value="$1"
  [[ "${value}" =~ ^[0-9a-fA-F]{40}$ && "${value,,}" != "${zero_sha}" ]]
}

case "${event_name}" in
  pull_request)
    if ! is_nonzero_sha "${pr_base_sha}"; then
      echo 'pull_request event is missing a valid base SHA' >&2
      exit 2
    fi
    printf '%s\n' "${pr_base_sha}"
    ;;
  push)
    if is_nonzero_sha "${push_before_sha}"; then
      printf '%s\n' "${push_before_sha}"
    fi
    ;;
  *)
    # An empty value makes the oracle derive merge-base HEAD origin/master.
    ;;
esac
