#!/usr/bin/env bash
# Static exact-head CI reachability contract for the isolated A1 attachment proof.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORKFLOW="$ROOT/.github/workflows/ci.yml"
MAKEFILE="$ROOT/Makefile"
FILTERS="$ROOT/.github/ci/path-filters.yml"
JOB='a1-attachment-restart-proof'

fail() { echo "FAIL: $*" >&2; exit 1; }

job_block() {
  local wanted="$1"
  awk -v wanted="$wanted" '
    $0 ~ "^  " wanted ":[[:space:]]*$" { found = 1; print; next }
    found && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ { exit }
    found { print }
  ' "$WORKFLOW"
}

job_file="$(mktemp)"
make_target_file="$(mktemp)"
filter_file="$(mktemp)"
trap 'rm -f "$job_file" "$make_target_file" "$filter_file"' EXIT
job_block "$JOB" >"$job_file"
[[ -s "$job_file" ]] || fail "missing ${JOB} workflow block"

grep -Eq '^[[:space:]]*needs:[[:space:]]+changes[[:space:]]*$' "$job_file" || fail "${JOB} must need changes"
grep -Eq '^[[:space:]]*runs-on:[[:space:]]*ubuntu-latest[[:space:]]*$' "$job_file" || fail "${JOB} must run on ubuntu-latest"
grep -Eq '^[[:space:]]*timeout-minutes:[[:space:]]*60[[:space:]]*$' "$job_file" || fail "${JOB} must retain its bounded runtime"
grep -Eq '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-file-attachment-restart-proof[[:space:]]*$' "$job_file" || fail "${JOB} must invoke the isolated restart proof"

# The proof must run on the PR's exact head when A1 paths changed; a
# post-merge-only push cannot establish pre-merge evidence for this slice.
grep -Fq "github.event_name == 'pull_request'" "$job_file" || fail "${JOB} must run on pull_request exact heads"
grep -Fq "needs.changes.outputs.a1_e2e == 'true'" "$job_file" || fail "${JOB} must remain A1 path-filtered"

awk '
  /^a1_e2e:[[:space:]]*$/ { capture = 1; next }
  capture && /^[^[:space:]][^:]*:/ { exit }
  capture { print }
' "$FILTERS" >"$filter_file"
grep -Eq '^[[:space:]]*-[[:space:]]*scripts/ci/a1-attachment-restart-ci-reachability_test\.sh[[:space:]]*$' "$filter_file" || \
  fail "a1_e2e must include the attachment CI reachability contract"

awk '
  /^ci-script-tests:[[:space:]]*/ { capture = 1; print; next }
  capture && /^[A-Za-z0-9_.-]+:[[:space:]]*/ { exit }
  capture { print }
' "$MAKEFILE" >"$make_target_file"
grep -Eq '^[[:space:]]*\$\(BASH\)[[:space:]]+"\$\(ROOT\)/scripts/ci/a1-attachment-restart-ci-reachability_test\.sh"[[:space:]]*$' "$make_target_file" || \
  fail "ci-script-tests must run the attachment CI reachability contract"

echo 'A1 attachment restart CI reachability contract passed.'
