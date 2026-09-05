#!/usr/bin/env bash
# RED contract for the isolated A1 compose job and its dedicated path filter.
# Dependency-free so CI can run it before installing Docker, Go, or a YAML helper.
set -euo pipefail

ROOT="$(cd "$(dirname "$BASH_SOURCE")/../.." && pwd -P)"
WORKFLOW="$ROOT/.github/workflows/ci.yml"
FILTERS="$ROOT/.github/ci/path-filters.yml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

[[ -f "$WORKFLOW" ]] || fail "missing workflow: $WORKFLOW"
[[ -f "$FILTERS" ]] || fail "missing path filters: $FILTERS"

# Extract one GitHub Actions job by its two-space top-level job key.
job_block() {
  local job="$1"
  awk -v wanted="$job" '
    /^  [A-Za-z0-9_-]+:/ {
      id = $0
      sub(/^  /, "", id)
      sub(/:.*/, "", id)
      if (inside) exit
      if (id == wanted) inside = 1
    }
    inside { print }
  ' "$WORKFLOW"
}

# Locate the unique top-level job containing the A1 make target. The later
# assertion checks the command itself, so near-miss targets cannot pass.
candidate_file="$(mktemp)"
a1_job_file=
changes_job_file=
gate_job_file=
filter_block_file=
filter_entries_file=
near_miss_file=
attachment_candidate_file=
attachment_job_file=
attachment_near_miss_file=
trap 'rm -f "$candidate_file" "$a1_job_file" "$changes_job_file" "$gate_job_file" "$filter_block_file" "$filter_entries_file" "$near_miss_file" "$attachment_candidate_file" "$attachment_job_file" "$attachment_near_miss_file"' EXIT
awk '
  function flush() {
    if (job != "" && block ~ /make[[:space:]]+compose-a1-multi-account-proof([^[:alnum:]_-]|$)/) {
      print "__A1_JOB__"
      printf "%s", block
    }
  }
  /^  [A-Za-z0-9_-]+:/ {
    flush()
    job = $0
    block = $0 "\n"
    next
  }
  job != "" { block = block $0 "\n" }
  END { flush() }
' "$WORKFLOW" >"$candidate_file"

candidate_count="$(grep -c '^__A1_JOB__$' "$candidate_file" || true)"
[[ "$candidate_count" -eq 1 ]] || fail "expected exactly one isolated job containing make compose-a1-multi-account-proof, got $candidate_count"

a1_job_file="$(mktemp)"
awk '/^__A1_JOB__$/ { capture = 1; next } capture { print }' "$candidate_file" >"$a1_job_file"

# Accept either one-line run: make ... or a literal command below run: |.
if ! grep -Eq '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-a1-multi-account-proof[[:space:]]*$' "$a1_job_file" && ! grep -Eq '^[[:space:]]+make[[:space:]]+compose-a1-multi-account-proof[[:space:]]*$' "$a1_job_file"; then
  fail "isolated A1 job must invoke the exact make compose-a1-multi-account-proof target"
fi

# A near-miss command must not satisfy the exact-target contract.
near_miss_file="$(mktemp)"
printf '%s\n' 'run: make compose-a1-multi-account-proof-extra' >"$near_miss_file"
if grep -Eq '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-a1-multi-account-proof[[:space:]]*$' "$near_miss_file"; then
  fail "near-miss A1 make target was accepted"
fi

grep -Eq '^[[:space:]]*timeout-minutes:[[:space:]]*60[[:space:]]*$' "$a1_job_file" || fail "isolated A1 job must have timeout-minutes: 60"
grep -Eq "VOICE_A1_MULTI_ACCOUNT_CLEANUP:[[:space:]]*[\"']?true[\"']?[[:space:]]*$" "$a1_job_file" || fail "isolated A1 job must set VOICE_A1_MULTI_ACCOUNT_CLEANUP=true"

for forbidden in "docker compose" "make compose-e2e" "compose-file-attachment-restart-proof" "compose-e2e-smoke" "compose-e2e-live" "compose-e2e-full"; do
  if grep -Fq "$forbidden" "$a1_job_file"; then
    fail "isolated A1 job must not use shared/generic compose path: $forbidden"
  fi
done

grep -Fq "needs: changes" "$a1_job_file" || fail "isolated A1 job must depend on the changes job"
grep -Fq "github.event_name == 'schedule'" "$a1_job_file" || fail "isolated A1 job must run on schedule"
grep -Fq "github.event_name == 'workflow_dispatch'" "$a1_job_file" || fail "isolated A1 job must support manual dispatch"
grep -Fq "inputs.profile == 'full'" "$a1_job_file" || fail "isolated A1 job must require the manual full profile"
grep -Fq "github.event_name == 'push'" "$a1_job_file" || fail "isolated A1 job must support master push"
grep -Fq "github.ref == 'refs/heads/master'" "$a1_job_file" || fail "isolated A1 job push path must be gated to master"
grep -Fq "needs.changes.outputs.a1_e2e == 'true'" "$a1_job_file" || fail "isolated A1 job push path must be gated by changes.outputs.a1_e2e"
if grep -Fq "pull_request" "$a1_job_file"; then
  fail "isolated A1 job must never run on pull_request"
fi
if grep -Fq "inputs.profile != 'auto'" "$a1_job_file"; then
  fail "isolated A1 job must not broaden manual dispatch beyond profile full"
fi

# The attachment restart proof has its own job and must not become a second
# command in a1-e2e. Count the exact job id and the command-bearing job
# independently so duplicate ids or a misplaced command are RED.
attachment_job_id_count="$(awk '/^  a1-attachment-restart-proof:[[:space:]]*$/ { count++ } END { print count + 0 }' "$WORKFLOW")"
[[ "$attachment_job_id_count" -eq 1 ]] || fail "expected exactly one a1-attachment-restart-proof job, got $attachment_job_id_count"

attachment_candidate_file="$(mktemp)"
awk '
  function flush() {
    if (job != "" && block ~ /make[[:space:]]+compose-file-attachment-restart-proof([^[:alnum:]_-]|$)/) {
      print "__ATTACHMENT_JOB__ " job
      printf "%s", block
    }
  }
  /^  [A-Za-z0-9_-]+:/ {
    flush()
    job = $0
    sub(/^  /, "", job)
    sub(/:.*/, "", job)
    block = $0 "\n"
    next
  }
  job != "" { block = block $0 "\n" }
  END { flush() }
' "$WORKFLOW" >"$attachment_candidate_file"
attachment_candidate_count="$(grep -c '^__ATTACHMENT_JOB__ ' "$attachment_candidate_file" || true)"
[[ "$attachment_candidate_count" -eq 1 ]] || fail "expected exactly one job containing the attachment make target, got $attachment_candidate_count"
attachment_candidate_job="$(sed -n 's/^__ATTACHMENT_JOB__ //p' "$attachment_candidate_file")"
[[ "$attachment_candidate_job" == "a1-attachment-restart-proof" ]] || fail "attachment make target must be isolated in a1-attachment-restart-proof, got $attachment_candidate_job"

attachment_job_file="$(mktemp)"
job_block a1-attachment-restart-proof >"$attachment_job_file"
attachment_exact_run_count="$(grep -Ec '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-file-attachment-restart-proof[[:space:]]*$|^[[:space:]]+make[[:space:]]+compose-file-attachment-restart-proof[[:space:]]*$' "$attachment_job_file" || true)"
[[ "$attachment_exact_run_count" -eq 1 ]] || fail "attachment job must have exactly one exact make compose-file-attachment-restart-proof command, got $attachment_exact_run_count"
attachment_target_count="$(grep -Ec 'make[[:space:]]+compose-file-attachment-restart-proof([^[:alnum:]_-]|$)' "$attachment_job_file" || true)"
[[ "$attachment_target_count" -eq 1 ]] || fail "attachment job must contain exactly one attachment target occurrence, got $attachment_target_count"
grep -Eq '^[[:space:]]*runs-on:[[:space:]]*ubuntu-latest[[:space:]]*$' "$attachment_job_file" || fail "attachment job must run on ubuntu-latest"
grep -Eq '^[[:space:]]*- uses:[[:space:]]+actions/setup-go@v5[[:space:]]*$' "$attachment_job_file" || fail "attachment job must set up Go"
grep -Eq '^[[:space:]]*timeout-minutes:[[:space:]]*60[[:space:]]*$' "$attachment_job_file" || fail "attachment job must have timeout-minutes: 60"
grep -Eq "VOICE_FILE_ATTACHMENT_RESTART_CLEANUP:[[:space:]]*[\"']?true[\"']?[[:space:]]*$" "$attachment_job_file" || fail "attachment job must set VOICE_FILE_ATTACHMENT_RESTART_CLEANUP=true"
grep -Fq "needs: changes" "$attachment_job_file" || fail "attachment job must depend on the changes job"
grep -Fq "github.event_name == 'schedule'" "$attachment_job_file" || fail "attachment job must run on schedule"
grep -Fq "github.event_name == 'workflow_dispatch'" "$attachment_job_file" || fail "attachment job must support manual dispatch"
grep -Fq "inputs.profile == 'full'" "$attachment_job_file" || fail "attachment job must require the manual full profile"
grep -Fq "github.event_name == 'push'" "$attachment_job_file" || fail "attachment job must support master push"
grep -Fq "github.ref == 'refs/heads/master'" "$attachment_job_file" || fail "attachment job push path must be gated to master"
grep -Fq "needs.changes.outputs.a1_e2e == 'true'" "$attachment_job_file" || fail "attachment job push path must be gated by changes.outputs.a1_e2e"
if grep -Fq "pull_request" "$attachment_job_file"; then
  fail "attachment job must never run on pull_request"
fi
if grep -Fq "inputs.profile != 'auto'" "$attachment_job_file"; then
  fail "attachment job must not broaden manual dispatch beyond profile full"
fi

attachment_near_miss_file="$(mktemp)"
printf '%s\n' 'run: make compose-file-attachment-restart-proof-extra' >"$attachment_near_miss_file"
if grep -Eq '^[[:space:]]*run:[[:space:]]+make[[:space:]]+compose-file-attachment-restart-proof[[:space:]]*$' "$attachment_near_miss_file"; then
  fail "near-miss attachment make target was accepted"
fi

changes_job_file="$(mktemp)"
job_block changes >"$changes_job_file"
grep -Fq 'a1_e2e:' "$changes_job_file" && grep -Fq 'steps.filter.outputs.a1_e2e' "$changes_job_file" || fail "changes job must expose the dedicated a1_e2e path-filter output"

# Keep the existing required PR gate intact.
grep -Eq '^  pull_request:[[:space:]]*$' "$WORKFLOW" || fail "workflow must preserve the pull_request trigger"
gate_job_file="$(mktemp)"
job_block ci-gate >"$gate_job_file"
grep -Fq "github.event_name == 'pull_request'" "$gate_job_file" || fail "ci-gate must remain a pull_request job"
grep -Fq 'verify-required-jobs.sh pull_request' "$gate_job_file" || fail "ci-gate must preserve verify-required-jobs.sh pull_request"

# Compare normalized YAML list entries in the dedicated filter. This makes
# omissions visible instead of allowing the broad global filter to mask them.
filter_block_file="$(mktemp)"
awk '/^a1_e2e:[[:space:]]*$/ { inside = 1; next } inside && /^[^[:space:]][^:]*:/ { exit } inside { print }' "$FILTERS" >"$filter_block_file"
[[ -s "$filter_block_file" ]] || fail "path-filters.yml must define a dedicated a1_e2e filter"

filter_entries_file="$(mktemp)"
sed -E -e 's/\r$//' -e 's/^[[:space:]]*-[[:space:]]*//' -e "s/^['\"]//; s/['\"][[:space:]]*$//" -e 's/[[:space:]]+#.*$//' "$filter_block_file" >"$filter_entries_file"

has_entry() {
  grep -Fxq "$1" "$filter_entries_file"
}

required_paths="Makefile
.github/workflows/ci.yml
.github/ci/path-filters.yml
.github/ci/e2e-features.yml
src/backend/auth/**
src/backend/user/**
src/backend/chat/**
src/backend/messaging/**
src/backend/social/**
src/backend/file/**
src/backend/gateway/**
src/backend/realtime/**
src/backend/pkg/**
src/frontend/**
src/backend/migrations/**
protos/**
docker/**
docker-compose*.yml
scripts/ci/compose-a1*
scripts/ci/compose-file-attachment-restart-proof.sh
scripts/ci/e2e-manifest*"
while IFS= read -r required; do
  [[ -n "$required" ]] || continue
  has_entry "$required" || fail "a1_e2e filter is missing required path: $required"
done <<<"$required_paths"

echo "A1 CI reachability contract passed."
