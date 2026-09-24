#!/usr/bin/env bash
# Exercise both live phases without starting Compose or running the real suites.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
scratch="$(mktemp -d)"
trap 'rm -rf "${scratch}"' EXIT

cat >"${scratch}/go" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${PHASE_LOG}/go"
exit "${FAKE_GO_STATUS}"
EOF
cat >"${scratch}/flutter" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${PHASE_LOG}/flutter"
exit "${FAKE_FLUTTER_STATUS}"
EOF
cat >"${scratch}/pkg-config" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "${scratch}/go" "${scratch}/flutter" "${scratch}/pkg-config"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

for go_status in 0 17; do
  for flutter_status in 0 23; do
    case_dir="${scratch}/${go_status}-${flutter_status}"
    mkdir "${case_dir}"
    export PHASE_LOG="${case_dir}" FAKE_GO_STATUS="${go_status}" FAKE_FLUTTER_STATUS="${flutter_status}"
    if output="$(PATH="${scratch}:${PATH}" bash "${ROOT}/scripts/ci/compose-e2e-live.sh" 2>&1)"; then
      result=0
    else
      result=$?
    fi

    expected=0
    if (( go_status != 0 || flutter_status != 0 )); then expected=1; fi
    [[ "${result}" -eq "${expected}" ]] || fail "exit ${result}, expected ${expected} for Go=${go_status} Flutter=${flutter_status}: ${output}"
    [[ "$(wc -l <"${case_dir}/go")" -eq 1 ]] || fail "Go phase did not run once"
    [[ "$(wc -l <"${case_dir}/flutter")" -eq 1 ]] || fail "Flutter phase did not run once"
    grep -F -- '-tags live -run TestCompose.*_live ./...' "${case_dir}/go" >/dev/null || fail "Go live selector changed"
    grep -F -- '--dart-define=VOICE_RUN_LIVE_INTEGRATION=true' "${case_dir}/flutter" >/dev/null || fail "Flutter live mode missing"
    grep -F -- 'test/gateway_dm_ws_live_integration_test.dart' "${case_dir}/flutter" >/dev/null || fail "full_flutter list missing"
    gateway_summary='Gateway live phase: PASSED'
    flutter_summary='Flutter live phase: PASSED'
    if (( go_status != 0 )); then gateway_summary="Gateway live phase: FAILED (exit code ${go_status})"; fi
    if (( flutter_status != 0 )); then flutter_summary="Flutter live phase: FAILED (exit code ${flutter_status})"; fi
    [[ "${output}" == *"${gateway_summary}"* ]] || fail "incorrect Go phase summary: ${output}"
    [[ "${output}" == *"${flutter_summary}"* ]] || fail "incorrect Flutter phase summary: ${output}"
  done
done

echo "Compose E2E live phase tests passed."
