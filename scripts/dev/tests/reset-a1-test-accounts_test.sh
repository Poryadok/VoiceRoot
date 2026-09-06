#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
SCRIPT="$ROOT/scripts/dev/reset-a1-test-accounts.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cat >"$TMP/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$DOCKER_LOG"
if [[ "$1" == "compose" ]] && [[ " $* " == *" ps -q postgres "* ]]; then echo pg-local; exit 0; fi
if [[ "$1" == "compose" ]] && [[ " $* " == *" ps -q auth "* ]]; then exit 0; fi
if [[ "$1" == "compose" ]] && [[ " $* " == *" ps -q user "* ]]; then exit 0; fi
if [[ "$1" == "inspect" ]]; then
  case "$3" in *compose.project*) echo voice ;; *compose.service*) echo postgres ;; *.Config.Image*) echo postgres:16-alpine ;; *.State.Running*) echo false ;; esac
  exit 0
fi
if [[ "$1" == "compose" ]] && [[ " $* " == *" exec -T postgres psql "* ]]; then
  payload="$(cat || true)"
  [[ -z "$payload" ]] || printf '%s\n' "$payload" >>"$DOCKER_LOG"
  echo 'mock psql'
  exit 0
fi
echo "unexpected docker call: $*" >&2; exit 1
EOF
chmod +x "$TMP/docker"
run() { PATH="$TMP:$PATH" DOCKER_LOG="$TMP/log" "$@"; }
run "$SCRIPT" --dry-run >"$TMP/out"
grep -q 'dry run only' "$TMP/out"
if grep -q 'TRUNCATE' "$TMP/log"; then echo 'dry run unexpectedly truncated tables' >&2; exit 1; fi
if run "$SCRIPT" --apply >"$TMP/apply-out" 2>&1; then echo '--apply without acknowledgement unexpectedly succeeded' >&2; exit 1; fi
grep -q 'VOICE_LOCAL_TEST_ACCOUNT_RESET' "$TMP/apply-out"
VOICE_LOCAL_TEST_ACCOUNT_RESET=DELETE_GENERATED_TEST_ACCOUNTS run "$SCRIPT" --apply >"$TMP/confirmed-out"
grep -q 'A1 generated test identities were removed' "$TMP/confirmed-out"
grep -q 'TRUNCATE TABLE account_deletion_operations' "$TMP/log"
grep -q 'TRUNCATE TABLE organization_verification_requests' "$TMP/log"
if run "$SCRIPT" --dry-run --project production >"$TMP/project-out" 2>&1; then echo 'non-Voice project unexpectedly succeeded' >&2; exit 1; fi
grep -q 'refusing non-Voice' "$TMP/project-out"
echo 'All reset-a1-test-accounts tests passed.'
