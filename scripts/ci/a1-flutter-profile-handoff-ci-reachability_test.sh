#!/usr/bin/env bash
# Static reachability contract for the isolated T-055 Flutter profile handoff.
# This intentionally parses only the local workflow text: it never invokes
# Docker, Flutter, a network client, or a YAML dependency.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORKFLOW="$ROOT/.github/workflows/ci.yml"
FILTERS="$ROOT/.github/ci/path-filters.yml"
MAKEFILE="$ROOT/Makefile"
JOB='a1-flutter-profile-handoff'

fail() { echo "FAIL: $*" >&2; exit 1; }

job_block() {
  local wanted="$1"
  awk -v wanted="$wanted" '
    $0 ~ "^  " wanted ":[[:space:]]*$" { found = 1; print; next }
    found && /^  [A-Za-z0-9_-]+:[[:space:]]*$/ { exit }
    found { print }
  ' "$WORKFLOW"
}

job_count="$(grep -Ec "^  ${JOB}:[[:space:]]*$" "$WORKFLOW" || true)"
[[ "$job_count" -eq 1 ]] || fail "expected exactly one ${JOB} job, got ${job_count}"

job_file="$(mktemp)"
filter_block_file="$(mktemp)"
filter_entries_file="$(mktemp)"
gate_file="$(mktemp)"
make_target_file="$(mktemp)"
trap 'rm -f "$job_file" "$filter_block_file" "$filter_entries_file" "$gate_file" "$make_target_file"' EXIT
job_block "$JOB" >"$job_file"
[[ -s "$job_file" ]] || fail "missing ${JOB} workflow block"

grep -Eq '^[[:space:]]*needs:[[:space:]]+changes[[:space:]]*$' "$job_file" || fail "${JOB} must need changes"
grep -Eq '^[[:space:]]*runs-on:[[:space:]]*ubuntu-latest[[:space:]]*$' "$job_file" || fail "${JOB} must run on ubuntu-latest"
grep -Eq '^[[:space:]]*timeout-minutes:[[:space:]]*60[[:space:]]*$' "$job_file" || fail "${JOB} must timeout after 60 minutes"
grep -Eq '^[[:space:]]*- uses:[[:space:]]+actions/checkout@v5[[:space:]]*$' "$job_file" || fail "${JOB} must use checkout@v5"
grep -Eq '^[[:space:]]*- uses:[[:space:]]+subosito/flutter-action@v2[[:space:]]*$' "$job_file" || fail "${JOB} must use flutter-action@v2"
grep -Fq 'flutter-version: ${{ env.FLUTTER_VERSION }}' "$job_file" || fail "${JOB} must use the workflow Flutter version"
grep -Eq '^[[:space:]]*cache:[[:space:]]*true[[:space:]]*$' "$job_file" || fail "${JOB} must enable Flutter cache"
grep -Eq '^[[:space:]]*run:[[:space:]]+bash[[:space:]]+scripts/ci/flutter-linux-prefetch-sqlite3\.sh[[:space:]]+host[[:space:]]*$' "$job_file" || fail "${JOB} must prefetch host sqlite3 assets"
grep -Eq '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-a1-flutter-profile-handoff[[:space:]]*$' "$job_file" || fail "${JOB} must invoke the exact profile-handoff target"
target_count="$(grep -Ec 'make[[:space:]]+compose-a1-flutter-profile-handoff([^[:alnum:]_-]|$)' "$job_file" || true)"
[[ "$target_count" -eq 1 ]] || fail "${JOB} must contain exactly one profile-handoff target occurrence, got ${target_count}"
grep -Eq "VOICE_A1_FLUTTER_PROFILE_HANDOFF_CLEANUP:[[:space:]]*[\"']?true[\"']?[[:space:]]*$" "$job_file" || fail "${JOB} must set cleanup=true"

trigger_terms="$(grep -E "github.event_name|github.ref|inputs.profile|needs.changes.outputs.a1_e2e" "$job_file" | sed -E 's/^[[:space:]]*//')"
expected_trigger_terms="$(printf '%s\n' \
  "github.event_name == 'schedule' ||" \
  "(github.event_name == 'workflow_dispatch' && inputs.profile == 'full') ||" \
  "github.event_name == 'push' &&" \
  "github.ref == 'refs/heads/master' &&" \
  "needs.changes.outputs.a1_e2e == 'true'")"
[[ "$trigger_terms" == "$expected_trigger_terms" ]] || \
  fail "${JOB} trigger terms must be exactly schedule, full dispatch, or filtered master push"
grep -Fq "needs.changes.result == 'success' || needs.changes.result == 'skipped'" "$job_file" || fail "${JOB} must tolerate skipped changes on schedule/manual runs"

job_block ci-gate >"$gate_file"
[[ -s "$gate_file" ]] || fail "missing ci-gate workflow block"
if grep -Fq "$JOB" "$gate_file"; then
  fail "${JOB} must not become a ci-gate dependency"
fi

awk '/^a1_e2e:[[:space:]]*$/ { inside = 1; next } inside && /^[^[:space:]][^:]*:/ { exit } inside { print }' "$FILTERS" >"$filter_block_file"
[[ -s "$filter_block_file" ]] || fail "path-filters.yml must define a1_e2e"
sed -E -e 's/\r$//' -e 's/^[[:space:]]*-[[:space:]]*//' -e "s/^['\"]//; s/['\"][[:space:]]*$//" -e 's/[[:space:]]+#.*$//' "$filter_block_file" >"$filter_entries_file"
has_filter() { grep -Fxq "$1" "$filter_entries_file"; }
for required in \
  'src/backend/scripts/**' \
  'scripts/ci/compose-a1*' \
  'scripts/ci/e2e-manifest*' \
  '.github/ci/e2e-features.yml'; do
  has_filter "$required" || fail "a1_e2e filter is missing required path: $required"
done

awk '
  /^ci-script-tests:[[:space:]]*/ { capture = 1; print; next }
  capture && /^[A-Za-z0-9_.-]+:[[:space:]]*/ { exit }
  capture { print }
' "$MAKEFILE" >"$make_target_file"
[[ -s "$make_target_file" ]] || fail "Makefile must define ci-script-tests"
grep -Eq '^[[:space:]]*\$\(BASH\)[[:space:]]+"\$\(ROOT\)/scripts/ci/compose-a1-flutter-profile-handoff_test\.sh"[[:space:]]*$' "$make_target_file" || \
  fail "ci-script-tests must run the profile-handoff runner contract"
grep -Eq '^[[:space:]]*\$\(BASH\)[[:space:]]+"\$\(ROOT\)/scripts/ci/a1-flutter-profile-handoff-ci-reachability_test\.sh"[[:space:]]*$' "$make_target_file" || \
  fail "ci-script-tests must run the profile-handoff reachability contract"

echo 'A1 Flutter profile-handoff CI reachability contract passed.'
