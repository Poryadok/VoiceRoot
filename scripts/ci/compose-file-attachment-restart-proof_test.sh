#!/usr/bin/env bash
# Offline contract test for attachment-proof startup diagnostics.
# The fixture never reaches Docker, Go, curl, or the network.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
SCRIPT="$ROOT/scripts/ci/compose-file-attachment-restart-proof.sh"
REAL_BASH="${BASH:-$(command -v bash)}"

fail() { echo "FAIL: $*" >&2; exit 1; }
assert_eq() { [[ "$1" == "$2" ]] || fail "expected '$2', got '$1'"; }
assert_contains() { grep -Eq -- "$2" "$1" || { cat "$1" >&2 || true; fail "expected $2"; }; }
assert_not_contains() { ! grep -Eq -- "$2" "$1" || { cat "$1" >&2 || true; fail "did not expect $2"; }; }

make_fake_tools() {
  local bin="$1"
  mkdir -p "$bin"
  cat >"$bin/bash" <<'EOF'
#!/bin/sh
set -eu
if [ "${1:-}" = "${FAKE_MANIFEST_SCRIPT:-}" ]; then
  printf '%s\n' TestComposeFileAttachmentRestartProof_live
  exit 0
fi
exec "${REAL_BASH:?REAL_BASH required}" "$@"
EOF
  cat >"$bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
log="${FAKE_LOG:?FAKE_LOG required}"
printf 'docker' >>"$log"
for arg in "$@"; do printf ' <%s>' "$arg" >>"$log"; done
printf '\n' >>"$log"

if [[ "${1:-}" == ps ]]; then
  # Collision and occupied-port checks must pass without creating containers.
  exit 0
fi
if [[ "${1:-}" == compose ]]; then
  line="$*"
  if [[ "$line" == *' config '* ]]; then exit 0; fi
  if [[ "$line" == *' up '* ]]; then exit "${FAKE_UP_RC:?FAKE_UP_RC required}"; fi
  if [[ "$line" == *' ps '* ]]; then
    echo fake-compose-ps >&2
    exit "${FAKE_PS_RC:-0}"
  fi
  if [[ "$line" == *' logs '* ]]; then
    echo fake-compose-db-init-logs >&2
    exit "${FAKE_LOGS_RC:-0}"
  fi
  if [[ "$line" == *' down '* ]]; then exit "${FAKE_DOWN_RC:-0}"; fi
  exit 0
fi
exit 0
EOF
  chmod +x "$bin/bash" "$bin/docker"
}

new_case() {
  local work
  work="$(mktemp -d "${TEST_TMP}/attachment-${1}.XXXXXXXX")"
  mkdir -p "$work/bin" "$work/tmp"
  make_fake_tools "$work/bin"
  printf '%s\n' "$work"
}

run_runner() {
  local work="$1"; shift
  : >"$work/commands.log"
  set +e
  (
    cd "$work/tmp"
    env PATH="$work/bin:$PATH" REAL_BASH="$REAL_BASH" FAKE_LOG="$work/commands.log" \
      FAKE_MANIFEST_SCRIPT="$ROOT/scripts/ci/e2e-manifest.sh" \
      TMPDIR="$work/tmp" VOICE_FILE_ATTACHMENT_RESTART_PORT_BASE=25000 \
      "$@" "$REAL_BASH" "$SCRIPT" >"$work/stdout" 2>"$work/stderr"
  )
  local rc=$?
  set -e
  printf '%s\n' "$rc" >"$work/rc"
  return 0
}

TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/attachment-runner-tests.XXXXXXXX")"
trap 'rm -rf -- "$TEST_TMP"' EXIT
[[ -f "$SCRIPT" ]] || fail "missing file: $SCRIPT"

echo '== initial Compose failure preserves exact status and emits diagnostics =='
case_dir="$(new_case startup-failure)"
FAKE_UP_RC=37 run_runner "$case_dir"
assert_eq "$(cat "$case_dir/rc")" 37
assert_contains "$case_dir/commands.log" 'compose.*<up> <-d> <--build>'
assert_contains "$case_dir/commands.log" 'compose.*<ps> <--all>'
assert_contains "$case_dir/commands.log" 'compose.*<logs> <--no-color> <--timestamps> <compose-db-init>'
ps_line="$(grep -n 'compose.*<ps> <--all>' "$case_dir/commands.log" | cut -d: -f1)"
logs_line="$(grep -n 'compose.*<logs>.*<compose-db-init>' "$case_dir/commands.log" | cut -d: -f1)"
(( ps_line < logs_line )) || fail 'ps --all must precede compose-db-init logs'

echo '== diagnostic failures and cleanup failure cannot replace original status =='
case_dir="$(new_case diagnostic-failure)"
FAKE_UP_RC=37 FAKE_PS_RC=71 FAKE_LOGS_RC=72 FAKE_DOWN_RC=73 SECRET_SENTINEL='do-not-leak-attachment-secret' \
  VOICE_FILE_ATTACHMENT_RESTART_CLEANUP=true run_runner "$case_dir"
assert_eq "$(cat "$case_dir/rc")" 37
assert_contains "$case_dir/commands.log" 'compose.*<ps> <--all>'
assert_contains "$case_dir/commands.log" 'compose.*<logs> <--no-color> <--timestamps> <compose-db-init>'
assert_contains "$case_dir/commands.log" 'compose.*<down> <--remove-orphans>'
assert_not_contains "$case_dir/commands.log" '--volumes|<-v>|volume prune|system prune'
assert_not_contains "$case_dir/stdout" 'do-not-leak-attachment-secret'
assert_not_contains "$case_dir/stderr" 'do-not-leak-attachment-secret'
ps_line="$(grep -n 'compose.*<ps> <--all>' "$case_dir/commands.log" | cut -d: -f1)"
logs_line="$(grep -n 'compose.*<logs>.*<compose-db-init>' "$case_dir/commands.log" | cut -d: -f1)"
down_line="$(grep -n 'compose.*<down> <--remove-orphans>' "$case_dir/commands.log" | cut -d: -f1)"
(( ps_line < logs_line && logs_line < down_line )) || fail 'diagnostics must run ps, then logs, before cleanup'

identity_fields() {
  local line="$1"
  printf '%s\n' "$line" | sed -n \
    's/.* <--env-file> <\([^>]*\)> <--project-name> <\([^>]*\)> <--project-directory> <\([^>]*\)> <-f> <\([^>]*\)>.*/\1|\2|\3|\4/p'
}
identity="$(identity_fields "$(grep -m1 '^docker <compose> ' "$case_dir/commands.log")")"
IFS='|' read -r identity_env identity_project identity_directory identity_file <<<"$identity"
[[ "$identity_env" == /* && "$identity_directory" == /* && "$identity_file" == /* ]] || fail "unsafe Compose identity: $identity"
[[ "$identity_project" =~ ^voice-a1-file-restart-[a-z0-9-]+$ ]] || fail "unexpected Compose project: $identity_project"
[[ "$identity_project" != voice ]] || fail 'Compose calls must never target shared voice project'
while IFS= read -r compose_line; do
  [[ "$(identity_fields "$compose_line")" == "$identity" ]] || fail 'Compose identity changed between startup, diagnostics, and cleanup'
done < <(grep '^docker <compose> ' "$case_dir/commands.log")

echo '== cleanup is disabled by default =='
case_dir="$(new_case no-cleanup)"
FAKE_UP_RC=37 run_runner "$case_dir"
assert_eq "$(cat "$case_dir/rc")" 37
assert_not_contains "$case_dir/commands.log" 'compose.*<down>'

echo 'All compose-file-attachment-restart-proof tests passed.'
