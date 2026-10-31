#!/usr/bin/env bash
# RED-F oracle for Voice durable-store runtime, provisioning, deployment, docs,
# CI routing, and the source-disabled R22.2 cut. No Docker or cluster required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
TMP_DIR="$(mktemp -d)"
f13_cleanup() {
  local status=$? child worker
  trap '' INT TERM
  if [[ -n "${f13_standalone_pid:-}" ]]; then
    # If cancellation cannot be published, still wait for the real suite rather
    # than removing inputs while its supervisor may be using them.
    : > "${TMP_DIR}/f13-standalone-cancel" || true
    wait "${f13_standalone_pid}" 2>/dev/null || true
    f13_standalone_pid=''
  fi
  for worker in "${f13_router_worker_pid:-}" "${f13_finite_J_worker_pid:-}"; do
  if [[ -n "${worker}" ]]; then
    # Signal only our still-running shell job; never a recycled numeric PID.
    while IFS= read -r child; do
      [[ "${child}" != "${worker}" ]] || kill "${child}" 2>/dev/null || true
    done < <(jobs -pr)
    wait "${worker}" 2>/dev/null || true
  fi
  done
  rm -rf "${TMP_DIR}"
  return "${status}"
}
trap f13_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

failures=0

report_unexpected_exit() {
  local status="$1"
  local source_line="$2"
  printf 'CI_DIAG stage=unexpected-command source_line=%s exit_status=%s\n' "$source_line" "$status" >&2
}

trap 'report_unexpected_exit "$?" "$LINENO"' ERR

fail() {
  printf 'CI_DIAG stage=assertion source_line=%s exit_status=1\n' "${BASH_LINENO[0]:-0}" >&2
  printf 'FAIL: %s\n' "$*" >&2
  failures=$((failures + 1))
}

expect_file() {
  local file="$1"
  local label="$2"
  [[ -f "${ROOT}/${file}" ]] || fail "${label}: missing ${file}"
}

expect_fixed() {
  local file="$1"
  local text="$2"
  local label="$3"
  grep -Fq -- "${text}" "${ROOT}/${file}" 2>/dev/null || fail "${label}: ${file} lacks ${text}"
}

expect_regex() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  grep -Eq -- "${pattern}" "${ROOT}/${file}" 2>/dev/null || fail "${label}: ${file} does not match ${pattern}"
}

paragraph_matches_regex() {
  local file="$1"
  local pattern="$2"
  awk -v pattern="${pattern}" '
    function flush_paragraph() {
      if (paragraph != "") {
        gsub(/[[:space:]]+/, " ", paragraph)
        if (paragraph ~ pattern) {
          found=1
        }
      }
      paragraph=""
    }
    /^[[:space:]]*$/ { flush_paragraph(); next }
    /^[[:space:]]*#{1,6}[[:space:]]/ { flush_paragraph(); next }
    {
      line=$0
      sub(/\r$/, "", line)
      paragraph=paragraph (paragraph == "" ? "" : " ") line
    }
    END {
      flush_paragraph()
      exit found ? 0 : 1
    }
  ' "${file}"
}

expect_paragraph_regex() {
  local file="$1"
  local pattern="$2"
  local label="$3"
  paragraph_matches_regex "${ROOT}/${file}" "${pattern}" || \
    fail "${label}: ${file} has no matching Markdown paragraph for ${pattern}"
}

expect_word_count() {
  local file="$1"
  local word="$2"
  local expected="$3"
  local label="$4"
  local actual
  actual="$({ grep -Eow -- "${word}" "${ROOT}/${file}" 2>/dev/null || true; } | wc -l | tr -d '[:space:]')"
  [[ "${actual}" == "${expected}" ]] || fail "${label}: expected ${word} ${expected} time(s) in ${file}, got ${actual}"
}

yaml_document() {
  local file="$1"
  local name="$2"
  awk -v wanted="${name}" '
    function emit() {
      if (document ~ /(^|\n)kind:[[:space:]]*Deployment([[:space:]]|\n)/ &&
          document ~ ("(^|\n)  name:[[:space:]]*" wanted "([[:space:]]|\n)")) {
        printf "%s", document
        exit
      }
      document=""
    }
    /^---[[:space:]]*$/ { emit(); next }
    { document=document $0 ORS }
    END { emit() }
  ' "${ROOT}/${file}"
}

changed_content() {
  local base_sha="$1"
  local file="$2"
  if git -C "${ROOT}" cat-file -e "${base_sha}:${file}" 2>/dev/null; then
    git -C "${ROOT}" diff --unified=0 "${base_sha}" -- "${file}" \
      | sed -n '/^+++ /d; /^+/s/^+//p'
  elif [[ -f "${ROOT}/${file}" ]]; then
    cat "${ROOT}/${file}"
  fi
}

compose_service() {
  local service="$1"
  awk -v wanted="${service}" '
    $0 == "  " wanted ":" { in_service=1 }
    in_service && $0 ~ /^  [[:alnum:]_-]+:$/ && $0 != "  " wanted ":" { exit }
    in_service { print }
  ' "${ROOT}/docker-compose.yml"
}

compose_migration_docs_block() {
  local file="$1"
  awk '
    /^\*\*Compose \(all Go-owned DBs\):\*\*/ { in_block=1; next }
    in_block && /^### / { exit }
    in_block { print }
  ' "${file}"
}

indented_block() {
  local key="$1"
  local file="$2"
  local wanted_indent="${3:-}"
  awk -v wanted="${key}" '
    function indentation(line, spaces) {
      spaces=line
      sub(/[^ ].*$/, "", spaces)
      return length(spaces)
    }
    !in_block && $0 ~ "^[ ]*" wanted ":[[:space:]]*$" &&
        (required_indent == "" || indentation($0) == required_indent) {
      in_block=1
      parent_indent=indentation($0)
      print
      next
    }
    in_block {
      if ($0 !~ /^[[:space:]]*$/ && indentation($0) <= parent_indent) {
        exit
      }
      print
    }
  ' required_indent="${wanted_indent}" "${file}"
}

forbidden_activation_content() {
  local file="$1"
  if grep -Ein '(type[[:space:]]+.*(Coordinator|Bridge|Publisher|Handler|Worker|Runner)|New.*(Coordinator|Bridge|Publisher|Handler|Worker|Runner)|Register.*Lifecycle|LifecycleStore[[:space:]]*:|go[[:space:]]+func|NewJetStreamPublisher|redis\.New|livekit\.New|time\.NewTicker|\.Publish[[:space:]]*\(|([[:alnum:]_]*(coordinator|worker|runner|bridge|publisher)[[:alnum:]_]*)\.(Run|Start)[[:space:]]*\()' "${file}" >/dev/null; then
    return 0
  fi
  grep -Eiz '\.(Handle|HandleFunc)[[:space:]]*\([^)]*(lifecycle|room)' "${file}" >/dev/null
}

cat >"${TMP_DIR}/f12-adjacent-lines" <<'EOF'
Apply lifecycle migrations before rolling or
starting the voice-voice Deployment.
EOF
cat >"${TMP_DIR}/f12-adjacent-paragraph" <<'EOF'
Apply 000001_room_lifecycle.up.sql to voice_db.
Verify the completed Job and schema state.
Only then, before rolling the voice-voice Deployment, continue the release.
EOF
cat >"${TMP_DIR}/f12-distant-paragraphs" <<'EOF'
Apply lifecycle migrations during the maintenance window.

Unrelated operational guidance belongs here.

Before rolling the voice-voice Deployment, notify the on-call engineer.
EOF
cat >"${TMP_DIR}/f12-distant-sections" <<'EOF'
## Database migrations
Apply lifecycle migrations during the maintenance window.
## Application rollout
Before rolling the voice-voice Deployment, notify the on-call engineer.
EOF
if ! declare -F paragraph_matches_regex >/dev/null; then
  printf '%s\n' 'F12 oracle bug: paragraph-scoped regex matcher is missing' >&2
  exit 2
fi
f12_lifecycle_pattern='(migrat|миграц).*(before|до).*(voice-voice|Voice)|(voice_db|000001_room_lifecycle).*(before|до).*(roll|запуск|депло)'
paragraph_matches_regex "${TMP_DIR}/f12-adjacent-lines" "${f12_lifecycle_pattern}" || {
  printf '%s\n' 'F12 oracle bug: adjacent-line lifecycle requirement was rejected' >&2
  exit 2
}
paragraph_matches_regex "${TMP_DIR}/f12-adjacent-paragraph" "${f12_lifecycle_pattern}" || {
  printf '%s\n' 'F12 oracle bug: split lifecycle paragraph was rejected' >&2
  exit 2
}
if paragraph_matches_regex "${TMP_DIR}/f12-distant-paragraphs" "${f12_lifecycle_pattern}"; then
  printf '%s\n' 'F12 oracle bug: distant paragraphs formed a false lifecycle requirement' >&2
  exit 2
fi
if paragraph_matches_regex "${TMP_DIR}/f12-distant-sections" "${f12_lifecycle_pattern}"; then
  printf '%s\n' 'F12 oracle bug: distant sections formed a false lifecycle requirement' >&2
  exit 2
fi

assert_forbidden_activation_fixture() {
  local label="$1"
  local fixture="$2"
  printf '%s\n' "${fixture}" >"${TMP_DIR}/f13-bad-activation"
  forbidden_activation_content "${TMP_DIR}/f13-bad-activation" || {
    printf 'F13 oracle bug: forbidden %s fixture was accepted\n' "${label}" >&2
    exit 2
  }
}

assert_forbidden_activation_fixture coordinator-run 'go coordinator.Run(ctx)'
assert_forbidden_activation_fixture worker-start 'deliveryWorker.Start(ctx)'
assert_forbidden_activation_fixture publisher-run 'lifecyclePublisher.Run(ctx)'
assert_forbidden_activation_fixture handler-registration 'router.HandleFunc("/lifecycle", lifecycleHandler)'
assert_forbidden_activation_fixture multiline-handler-registration $'mux.Handle(\n  "/room-lifecycle",\n  lifecycleHandler,\n)'
cat >"${TMP_DIR}/f13-approved-factory" <<'EOF'
lifecycleStore, closeStore, enabled, err := openLifecycleDatabase(ctx)
postgresStore := roomlifecycle.NewPostgresStore(pool)
readinessChecker := lifecycleStore.CheckReady
EOF
if forbidden_activation_content "${TMP_DIR}/f13-approved-factory"; then
  printf '%s\n' 'F13 oracle bug: approved database/store factory fixture was rejected' >&2
  exit 2
fi

cat >"${TMP_DIR}/f05-bad-depends" <<'EOF'
  compose-db-init:
    depends_on:
      redis:
        condition: service_healthy
      postgres:
        condition: service_started
EOF
indented_block depends_on "${TMP_DIR}/f05-bad-depends" 4 >"${TMP_DIR}/f05-bad-depends-block"
indented_block postgres "${TMP_DIR}/f05-bad-depends-block" 6 >"${TMP_DIR}/f05-bad-postgres-block"
if grep -Eq '^        condition:[[:space:]]*service_healthy[[:space:]]*$' "${TMP_DIR}/f05-bad-postgres-block"; then
  printf '%s\n' 'F05 oracle bug: PostgreSQL borrowed another dependency health condition' >&2
  exit 2
fi

cat >"${TMP_DIR}/f10-bad-probes" <<'EOF'
          readinessProbe:
            exec:
              command: ["curl", "/health"]
          livenessProbe:
            httpGet:
              path: /ready
EOF
indented_block readinessProbe "${TMP_DIR}/f10-bad-probes" >"${TMP_DIR}/f10-bad-readiness"
indented_block httpGet "${TMP_DIR}/f10-bad-readiness" >"${TMP_DIR}/f10-bad-readiness-http"
if grep -Eq '^[ ]+path:[[:space:]]*/ready[[:space:]]*$' "${TMP_DIR}/f10-bad-readiness-http"; then
  printf '%s\n' 'F10 oracle bug: readiness borrowed /ready from the liveness probe' >&2
  exit 2
fi
grep -Eq '^[ ]+exec:[[:space:]]*$' "${TMP_DIR}/f10-bad-readiness" || {
  printf '%s\n' 'F10 oracle bug: readiness subtree did not retain its exec probe' >&2
  exit 2
}

printf '%s\n' '== F05-F06: local provisioning and migration entrypoints =='
expect_word_count 'docker/postgres/initdb.d/01-init-databases.sh' 'voice_db' 1 F05
compose_service compose-db-init >"${TMP_DIR}/compose-db-init"
grep -Eq '^      PGSSLMODE:[[:space:]]*disable[[:space:]]*$' "${TMP_DIR}/compose-db-init" || fail 'F05: local compose-db-init must explicitly use the PostgreSQL service TLS mode'
indented_block depends_on "${TMP_DIR}/compose-db-init" 4 >"${TMP_DIR}/compose-db-init-depends"
grep -Eq '^      postgres:[[:space:]]*$' "${TMP_DIR}/compose-db-init-depends" || fail 'F05: compose-db-init must depend on PostgreSQL'
indented_block postgres "${TMP_DIR}/compose-db-init-depends" 6 >"${TMP_DIR}/compose-db-init-postgres"
grep -Eq '^        condition:[[:space:]]*service_healthy[[:space:]]*$' "${TMP_DIR}/compose-db-init-postgres" || fail 'F05: compose-db-init must wait for healthy PostgreSQL'
expect_word_count 'docker/postgres/ensure-compose-schema.sh' 'voice_db' 1 F06
expect_word_count 'docker/postgres/compose-migrate-dbs.sh' 'voice_db' 1 F06
expect_word_count 'scripts/dev/compose-migrate-all.sh' 'voice_db' 1 F06
expect_fixed 'scripts/dev/compose-migrate-all.sh' 'migrate_db voice_db' F06
expect_regex 'scripts/dev/compose-migrate-all.sh' 'voice\)[[:space:]]+run_voice' F06
expect_regex 'Makefile' '^compose-migrate-voice:' F06
expect_fixed 'Makefile' 'scripts/dev/compose-migrate-all.sh" voice' F06
expect_file 'src/backend/migrations/voice_db/000001_room_lifecycle.up.sql' F06
expect_file 'src/backend/migrations/voice_db/000001_room_lifecycle.down.sql' F06

compose_service voice >"${TMP_DIR}/compose-voice"
grep -Eq 'VOICE_DATABASE_URL:.*voice_db' "${TMP_DIR}/compose-voice" || fail 'F05: Compose Voice lacks the voice_db DSN'
grep -Fq 'compose-db-init:' "${TMP_DIR}/compose-voice" || fail 'F05: Compose Voice must depend on compose-db-init'
grep -Fq 'condition: service_completed_successfully' "${TMP_DIR}/compose-voice" || fail 'F05: Compose Voice must wait for successful schema init'
grep -Fq 'http://127.0.0.1:8080/ready' "${TMP_DIR}/compose-voice" || fail 'F10: Compose Voice readiness must use /ready'

printf '%s\n' '== F07-F08: cluster database and migration wiring =='
expect_word_count 'scripts/staging/init-postgres-databases.sh' 'voice_db' 1 F07
expect_word_count 'scripts/staging/reset-databases-from-scratch.sh' 'voice_db' 2 F07
expect_file 'deploy/templates/migrate-voice-db-job.yaml' F08
expect_fixed 'deploy/templates/migrate-voice-db-job.yaml' 'voice-migrate-voice-db' F08
expect_fixed 'deploy/templates/migrate-voice-db-job.yaml' 'voice-voice-db-migrations' F08
expect_fixed 'deploy/templates/migrate-voice-db-job.yaml' '__K_NAMESPACE__' F08
expect_fixed 'scripts/staging/apply-migrate-jobs.sh' 'apply_migrate voice_db' F08
expect_fixed 'scripts/staging/apply-migrate-jobs.sh' 'voice-migrate-voice-db' F08
expect_fixed 'scripts/staging/apply-migrate-jobs.sh' 'voice-voice-db-migrations' F08
expect_fixed 'scripts/staging/apply-migrate-jobs.sh' 'src/backend/migrations/voice_db' F08
expect_regex 'scripts/staging/apply-migrate-jobs.sh' 'kubectl wait .*condition=complete' F08
expect_file 'scripts/staging/apply-migrate-jobs_test.sh' F08
expect_regex 'scripts/prod/apply-infra.sh' 'VOICE_K8S_NAMESPACE:-voice-prod' F08
expect_fixed 'scripts/prod/apply-infra.sh' 'scripts/staging/apply-migrate-jobs.sh' F08

printf '%s\n' '== F09-F10: secrets and shipped Voice readiness =='
for file in \
  scripts/staging/ensure-app-secrets.sh \
  scripts/staging/patch-app-secrets-database-urls.sh \
  deploy/staging/secret.example.yaml \
  deploy/prod/secret.example.yaml; do
  expect_fixed "${file}" 'VOICE_DATABASE_URL' F09
done
for file in deploy/staging/services.yaml deploy/prod/services.yaml; do
  yaml_document "${file}" voice-voice >"${TMP_DIR}/$(basename "$(dirname "${file}")")-voice"
  block="${TMP_DIR}/$(basename "$(dirname "${file}")")-voice"
  grep -Fq 'name: VOICE_DATABASE_URL' "${block}" || fail "F10: ${file} Voice deployment lacks secret DSN env"
  grep -Fq 'key: VOICE_DATABASE_URL' "${block}" || fail "F10: ${file} Voice deployment does not read the Voice DSN secret"
  indented_block readinessProbe "${block}" >"${TMP_DIR}/readiness-probe"
  grep -Eq '^[ ]+readinessProbe:[[:space:]]*$' "${TMP_DIR}/readiness-probe" || fail "F10: ${file} Voice deployment lacks readinessProbe"
  indented_block httpGet "${TMP_DIR}/readiness-probe" >"${TMP_DIR}/readiness-http-get"
  grep -Eq '^[ ]+httpGet:[[:space:]]*$' "${TMP_DIR}/readiness-http-get" || fail "F10: ${file} Voice readiness must use httpGet"
  grep -Eq '^[ ]+path:[[:space:]]*/ready[[:space:]]*$' "${TMP_DIR}/readiness-http-get" || fail "F10: ${file} Voice readiness httpGet must use /ready"
  if grep -Eq '^[ ]+exec:[[:space:]]*$' "${TMP_DIR}/readiness-probe"; then
    fail "F10: ${file} Voice readiness must not use exec"
  fi
done

printf '%s\n' '== F11: CI routing =='
svc_voice_block="$(awk '
  /^svc_voice:[[:space:]]*$/ { in_block=1; next }
  in_block && /^[^[:space:]]/ { exit }
  in_block { print }
' "${ROOT}/.github/ci/path-filters.yml")"
[[ "${svc_voice_block}" == *'src/backend/migrations/voice_db/**'* ]] || fail 'F11: svc_voice must include voice_db migrations'
expect_fixed 'Makefile' 'voice-db-runtime-provisioning-contract_test.sh' F11
expect_fixed 'Makefile' 'apply-migrate-jobs_test.sh' F11
ci_script_tests_block="$(awk '
  /^ci-script-tests:/ { in_block=1 }
  in_block && !/^ci-script-tests:/ && /^[[:alnum:]_.-]+:/ { exit }
  in_block { print }
' "${ROOT}/Makefile")"
if [[ "${ci_script_tests_block}" != *'voice-db-runtime-provisioning-contract-test'* &&
      "${ci_script_tests_block}" != *'voice-db-runtime-provisioning-contract_test.sh'* ]]; then
  fail 'F11: ci-script-tests must invoke the Voice DB runtime/provisioning contract'
fi

make_target_block() {
  local target="$1"
  awk -v target="${target}" '
    $0 ~ ("^" target ":[[:space:]]*") { in_block=1 }
    in_block && $0 !~ ("^" target ":[[:space:]]*") && /^[[:alnum:]_.-]+:/ { exit }
    in_block { print }
  ' "${ROOT}/Makefile"
}

ci_script_tests_header="$(head -n1 <<<"${ci_script_tests_block}")"
ci_script_tests_dependencies="${ci_script_tests_header#*:}"
apply_contract_reachable=false
if grep -Eq '^[[:space:]]+[^#].*scripts/staging/apply-migrate-jobs_test\.sh' \
    <<<"${ci_script_tests_block}"; then
  apply_contract_reachable=true
else
  pending_dependencies="${ci_script_tests_dependencies}"
  visited_dependencies=' '
  while [[ -n "${pending_dependencies// /}" ]]; do
    dependency="${pending_dependencies%% *}"
    if [[ "${pending_dependencies}" == *' '* ]]; then
      pending_dependencies="${pending_dependencies#* }"
    else
      pending_dependencies=''
    fi
    [[ -n "${dependency}" ]] || continue
    [[ "${visited_dependencies}" != *" ${dependency} "* ]] || continue
    visited_dependencies+="${dependency} "
    dependency_block="$(make_target_block "${dependency}")"
    if grep -Eq '^[[:space:]]+[^#].*scripts/staging/apply-migrate-jobs_test\.sh' \
        <<<"${dependency_block}"; then
      apply_contract_reachable=true
      break
    fi
    dependency_header="$(head -n1 <<<"${dependency_block}")"
    if [[ "${dependency_header}" == *:* ]]; then
      pending_dependencies+=" ${dependency_header#*:}"
    fi
  done
fi
[[ "${apply_contract_reachable}" == true ]] || \
  fail 'F11: ci-script-tests must reach scripts/staging/apply-migrate-jobs_test.sh'

printf '%s\n' '== F12: canonical documentation =='
expect_regex 'docs/microservices/voice-service.md' 'voice_db' F12
expect_regex 'docs/microservices/voice-service.md' '(PostgreSQL|Postgres)' F12
expect_regex 'docs/microservices/voice-service.md' '(durable|долговеч|источник истины|source of truth)' F12
expect_regex 'docs/microservices/voice-service.md' 'Redis.*(projection|проекц|rebuild|перестро)|(projection|проекц|rebuild|перестро).*Redis' F12
expect_regex 'docs/microservices/voice-service.md' '(source-disabled|не зарегистрирован|not registered|оста[её]тся выключен)' F12
expect_regex 'docs/DATA_STORES.md' 'Voice Service[[:space:]]*\|[[:space:]]*`voice_db`' F12
expect_regex 'docs/DATA_STORES.md' '\*\*17\*\*.*PostgreSQL' F12
expect_regex 'docs/DATA_STORES.md' '\*\*17\*\*.*(provision|созда|разв[её]р)' F12
expect_fixed 'src/backend/migrations/README.md' 'voice_db' F12
expect_fixed 'src/backend/migrations/README.md' 'compose-migrate-voice' F12
expect_fixed 'src/backend/migrations/README.md' 'compose-db-init' F12
expect_regex 'src/backend/migrations/README.md' 'compose-db-init.*(кажд|every|startup|запуск)' F12
expect_regex 'src/backend/migrations/README.md' '(DOWN|down).*(refus|отказ|запрещ|не удал)' F12
compose_migration_docs="$(compose_migration_docs_block "${ROOT}/src/backend/migrations/README.md")"
cat >"${TMP_DIR}/compose-migration-docs-duplicate.md" <<'EOF'
**Compose (all Go-owned DBs):**
space_db space_db

### Space Native44/45/50 provisioning
space_db
EOF
duplicate_block="$(compose_migration_docs_block "${TMP_DIR}/compose-migration-docs-duplicate.md")"
duplicate_space_count="$({ grep -Eow 'space_db' <<<"${duplicate_block}" || true; } | wc -l | tr -d '[:space:]')"
[[ "${duplicate_space_count}" == '2' ]] || fail 'F12: the Compose list parser must count duplicate list entries while excluding the following Native section'
for db in chat_db messaging_db bot_db story_db user_db social_db file_db space_db role_db notification_db matchmaking_db search_db moderation_db gateway_db subscription_db voice_db; do
  count="$({ grep -Eow "${db}" <<<"${compose_migration_docs}" || true; } | wc -l | tr -d '[:space:]')"
  [[ "${count}" == '1' ]] || fail "F12: Compose Go-owned migration list must contain ${db} exactly once, got ${count}"
done
expect_fixed 'docs/DEPLOYMENT.md' 'voice_db' F12
expect_fixed 'docs/DEPLOYMENT.md' '000001_room_lifecycle' F12
expect_fixed 'docs/DEPLOYMENT.md' 'voice-migrate-voice-db' F12
expect_fixed 'docs/DEPLOYMENT.md' 'voice-voice-db-migrations' F12
expect_fixed 'docs/DEPLOYMENT.md' '/ready' F12
expect_paragraph_regex 'docs/DEPLOYMENT.md' "${f12_lifecycle_pattern}" F12
expect_regex 'docs/DEPLOYMENT.md' '(backup|резерв).*(voice_db)|voice_db.*(backup|резерв)' F12
expect_regex 'docs/DEPLOYMENT.md' '(restore|восстанов).*(voice_db)|voice_db.*(restore|восстанов)' F12
expect_fixed 'docs/OPERATIONS.md' 'voice_db' F12
expect_regex 'docs/OPERATIONS.md' '(Redis).*(projection|проекц|rebuild)|(projection|проекц|rebuild).*(Redis)' F12
expect_regex 'docs/OPERATIONS.md' '(isolated|изолирован).*(restore|восстанов)|(restore|восстанов).*(isolated|изолирован)' F12
expect_regex 'docs/OPERATIONS.md' '(never|не).*(write|писать|пишет).*(source|источник|исходн)' F12
expect_regex 'docs/MICROSERVICES.md' 'Voice Service.*(PostgreSQL|Postgres).*(voice_db)' F12
expect_regex 'docs/MICROSERVICES.md' 'Voice Service.*Redis.*(projection|проекц|rebuild|перестро)' F12
expect_regex 'docs/MICROSERVICES.md' '(source-disabled|не зарегистрирован|not registered).*(coordinator|handler|обработчик)' F12
expect_regex 'src/backend/voice/README.md' 'VOICE_DATABASE_URL.*voice_db' F12
expect_fixed 'src/backend/voice/README.md' 'POSTGRES_CONNECT_TIMEOUT' F12
expect_regex 'src/backend/voice/README.md' '/health.*(liveness|живуч)|((liveness|живуч).*/health)' F12
expect_regex 'src/backend/voice/README.md' '/ready.*(schema|схем)' F12
expect_regex 'src/backend/voice/README.md' '(missing|absent|отсутств).*(VOICE_DATABASE_URL).*(source-disabled|disabled|выключ)' F12
expect_regex 'deploy/staging/README.md' 'voice_db.*000001_room_lifecycle|000001_room_lifecycle.*voice_db' F12
expect_fixed 'deploy/staging/README.md' 'voice-voice-db-migrations' F12
expect_fixed 'deploy/staging/README.md' 'voice-migrate-voice-db' F12
expect_regex 'deploy/staging/README.md' '(migrat|миграц).*(before|до).*(voice-voice|Voice)' F12
expect_regex 'deploy/staging/README.md' 'VOICE_DATABASE_URL.*voice-app-secrets|voice-app-secrets.*VOICE_DATABASE_URL' F12

# F05-F12 remain part of both profiles. Only historical provenance replay moves
# out of ordinary PR validation; current input validation also runs as its own
# required job for every PR, even when this broad script suite is unselected.
case "${VOICE_CONTRACT_PROFILE:-full}" in
  current)
    ((failures == 0)) || exit 1
    echo 'Voice DB F05-F12 current provisioning/docs contract: PASS'
    exit 0
    ;;
  full) ;;
  *) echo 'Unknown VOICE_CONTRACT_PROFILE (expected current or full)' >&2; exit 2 ;;
esac

f13_standalone_suites() {
  python3 "${ROOT}/scripts/ci/f13_space_audit_test.py"
  if [[ -f "${ROOT}/scripts/ci/f13_production_source_test.py" ]]; then
    python3 "${ROOT}/scripts/ci/f13_production_source_test.py"
  fi
  if [[ -f "${ROOT}/scripts/ci/f13_release_source_test.py" ]]; then
    python3 "${ROOT}/scripts/ci/f13_release_source_test.py"
  fi
  if [[ -f "${ROOT}/scripts/ci/f13_native_p3_documentation_test.py" ]]; then
    python3 "${ROOT}/scripts/ci/f13_native_p3_documentation_test.py"
  fi
}

f13_start_standalone_suites() {
  local deferred=0
  trap 'deferred=130' INT
  trap 'deferred=143' TERM
  mkdir "${TMP_DIR}/f13-standalone"
  F13_SUITE_ROOT="${ROOT}" F13_SUITE_TMP="${TMP_DIR}" \
    python3 - "$(declare -f f13_standalone_suites)" <<'PY' &
import ctypes, os, signal, subprocess, sys, time

cancel = os.path.join(os.environ['F13_SUITE_TMP'], 'f13-standalone-cancel')
received = 0
def remember(sig, frame):
    global received
    received = received or sig
signal.signal(signal.SIGINT, remember)
signal.signal(signal.SIGTERM, remember)
if ctypes.CDLL(None, use_errno=True).prctl(36, 1, 0, 0, 0) != 0:
    raise OSError(ctypes.get_errno(), 'PR_SET_CHILD_SUBREAPER')
env = dict(os.environ, ROOT=os.environ['F13_SUITE_ROOT'], TMPDIR=os.path.join(os.environ['F13_SUITE_TMP'], 'f13-standalone'))
child = None
result = 1
try:
    child = subprocess.Popen(['/bin/bash', '-c', 'set -euo pipefail\n' + sys.argv[1] + '\nf13_standalone_suites'], env=env, start_new_session=True)
    observed = None
    while not received and not os.path.exists(cancel):
        observed = os.waitid(os.P_PID, child.pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
        if observed is not None:
            break
        time.sleep(.05)
    if observed is not None:
        result = observed.si_status if observed.si_code == os.CLD_EXITED else 128 + observed.si_status
    elif received:
        result = 128 + received
finally:
    if child is not None:
        # The unreaped leader reserves the process group identity throughout.
        for sig, delay in ((signal.SIGTERM, .2), (signal.SIGKILL, 0)):
            try:
                os.killpg(child.pid, sig)
            except ProcessLookupError:
                pass
            if delay:
                time.sleep(delay)
        while True:
            with open('/proc/self/task/%d/children' % os.getpid()) as stream:
                children = [int(pid) for pid in stream.read().split()]
            for pid in children:
                try:
                    os.kill(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
            try:
                while os.waitpid(-1, os.WNOHANG)[0]:
                    pass
            except ChildProcessError:
                break
            time.sleep(.01)
sys.exit(result)
PY
  f13_standalone_pid=$!
  trap 'exit 130' INT
  trap 'exit 143' TERM
  ((deferred == 0)) || exit "${deferred}"
}

f13_wait_standalone_suites() {
  local status=0
  wait "${f13_standalone_pid}" || status=$?
  f13_standalone_pid=''
  return "${status}"
}

printf '%s\n' '== F13/G7: source-disabled scope boundary =='
python3 "${ROOT}/scripts/ci/f13_native_p3_source_test.py"
python3 "${ROOT}/scripts/ci/f13_native_p3_source_history_test.py"
if [[ -f "${ROOT}/scripts/ci/f13_native_p3_successor.py" ]]; then
  python3 "${ROOT}/scripts/ci/f13_native_p3_successor_test.py"
  python3 "${ROOT}/scripts/ci/f13_native_p3_successor_integration_test.py"
fi
python3 "${ROOT}/scripts/ci/f13_guarded_merge_test.py"
python3 "${ROOT}/scripts/ci/f13_reviewed_test_merge_test.py"
python3 "${ROOT}/scripts/ci/f13_historical_composition_test.py"
python3 "${ROOT}/scripts/ci/f13_role_conditional_test.py"
if [[ -f "${ROOT}/scripts/ci/f13_role_conditional_successor_test.py" ]]; then
  python3 "${ROOT}/scripts/ci/f13_role_conditional_successor_test.py"
fi
f13_start_standalone_suites
base_sha="${VOICE_R22_BASE_SHA:-$(git -C "${ROOT}" merge-base HEAD origin/master)}"
# The later Game sprint authorizes the verified opt-in runtime checkpoint, not
# arbitrary future runtime or deployment changes. Exempt only its exact paths
# and Git blobs; the original R22.2 oracle still checks every remaining delta.
accepted_game_checkpoint='5ff441e04422e3ff1a4c0e1e7e81e3d17b8470eb'
# This immutable text record pins the owner-approved source implementation
# checkpoint. The record is a descendant of H and does not self-hash. These are
# source constants, never environment-provided proof; producer checks still
# validate the actual PR rows before merge and the actual master push afterward.
game_checkpoint_snapshot_base='28f1a0a26f3f23bfb9242592aea9c92fd659ee2d'
game_checkpoint_snapshot_head='6fe3f778538c0cfff689bb8fc40543010d158944'
game_checkpoint_snapshot_record='6b2c9a602c920bcbfd785f400f0f3d8560587db9'
game_checkpoint_aggregate_active=false
game_checkpoint_aggregate_base=''
game_checkpoint_aggregate_head=''
game_checkpoint_aggregate_target=''
game_checkpoint_aggregate_paths=''
game_checkpoint_aggregate_map=''
game_checkpoint_aggregate_record=''
game_checkpoint_aggregate_alias_map=''
game_checkpoint_enabled=false
if git -C "${ROOT}" merge-base --is-ancestor "${accepted_game_checkpoint}" HEAD; then
  game_checkpoint_enabled=true
fi
git -C "${ROOT}" ls-tree -r "${accepted_game_checkpoint}" >"${TMP_DIR}/game-checkpoint-tree"
game_checkpoint_path_authorized_in_delta() {
  local path="$1" delta="$2" actual_blob="$3" expected_blob
  case "${path}" in
    protos/*|src/backend/voice/*|src/backend/role/*|src/backend/space/*|src/backend/*/pb/*|src/backend/migrations/voice_db/*) ;;
    *) return 1 ;;
  esac
  grep -Fxq -- "${path}" "${delta}" || return 1
  expected_blob="$(awk -F '\t' -v path="${path}" '$2 == path { split($1, fields, " "); print fields[3] }' "${TMP_DIR}/game-checkpoint-tree")"
  [[ -n "${expected_blob}" && "${actual_blob}" == "${expected_blob}" ]]
}
game_checkpoint_path_allowed() {
  local path="$1" delta="$2" actual_blob
  [[ "${game_checkpoint_enabled}" == true && -f "${ROOT}/${path}" ]] || return 1
  actual_blob="$(git -C "${ROOT}" hash-object --path="${path}" "${ROOT}/${path}")"
  game_checkpoint_path_authorized_in_delta "${path}" "${delta}" "${actual_blob}"
}

# A later owner-approved Game snapshot is admitted as one immutable projection,
# never as a per-file union with the legacy 5ff checkpoint. Callers supply the
# actual F13 row base/head plus a fixed approved snapshot tuple (base, head,
# record commit); no environment value can select or construct a proof.
game_checkpoint_domain_path() {
  case "$1" in
    protos/*|src/backend/voice/*|src/backend/role/*|src/backend/space/*|src/backend/*/pb/*|src/backend/migrations/voice_db/*)
      return 0 ;;
    *) return 1 ;;
  esac
}

game_checkpoint_projection_authorized_in_delta() {
  local repo="$1" target="$2" source_base="$3" source_head="$4"
  local checkpoint_base="$5" checkpoint_head="$6" checkpoint_commit="$7" delta="$8"
  local expected="${TMP_DIR}/game-projection-expected" actual="${TMP_DIR}/game-projection-actual"
  local path checkpoint_entry target_entry

  git -C "${repo}" rev-parse --verify "${checkpoint_base}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" rev-parse --verify "${checkpoint_head}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" rev-parse --verify "${checkpoint_commit}^{commit}" >/dev/null 2>&1 || return 1
  [[ "${checkpoint_base}" != "${checkpoint_head}" &&
    "${source_base}" != "${source_head}" &&
    "${source_head}" != "${target}" && "${source_base}" != "${target}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${checkpoint_base}" "${source_base}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${checkpoint_base}" "${checkpoint_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${source_base}" "${source_head}" 2>/dev/null || return 1
  [[ "${checkpoint_head}" != "${checkpoint_commit}" && "${checkpoint_commit}" != "${source_head}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${checkpoint_head}" "${checkpoint_commit}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${checkpoint_commit}" "${source_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${source_head}" "${target}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${source_base}" "${target}" 2>/dev/null || return 1

  : >"${expected}"
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    game_checkpoint_domain_path "${path}" && printf '%s\n' "${path}" >>"${expected}"
  done < <(git -C "${repo}" diff --name-only --no-renames "${checkpoint_base}" "${checkpoint_head}")
  LC_ALL=C sort -u -o "${expected}" "${expected}"
  : >"${actual}"
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    game_checkpoint_domain_path "${path}" && printf '%s\n' "${path}" >>"${actual}"
  done <"${delta}"
  LC_ALL=C sort -u -o "${actual}" "${actual}"
  cmp -s "${expected}" "${actual}" || return 1

  # P is exactly the Game-family contribution captured at H. Compare complete
  # literal entries only for P; later base-owned Game paths remain subject to
  # their own provenance gates and are not claimed by this snapshot.
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    checkpoint_entry="$(git -C "${repo}" ls-tree -r "${checkpoint_head}" -- ":(literal)${path}")" || return 1
    target_entry="$(git -C "${repo}" ls-tree -r "${target}" -- ":(literal)${path}")" || return 1
    [[ "${checkpoint_entry}" == "${target_entry}" ]] || return 1
  done <"${expected}"
  return 0
}

game_checkpoint_contribution_allowed() {
  local repo="$1" target="$2" source_base="$3" source_head="$4" delta="$5"
  [[ -n "${game_checkpoint_snapshot_base}" && -n "${game_checkpoint_snapshot_head}" &&
    -n "${game_checkpoint_snapshot_record}" ]] || return 1
  game_checkpoint_projection_authorized_in_delta "${repo}" "${target}" \
    "${source_base}" "${source_head}" "${game_checkpoint_snapshot_base}" \
    "${game_checkpoint_snapshot_head}" "${game_checkpoint_snapshot_record}" "${delta}"
}

game_checkpoint_aggregate_binding_valid() {
  local repo="$1"
  [[ "${game_checkpoint_aggregate_active}" == true && -s "${game_checkpoint_aggregate_paths}" &&
    -n "${game_checkpoint_snapshot_base}" && -n "${game_checkpoint_snapshot_head}" &&
    -n "${game_checkpoint_snapshot_record}" && -n "${game_checkpoint_aggregate_base}" &&
    -n "${game_checkpoint_aggregate_head}" && -n "${game_checkpoint_aggregate_target}" &&
    "${game_checkpoint_aggregate_record}" == "${game_checkpoint_snapshot_record}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_snapshot_base}" \
    "${game_checkpoint_aggregate_base}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_snapshot_base}" \
    "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_aggregate_base}" \
    "${game_checkpoint_aggregate_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_snapshot_head}" \
    "${game_checkpoint_snapshot_record}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_snapshot_record}" \
    "${game_checkpoint_aggregate_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${game_checkpoint_aggregate_head}" \
    "${game_checkpoint_aggregate_target}" 2>/dev/null
}

f13_tree_entry_tuple_for_path() {
  local path="$1" entry="$2" tuple entry_path mode type oid extra
  if [[ -z "${entry}" ]]; then
    printf '\n'
    return 0
  fi
  if [[ "${entry}" == *$'\t'* ]]; then
    entry_path="${entry#*$'\t'}"
    [[ "${entry_path}" == "${path}" ]] || return 1
    tuple="${entry%%$'\t'*}"
  else
    tuple="${entry}"
  fi
  IFS=' ' read -r mode type oid extra <<<"${tuple}"
  [[ -z "${extra}" &&
    "${mode}" =~ ^(040000|100644|100755|120000|160000)$ &&
    "${type}" =~ ^(blob|tree|commit)$ &&
    "${oid}" =~ ^[0-9a-f]{40,64}$ ]] || return 1
  printf '%s\n' "${tuple}"
}

f13_tree_entry_from_records() {
  local path="$1" raw_file="$2" entry
  local -a entries=()
  mapfile -d '' -t entries <"${raw_file}" || return 1
  [[ "${#entries[@]}" -le 1 ]] || return 1
  if [[ "${#entries[@]}" == 0 ]]; then
    printf '\n'
    return 0
  fi
  entry="${entries[0]}"
  [[ "${entry}" == *$'\t'* ]] || return 1
  f13_tree_entry_tuple_for_path "${path}" "${entry}"
}

f13_tree_entry_raw_for_path() {
  local path="$1" raw_file="$2" entry
  local -a entries=()
  mapfile -d '' -t entries <"${raw_file}" || return 1
  [[ "${#entries[@]}" -le 1 ]] || return 1
  if [[ "${#entries[@]}" == 0 ]]; then
    printf '\n'
    return 0
  fi
  entry="${entries[0]}"
  [[ "${entry}" == *$'\t'* && "${entry#*$'\t'}" == "${path}" ]] || return 1
  # Validate the mode/type/OID while retaining ls-tree's raw record form for
  # callers whose established comparison contract is raw-to-raw.
  f13_tree_entry_tuple_for_path "${path}" "${entry}" >/dev/null || return 1
  printf '%s\n' "${entry}"
}

f13_git_tree_entry_record() {
  local repo="$1" commit="$2" path="$3" raw_file="${TMP_DIR}/f13-tree-record-${BASHPID}"
  git -C "${repo}" ls-tree -z "${commit}" -- ":(literal)${path}" >"${raw_file}" 2>/dev/null || return 1
  f13_tree_entry_raw_for_path "${path}" "${raw_file}"
}

f13_git_tree_entry() {
  local repo="$1" commit="$2" path="$3" raw_file="${TMP_DIR}/f13-tree-entry-${BASHPID}"
  git -C "${repo}" ls-tree -z "${commit}" -- ":(literal)${path}" >"${raw_file}" 2>/dev/null || return 1
  f13_tree_entry_from_records "${path}" "${raw_file}"
}

f13_tree_entry_normalizer_fixtures() {
  local records="${TMP_DIR}/f13-tree-entry-normalizer-records" actual
  printf '%s\0' '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395	:(glob)literal.sql' >"${records}" || return 1
  actual="$(f13_tree_entry_from_records ':(glob)literal.sql' "${records}")" || return 1
  [[ "${actual}" == '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' ]] || return 1
  if f13_tree_entry_from_records 'different.sql' "${records}" >/dev/null; then
    return 1
  fi
  printf '%s\0%s\0' \
    '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395	:(glob)literal.sql' \
    '100644 blob 9c506399d95a0f460bbdff8972b14fff9bd645ca	:(glob)literal.sql' >"${records}" || return 1
  if f13_tree_entry_from_records ':(glob)literal.sql' "${records}" >/dev/null; then
    return 1
  fi
  : >"${records}" || return 1
  actual="$(f13_tree_entry_from_records 'missing.sql' "${records}")" || return 1
  [[ -z "${actual}" ]] || return 1
  printf '%s\0' '100755 blob 19bd5687fea0f556260d27fd1ecf5308de835395	mode.sql' >"${records}" || return 1
  actual="$(f13_tree_entry_from_records 'mode.sql' "${records}")" || return 1
  [[ "${actual}" != '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' ]] || return 1
  printf '%s\0' '100644 tree 19bd5687fea0f556260d27fd1ecf5308de835395	type.sql' >"${records}" || return 1
  actual="$(f13_tree_entry_from_records 'type.sql' "${records}")" || return 1
  [[ "${actual}" != '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' ]]
}

game_checkpoint_aggregate_path_allowed() {
  local repo="$1" commit="$2" path="$3" source_entry="$4" checkpoint_entry
  game_checkpoint_aggregate_binding_valid "${repo}" || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" || return 1
  git -C "${repo}" merge-base --is-ancestor "${commit}" "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  checkpoint_entry="$(f13_git_tree_entry_record "${repo}" "${game_checkpoint_snapshot_head}" "${path}")" || return 1
  [[ "${source_entry}" == "${checkpoint_entry}" ]]
}

# A source path may be represented by two nested owners when a pre-H merge
# integrates both histories and produces the exact immutable checkpoint entry.
# Replace the prior row only for that fully verified integration; ordinary
# independent or post-H duplicate owners remain ambiguous and fail closed.
game_checkpoint_duplicate_owner_supersession_allowed() {
  local repo="$1" path="$2" prior_owner="$3" prior_source_head="$4"
  local current_owner="$5" current_source_head="$6" integration_commit="$7"
  local merge_outcome="${8:-unverified}" integration_entry checkpoint_entry
  game_checkpoint_aggregate_binding_valid "${repo}" || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" || return 1
  for candidate in "${prior_owner}" "${prior_source_head}" "${current_owner}" \
    "${current_source_head}" "${integration_commit}"; do
    [[ -n "${candidate}" ]] || return 1
    git -C "${repo}" merge-base --is-ancestor "${candidate}" \
      "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  done
  [[ "${merge_outcome}" == clean || "${merge_outcome}" == guarded_conflict ]] || return 1
  integration_entry="$(f13_git_tree_entry_record "${repo}" "${integration_commit}" "${path}")" || return 1
  checkpoint_entry="$(f13_git_tree_entry_record "${repo}" "${game_checkpoint_snapshot_head}" "${path}")" || return 1
  if [[ "${integration_entry}" == "${checkpoint_entry}" ]]; then
    return 0
  fi
  if [[ "${merge_outcome}" == clean ]]; then
    # A verified clean merge can carry a temporary non-H entry. This is only a
    # taint-preserving owner replacement; a later exact-H pre-H owner must clear
    # it, or the strict aggregate final-entry check rejects the result.
    return 2
  fi
  # A path-specific guarded conflict with a non-H result may replace the
  # intermediate owner only while retaining its guarded-entry taint. The
  # strict aggregate A/T comparison still requires a later exact-H owner.
  # Unknown outcomes were rejected above; this branch is reached only for the
  # caller's parsed guarded-path conflict result.
  return 2
}

# A guarded path from an approved checkpoint can be carried through an ordinary
# merge when the merge producer combines disjoint edits from both parents. The
# checkpoint still authorizes only the source-side contribution; this helper
# independently verifies that the result is exactly Git's clean three-way
# merge for that path. It tolerates conflicts on other paths and reports a
# guarded-path conflict distinctly so only a qualified intermediate checkpoint
# frame can defer it; the aggregate boundary still requires the pinned entry.
f13_checkpoint_merge_tree_entry_matches() {
  local repo="$1" expected_base="$2" first_parent="$3" source_head="$4"
  local path="$5" result_entry="$6" bases_file output_file status tree_oid expected_entry output_fd
  local conflict_path guarded_conflict=false
  local exact_merge="${7:-}" exact_capability="${8:-}" exact_observer="${9:-}" exact_receipt exact_3071_capability="${10:-}"
  local corrected_F_capability="${11:-}"
  local collector_base="${12:-}" collector_target="${13:-}"
  bases_file="${TMP_DIR}/f13-checkpoint-merge-bases"
  output_file="${TMP_DIR}/f13-checkpoint-merge-tree"
  git -C "${repo}" merge-base --all "${first_parent}" "${source_head}" \
    >"${bases_file}" 2>/dev/null || return 1
  if [[ "$(wc -l <"${bases_file}" | tr -d '[:space:]')" != 1 ||
    "$(<"${bases_file}")" != "${expected_base}" ]]; then
    if [[ "${exact_merge}" == 132437c43f6f0625fae8954542f8d9c2f6aa3037 &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]]; then
      local -a exact_132_trace=()
      if [[ "${collector_base}" == 2c8bdf9e2f420b724575096beb4fcf5cb4ea00be &&
        "${collector_target}" == 011c53f4a2be3c4e8f660fa9dc6ce40492da20e2 ]]; then
        exact_132_trace=("${f13_f54_trace_file:?}")
      fi
      exact_receipt="$(python3 "${exact_observer}" 132-composition "${repo}" "${exact_merge}" \
        "${expected_base}" "${first_parent}" "${source_head}" "${path}" "${result_entry}" \
        "${collector_base}" "${collector_target}" "${exact_132_trace[@]}")" || return 1
      # Whole automatic-result evidence only. Existing Native owner/runtime
      # checks remain mandatory and receive no permission from this receipt.
      f13_f54_trace_event composition_132_capability "${repo}" "${exact_receipt}" || return 1
      return 0
    fi
    if [[ -n "${exact_3071_capability}" && "${exact_merge}" == 3071ddad390443de4119c796cdad9899ff08f7fd &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]] &&
      exact_receipt="$(python3 "${exact_observer}" 3071-composition "${repo}" "${exact_3071_capability}" \
        "${exact_merge}" "${expected_base}" "${first_parent}" "${source_head}" "${path}" "${result_entry}")"; then
      # A per-path result references the separate reviewed whole composition.
      # Its receipt retains actual Git CONFLICT/EXIT1 and every conflict path.
      f13_f54_trace_event composition_3071_capability "${repo}" "${exact_receipt}" || return 1
      return 0
    fi
    if [[ -n "${exact_merge}" && -n "${exact_capability}" &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]] &&
      exact_receipt="$(python3 "${exact_observer}" cap-composition "${repo}" "${exact_capability}" \
        "${exact_merge}" "${expected_base}" "${first_parent}" "${source_head}" "${path}" "${result_entry}" \
        "${collector_base}" "${collector_target}")"; then
      # A distinct complete-unit whole-parent proof answers this exact seam.
      # It does not relabel O as a best base or waive any native/source gate.
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event composition_capability "${repo}" "${exact_receipt}" || return 1
      fi
      return 0
    fi
    # Prototype observation is a distinct whole-composition result, NEVER an
    # admission or a conversion of the original status1 into status2/clean.
    if [[ -n "${f13_f54_trace_file:-}" && -n "${f13_f54_observer:-}" &&
      "${first_parent}" == 0a0024900b5bb3ba2f6fee391615f0f4fa0864e3 &&
      "${source_head}" == f15d718b2d20837ae5503dc049134b66592b3675 ]]; then
      local f13_f54_composition
      if f13_f54_composition="$(python3 "${f13_f54_observer}" composition "${repo}" \
        "${expected_base}" "${first_parent}" "${source_head}" "${path}" "${result_entry}")"; then
        f13_f54_trace_event composition_observation "${repo}" "${f13_f54_composition}" || return 1
      fi
    fi
    return 1
  fi

  if git -C "${repo}" merge-tree --write-tree --name-only -z --no-messages \
    "${first_parent}" "${source_head}" \
    >"${output_file}" 2>/dev/null; then
    status=0
  else
    status=$?
  fi
  # merge-tree uses 1 for an ordinary content conflict; other errors fail
  # closed. Its first output line remains the resulting tree object.
  [[ "${status}" == 0 || "${status}" == 1 ]] || return 1
  exec {output_fd}<"${output_file}" || return 1
  IFS= read -r -d '' tree_oid <&"${output_fd}" || {
    exec {output_fd}<&-
    return 1
  }
  if [[ ! "${tree_oid}" =~ ^[0-9a-f]{40,64}$ ]]; then
    exec {output_fd}<&-
    return 1
  fi
  while IFS= read -r -d '' conflict_path <&"${output_fd}"; do
    [[ "${conflict_path}" != "${path}" ]] || guarded_conflict=true
  done
  exec {output_fd}<&-
  if [[ -n "${corrected_F_capability}" && "${exact_merge}" == fa1528a7c232a644218edf49811e2bb2e373105f &&
    "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]] &&
    exact_receipt="$(python3 "${exact_observer}" corrected-F-composition "${repo}" "${corrected_F_capability}" \
      "${exact_merge}" "${expected_base}" "${first_parent}" "${source_head}" "${path}" "${result_entry}")"; then
    # Qualified historical composition retains Git's original EXIT1/conflicts,
    # all33 manual entries and the known defect plus mandatory current repair.
    # No source/owner/raw runtime flag or ordinary native policy is rewritten.
    if [[ -n "${f13_f54_trace_file:-}" ]]; then
      f13_f54_trace_event corrected_F_composition "${repo}" "${exact_receipt}" || return 1
    fi
    return 0
  fi
  if [[ "${guarded_conflict}" == true ]]; then
    # A parsed, path-specific conflict is distinct from a failed or malformed
    # merge-tree computation. The collector may defer only this known result
    # for a qualified intermediate checkpoint frame; its aggregate boundary
    # still verifies the final entry against the pinned snapshot.
    return 2
  fi
  expected_entry="$(git -C "${repo}" ls-tree "${tree_oid}" -- "${path}" 2>/dev/null)" || return 1
  [[ "${expected_entry}" == "${result_entry}" ]]
}

f13_checkpoint_merge_tree_fixtures() {
  local repo guarded other base source parent result result_entry expected_entry tree_oid
  local merge_output merge_status wrong_blob wrong_entry mode_entry type_entry target
  local index_file manual_tree real_git wrapper_dir conflict_record conflict_target post_h
  local guarded_conflict_status supersession_status source_entry checkpoint_entry
  local -A fixture_guarded_entries=()
  repo="${TMP_DIR}/f13-checkpoint-merge-tree-fixture"
  guarded='protos/voice/chat/v1/fixture.proto'
  other='protos/voice/chat/v1/unrelated.proto'
  mkdir -p "${repo}"
  git -C "${repo}" init -q || return 1
  git -C "${repo}" config user.name 'Voice CI fixture'
  git -C "${repo}" config user.email 'voice-ci@example.invalid'
  git -C "${repo}" config commit.gpgsign false
  mkdir -p "${repo}/protos/voice/chat/v1"
  printf '%s\n' 'left base' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right base' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  printf '%s\n' 'other base' >"${repo}/${other}"
  git -C "${repo}" add . && git -C "${repo}" commit -qm 'F13 merge fixture base' || return 1
  base="$(git -C "${repo}" rev-parse HEAD)" || return 1

  # Both sides change disjoint lines in the guarded path; Git's clean merge is
  # the exact expected entry even though it differs from the source parent.
  git -C "${repo}" checkout -qb clean-source "${base}" || return 1
  printf '%s\n' 'left source' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right base' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  git -C "${repo}" add "${guarded}" && git -C "${repo}" commit -qm 'F13 clean source edit' || return 1
  source="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" checkout -qb clean-parent "${base}" || return 1
  printf '%s\n' 'left base' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right parent' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  git -C "${repo}" add "${guarded}" && git -C "${repo}" commit -qm 'F13 clean parent edit' || return 1
  parent="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" merge --no-ff -qm 'F13 clean disjoint guarded merge' clean-source || return 1
  result="$(git -C "${repo}" rev-parse HEAD)" || return 1
  result_entry="$(git -C "${repo}" ls-tree "${result}" -- "${guarded}")" || return 1
  f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${result_entry}" || {
    printf '%s\n' 'F13 oracle bug: clean disjoint guarded merge was rejected' >&2
    return 1
  }

  # A manual resolution that changes the auto-merged guarded result is not
  # equivalent to the deterministic merge tree and must remain rejected.
  merge_output="${TMP_DIR}/f13-checkpoint-manual-merge-tree"
  git -C "${repo}" merge-tree --write-tree --name-only -z --no-messages \
    "${parent}" "${source}" >"${merge_output}" 2>/dev/null || return 1
  tree_oid="$(git -C "${repo}" rev-parse "${result}^{tree}")" || return 1
  wrong_blob="$(printf '%s\n' 'manual guarded resolution' | git -C "${repo}" hash-object -w --stdin)" || return 1
  index_file="${TMP_DIR}/f13-checkpoint-manual.index"
  GIT_INDEX_FILE="${index_file}" git -C "${repo}" read-tree "${tree_oid}" || return 1
  GIT_INDEX_FILE="${index_file}" git -C "${repo}" update-index --cacheinfo \
    "100644,${wrong_blob},${guarded}" || return 1
  manual_tree="$(GIT_INDEX_FILE="${index_file}" git -C "${repo}" write-tree)" || return 1
  target="$(git -C "${repo}" commit-tree "${manual_tree}" -p "${parent}" -p "${source}" \
    <<< 'F13 manual guarded merge resolution')" || return 1
  wrong_entry="$(git -C "${repo}" ls-tree "${target}" -- "${guarded}")" || return 1
  if f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${wrong_entry}"; then
    printf '%s\n' 'F13 oracle bug: manual guarded resolution differing from Git merge was accepted' >&2
    return 1
  fi

  # An unrelated file conflict is allowed for this path-level comparison when
  # Git reports the guarded path as clean and its full entry matches the tree.
  git -C "${repo}" checkout -qb unrelated-source "${base}" || return 1
  printf '%s\n' 'left source' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right base' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  printf '%s\n' 'unrelated source' >"${repo}/${other}"
  git -C "${repo}" add . && git -C "${repo}" commit -qm 'F13 unrelated-conflict source' || return 1
  source="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" checkout -qb unrelated-parent "${base}" || return 1
  printf '%s\n' 'left base' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right parent' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  printf '%s\n' 'unrelated parent' >"${repo}/${other}"
  git -C "${repo}" add . && git -C "${repo}" commit -qm 'F13 unrelated-conflict parent' || return 1
  parent="$(git -C "${repo}" rev-parse HEAD)" || return 1
  merge_output="${TMP_DIR}/f13-checkpoint-unrelated-merge-tree"
  if git -C "${repo}" merge-tree --write-tree --name-only --no-messages \
    "${parent}" "${source}" >"${merge_output}" 2>/dev/null; then
    merge_status=0
  else
    merge_status=$?
  fi
  [[ "${merge_status}" == 1 ]] || return 1
  tree_oid="$(head -n 1 "${merge_output}")" || return 1
  [[ "${tree_oid}" =~ ^[0-9a-f]{40,64}$ ]] || return 1
  ! tail -n +2 "${merge_output}" | grep -Fxq -- "${guarded}" || return 1
  git -C "${repo}" commit-tree "${tree_oid}" -p "${parent}" -p "${source}" \
    <<< 'F13 unrelated conflict result' >"${TMP_DIR}/f13-checkpoint-unrelated-result" || return 1
  result="$(<"${TMP_DIR}/f13-checkpoint-unrelated-result")"
  result_entry="$(git -C "${repo}" ls-tree "${result}" -- "${guarded}")" || return 1
  f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${result_entry}" || {
    printf '%s\n' 'F13 oracle bug: unrelated conflict rejected a clean guarded merge entry' >&2
    return 1
  }

  # A conflict on the guarded path and a manual resolution that differs from
  # Git's clean result must both fail closed.
  git -C "${repo}" checkout -qb overlap-source "${base}" || return 1
  printf '%s\n' 'left source' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right base' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  git -C "${repo}" add "${guarded}" && git -C "${repo}" commit -qm 'F13 overlapping source edit' || return 1
  source="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" checkout -qb overlap-parent "${base}" || return 1
  printf '%s\n' 'left parent' 'stable one' 'stable two' 'stable three' \
    'stable four' 'stable five' 'stable six' 'stable seven' 'stable eight' \
    'stable nine' 'stable ten' 'stable eleven' 'stable twelve' 'stable thirteen' \
    'stable fourteen' 'right base' 'stable fifteen' 'stable sixteen' >"${repo}/${guarded}"
  git -C "${repo}" add "${guarded}" && git -C "${repo}" commit -qm 'F13 overlapping parent edit' || return 1
  parent="$(git -C "${repo}" rev-parse HEAD)" || return 1
  merge_output="${TMP_DIR}/f13-checkpoint-overlap-merge-tree"
  if git -C "${repo}" merge-tree --write-tree --name-only --no-messages \
    "${parent}" "${source}" >"${merge_output}" 2>/dev/null; then
    merge_status=0
  else
    merge_status=$?
  fi
  [[ "${merge_status}" == 1 ]] && tail -n +2 "${merge_output}" | grep -Fxq -- "${guarded}" || return 1
  tree_oid="$(head -n 1 "${merge_output}")" || return 1
  target="$(git -C "${repo}" commit-tree "${tree_oid}" -p "${parent}" -p "${source}" \
    <<< 'F13 guarded conflict result')" || return 1
  result_entry="$(git -C "${repo}" ls-tree "${target}" -- "${guarded}")" || return 1
  if f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${result_entry}"; then
    printf '%s\n' 'F13 oracle bug: guarded-path conflict was accepted' >&2
    return 1
  fi

  # A path-specific conflict can be superseded only when the immutable H tree
  # contains that exact integration result and every owner is within H. The
  # result is a real merge commit in the fixture graph, not a synthesized row.
  guarded_conflict_status=0
  if f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${result_entry}"; then
    guarded_conflict_status=0
  else
    guarded_conflict_status=$?
  fi
  [[ "${guarded_conflict_status}" == 2 ]] || return 1
  conflict_record="$(git -C "${repo}" commit-tree "${target}^{tree}" -p "${target}" \
    <<< 'F13 immutable conflict snapshot record')" || return 1
  conflict_target="$(git -C "${repo}" commit-tree "${conflict_record}^{tree}" -p "${conflict_record}" \
    <<< 'F13 checkpoint-qualified conflict result')" || return 1
  post_h="$(git -C "${repo}" commit-tree "${conflict_target}^{tree}" -p "${conflict_target}" \
    <<< 'F13 post-H conflict owner')" || return 1
  game_checkpoint_snapshot_base="${base}"
  game_checkpoint_snapshot_head="${target}"
  game_checkpoint_snapshot_record="${conflict_record}"
  game_checkpoint_aggregate_active=true
  game_checkpoint_aggregate_base="${base}"
  game_checkpoint_aggregate_head="${conflict_record}"
  game_checkpoint_aggregate_target="${conflict_target}"
  game_checkpoint_aggregate_record="${conflict_record}"
  game_checkpoint_aggregate_paths="${TMP_DIR}/f13-checkpoint-conflict-paths"
  printf '%s\n' "${guarded}" >"${game_checkpoint_aggregate_paths}"
  if ! game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${target}" guarded_conflict; then
    printf '%s\n' 'F13 oracle bug: exact-H guarded-path conflict integration was rejected' >&2
    return 1
  fi
  source_entry="$(git -C "${repo}" ls-tree "${source}" -- "${guarded}")" || return 1
  checkpoint_entry="$(git -C "${repo}" ls-tree "${target}" -- "${guarded}")" || return 1
  fixture_guarded_entries["${guarded}"]="${source_entry}"
  if game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${source}" guarded_conflict; then
    supersession_status=0
  else
    supersession_status=$?
  fi
  [[ "${supersession_status}" == 2 ]] || {
    printf '%s\n' 'F13 oracle bug: qualified non-H conflict did not retain guarded taint' >&2
    return 1
  }
  [[ "${fixture_guarded_entries[${guarded}]}" == "${source_entry}" &&
    "${fixture_guarded_entries[${guarded}]}" != "${checkpoint_entry}" ]] || {
    printf '%s\n' 'F13 oracle bug: non-H conflict cleared or lost guarded-entry taint' >&2
    return 1
  }
  # A missing or wrong restoration cannot satisfy the strict final target
  # comparison; only an actual pre-H owner with the exact H entry may clear it.
  if f13_guarded_entry_matches_final_target \
    "${fixture_guarded_entries[${guarded}]}" "${checkpoint_entry}"; then
    printf '%s\n' 'F13 oracle bug: missing H restoration passed final guarded-entry comparison' >&2
    return 1
  fi
  if f13_guarded_entry_matches_final_target "" "${checkpoint_entry}" ||
    f13_guarded_entry_matches_final_target "100644 blob wrong-entry" "${checkpoint_entry}"; then
    printf '%s\n' 'F13 oracle bug: missing or wrong guarded owner passed final target comparison' >&2
    return 1
  fi
  fixture_guarded_entries["${guarded}"]=''
  [[ -n "${fixture_guarded_entries[${guarded}]+present}" ]] && \
    f13_guarded_entry_matches_final_target "${fixture_guarded_entries[${guarded}]}" '' || {
    printf '%s\n' 'F13 oracle bug: exact deleted-path entry was not distinguishable from missing owner' >&2
    return 1
  }
  unset 'fixture_guarded_entries[$guarded]'
  [[ -z "${fixture_guarded_entries[${guarded}]+present}" ]] || {
    printf '%s\n' 'F13 oracle bug: missing guarded-owner map key was not detected' >&2
    return 1
  }
  if game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${target}" guarded_conflict; then
    supersession_status=0
  else
    supersession_status=$?
  fi
  [[ "${supersession_status}" == 0 ]] || {
    printf '%s\n' 'F13 oracle bug: exact pre-H restoration did not clear guarded taint' >&2
    return 1
  }
  fixture_guarded_entries["${guarded}"]="${checkpoint_entry}"
  f13_guarded_entry_matches_final_target \
    "${fixture_guarded_entries[${guarded}]}" "${checkpoint_entry}" || {
    printf '%s\n' 'F13 oracle bug: exact-H owner did not satisfy final guarded-entry comparison' >&2
    return 1
  }
  if game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${source}" guarded_conflict ||
    game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${other}" \
      "${source}" "${source}" "${source}" "${source}" "${target}" guarded_conflict ||
    game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
      "${source}" "${source}" "${source}" "${source}" "${post_h}" guarded_conflict ||
    game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
      "${post_h}" "${source}" "${source}" "${source}" "${target}" guarded_conflict; then
    printf '%s\n' 'F13 oracle bug: wrong result, non-P path, or post-H conflict owner/integration was accepted' >&2
    return 1
  fi
  game_checkpoint_aggregate_record="$(printf '%040d' 0)"
  if game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${target}" guarded_conflict; then
    printf '%s\n' 'F13 oracle bug: guarded conflict used an unpinned checkpoint record' >&2
    return 1
  fi
  game_checkpoint_aggregate_active=false
  game_checkpoint_aggregate_record="${conflict_record}"
  if game_checkpoint_duplicate_owner_supersession_allowed "${repo}" "${guarded}" \
    "${source}" "${source}" "${source}" "${source}" "${target}" guarded_conflict; then
    printf '%s\n' 'F13 oracle bug: guarded conflict supersession worked with no active checkpoint' >&2
    return 1
  fi
  game_checkpoint_aggregate_active=false

  git -C "${repo}" checkout -qb mode-source "${base}" || return 1
  printf '%s\n' 'left source' 'right base' >"${repo}/${guarded}"
  git -C "${repo}" add "${guarded}" && git -C "${repo}" commit -qm 'F13 mode source edit' || return 1
  source="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" checkout -qb mode-parent "${base}" || return 1
  git -C "${repo}" update-index --chmod=+x "${guarded}" && \
    git -C "${repo}" commit -qm 'F13 mode parent edit' || return 1
  parent="$(git -C "${repo}" rev-parse HEAD)" || return 1
  merge_output="${TMP_DIR}/f13-checkpoint-mode-merge-tree"
  git -C "${repo}" merge-tree --write-tree --name-only --no-messages \
    "${parent}" "${source}" >"${merge_output}" 2>/dev/null || return 1
  tree_oid="$(head -n 1 "${merge_output}")" || return 1
  expected_entry="$(git -C "${repo}" ls-tree "${tree_oid}" -- "${guarded}")" || return 1
  [[ "${expected_entry%% *}" == 100755 ]] || return 1
  result_entry="100644${expected_entry#100755}"
  if f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${result_entry}"; then
    printf '%s\n' 'F13 oracle bug: mode mismatch was accepted' >&2
    return 1
  fi
  type_entry="${expected_entry/ blob / commit }"
  if f13_checkpoint_merge_tree_entry_matches "${repo}" "${base}" "${parent}" \
    "${source}" "${guarded}" "${type_entry}"; then
    printf '%s\n' 'F13 oracle bug: object type mismatch was accepted' >&2
    return 1
  fi

  real_git="$(command -v git)" || return 1
  wrapper_dir="${TMP_DIR}/f13-malformed-merge-tree-bin"
  mkdir -p "${wrapper_dir}" || return 1
  {
    printf '%s\n' '#!/usr/bin/env bash' 'for argument in "$@"; do' \
      '  if [[ "$argument" == merge-tree ]]; then printf "malformed-output"; exit 0; fi' \
      'done'
    printf 'exec %q "$@"\n' "${real_git}"
  } >"${wrapper_dir}/git" || return 1
  chmod +x "${wrapper_dir}/git" || return 1
  if PATH="${wrapper_dir}:${PATH}" f13_checkpoint_merge_tree_entry_matches \
    "${repo}" "${base}" "${parent}" "${source}" "${guarded}" "${expected_entry}"; then
    printf '%s\n' 'F13 oracle bug: malformed merge-tree output was accepted' >&2
    return 1
  fi
}

game_checkpoint_aggregate_authorized() {
  local repo="$1" event_base="$2" source_head="$3" target="$4" delta="$5" paths="$6"
  [[ -n "${game_checkpoint_snapshot_base}" && -n "${game_checkpoint_snapshot_head}" &&
    -n "${game_checkpoint_snapshot_record}" ]] || return 1
  game_checkpoint_projection_authorized_in_delta "${repo}" "${target}" "${event_base}" \
    "${source_head}" "${game_checkpoint_snapshot_base}" "${game_checkpoint_snapshot_head}" \
    "${game_checkpoint_snapshot_record}" "${delta}" || return 1
  cp "${TMP_DIR}/game-projection-expected" "${paths}"
}

f13_extract_versioned_alias_section() {
  local repo="$1" record="$2" begin="$3" end="$4" output="$5"
  local document="${TMP_DIR}/f13-versioned-alias-record-${record}.md"
  git -C "${repo}" show "${record}:docs/testing/game-integrations-exec-plan.md" >"${document}" 2>/dev/null || return 1
  awk -v begin="${begin}" -v end="${end}" '
    $0 == begin { if (inside || seen) exit 2; inside=1; seen++; next }
    $0 == end { if (!inside) exit 2; inside=0; ended++; next }
    inside { print }
    END { if (inside || seen != 1 || ended != 1) exit 2 }
  ' "${document}" >"${output}" || return 1
  [[ -s "${output}" ]]
}

# The R3 record grants one explicit migration renumber only. Both complete
# source rows, their original owner/base/integration lineage, the competing
# baseline pair, and the destination projection must match the immutable R3
# document and Git trees. This never widens P or treats equal bytes as authority.
f13_migration_alias_tables_match_expected() {
  local aliases="$1" collisions="$2" expected_aliases="$3" expected_collisions="$4"
  cmp -s "${expected_aliases}" "${aliases}" && cmp -s "${expected_collisions}" "${collisions}"
}

f13_migration_alias_record_load() {
  local repo="$1" record="$2" alias_file="$3" collision_file="$4"
  local alias_raw="${TMP_DIR}/f13-migration-alias-raw" collision_raw="${TMP_DIR}/f13-migration-collision-raw"
  local expected_aliases="${TMP_DIR}/f13-migration-alias-expected" expected_collisions="${TMP_DIR}/f13-migration-collision-expected"
  f13_extract_versioned_alias_section "${repo}" "${record}" \
    '<!-- F13-MIGRATION-ALIASES-V3-BEGIN -->' '<!-- F13-MIGRATION-ALIASES-V3-END -->' \
    "${alias_raw}" || return 1
  f13_extract_versioned_alias_section "${repo}" "${record}" \
    '<!-- F13-MIGRATION-COLLISION-V3-BEGIN -->' '<!-- F13-MIGRATION-COLLISION-V3-END -->' \
    "${collision_raw}" || return 1
  awk -F'|' 'NF != 21 { exit 1 } {
      for (i=1; i<=NF; i++) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", $i) }
      printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", \
        $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
    }' "${alias_raw}" >"${alias_file}" || return 1
  awk -F'|' 'NF != 2 { exit 1 } {
      for (i=1; i<=NF; i++) { gsub(/^[[:space:]]+|[[:space:]]+$/, "", $i) }
      printf "%s\t%s\n", $1,$2
    }' "${collision_raw}" >"${collision_file}" || return 1
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    'src/backend/migrations/voice_db/000010_match_squad_operations.up.sql' \
    '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' \
    '0590ac862c5027ff9266f8403af1011ef2134f63' \
    '5a42388c95377d7d0b718e823b35f317a71bf851' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '5a42388c95377d7d0b718e823b35f317a71bf851' \
    '07d752a2f0f4daa0fe3aecc36bdbe73812dfd833' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '1909f0ee3a3d038d83c468158fe99f6ef0a120bb' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    '100644 blob b4442e910cad45e8d4bd31c0d1ff14426199553d' \
    '020911cf4cb70be47bd575dcd8268685b5c6002d' \
    'f79dfa4d93c2d5bc7efa5fe1f713dcecde761b97' \
    'ce05d869e133cfec521439386320b714a32ffdb3' \
    'e8f5c812ad1ac1880e0850f5863312ff9502183d' \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
    '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' \
    >"${expected_aliases}" || return 1
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    'src/backend/migrations/voice_db/000010_match_squad_operations.down.sql' \
    '100644 blob 9c506399d95a0f460bbdff8972b14fff9bd645ca' \
    '0590ac862c5027ff9266f8403af1011ef2134f63' \
    '5a42388c95377d7d0b718e823b35f317a71bf851' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '5a42388c95377d7d0b718e823b35f317a71bf851' \
    '07d752a2f0f4daa0fe3aecc36bdbe73812dfd833' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    'e33b8a3b09828055f690957b596b843177410ff6' \
    '1909f0ee3a3d038d83c468158fe99f6ef0a120bb' \
    '53529ea60dec670e64d1ae61724a7b8daff582bc' \
    '100644 blob 08e3a0ea2536307c1affc0c6ddfa0376b8083c67' \
    '020911cf4cb70be47bd575dcd8268685b5c6002d' \
    'f79dfa4d93c2d5bc7efa5fe1f713dcecde761b97' \
    'ce05d869e133cfec521439386320b714a32ffdb3' \
    'e8f5c812ad1ac1880e0850f5863312ff9502183d' \
    'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql' \
    '100644 blob 9c506399d95a0f460bbdff8972b14fff9bd645ca' \
    >>"${expected_aliases}" || return 1
  printf '%s\t%s\n' \
    'src/backend/migrations/voice_db/000010_space_media_admission.up.sql' \
    '100644 blob 171033aaf172370f76ede86349b042154bd2ff71' \
    'src/backend/migrations/voice_db/000010_space_media_admission.down.sql' \
    '100644 blob 83953384c9caa2ae2aa8d9485a208bb5ead5e33f' \
    >"${expected_collisions}" || return 1
  f13_migration_alias_tables_match_expected \
    "${alias_file}" "${collision_file}" "${expected_aliases}" "${expected_collisions}"
}

f13_migration_alias_expect_bad_tables() {
  local label="$1" aliases="$2" collisions="$3"
  if f13_migration_alias_tables_match_expected \
    "${aliases}" "${collisions}" \
    "${TMP_DIR}/f13-migration-alias-expected" \
    "${TMP_DIR}/f13-migration-collision-expected"; then
    printf 'F13 oracle bug: migration alias record accepted invalid %s fixture\n' "${label}" >&2
    return 1
  fi
}

f13_migration_alias_record_negative_fixtures() {
  local aliases="$1" collisions="$2" case_alias case_collision column
  case_alias="${TMP_DIR}/f13-alias-negative.tsv"
  case_collision="${TMP_DIR}/f13-collision-negative.tsv"

  head -n 1 "${aliases}" >"${case_alias}" || return 1
  cp "${collisions}" "${case_collision}" || return 1
  f13_migration_alias_expect_bad_tables 'missing/half-pair row' "${case_alias}" "${case_collision}" || return 1

  cat "${aliases}" "${aliases}" >"${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'duplicate alias rows' "${case_alias}" "${collisions}" || return 1
  cp "${aliases}" "${case_alias}" || return 1
  printf '%s\n' 'unexpected|extra|alias|row' >>"${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'extra alias row' "${case_alias}" "${collisions}" || return 1

  for column in 1 {3..21}; do
    awk -F'\t' -v col="${column}" 'BEGIN { OFS="\t" } NR == 1 { $col = $col "-changed" } { print }' \
      "${aliases}" >"${case_alias}" || return 1
    f13_migration_alias_expect_bad_tables "alias field ${column}" "${case_alias}" "${collisions}" || return 1
  done
  awk -F'\t' 'BEGIN { OFS="\t" } NR == 1 { $2 = "100755 blob 19bd5687fea0f556260d27fd1ecf5308de835395" } { print }' \
    "${aliases}" >"${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'alias mode' "${case_alias}" "${collisions}" || return 1
  awk -F'\t' 'BEGIN { OFS="\t" } NR == 1 { $2 = "100644 tree 19bd5687fea0f556260d27fd1ecf5308de835395" } { print }' \
    "${aliases}" >"${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'alias type' "${case_alias}" "${collisions}" || return 1
  awk -F'\t' 'BEGIN { OFS="\t" } NR == 1 { $2 = "100644 blob 0000000000000000000000000000000000000000" } { print }' \
    "${aliases}" >"${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'alias blob' "${case_alias}" "${collisions}" || return 1

  head -n 1 "${collisions}" >"${case_collision}" || return 1
  cp "${aliases}" "${case_alias}" || return 1
  f13_migration_alias_expect_bad_tables 'missing collision pair row' "${case_alias}" "${case_collision}" || return 1
  cp "${collisions}" "${case_collision}" || return 1
  printf '%s\n' 'unexpected|100644 blob 0000000000000000000000000000000000000000' >>"${case_collision}" || return 1
  f13_migration_alias_expect_bad_tables 'extra collision row' "${aliases}" "${case_collision}" || return 1
  awk -F'\t' 'BEGIN { OFS="\t" } NR == 1 { $1 = $1 ".unexpected" } { print }' \
    "${collisions}" >"${case_collision}" || return 1
  f13_migration_alias_expect_bad_tables 'collision path' "${aliases}" "${case_collision}" || return 1
  awk -F'\t' 'BEGIN { OFS="\t" } NR == 1 { $2 = "100644 blob 0000000000000000000000000000000000000000" } { print }' \
    "${collisions}" >"${case_collision}" || return 1
  f13_migration_alias_expect_bad_tables 'collision entry' "${aliases}" "${case_collision}" || return 1
}

f13_migration_alias_reconciles_guarded_path() {
  local repo="$1" base="$2" target="$3" path="$4" owner="$5"
  local content_base="$6" source_head="$7" saved_guarded_entry="$8"
  local saved_source_entry="$9" output="${10}"
  local aliases="${TMP_DIR}/f13-migration-aliases.tsv" collisions="${TMP_DIR}/f13-migration-collisions.tsv"
  local row_count row alias_path old_entry record_row_owner record_row_first_parent record_row_source_base record_row_source_head record_row_content_base record_row_candidates
  local nested_owner nested_first_parent nested_source_base nested_source_head nested_content_base nested_candidates source_base_entry
  local row_commit row_nested_base row_nested_head record_base record_head integration
  local alias_record_row
  local integration_first integration_second integration_base destination destination_entry
  local expected_up='src/backend/migrations/voice_db/000010_match_squad_operations.up.sql'
  local expected_down='src/backend/migrations/voice_db/000010_match_squad_operations.down.sql'
  local destination_up='src/backend/migrations/voice_db/000011_match_squad_operations.up.sql'
  local destination_down='src/backend/migrations/voice_db/000011_match_squad_operations.down.sql'
  local collision_up='src/backend/migrations/voice_db/000010_space_media_admission.up.sql'
  local collision_down='src/backend/migrations/voice_db/000010_space_media_admission.down.sql'
  local base_up base_down snapshot_base_up snapshot_base_down h_entry target_entry integration_entry integration_entry_record
  local base_old snapshot_base_old h_old integration_old integration_first_old integration_second_old integration_base_old target_old aggregate_old resolved_content_base
  local normalized_saved_guarded normalized_saved_source normalized_integration_record
  local alias_row_hash alias_approval_file alias_approval_count
  [[ "${game_checkpoint_aggregate_active}" == true &&
    "${target}" == "${game_checkpoint_aggregate_head}" &&
    -n "${game_checkpoint_snapshot_record}" &&
    -n "${owner}" && -n "${content_base}" && -n "${source_head}" &&
    -n "${owners[${path}]+present}" && -n "${owner_content_bases[${path}]+present}" &&
    -n "${owner_source_heads[${path}]+present}" && -n "${owner_source_entries[${path}]+present}" &&
    -n "${guarded_entries[${path}]+present}" && -n "${guarded_entry_commits[${path}]+present}" ]] || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" && return 1
  game_checkpoint_aggregate_binding_valid "${repo}" || return 1
  [[ "${guarded_entry_commits[${path}]}" == "${owner}" &&
    "${owner_content_bases[${path}]}" == "${content_base}" &&
    "${owner_source_heads[${path}]}" == "${source_head}" &&
    "${owner_source_entries[${path}]}" == "${saved_source_entry}" &&
    "${guarded_entries[${path}]}" == "${saved_guarded_entry}" &&
    "${guarded_entry_commits[${path}]}" == "${owner}" ]] || return 1
  f13_migration_alias_record_load "${repo}" "${game_checkpoint_snapshot_record}" \
    "${aliases}" "${collisions}" || return 1

  [[ "$(awk -F'\t' -v p="${expected_up}" '$1 == p { n++ } END { print n+0 }' "${aliases}")" == 1 &&
    "$(awk -F'\t' -v p="${expected_down}" '$1 == p { n++ } END { print n+0 }' "${aliases}")" == 1 ]] || return 1
  awk -F'\t' -v up="${expected_up}" -v down="${expected_down}" \
    '$1 != up && $1 != down { bad=1 } END { exit bad }' "${aliases}" || return 1
  [[ "$(awk -F'\t' -v p="${collision_up}" '$1 == p { n++ } END { print n+0 }' "${collisions}")" == 1 &&
    "$(awk -F'\t' -v p="${collision_down}" '$1 == p { n++ } END { print n+0 }' "${collisions}")" == 1 ]] || return 1
  awk -F'\t' -v up="${collision_up}" -v down="${collision_down}" \
    '$1 != up && $1 != down { bad=1 } END { exit bad }' "${collisions}" || return 1
  [[ "$(awk -F'\t' -v p="${collision_up}" '$1 == p { print $2; exit }' "${collisions}")" == \
      '100644 blob 171033aaf172370f76ede86349b042154bd2ff71' &&
    "$(awk -F'\t' -v p="${collision_down}" '$1 == p { print $2; exit }' "${collisions}")" == \
      '100644 blob 83953384c9caa2ae2aa8d9485a208bb5ead5e33f' ]] || return 1
  row_count="$(awk -F'\t' -v candidate="${path}" '$1 == candidate { n++; row=$0 } END { print n+0 }' "${aliases}")"
  [[ "${row_count}" == 1 ]] || return 1
  row="$(awk -F'\t' -v candidate="${path}" '$1 == candidate { print; exit }' "${aliases}")"
  alias_record_row="${row}"
  IFS=$'\t' read -r alias_path old_entry record_row_owner record_row_first_parent record_row_source_base record_row_source_head record_row_content_base record_row_candidates \
    nested_owner nested_first_parent nested_source_base nested_source_head nested_content_base nested_candidates source_base_entry \
    integration integration_first integration_second integration_base destination destination_entry <<<"${row}"
  record_base="${nested_source_base}"
  record_head="${nested_source_head}"
  normalized_saved_guarded="$(f13_tree_entry_tuple_for_path "${path}" "${saved_guarded_entry}")" || return 1
  normalized_saved_source="$(f13_tree_entry_tuple_for_path "${path}" "${saved_source_entry}")" || return 1
  [[ "${owners[${path}]}" == "${record_row_owner}" &&
    "${alias_path}" == "${path}" && "${owner}" == "${nested_owner}" &&
    "${source_head}" == "${nested_source_head}" &&
    "${normalized_saved_guarded}" == "${old_entry}" && "${normalized_saved_source}" == "${old_entry}" &&
    "${record_row_owner}" == 0590ac862c5027ff9266f8403af1011ef2134f63 &&
    "${record_row_first_parent}" == 5a42388c95377d7d0b718e823b35f317a71bf851 &&
    "${record_row_source_base}" == 53529ea60dec670e64d1ae61724a7b8daff582bc &&
    "${record_row_source_head}" == e33b8a3b09828055f690957b596b843177410ff6 &&
    "${record_row_content_base}" == 5a42388c95377d7d0b718e823b35f317a71bf851 &&
    "${record_row_candidates}" == 07d752a2f0f4daa0fe3aecc36bdbe73812dfd833 &&
    "${nested_owner}" == e33b8a3b09828055f690957b596b843177410ff6 &&
    "${nested_first_parent}" == 53529ea60dec670e64d1ae61724a7b8daff582bc &&
    "${nested_source_base}" == 53529ea60dec670e64d1ae61724a7b8daff582bc &&
    "${nested_source_head}" == e33b8a3b09828055f690957b596b843177410ff6 &&
    "${nested_content_base}" == 1909f0ee3a3d038d83c468158fe99f6ef0a120bb &&
    "${nested_candidates}" == 53529ea60dec670e64d1ae61724a7b8daff582bc &&
    "${content_base}" == "${record_row_content_base}" &&
    "${record_base}" == 53529ea60dec670e64d1ae61724a7b8daff582bc &&
    "${record_head}" == e33b8a3b09828055f690957b596b843177410ff6 &&
    "${integration}" == 020911cf4cb70be47bd575dcd8268685b5c6002d &&
    "${integration_first}" == f79dfa4d93c2d5bc7efa5fe1f713dcecde761b97 &&
    "${integration_second}" == ce05d869e133cfec521439386320b714a32ffdb3 &&
    "${integration_base}" == e8f5c812ad1ac1880e0850f5863312ff9502183d ]] || return 1
  case "${path}" in
    "${expected_up}") [[ "${destination}" == "${destination_up}" && "${old_entry}" == '100644 blob 19bd5687fea0f556260d27fd1ecf5308de835395' && "${destination_entry}" == "${old_entry}" ]] || return 1 ;;
    "${expected_down}") [[ "${destination}" == "${destination_down}" && "${old_entry}" == '100644 blob 9c506399d95a0f460bbdff8972b14fff9bd645ca' && "${destination_entry}" == "${old_entry}" ]] || return 1 ;;
    *) return 1 ;;
  esac
  git -C "${repo}" merge-base --is-ancestor "${record_base}" "${nested_owner}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${record_row_owner}" "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${record_row_candidates}" "${record_row_source_base}" 2>/dev/null || return 1
  git -C "${repo}" rev-parse --verify "${record_row_content_base}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" rev-parse --verify "${nested_content_base}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" merge-base --is-ancestor "${record_head}" "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${integration}" "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  resolved_content_base="$(git -C "${repo}" rev-parse --verify "${content_base}^{commit}" 2>/dev/null)" || return 1
  [[ "${resolved_content_base}" == "${content_base}" ]] || return 1
  [[ "$(git -C "${repo}" rev-list --parents -n 1 "${integration}" 2>/dev/null)" == \
    "${integration} ${integration_first} ${integration_second}" ]] || return 1
  [[ "$(git -C "${repo}" rev-list --parents -n 1 "${record_row_owner}" 2>/dev/null)" == \
    "${record_row_owner} ${record_row_first_parent} ${nested_owner}" &&
    "$(git -C "${repo}" rev-list --parents -n 1 "${nested_owner}" 2>/dev/null)" == \
    "${nested_owner} ${nested_first_parent}" ]] || return 1
  if git -C "${repo}" merge-base --is-ancestor "${integration}" "${record_row_owner}" 2>/dev/null; then
    return 1
  else
    [[ "$?" == 1 ]] || return 1
  fi
  [[ "$(git -C "${repo}" merge-base --all "${integration_first}" "${integration_second}" 2>/dev/null)" == "${integration_base}" ]] || return 1
  [[ "$(f13_git_tree_entry "${repo}" "${record_row_owner}" "${path}")" == "${old_entry}" &&
    "$(f13_git_tree_entry "${repo}" "${record_row_first_parent}" "${path}")" == "" &&
    "$(f13_git_tree_entry "${repo}" "${nested_owner}" "${path}")" == "${old_entry}" &&
    "$(f13_git_tree_entry "${repo}" "${record_head}" "${path}")" == "${old_entry}" ]] || return 1
  [[ "$(f13_git_tree_entry "${repo}" "${record_base}" "${path}")" == "${source_base_entry}" ]] || return 1
  base_old="$(f13_git_tree_entry "${repo}" "${base}" "${path}")" || return 1
  snapshot_base_old="$(f13_git_tree_entry "${repo}" "${game_checkpoint_snapshot_base}" "${path}")" || return 1
  h_old="$(f13_git_tree_entry "${repo}" "${game_checkpoint_snapshot_head}" "${path}")" || return 1
  integration_old="$(f13_git_tree_entry "${repo}" "${integration}" "${path}")" || return 1
  integration_first_old="$(f13_git_tree_entry "${repo}" "${integration_first}" "${path}")" || return 1
  integration_second_old="$(f13_git_tree_entry "${repo}" "${integration_second}" "${path}")" || return 1
  integration_base_old="$(f13_git_tree_entry "${repo}" "${integration_base}" "${path}")" || return 1
  target_old="$(f13_git_tree_entry "${repo}" "${target}" "${path}")" || return 1
  aggregate_old="$(f13_git_tree_entry "${repo}" "${game_checkpoint_aggregate_target}" "${path}")" || return 1
  [[ "${integration_first_old}" == "${old_entry}" && -z "${integration_second_old}" &&
    -z "${integration_base_old}" && -z "${base_old}" && -z "${snapshot_base_old}" &&
    -z "${h_old}" && -z "${integration_old}" &&
    -z "${target_old}" && -z "${aggregate_old}" ]] || return 1
  snapshot_base_up="$(f13_git_tree_entry "${repo}" "${game_checkpoint_snapshot_base}" "${collision_up}")" || return 1
  snapshot_base_down="$(f13_git_tree_entry "${repo}" "${game_checkpoint_snapshot_base}" "${collision_down}")" || return 1
  base_up="$(f13_git_tree_entry "${repo}" "${base}" "${collision_up}")" || return 1
  base_down="$(f13_git_tree_entry "${repo}" "${base}" "${collision_down}")" || return 1
  [[ "${base_up}" == '100644 blob 171033aaf172370f76ede86349b042154bd2ff71' &&
    "${base_down}" == '100644 blob 83953384c9caa2ae2aa8d9485a208bb5ead5e33f' &&
    "${snapshot_base_up}" == "${base_up}" && "${snapshot_base_down}" == "${base_down}" ]] || return 1
  row_count="$(awk -F'\t' -v candidate="${path}" '$1 == candidate { n++; row=$0 } END { print n+0 }' "${output}")"
  [[ "${row_count}" == 1 ]] || return 1
  row="$(awk -F'\t' -v candidate="${path}" '$1 == candidate { print; exit }' "${output}")"
  IFS=$'\t' read -r _ _ actual_row_commit actual_row_first_parent actual_row_nested_base actual_row_nested_head \
    actual_row_content_base actual_row_source_candidates <<<"${row}"
  [[ "${actual_row_commit}" == "${record_row_owner}" &&
    "${owners[${path}]}" == "${record_row_owner}" &&
    "${guarded_entry_commits[${path}]}" == "${owner}" &&
    "${actual_row_first_parent}" == "${record_row_first_parent}" &&
    "${actual_row_nested_base}" == "${record_row_source_base}" &&
    "${actual_row_nested_head}" == "${record_row_source_head}" &&
    "${actual_row_content_base}" == "${record_row_content_base}" &&
    "${actual_row_source_candidates}" == "${record_row_candidates}" ]] || return 1
  grep -Fxq -- "${destination}" "${game_checkpoint_aggregate_paths}" || return 1
  h_entry="$(f13_git_tree_entry "${repo}" "${game_checkpoint_snapshot_head}" "${destination}")" || return 1
  target_entry="$(f13_git_tree_entry "${repo}" "${game_checkpoint_aggregate_target}" "${destination}")" || return 1
  integration_entry="$(f13_git_tree_entry "${repo}" "${integration}" "${destination}")" || return 1
  integration_entry_record="$(f13_git_tree_entry_record "${repo}" "${integration}" "${destination}")" || return 1
  normalized_integration_record="$(f13_tree_entry_tuple_for_path "${destination}" "${integration_entry_record}")" || return 1
  [[ "${h_entry}" == "100644 blob ${destination_entry##* }" &&
    "${target_entry}" == "${h_entry}" && "${integration_entry}" == "${h_entry}" &&
    "${destination_entry}" == "${old_entry}" &&
    "${normalized_integration_record}" == "${destination_entry}" ]] || return 1
  game_checkpoint_aggregate_path_allowed "${repo}" "${integration}" \
    "${destination}" "${integration_entry_record}" || return 1
  alias_approval_file="${game_checkpoint_aggregate_alias_map}"
  [[ -n "${alias_approval_file}" ]] || return 1
  alias_approval_count="$(awk -F '\t' -v p="${destination}" '$1 == p { n++ } END { print n+0 }' \
    "${alias_approval_file}")"
  [[ "${alias_approval_count}" == 0 ]] || return 1
  alias_row_hash="$(printf '%s\n' "${alias_record_row}" | git -C "${repo}" hash-object --stdin 2>/dev/null)" || return 1
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${destination}" "${path}" "${alias_row_hash}" \
    "${game_checkpoint_snapshot_base}" "${game_checkpoint_snapshot_head}" \
    "${game_checkpoint_snapshot_record}" "${game_checkpoint_aggregate_base}" \
    "${game_checkpoint_aggregate_head}" "${game_checkpoint_aggregate_target}" \
    "${integration}" "${integration_first}" "${integration_second}" "${integration_base}" \
    "${h_entry}" "${target_entry}" "${integration_entry}" "${normalized_integration_record}" \
    >>"${alias_approval_file}" || return 1
  return 0
}

f13_migration_alias_approval_valid() {
  local repo="$1" map="$2" path="$3" ledger row_count row approval_count approval_row
  local alias_path alias_hash snapshot_base snapshot_head snapshot_record aggregate_base aggregate_head aggregate_target
  local integration integration_first integration_second integration_base h_entry target_entry integration_entry normalized_integration_entry
  local aliases collisions record_row actual_hash expected_destination expected_entry actual_entry actual_record normalized_actual_record tuple prior_aggregate_active
  [[ "${game_checkpoint_aggregate_map}" == "${map}" &&
    -n "${game_checkpoint_aggregate_alias_map}" &&
    "${game_checkpoint_aggregate_alias_map}" == "${map}.game-snapshot-alias-approvals" &&
    -n "${game_checkpoint_aggregate_base}" && -n "${game_checkpoint_aggregate_head}" &&
    -n "${game_checkpoint_aggregate_target}" && -n "${game_checkpoint_aggregate_record}" &&
    "${game_checkpoint_aggregate_record}" == "${game_checkpoint_snapshot_record}" ]] || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" || return 1
  row_count="$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${map}")"
  [[ "${row_count}" == 0 ]] || return 1
  approval_count="$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${map}.game-snapshot-approvals")"
  [[ "${approval_count}" == 0 ]] || return 1
  ledger="${game_checkpoint_aggregate_alias_map}"
  [[ -f "${ledger}" ]] || return 1
  [[ "$(awk 'END { print NR+0 }' "${ledger}")" == 2 ]] || return 1
  awk -F '\t' '
    $1 != "src/backend/migrations/voice_db/000011_match_squad_operations.up.sql" &&
    $1 != "src/backend/migrations/voice_db/000011_match_squad_operations.down.sql" { bad=1 }
    END { exit bad }
  ' "${ledger}" || return 1
  approval_count="$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${ledger}")"
  [[ "${approval_count}" == 1 ]] || return 1
  [[ "$(awk -F '\t' -v p="${path}" '$1 == p { if (NF != 17) bad=1 } END { print bad+0 }' "${ledger}")" == 0 ]] || return 1
  approval_row="$(awk -F '\t' -v p="${path}" '$1 == p { row=$0 } END { print row }' "${ledger}")"
  IFS=$'\t' read -r _ alias_path alias_hash snapshot_base snapshot_head snapshot_record \
    aggregate_base aggregate_head aggregate_target integration integration_first integration_second \
    integration_base h_entry target_entry integration_entry normalized_integration_entry <<<"${approval_row}"
  [[ "${snapshot_base}" == "${game_checkpoint_snapshot_base}" &&
    "${snapshot_head}" == "${game_checkpoint_snapshot_head}" &&
    "${snapshot_record}" == "${game_checkpoint_snapshot_record}" &&
    "${aggregate_base}" == "${game_checkpoint_aggregate_base}" &&
    "${aggregate_head}" == "${game_checkpoint_aggregate_head}" &&
    "${aggregate_target}" == "${game_checkpoint_aggregate_target}" ]] || return 1
  f13_migration_alias_record_load "${repo}" "${snapshot_record}" \
    "${TMP_DIR}/f13-alias-ledger-record.tsv" "${TMP_DIR}/f13-alias-ledger-collisions.tsv" || return 1
  record_row="$(awk -F '\t' -v p="${alias_path}" '$1 == p { n++; row=$0 } END { if (n == 1) print row }' \
    "${TMP_DIR}/f13-alias-ledger-record.tsv")"
  [[ -n "${record_row}" ]] || return 1
  actual_hash="$(printf '%s\n' "${record_row}" | git -C "${repo}" hash-object --stdin 2>/dev/null)" || return 1
  [[ "${actual_hash}" == "${alias_hash}" ]] || return 1
  IFS=$'\t' read -r _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ _ expected_destination expected_entry <<<"${record_row}"
  [[ "${expected_destination}" == "${path}" && -n "${expected_entry}" ]] || return 1
  case "${alias_path}" in
    src/backend/migrations/voice_db/000010_match_squad_operations.up.sql)
      [[ "${path}" == src/backend/migrations/voice_db/000011_match_squad_operations.up.sql ]] || return 1 ;;
    src/backend/migrations/voice_db/000010_match_squad_operations.down.sql)
      [[ "${path}" == src/backend/migrations/voice_db/000011_match_squad_operations.down.sql ]] || return 1 ;;
    *) return 1 ;;
  esac
  grep -Fxq -- "${alias_path}" "${game_checkpoint_aggregate_paths}" && return 1
  [[ "$(git -C "${repo}" rev-list --parents -n 1 "${integration}" 2>/dev/null)" == \
    "${integration} ${integration_first} ${integration_second}" ]] || return 1
  [[ "$(git -C "${repo}" merge-base --all "${integration_first}" "${integration_second}" 2>/dev/null)" == \
    "${integration_base}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${integration}" "${snapshot_head}" 2>/dev/null || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" || return 1
  h_entry="$(f13_git_tree_entry "${repo}" "${snapshot_head}" "${path}")" || return 1
  target_entry="$(f13_git_tree_entry "${repo}" "${game_checkpoint_aggregate_target}" "${path}")" || return 1
  actual_entry="$(f13_git_tree_entry "${repo}" "${integration}" "${path}")" || return 1
  actual_record="$(f13_git_tree_entry_record "${repo}" "${integration}" "${path}")" || return 1
  normalized_actual_record="$(f13_tree_entry_tuple_for_path "${path}" "${actual_record}")" || return 1
  [[ "${h_entry}" == "${expected_entry}" && "${target_entry}" == "${h_entry}" &&
    "${actual_entry}" == "${h_entry}" &&
    "${normalized_actual_record}" == "${h_entry}" &&
    "${normalized_integration_entry}" == "${actual_entry}" ]] || return 1
  prior_aggregate_active="${game_checkpoint_aggregate_active}"
  game_checkpoint_aggregate_active=true
  if ! game_checkpoint_aggregate_path_allowed "${repo}" "${integration}" "${path}" "${actual_record}"; then
    game_checkpoint_aggregate_active="${prior_aggregate_active}"
    return 1
  fi
  game_checkpoint_aggregate_active="${prior_aggregate_active}"
  [[ -f "${repo}/${path}" ]] || return 1
  tuple="$(git -C "${repo}" hash-object --path="${path}" "${repo}/${path}" 2>/dev/null)" || return 1
  [[ "100644 blob ${tuple}" == "${h_entry}" ]]
}

f13_release_projection_path_accounted() {
  local repo="$1" output="$2" path="$3" row_count alias_count
  row_count="$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${output}")" || return 1
  alias_count="$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' \
    "${game_checkpoint_aggregate_alias_map}")" || return 1
  if [[ "${row_count}" == 1 && "${alias_count}" == 0 ]]; then return 0; fi
  [[ "${row_count}" == 0 && "${alias_count}" == 1 ]] || return 1
  f13_migration_alias_approval_valid "${repo}" "${output}" "${path}"
}

game_checkpoint_alias_path_allowed_for_provenance() {
  local repo="$1" map="$2" path="$3"
  game_checkpoint_domain_path "${path}" || return 1
  f13_migration_alias_approval_valid "${repo}" "${map}" "${path}"
}

f13_migration_alias_worktree_bytes_fixture() {
  local repo="$1" map="$2" target="$3" path="$4"
  local worktree="${TMP_DIR}/f13-alias-worktree-bytes-${RANDOM}-${RANDOM}"
  local fixture_file="${worktree}/${path}"
  local result=0
  git -C "${repo}" worktree add --quiet --detach --no-checkout "${worktree}" "${target}" || return 1
  if ! mkdir -p "${fixture_file%/*}" ||
    ! git -C "${repo}" show "${target}:${path}" >"${fixture_file}" ||
    ! printf '\nF13 fixture-only worktree mutation\n' >>"${fixture_file}"; then
    result=1
  elif game_checkpoint_alias_path_allowed_for_provenance "${worktree}" "${map}" "${path}"; then
    result=1
  fi
  if ((result == 0)); then
    git -C "${repo}" show "${target}:${path}" >"${fixture_file}" || result=1
    if ((result == 0)) && ! game_checkpoint_alias_path_allowed_for_provenance "${worktree}" "${map}" "${path}"; then
      result=1
    fi
  fi
  git -C "${repo}" worktree remove --force "${worktree}" >/dev/null 2>&1 || result=1
  return "${result}"
}

f13_migration_alias_wrapper_result_fixture() {
  local repo="$1" map="$2" path
  [[ "${game_checkpoint_aggregate_map}" == "${map}" &&
    "${game_checkpoint_aggregate_alias_map}" == "${map}.game-snapshot-alias-approvals" ]] || return 1
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    f13_release_projection_path_accounted "${repo}" "${map}" "${path}" || return 1
  done <"${game_checkpoint_aggregate_paths}"
  for path in \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
    'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql'; do
    f13_release_projection_path_accounted "${repo}" "${map}" "${path}" || return 1
    game_checkpoint_alias_path_allowed_for_provenance "${repo}" "${map}" "${path}" || return 1
  done
}

# Exercise the guarded-path helper with the exact immutable R3 row while the
# checkout is a real master-target pull-request merge. This catches shell
# nounset regressions in the record-to-owner binding before the full collector
# walks the same row.
f13_migration_alias_reconciliation_execution_fixture() {
  local repo="$1" event_base="$2" target="$3" source_head="$4"
  local delta="${TMP_DIR}/f13-alias-execution-delta" paths="${TMP_DIR}/f13-alias-execution-paths"
  local r2_delta="${TMP_DIR}/f13-r2-stale-projection-delta"
  local aliases="${TMP_DIR}/f13-alias-execution-record.tsv" collisions="${TMP_DIR}/f13-alias-execution-collisions.tsv"
  local path row fields old_entry row_owner row_first row_source_base row_source_head row_content_base row_candidates
  local nested_owner nested_first nested_base nested_head nested_content nested_candidates source_base_entry
  local integration integration_first integration_second integration_base destination destination_entry
  local output old_path generic_path ledger backup_ledger
  local fixture_old_record
  local -A owners=() owner_content_bases=() owner_source_heads=() owner_source_entries=()
  local -A guarded_entries=() guarded_entry_commits=()

  [[ "$(git -C "${repo}" rev-list --parents -n 1 "${target}" | awk '{ print NF }')" == 3 &&
    "$(git -C "${repo}" rev-parse "${target}^1")" == "${event_base}" &&
    "$(git -C "${repo}" rev-parse "${target}^2")" == "${source_head}" ]] || return 1
  git -C "${repo}" diff --name-only --no-renames "${event_base}" "${source_head}" >"${r2_delta}" || return 1
  if game_checkpoint_projection_authorized_in_delta "${repo}" "${target}" \
    "${event_base}" "${source_head}" \
    '28f1a0a26f3f23bfb9242592aea9c92fd659ee2d' \
    '7270d22d05475afee9e20fbb1b95377c138ebdf4' \
    'a3fe5aad4fea5ff9ef18a7786d80916c06776e40' "${r2_delta}"; then
    printf '%s\n' 'F13 oracle bug: stale R2 projection authorized the updated R3 source tree' >&2
    return 1
  fi
  git -C "${repo}" diff --name-only --no-renames "${event_base}" "${source_head}" >"${delta}" || return 1
  game_checkpoint_aggregate_authorized "${repo}" "${event_base}" "${source_head}" \
    "${target}" "${delta}" "${paths}" || return 1
  game_checkpoint_aggregate_active=true
  game_checkpoint_aggregate_base="${event_base}"
  game_checkpoint_aggregate_head="${source_head}"
  game_checkpoint_aggregate_target="${target}"
  game_checkpoint_aggregate_paths="${paths}"
  output="${TMP_DIR}/f13-alias-execution-map"
  game_checkpoint_aggregate_map="${output}"
  game_checkpoint_aggregate_record="${game_checkpoint_snapshot_record}"
  game_checkpoint_aggregate_alias_map="${output}.game-snapshot-alias-approvals"
  : >"${output}" || return 1
  : >"${output}.game-snapshot-approvals" || return 1
  : >"${game_checkpoint_aggregate_alias_map}" || return 1
  f13_migration_alias_record_load "${repo}" "${game_checkpoint_snapshot_record}" \
    "${aliases}" "${collisions}" || return 1

  for path in \
    'src/backend/migrations/voice_db/000010_match_squad_operations.up.sql' \
    'src/backend/migrations/voice_db/000010_match_squad_operations.down.sql'; do
    row="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { print; exit }' "${aliases}")"
    [[ -n "${row}" ]] || return 1
    IFS=$'\t' read -r _ old_entry row_owner row_first row_source_base row_source_head row_content_base row_candidates \
      nested_owner nested_first nested_base nested_head nested_content nested_candidates source_base_entry \
      integration integration_first integration_second integration_base destination destination_entry <<<"${row}"
    owners["${path}"]="${row_owner}"
    owner_content_bases["${path}"]="${row_content_base}"
    owner_source_heads["${path}"]="${row_source_head}"
    fixture_old_record="${old_entry}"$'\t'"${path}"
    owner_source_entries["${path}"]="${fixture_old_record}"
    guarded_entries["${path}"]="${fixture_old_record}"
    guarded_entry_commits["${path}"]="${nested_owner}"
    output="${TMP_DIR}/f13-alias-execution-map"
    awk -F '\t' -v candidate="${path}" '$1 == candidate { for (i=1; i<=8; i++) printf "%s%s", $i, (i==8 ? ORS : FS) }' \
      "${aliases}" >"${output}" || return 1
    if ! f13_migration_alias_reconciles_guarded_path "${repo}" "${event_base}" \
      "${source_head}" "${path}" "${nested_owner}" "${row_content_base}" \
      "${nested_head}" "${fixture_old_record}" "${fixture_old_record}" "${output}"; then
      game_checkpoint_aggregate_active=false
      return 1
    fi
    awk -F '\t' -v candidate="${path}" '$1 != candidate' "${output}" >"${TMP_DIR}/f13-alias-execution-map-rewrite" || return 1
    cat "${TMP_DIR}/f13-alias-execution-map-rewrite" >"${output}" || return 1
  done
  game_checkpoint_aggregate_map="${output}"
  ledger="${game_checkpoint_aggregate_alias_map}"
  for path in \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
    'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql'; do
    if ! f13_release_projection_path_accounted "${repo}" "${output}" "${path}" ||
      ! game_checkpoint_alias_path_allowed_for_provenance "${repo}" "${output}" "${path}"; then
      game_checkpoint_aggregate_active=false
      return 1
    fi
    if ! f13_migration_alias_worktree_bytes_fixture \
      "${repo}" "${output}" "${game_checkpoint_aggregate_target}" "${path}"; then
      game_checkpoint_aggregate_active=false
      return 1
    fi
    printf '%s\t%s\n' "${path}" 'true	fixture-owner	fixture-parent	fixture-base	fixture-head	fixture-content	fixture-candidates' >>"${output}" || return 1
    if f13_release_projection_path_accounted "${repo}" "${output}" "${path}"; then
      game_checkpoint_aggregate_active=false
      return 1
    fi
    awk -F '\t' -v candidate="${path}" '$1 != candidate' "${output}" >"${TMP_DIR}/f13-alias-execution-map-rewrite" || return 1
    cat "${TMP_DIR}/f13-alias-execution-map-rewrite" >"${output}" || return 1
  done
  generic_path="$(awk -v up='src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
    -v down='src/backend/migrations/voice_db/000011_match_squad_operations.down.sql' \
    '$0 != up && $0 != down && NF { print; exit }' "${paths}")"
  [[ -n "${generic_path}" ]] || return 1
  : >"${output}" || return 1
  if f13_release_projection_path_accounted "${repo}" "${output}" "${generic_path}"; then
    game_checkpoint_aggregate_active=false
    return 1
  fi
  printf '%s\ttrue\tfixture-owner\tfixture-parent\tfixture-base\tfixture-head\tfixture-content\tfixture-candidates\n' \
    "${generic_path}" >"${output}" || return 1
  f13_release_projection_path_accounted "${repo}" "${output}" "${generic_path}" || return 1
  cat "${output}" "${output}" >"${TMP_DIR}/f13-alias-execution-map-duplicate" || return 1
  if f13_release_projection_path_accounted "${repo}" "${TMP_DIR}/f13-alias-execution-map-duplicate" "${generic_path}"; then
    game_checkpoint_aggregate_active=false
    return 1
  fi
  : >"${output}" || return 1
  backup_ledger="${TMP_DIR}/f13-alias-execution-ledger-backup.tsv"
  cp "${ledger}" "${backup_ledger}" || return 1
  : >"${ledger}" || return 1
  if f13_release_projection_path_accounted "${repo}" "${output}" \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql'; then
    game_checkpoint_aggregate_active=false
    return 1
  fi
  cp "${backup_ledger}" "${ledger}" || return 1
  cat "${ledger}" "${ledger}" >"${TMP_DIR}/f13-alias-execution-ledger-duplicate" || return 1
  game_checkpoint_aggregate_alias_map="${TMP_DIR}/f13-alias-execution-ledger-duplicate"
  if f13_release_projection_path_accounted "${repo}" "${output}" \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql'; then
    game_checkpoint_aggregate_active=false
    return 1
  fi
  game_checkpoint_aggregate_alias_map="${ledger}"
  awk -F '\t' 'BEGIN { OFS="\t" } NR == 1 { $10 = "0000000000000000000000000000000000000000" } { print }' \
    "${ledger}" >"${TMP_DIR}/f13-alias-execution-ledger-wrong-tuple" || return 1
  game_checkpoint_aggregate_alias_map="${TMP_DIR}/f13-alias-execution-ledger-wrong-tuple"
  if f13_release_projection_path_accounted "${repo}" "${output}" \
    'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql'; then
    game_checkpoint_aggregate_active=false
    return 1
  fi
  game_checkpoint_aggregate_alias_map="${ledger}"
  f13_tree_entry_normalizer_fixtures || return 1
  game_checkpoint_aggregate_active=false
  printf '%s\n' 'F13 R3 alias early-helper fixture: PASS'
}

# These exact paths are the reviewed Role private room-grant handler and its
# Voice wiring. The original implementation and the later fail-closed test
# correction are separate source contributions, so authorize each exact
# path/blob at its own reviewed checkpoint.
voice_room_grants_wiring_commit='c758b455103187fd1e40f76eabb35e90f62b7cf3'
voice_room_grants_test_correction_commit='b4b04b9924bcca2b3e912406464bbffeda96a84f'
voice_room_grants_listener_correction_commit='0e59d34239bd9f8bb375c7b1c4d1df5444175fb4'
voice_room_grants_main_blob='952368ad43ee5631fe3b7049687f08e85fb93350'
voice_room_grants_server_blob='9f6e879ff084469104810540fa2a3cdd1ea6dcbf'
voice_room_grants_handler_blob='81853ed329b723941d6692efa32b66df2f56fba2'
voice_room_grants_original_handler_test_blob='6f648af7fade3c3a177b1def605df11eacc7ae2f'
voice_room_grants_original_listener_test_blob='96ac884d2d7774f37b3b370ab8a00b969fceae94'
voice_room_grants_test_blob='4a9f707184175141fd9cf048430bbf21a00cf92b'
voice_room_grants_listener_test_blob='112ed4f5c68e28fae373b7a23b76856ce2062cba'
voice_room_grants_listener_final_blob='29abb80873eb6da42c08065f9df806861f134a52'
voice_room_grants_checkpoint_for_path_blob() {
  local path="$1" actual_blob="$2"
  case "${path}:${actual_blob}" in
    "src/backend/voice/main.go:${voice_room_grants_main_blob}"|\
    "src/backend/role/internal/grpcsvc/server.go:${voice_room_grants_server_blob}"|\
    "src/backend/role/internal/grpcsvc/voice_room_grants.go:${voice_room_grants_handler_blob}"|\
    "src/backend/role/internal/grpcsvc/voice_room_grants_test.go:${voice_room_grants_original_handler_test_blob}"|\
    "src/backend/role/internal/principalruntime/listener_v2_matrix_test.go:${voice_room_grants_original_listener_test_blob}")
      printf '%s\n' "${voice_room_grants_wiring_commit}" ;;
    "src/backend/role/internal/grpcsvc/voice_room_grants_test.go:${voice_room_grants_test_blob}"|\
    "src/backend/role/internal/principalruntime/listener_v2_matrix_test.go:${voice_room_grants_listener_test_blob}")
      printf '%s\n' "${voice_room_grants_test_correction_commit}" ;;
    "src/backend/role/internal/principalruntime/listener_v2_matrix_test.go:${voice_room_grants_listener_final_blob}")
      printf '%s\n' "${voice_room_grants_listener_correction_commit}" ;;
    *) return 1 ;;
  esac
}
voice_room_grants_checkpoint_authorized_in_delta() {
  local repo="$1" target="$2" path="$3" delta="$4" actual_blob="$5" checkpoint
  checkpoint="$(voice_room_grants_checkpoint_for_path_blob "${path}" "${actual_blob}")" || return 1
  git -C "${repo}" merge-base --is-ancestor "${checkpoint}" "${target}" || return 1
  grep -Fxq -- "${path}" "${delta}" || return 1
  return 0
}
voice_room_grants_checkpoint_allowed() {
  local path="$1" delta="$2" actual_blob
  [[ -f "${ROOT}/${path}" ]] || return 1
  actual_blob="$(git -C "${ROOT}" hash-object --path="${path}" "${ROOT}/${path}")"
  voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" HEAD "${path}" "${delta}" "${actual_blob}"
}

voice_room_grants_main_fixture='src/backend/voice/main.go'
voice_room_grants_original_paths=(
  'src/backend/voice/main.go'
  'src/backend/role/internal/grpcsvc/server.go'
  'src/backend/role/internal/grpcsvc/voice_room_grants.go'
  'src/backend/role/internal/grpcsvc/voice_room_grants_test.go'
  'src/backend/role/internal/principalruntime/listener_v2_matrix_test.go'
)
voice_room_grants_corrected_test_paths=(
  'src/backend/role/internal/grpcsvc/voice_room_grants_test.go'
  'src/backend/role/internal/principalruntime/listener_v2_matrix_test.go'
)
git -C "${ROOT}" diff --name-only "${voice_room_grants_wiring_commit}^" "${voice_room_grants_wiring_commit}" >"${TMP_DIR}/voice-room-grants-original-delta"
git -C "${ROOT}" diff --name-only "${voice_room_grants_test_correction_commit}^" "${voice_room_grants_test_correction_commit}" >"${TMP_DIR}/voice-room-grants-correction-delta"
git -C "${ROOT}" diff --name-only "${voice_room_grants_listener_correction_commit}^" "${voice_room_grants_listener_correction_commit}" >"${TMP_DIR}/voice-room-grants-listener-correction-delta"
: >"${TMP_DIR}/voice-room-grants-empty-delta"
for voice_room_grants_checkpoint_path in "${voice_room_grants_original_paths[@]}"; do
  voice_room_grants_checkpoint_blob="$(git -C "${ROOT}" rev-parse "${voice_room_grants_wiring_commit}:${voice_room_grants_checkpoint_path}")"
  voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${voice_room_grants_wiring_commit}" "${voice_room_grants_checkpoint_path}" \
    "${TMP_DIR}/voice-room-grants-original-delta" "${voice_room_grants_checkpoint_blob}" || {
    printf 'F13 oracle bug: exact Voice room-grant checkpoint was rejected: %s\n' "${voice_room_grants_checkpoint_path}" >&2
    exit 2
  }
done
for voice_room_grants_checkpoint_path in "${voice_room_grants_corrected_test_paths[@]}"; do
  voice_room_grants_checkpoint_blob="$(git -C "${ROOT}" rev-parse "${voice_room_grants_test_correction_commit}:${voice_room_grants_checkpoint_path}")"
  voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${voice_room_grants_test_correction_commit}" "${voice_room_grants_checkpoint_path}" \
    "${TMP_DIR}/voice-room-grants-correction-delta" "${voice_room_grants_checkpoint_blob}" || {
    printf 'F13 oracle bug: exact Voice room-grant test correction was rejected: %s\n' "${voice_room_grants_checkpoint_path}" >&2
    exit 2
  }
done
voice_room_grants_checkpoint_path='src/backend/role/internal/principalruntime/listener_v2_matrix_test.go'
voice_room_grants_checkpoint_blob="$(git -C "${ROOT}" rev-parse "${voice_room_grants_listener_correction_commit}:${voice_room_grants_checkpoint_path}")"
voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${voice_room_grants_listener_correction_commit}" \
  "${voice_room_grants_checkpoint_path}" "${TMP_DIR}/voice-room-grants-listener-correction-delta" \
  "${voice_room_grants_checkpoint_blob}" || {
  printf 'F13 oracle bug: exact Voice room-grant listener correction was rejected: %s\n' "${voice_room_grants_checkpoint_path}" >&2
  exit 2
}
if voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${voice_room_grants_listener_correction_commit}^" \
  "${voice_room_grants_checkpoint_path}" "${TMP_DIR}/voice-room-grants-listener-correction-delta" \
  "${voice_room_grants_checkpoint_blob}"; then
  printf 'F13 oracle bug: listener correction checkpoint accepted before its commit: %s\n' "${voice_room_grants_checkpoint_path}" >&2
  exit 2
fi
for voice_room_grants_checkpoint_path in "${voice_room_grants_corrected_test_paths[@]}"; do
  voice_room_grants_checkpoint_blob="$(git -C "${ROOT}" rev-parse "${voice_room_grants_test_correction_commit}:${voice_room_grants_checkpoint_path}")"
  if voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${voice_room_grants_test_correction_commit}^" \
    "${voice_room_grants_checkpoint_path}" "${TMP_DIR}/voice-room-grants-correction-delta" "${voice_room_grants_checkpoint_blob}"; then
    printf 'F13 oracle bug: correction checkpoint accepted before its commit: %s\n' "${voice_room_grants_checkpoint_path}" >&2
    exit 2
  fi
done
voice_room_grants_main_fixture_blob="${voice_room_grants_main_blob}"
for rejected_voice_main in changed_blob near_path absent_delta non_ancestor; do
  rejected_path="${voice_room_grants_main_fixture}"
  rejected_delta="${TMP_DIR}/voice-room-grants-original-delta"
  rejected_blob="${voice_room_grants_main_fixture_blob}"
  rejected_target=HEAD
  case "${rejected_voice_main}" in
    changed_blob) rejected_blob='0000000000000000000000000000000000000000' ;;
    near_path) rejected_path="${voice_room_grants_main_fixture}.extra" ;;
    absent_delta) rejected_delta="${TMP_DIR}/voice-room-grants-empty-delta" ;;
    non_ancestor) rejected_target="${voice_room_grants_wiring_commit}^" ;;
  esac
  if voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${rejected_target}" "${rejected_path}" \
    "${rejected_delta}" "${rejected_blob}"; then
    printf 'F13 oracle bug: unapproved Voice room-grant checkpoint accepted: %s\n' "${rejected_voice_main}" >&2
    exit 2
  fi
done

game_fixture_path='src/backend/voice/internal/federationmedia/runtime.go'
printf '%s\n' "${game_fixture_path}" >"${TMP_DIR}/game-fixture-delta"
game_fixture_blob="$(git -C "${ROOT}" rev-parse "${accepted_game_checkpoint}:${game_fixture_path}")"
game_checkpoint_path_authorized_in_delta "${game_fixture_path}" "${TMP_DIR}/game-fixture-delta" "${game_fixture_blob}" || {
  printf '%s\n' 'F13 oracle bug: exact approved Game checkpoint blob was rejected' >&2
  exit 2
}
for forbidden_game_path in \
  'src/backend/voice/internal/federationmedia/unapproved.go' \
  'src/backend/voice/internal/federationmedia/runtime.go.extra' \
  'deploy/staging/backend-services.yaml'; do
  printf '%s\n' "${forbidden_game_path}" >"${TMP_DIR}/game-forbidden-delta"
  if game_checkpoint_path_authorized_in_delta "${forbidden_game_path}" "${TMP_DIR}/game-forbidden-delta" "${game_fixture_blob}"; then
    printf 'F13 oracle bug: unapproved Game path was accepted: %s\n' "${forbidden_game_path}" >&2
    exit 2
  fi
done
if game_checkpoint_path_authorized_in_delta "${game_fixture_path}" "${TMP_DIR}/game-fixture-delta" '0000000000000000000000000000000000000000' ||
  game_checkpoint_path_authorized_in_delta "${game_fixture_path}" "${TMP_DIR}/game-forbidden-delta" "${game_fixture_blob}"; then
  printf '%s\n' 'F13 oracle bug: changed Game blob or absent delta was accepted' >&2
  exit 2
fi

# Exercise the later whole-snapshot verifier against a producer-shaped nested
# merge. The immutable record binds source base B0, snapshot H, and source A;
# event base B may equal B0 or add separately governed paths before result T.
f13_game_checkpoint_projection_fixtures() {
local game_projection_path game_projection_h_entry game_projection_a_entry game_projection_t_entry
f13_checkpoint_merge_tree_fixtures || return 1
f13_diag_stderr="${TMP_DIR}/f13-guarded-diagnostic-stderr"
f13_diag_stdout="$(f13_emit_guarded_failure_diagnostic guarded_target_mismatch \
  'protos/voice/fixture:(glob)/fixture.proto' '0123456789012345678901234567890123456789' \
  'abcdefabcdefabcdefabcdefabcdefabcdefabcd' 'fedcbafedcbafedcbafedcbafedcbafedcbafedc' \
  '1111111111111111111111111111111111111111' \
  '100644 blob 0123456789012345678901234567890123456789 path' \
  '100755 blob abcdefabcdefabcdefabcdefabcdefabcdefabcd path' \
  stored_entry_must_equal_final_target 2>"${f13_diag_stderr}")"
[[ -z "${f13_diag_stdout}" ]] || {
  printf '%s\n' 'F13 oracle bug: guarded failure diagnostics leaked onto collector stdout' >&2
  return 1
}
grep -Fq 'stage=f13-collector category=guarded_target_mismatch' "${f13_diag_stderr}" && \
  grep -Fq 'path=protos/voice/fixture' "${f13_diag_stderr}" && \
  grep -Fq 'source_head=1111111111111111111111111111111111111111' "${f13_diag_stderr}" && \
  grep -Fq 'stored=100644:blob:0123456789012345678901234567890123456789' "${f13_diag_stderr}" && \
  grep -Fq 'target=100755:blob:abcdefabcdefabcdefabcdefabcdefabcdefabcd' "${f13_diag_stderr}" && \
  grep -Fq 'predicate=stored_entry_must_equal_final_target' "${f13_diag_stderr}" || {
  printf '%s\n' 'F13 oracle bug: bounded guarded failure diagnostic omitted safe Git metadata' >&2
  return 1
}
game_projection_repo="${TMP_DIR}/game-coherent-projection-fixture"
mkdir -p "${game_projection_repo}"
git -C "${game_projection_repo}" init -q
git -C "${game_projection_repo}" config user.name 'Voice CI fixture'
git -C "${game_projection_repo}" config user.email 'voice-ci@example.invalid'
git -C "${game_projection_repo}" config commit.gpgsign false
mkdir -p "${game_projection_repo}/protos/voice/fixture/v1" \
  "${game_projection_repo}/src/backend/voice/internal/fixture" \
  "${game_projection_repo}/src/backend/role/internal/fixture"
printf '%s\n' 'baseline proto' >"${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
printf '%s\n' 'baseline voice' >"${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
printf '%s\n' 'removed by accepted source' >"${game_projection_repo}/src/backend/role/internal/fixture/obsolete.go"
printf '%s\n' 'fixture base' >"${game_projection_repo}/README.md"
git -C "${game_projection_repo}" add .
git -C "${game_projection_repo}" commit -qm 'F13 coherent Game base'
game_projection_base="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" checkout -qb game-source
printf '%s\n' 'accepted proto' >"${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
printf '%s\n' 'accepted voice' >"${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
rm "${game_projection_repo}/src/backend/role/internal/fixture/obsolete.go"
git -C "${game_projection_repo}" add -A
git -C "${game_projection_repo}" commit -qm 'F13 coherent Game source contribution'
game_projection_head="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" checkout -qb game-current-base "${game_projection_base}"
mkdir -p "${game_projection_repo}/docs" "${game_projection_repo}/src/backend/space/internal/fixture"
printf '%s\n' '718 non-Game base advance' >"${game_projection_repo}/docs/base-advance.md"
printf '%s\n' 'base-owned Space path' >"${game_projection_repo}/src/backend/space/internal/fixture/base-owned.go"
git -C "${game_projection_repo}" add docs/base-advance.md src/backend/space/internal/fixture/base-owned.go
git -C "${game_projection_repo}" commit -qm 'F13 unrelated 718 base advance'
game_projection_actual_base="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" checkout -q game-source
mkdir -p "${game_projection_repo}/scripts/ci"
printf '%s\n' 'approved snapshot tuple metadata' >"${game_projection_repo}/scripts/ci/game-checkpoint-record.fixture"
git -C "${game_projection_repo}" add scripts/ci/game-checkpoint-record.fixture
git -C "${game_projection_repo}" commit -qm 'F13 record immutable Game snapshot H'
game_projection_record="$(git -C "${game_projection_repo}" rev-parse HEAD)"
printf '%s\n' 'ordinary metadata-only follow-up A' >"${game_projection_repo}/scripts/ci/follow-up.fixture"
git -C "${game_projection_repo}" add scripts/ci/follow-up.fixture
git -C "${game_projection_repo}" commit -qm 'F13 ordinary metadata-only follow-up A'
game_projection_actual_head="$(git -C "${game_projection_repo}" rev-parse HEAD)"
game_projection_equal_source_head="${game_projection_actual_head}"
git -C "${game_projection_repo}" merge --no-ff -qm 'F13 source lane syncs exact advanced master base' game-current-base
game_projection_actual_head="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" checkout -qb game-integrate "${game_projection_actual_base}"
git -C "${game_projection_repo}" merge --no-ff -qm 'F13 nested merge of accepted Game source' game-source
game_projection_target="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" diff --name-only "${game_projection_actual_base}" "${game_projection_actual_head}" >"${TMP_DIR}/game-projection-delta"
game_checkpoint_snapshot_base="${game_projection_base}"
game_checkpoint_snapshot_head="${game_projection_head}"
game_checkpoint_snapshot_record="${game_projection_record}"

# The fixture snapshot is an exact projection at H; the post-record source
# head A and integration target T carry no further Game-path changes. Check
# the complete entries, including the intentional deletion, before testing
# the aggregate ancestry predicates below.
for game_projection_path in \
  'protos/voice/fixture/v1/fixture.proto' \
  'src/backend/voice/internal/fixture/runtime.go' \
  'src/backend/role/internal/fixture/obsolete.go'; do
  game_projection_h_entry="$(f13_git_tree_entry_record \
    "${game_projection_repo}" "${game_projection_head}" "${game_projection_path}")" || return 1
  game_projection_a_entry="$(f13_git_tree_entry_record \
    "${game_projection_repo}" "${game_projection_actual_head}" "${game_projection_path}")" || return 1
  game_projection_t_entry="$(f13_git_tree_entry_record \
    "${game_projection_repo}" "${game_projection_target}" "${game_projection_path}")" || return 1
  [[ "${game_projection_h_entry}" == "${game_projection_a_entry}" &&
    "${game_projection_h_entry}" == "${game_projection_t_entry}" ]] || {
    printf '%s\n' 'F13 oracle bug: valid B0/H/R/A/T fixture changed a full Game path entry' >&2
    return 1
  }
done

# Exercise every boundary of the narrow pre-H duplicate-owner supersession
# rule independently of the full producer fixture below.
game_checkpoint_aggregate_active=true
game_checkpoint_aggregate_base="${game_projection_base}"
game_checkpoint_aggregate_head="${game_projection_actual_head}"
game_checkpoint_aggregate_target="${game_projection_target}"
game_checkpoint_aggregate_record="${game_projection_record}"
game_checkpoint_aggregate_paths="${TMP_DIR}/game-projection-supersession-paths"
printf '%s\n' 'protos/voice/fixture/v1/fixture.proto' >"${game_checkpoint_aggregate_paths}"
game_checkpoint_aggregate_binding_valid "${game_projection_repo}" || {
  printf '%s\n' 'F13 oracle bug: valid B0/H/R/A/T aggregate fixture was rejected' >&2
  exit 2
}
if ! game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: exact pre-H integration owner was rejected' >&2
  exit 2
fi
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_actual_head}" "${game_projection_head}" clean ||
  game_checkpoint_duplicate_owner_supersession_allowed \
    "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
    "${game_projection_base}" "${game_projection_base}" \
    "${game_projection_head}" "${game_projection_head}" "${game_projection_base}" clean ||
  game_checkpoint_duplicate_owner_supersession_allowed \
    "${game_projection_repo}" 'scripts/ci/game-checkpoint-record.fixture' \
    "${game_projection_base}" "${game_projection_base}" \
    "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: out-of-H owner, wrong result entry, or non-P path was accepted' >&2
  exit 2
fi
supersession_status=0
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_base}" clean; then
  supersession_status=0
else
  supersession_status=$?
fi
[[ "${supersession_status}" == 2 ]] || {
  printf '%s\n' 'F13 oracle bug: verified non-H integration did not retain a tainted owner row' >&2
  exit 2
}
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" unverified; then
  printf '%s\n' 'F13 oracle bug: unverified merge outcome was accepted for duplicate supersession' >&2
  exit 2
fi
game_checkpoint_aggregate_active=false
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: inactive aggregate accepted duplicate owner' >&2
  exit 2
fi
game_checkpoint_aggregate_active=true
game_checkpoint_aggregate_record='0000000000000000000000000000000000000000'
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: mismatched checkpoint record accepted duplicate owner' >&2
  exit 2
fi
game_checkpoint_aggregate_record="${game_projection_record}"
game_checkpoint_aggregate_head="${game_projection_base}"
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: source aggregate not descended from H was accepted' >&2
  exit 2
fi
game_checkpoint_aggregate_head="${game_projection_actual_head}"
game_checkpoint_aggregate_target="${game_projection_target}"
game_checkpoint_aggregate_binding_valid "${game_projection_repo}" || {
  printf '%s\n' 'F13 oracle bug: valid B0/H/R/A/T aggregate fixture was rejected before target-negative case' >&2
  exit 2
}
game_checkpoint_aggregate_target="${game_projection_base}"
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: aggregate target not descended from A was accepted' >&2
  exit 2
fi
game_checkpoint_aggregate_target="${game_projection_target}"
game_projection_unrelated_base="$(printf '%s\n' 'F13 unrelated checkpoint base' | \
  git -C "${game_projection_repo}" commit-tree "${game_projection_target}^{tree}")"
game_checkpoint_aggregate_base="${game_projection_unrelated_base}"
if game_checkpoint_duplicate_owner_supersession_allowed \
  "${game_projection_repo}" 'protos/voice/fixture/v1/fixture.proto' \
  "${game_projection_base}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${game_projection_head}" clean; then
  printf '%s\n' 'F13 oracle bug: unrelated aggregate base was accepted' >&2
  exit 2
fi
game_checkpoint_aggregate_base="${game_projection_base}"
game_checkpoint_aggregate_active=false

game_projection_event_sha="$(git -C "${game_projection_repo}" rev-parse HEAD)"
if ! f13_master_pr_contribution_enabled "${game_projection_repo}" pull_request master   codex/matchfound-deadline-recovery refs/pull/667/merge "${game_projection_event_sha}"   "${game_projection_actual_base}" "${game_projection_target}"; then
  printf '%s\n' 'F13 oracle bug: exact checkpoint PR graph on an arbitrary feature branch was not routed' >&2
  exit 2
fi
if f13_master_pr_contribution_enabled "${game_projection_repo}" pull_request master   codex/matchfound-deadline-recovery refs/pull/667/merge "${game_projection_event_sha}"   "${game_projection_actual_base}" "${game_projection_actual_head}" ||
  f13_master_pr_contribution_enabled "${game_projection_repo}" pull_request develop   codex/matchfound-deadline-recovery refs/pull/667/merge "${game_projection_event_sha}"   "${game_projection_actual_base}" "${game_projection_target}" ||
  f13_master_pr_contribution_enabled "${game_projection_repo}" pull_request master   codex/matchfound-deadline-recovery refs/pull/667/head "${game_projection_event_sha}"   "${game_projection_actual_base}" "${game_projection_target}"; then
  printf '%s\n' 'F13 oracle bug: wrong event, base, ref, or merge identity selected checkpoint routing' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-pr-drift-source "${game_projection_actual_head}"
printf '%s\n' 'unapproved post-checkpoint drift' >"${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
git -C "${game_projection_repo}" add protos/voice/fixture/v1/fixture.proto
git -C "${game_projection_repo}" commit -qm 'F13 reject post-checkpoint Game drift'
game_projection_drift_head="$(git -C "${game_projection_repo}" rev-parse HEAD)"
game_projection_drift_target="$(git -C "${game_projection_repo}" commit-tree \
  "${game_projection_drift_head}^{tree}" -p "${game_projection_actual_base}" -p "${game_projection_drift_head}" \
  <<< 'F13 reject drifted checkpoint PR')"
if f13_master_pr_contribution_enabled "${game_projection_repo}" pull_request master \
  codex/matchfound-deadline-recovery refs/pull/667/merge "${game_projection_drift_target}" \
  "${game_projection_actual_base}" "${game_projection_drift_target}"; then
  printf '%s\n' 'F13 oracle bug: post-checkpoint guarded drift retained checkpoint routing' >&2
  exit 2
fi
f13_collect_master_release_pr_contributions "${game_projection_repo}" "${game_projection_actual_base}"   "${game_projection_target}" "${TMP_DIR}/game-projection-pr-map"   "${TMP_DIR}/game-projection-pr-category" || {
  printf '%s\n' 'F13 oracle bug: checkpoint-qualified feature PR failed the existing release-PR collector' >&2
  exit 2
}
awk -F '\t' '$1 == "protos/voice/fixture/v1/fixture.proto" { found=1 } END { exit !found }' \
  "${TMP_DIR}/game-projection-pr-map" || {
  printf '%s\n' 'F13 oracle bug: release-PR collector omitted the checkpointed path row' >&2
}
game_checkpoint_path_allowed_for_provenance "${game_projection_repo}"   "${TMP_DIR}/game-projection-pr-map" 'protos/voice/fixture/v1/fixture.proto' || {
  printf '%s\n' 'F13 oracle bug: actual release-PR row failed pinned checkpoint path authorization' >&2
  exit 2
}
f13_collect_master_push_contributions "${game_projection_repo}" push refs/heads/master \
  "${game_projection_actual_base}" "${game_projection_actual_base}" "${game_projection_target}" \
  "${TMP_DIR}/game-projection-map" || {
  printf '%s\n' 'F13 oracle bug: real contribution collector rejected the approved snapshot fixture' >&2
  exit 2
}
game_projection_row="$(awk -F '\t' '$1 == "protos/voice/fixture/v1/fixture.proto" { row = $0 } END { print row }' \
  "${TMP_DIR}/game-projection-map")"
IFS=$'\t' read -r _ game_projection_trigger game_projection_row_target _ \
  game_projection_row_base game_projection_row_head _ _ <<<"${game_projection_row}"
[[ "${game_projection_trigger}" == false && \
  -n "${game_projection_row_target}" && \
  "${game_projection_row_base}" == "${game_projection_base}" && \
  "${game_projection_row_head}" == "${game_projection_head}" ]] || {
  printf '%s\n' 'F13 oracle bug: master-push partition did not preserve the nested approved owner' >&2
  exit 2
}
game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" "${TMP_DIR}/game-projection-map" \
  'protos/voice/fixture/v1/fixture.proto' || {
  printf '%s\n' 'F13 oracle bug: exact coherent Game source projection was rejected' >&2
  exit 2
}
game_projection_approval_map="${TMP_DIR}/game-projection-map.game-snapshot-approvals"
cp "${game_projection_approval_map}" "${TMP_DIR}/game-projection-approval-saved"
: >"${game_projection_approval_map}"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: missing aggregate sidecar was accepted' >&2
  exit 2
fi
cp "${TMP_DIR}/game-projection-approval-saved" "${game_projection_approval_map}"
cat "${TMP_DIR}/game-projection-approval-saved" >>"${game_projection_approval_map}"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: ambiguous aggregate sidecar row was accepted' >&2
  exit 2
fi
cp "${TMP_DIR}/game-projection-approval-saved" "${game_projection_approval_map}"
awk -F '\t' -v OFS='\t' '{$5="0000000000000000000000000000000000000000"; print}' \
  "${TMP_DIR}/game-projection-approval-saved" >"${game_projection_approval_map}"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: stale aggregate target binding was accepted' >&2
  exit 2
fi
awk -F '\t' -v OFS='\t' '{$8="0000000000000000000000000000000000000000"; print}' \
  "${TMP_DIR}/game-projection-approval-saved" >"${game_projection_approval_map}"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: mixed snapshot record binding was accepted' >&2
  exit 2
fi
cp "${TMP_DIR}/game-projection-approval-saved" "${game_projection_approval_map}"
cp "${TMP_DIR}/game-projection-map" "${TMP_DIR}/game-projection-map-saved"
awk -F '\t' '$1 == "protos/voice/fixture/v1/fixture.proto" { print }' \
  "${TMP_DIR}/game-projection-map-saved" >>"${TMP_DIR}/game-projection-map"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: ambiguous partition owner row was accepted' >&2
  exit 2
fi
cp "${TMP_DIR}/game-projection-map-saved" "${TMP_DIR}/game-projection-map"
awk -F '\t' -v OFS='\t' '$1 == "protos/voice/fixture/v1/fixture.proto" {$6="0000000000000000000000000000000000000000"} {print}' \
  "${TMP_DIR}/game-projection-map-saved" >"${TMP_DIR}/game-projection-map"
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'protos/voice/fixture/v1/fixture.proto'; then
  printf '%s\n' 'F13 oracle bug: sidecar accepted a changed nested owner row' >&2
  exit 2
fi
cp "${TMP_DIR}/game-projection-map-saved" "${TMP_DIR}/game-projection-map"
game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'src/backend/role/internal/fixture/obsolete.go' || {
  printf '%s\n' 'F13 oracle bug: exact checkpoint deletion with its unique owner row was rejected' >&2
  exit 2
}
game_projection_metadata_row="$(awk -F '\t' '$1 == "scripts/ci/follow-up.fixture" { row = $0 } END { print row }' \
  "${TMP_DIR}/game-projection-map")"
IFS=$'\t' read -r _ game_projection_metadata_trigger game_projection_metadata_owner game_projection_metadata_target \
  game_projection_metadata_base game_projection_metadata_head _ _ <<<"${game_projection_metadata_row}"
[[ "${game_projection_metadata_trigger}" == false && \
  "${game_projection_metadata_owner}" == "${game_projection_equal_source_head}" && \
  -n "${game_projection_metadata_target}" && \
  "${game_projection_metadata_base}" == "${game_projection_record}" && \
  "${game_projection_metadata_head}" == "${game_projection_equal_source_head}" ]] || {
  printf '%s\n' 'F13 oracle bug: metadata-only path lost its ordinary contribution provenance' >&2
  exit 2
}
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'scripts/ci/follow-up.fixture'; then
  printf '%s\n' 'F13 oracle bug: Game snapshot authorization bypassed ordinary metadata policy' >&2
  exit 2
fi
if game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-map" 'src/backend/space/internal/fixture/base-owned.go'; then
  printf '%s\n' 'F13 oracle bug: Game snapshot claimed an out-of-P Space path from the current base' >&2
  exit 2
fi
[[ "$(git -C "${game_projection_repo}" rev-parse "${game_projection_target}:src/backend/space/internal/fixture/base-owned.go")" == \
  "$(git -C "${game_projection_repo}" rev-parse "${game_projection_actual_base}:src/backend/space/internal/fixture/base-owned.go")" ]] || {
  printf '%s\n' 'F13 oracle bug: accepted snapshot projection failed to preserve current-base Space path' >&2
  exit 2
}
# A post-H change cannot regain checkpoint authority by restoring the exact
# pinned bytes later in the same source lane. Keep this producer-shaped through
# the release-PR wrapper so the full PR tree and aggregate owner path are used.
git -C "${game_projection_repo}" checkout -qb game-post-h-restore "${game_projection_actual_head}"
printf '%s\n' 'unauthorized post-H edit' >"${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
git -C "${game_projection_repo}" add protos/voice/fixture/v1/fixture.proto
git -C "${game_projection_repo}" commit -qm 'F13 post-H checkpoint drift'
printf '%s\n' 'accepted proto' >"${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
git -C "${game_projection_repo}" add protos/voice/fixture/v1/fixture.proto
git -C "${game_projection_repo}" commit -qm 'F13 restore checkpoint bytes after H'
game_projection_post_h_restore="$(git -C "${game_projection_repo}" rev-parse HEAD)"
game_projection_post_h_restore_target="$(git -C "${game_projection_repo}" commit-tree \
  "${game_projection_post_h_restore}^{tree}" -p "${game_projection_actual_base}" \
  -p "${game_projection_post_h_restore}" <<< 'F13 reject post-H restore PR')"
if f13_collect_master_release_pr_contributions "${game_projection_repo}" \
  "${game_projection_actual_base}" "${game_projection_post_h_restore_target}" \
  "${TMP_DIR}/game-projection-post-h-restore-map" \
  "${TMP_DIR}/game-projection-post-h-restore-category"; then
  printf '%s\n' 'F13 oracle bug: post-H drift restored to H retained checkpoint authority' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/game-projection-post-h-restore-category")" == checkpoint_post_h_owner ]] || {
  printf '%s\n' 'F13 oracle bug: post-H restore failed for an unexpected reason' >&2
  exit 2
}

# The release wrapper must still compare every changed tree entry at the
# synthetic merge target, including checkpoint paths, before collector routing.
game_projection_tampered_index="${TMP_DIR}/game-projection-tampered.index"
GIT_INDEX_FILE="${game_projection_tampered_index}" git -C "${game_projection_repo}" \
  read-tree "${game_projection_target}^{tree}"
printf '%s\n' 'tampered synthetic target' | git -C "${game_projection_repo}" hash-object -w --stdin \
  >"${TMP_DIR}/game-projection-tampered-blob"
game_projection_tampered_blob="$(<"${TMP_DIR}/game-projection-tampered-blob")"
GIT_INDEX_FILE="${game_projection_tampered_index}" git -C "${game_projection_repo}" \
  update-index --cacheinfo "100644,${game_projection_tampered_blob},protos/voice/fixture/v1/fixture.proto"
game_projection_tampered_tree="$(GIT_INDEX_FILE="${game_projection_tampered_index}" \
  git -C "${game_projection_repo}" write-tree)"
game_projection_tampered_target="$(printf '%s\n' 'F13 tamper PR merge target' | \
  git -C "${game_projection_repo}" commit-tree "${game_projection_tampered_tree}" \
  -p "${game_projection_actual_base}" -p "${game_projection_actual_head}")"
if f13_collect_master_release_pr_contributions "${game_projection_repo}" \
  "${game_projection_actual_base}" "${game_projection_tampered_target}" \
  "${TMP_DIR}/game-projection-tampered-map" "${TMP_DIR}/game-projection-tampered-category"; then
  printf '%s\n' 'F13 oracle bug: tampered synthetic merge target was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/game-projection-tampered-category")" == release_pr_merge_tree_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: tampered synthetic target failed outside full-tree verification' >&2
  exit 2
}

# A conflict-carry owner is temporary: the release-PR producer must reject a
# result whose checkpoint path never returns to the approved H entry.
game_projection_missing_restore_index="${TMP_DIR}/game-projection-missing-restore.index"
GIT_INDEX_FILE="${game_projection_missing_restore_index}" git -C "${game_projection_repo}" read-tree "${game_projection_target}^{tree}"
printf '%s\n' 'unrestored intermediate conflict entry' | git -C "${game_projection_repo}" hash-object -w --stdin >"${TMP_DIR}/game-projection-missing-restore-blob"
game_projection_missing_restore_blob="$(<"${TMP_DIR}/game-projection-missing-restore-blob")"
GIT_INDEX_FILE="${game_projection_missing_restore_index}" git -C "${game_projection_repo}" update-index --cacheinfo "100644,${game_projection_missing_restore_blob},protos/voice/fixture/v1/fixture.proto"
game_projection_missing_restore_tree="$(GIT_INDEX_FILE="${game_projection_missing_restore_index}" git -C "${game_projection_repo}" write-tree)"
game_projection_missing_restore_target="$(printf '%s\n' 'F13 reject missing checkpoint restore' | git -C "${game_projection_repo}" commit-tree "${game_projection_missing_restore_tree}" -p "${game_projection_actual_base}" -p "${game_projection_actual_head}")"
if f13_collect_master_release_pr_contributions "${game_projection_repo}" "${game_projection_actual_base}" \
  "${game_projection_missing_restore_target}" "${TMP_DIR}/game-projection-missing-restore-map" \
  "${TMP_DIR}/game-projection-missing-restore-category"; then
  printf '%s\n' 'F13 oracle bug: missing exact-H restore survived the release-PR producer' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/game-projection-missing-restore-category")" == release_pr_merge_tree_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: missing restore did not fail at strict release target verification' >&2
  exit 2
}

# A conflict-carry owner is only temporary. If the approved P entry is never
# restored before the release result, the real PR producer rejects the target.
game_projection_missing_restore_index="${TMP_DIR}/game-projection-missing-restore.index"
GIT_INDEX_FILE="${game_projection_missing_restore_index}" git -C "${game_projection_repo}" \
  read-tree "${game_projection_target}^{tree}"
printf '%s\n' 'unrestored intermediate conflict entry' | \
  git -C "${game_projection_repo}" hash-object -w --stdin >"${TMP_DIR}/game-projection-missing-restore-blob"
game_projection_missing_restore_blob="$(<"${TMP_DIR}/game-projection-missing-restore-blob")"
GIT_INDEX_FILE="${game_projection_missing_restore_index}" git -C "${game_projection_repo}" \
  update-index --cacheinfo "100644,${game_projection_missing_restore_blob},protos/voice/fixture/v1/fixture.proto"
game_projection_missing_restore_tree="$(GIT_INDEX_FILE="${game_projection_missing_restore_index}" \
  git -C "${game_projection_repo}" write-tree)"
game_projection_missing_restore_target="$(printf '%s\n' 'F13 reject missing checkpoint restore' | \
  git -C "${game_projection_repo}" commit-tree "${game_projection_missing_restore_tree}" \
  -p "${game_projection_actual_base}" -p "${game_projection_actual_head}")"
if f13_collect_master_release_pr_contributions "${game_projection_repo}" \
  "${game_projection_actual_base}" "${game_projection_missing_restore_target}" \
  "${TMP_DIR}/game-projection-missing-restore-map" \
  "${TMP_DIR}/game-projection-missing-restore-category"; then
  printf '%s\n' 'F13 oracle bug: missing exact-H restore survived the release-PR producer' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/game-projection-missing-restore-category")" == release_pr_merge_tree_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: missing restore did not fail at strict release target verification' >&2
  exit 2
}

git -C "${game_projection_repo}" checkout -qb game-equal-source "${game_projection_base}"
git -C "${game_projection_repo}" merge --no-ff -qm 'F13 equal-base source contribution' "${game_projection_equal_source_head}"
git -C "${game_projection_repo}" checkout -qb game-integrate-equal-base "${game_projection_base}"
git -C "${game_projection_repo}" merge --no-ff -qm 'F13 snapshot contribution on unchanged base' game-equal-source
game_projection_equal_target="$(git -C "${game_projection_repo}" rev-parse HEAD)"
f13_collect_master_push_contributions "${game_projection_repo}" push refs/heads/master \
  "${game_projection_base}" "${game_projection_base}" "${game_projection_equal_target}" \
  "${TMP_DIR}/game-projection-equal-map" || {
  printf '%s\n' 'F13 oracle bug: coherent snapshot on B == B0 was rejected by the producer' >&2
  exit 2
}
game_checkpoint_path_allowed_for_provenance "${game_projection_repo}" \
  "${TMP_DIR}/game-projection-equal-map" 'protos/voice/fixture/v1/fixture.proto' || {
  printf '%s\n' 'F13 oracle bug: coherent snapshot on B == B0 was rejected' >&2
  exit 2
}
game_checkpoint_snapshot_base=''
game_checkpoint_snapshot_head=''
game_checkpoint_snapshot_record=''
if game_checkpoint_contribution_allowed "${game_projection_repo}" "${game_projection_target}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: missing trusted snapshot record was accepted' >&2
  exit 2
fi
printf '%s\n' 'src/backend/voice/internal/fixture/unapproved.go' >>"${TMP_DIR}/game-projection-delta"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_target}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: extra Game-domain delta path was accepted' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-checkpoint-drift "${game_projection_target}"
printf '%s\n' 'drifted after checkpoint' >"${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
git -C "${game_projection_repo}" add src/backend/voice/internal/fixture/runtime.go
git -C "${game_projection_repo}" commit -qm 'F13 reject Game snapshot drift'
game_projection_drift="$(git -C "${game_projection_repo}" rev-parse HEAD)"
git -C "${game_projection_repo}" diff --name-only "${game_projection_base}" "${game_projection_actual_head}" >"${TMP_DIR}/game-projection-delta"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_drift}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: changed Game tree entry was accepted after checkpoint' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-checkpoint-extra "${game_projection_target}"
printf '%s\n' 'extra file' >"${game_projection_repo}/src/backend/space/internal/fixture/extra.go"
git -C "${game_projection_repo}" add src/backend/space/internal/fixture/extra.go
git -C "${game_projection_repo}" commit -qm 'F13 reject extra Game snapshot path'
game_projection_extra="$(git -C "${game_projection_repo}" rev-parse HEAD)"
game_projection_extra_target="$(git -C "${game_projection_repo}" commit-tree \
  "${game_projection_extra}^{tree}" -p "${game_projection_actual_base}" -p "${game_projection_actual_head}" \
  <<< 'F13 reject target-only Game path')"
if f13_collect_master_push_contributions "${game_projection_repo}" push refs/heads/master \
  "${game_projection_actual_base}" "${game_projection_actual_base}" "${game_projection_extra_target}" \
  "${TMP_DIR}/game-projection-extra-map" "${TMP_DIR}/game-projection-extra-category"; then
  printf '%s\n' 'F13 oracle bug: master-push wrapper accepted a target-only Game path' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/game-projection-extra-category")" == release_pr_delta_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: target-only Game path failed outside synthetic merge parity' >&2
  exit 2
}
git -C "${game_projection_repo}" checkout -qb game-checkpoint-restored-deletion "${game_projection_target}"
mkdir -p "${game_projection_repo}/src/backend/role/internal/fixture"
printf '%s\n' 'unexpected restoration' >"${game_projection_repo}/src/backend/role/internal/fixture/obsolete.go"
git -C "${game_projection_repo}" add src/backend/role/internal/fixture/obsolete.go
git -C "${game_projection_repo}" commit -qm 'F13 reject restoration of checkpoint deletion'
game_projection_restored="$(git -C "${game_projection_repo}" rev-parse HEAD)"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_restored}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: restored checkpoint deletion was accepted' >&2
  exit 2
fi
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_target}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-forbidden-delta"; then
  printf '%s\n' 'F13 oracle bug: mixed or wrong delta checkpoint projection was accepted' >&2
  exit 2
fi
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_target}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_head}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: snapshot head without a later record commit was accepted' >&2
  exit 2
fi
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_target}" \
  "${game_projection_actual_base}" "${game_projection_actual_head}" "${game_projection_head}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta" ||
  game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_target}" \
    "${game_projection_actual_base}" "${game_projection_record}" "${game_projection_base}" \
    "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: wrong base or source identity was accepted' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-checkpoint-mixed "${game_projection_target}"
printf '%s\n' 'mixed checkpoint source' >"${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
git -C "${game_projection_repo}" add src/backend/voice/internal/fixture/runtime.go
git -C "${game_projection_repo}" commit -qm 'F13 reject mixed checkpoint projection'
game_projection_mixed="$(git -C "${game_projection_repo}" rev-parse HEAD)"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_mixed}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: mixed snapshot entries from checkpoints were accepted' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-checkpoint-mode "${game_projection_target}"
chmod +x "${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
git -C "${game_projection_repo}" add src/backend/voice/internal/fixture/runtime.go
git -C "${game_projection_repo}" commit -qm 'F13 reject mode drift'
game_projection_mode="$(git -C "${game_projection_repo}" rev-parse HEAD)"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_mode}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: changed Game entry mode was accepted' >&2
  exit 2
fi
git -C "${game_projection_repo}" checkout -qb game-checkpoint-type "${game_projection_target}"
rm "${game_projection_repo}/src/backend/voice/internal/fixture/runtime.go"
rm "${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
ln -s fixture.proto.link-target "${game_projection_repo}/protos/voice/fixture/v1/fixture.proto"
git -C "${game_projection_repo}" add -A
git -C "${game_projection_repo}" commit -qm 'F13 reject entry type drift'
game_projection_type="$(git -C "${game_projection_repo}" rev-parse HEAD)"
if game_checkpoint_projection_authorized_in_delta "${game_projection_repo}" "${game_projection_type}" \
  "${game_projection_base}" "${game_projection_actual_head}" "${game_projection_base}" \
  "${game_projection_head}" "${game_projection_record}" "${TMP_DIR}/game-projection-delta"; then
  printf '%s\n' 'F13 oracle bug: changed Game entry type/path was accepted' >&2
  exit 2
fi
}

accepted_r23_base='edc52d46406d97283f81dfbdfc916660f50dcc69'
r23_contract_base_allowed() {
  local candidate="$1"
  git -C "${ROOT}" rev-parse --verify "${candidate}^{commit}" >/dev/null 2>&1 &&
    git -C "${ROOT}" merge-base --is-ancestor "${accepted_r23_base}" "${candidate}" &&
    ! git -C "${ROOT}" cat-file -e "${candidate}:protos/voice/r23_contract_manifest.json" 2>/dev/null
}

r23_allowlist_enabled=false
if r23_contract_base_allowed "${base_sha}"; then
  r23_allowlist_enabled=true
fi

{
  git -C "${ROOT}" diff --name-only "${accepted_r23_base}" --
  git -C "${ROOT}" ls-files --others --exclude-standard
} | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u >"${TMP_DIR}/r23-fixed-delta"

cat >"${TMP_DIR}/r23-allowlist.py" <<'PY'
import json
import sys

manifest_path, targets_path, accepted_base = sys.argv[1:]
with open(manifest_path, encoding="utf-8") as stream:
    manifest = json.load(stream)
with open(targets_path, encoding="utf-8") as stream:
    targets = json.load(stream)

if manifest.get("metadata", {}).get("base_sha") != accepted_base:
    raise SystemExit("R23 manifest base_sha does not match the accepted base")
if not isinstance(manifest.get("files"), list) or not isinstance(targets.get("targets"), list):
    raise SystemExit("R23 contract manifests have invalid collection fields")

proto_paths = []
for item in manifest["files"]:
    if not isinstance(item, dict) or not isinstance(item.get("path"), str):
        raise SystemExit("R23 manifest contains an invalid proto path")
    proto_paths.append("protos/" + item["path"])

committed_targets = []
for item in targets["targets"]:
    if (
        not isinstance(item, dict)
        or not isinstance(item.get("path"), str)
        or type(item.get("committed")) is not bool
    ):
        raise SystemExit("R23 generated-target manifest contains an invalid target")
    if item["committed"]:
        committed_targets.append(item["path"])

generated_module_roots = {
    "src/backend/file/pb/voice/file",
    "src/backend/role/pb/voice/role",
    "src/backend/voice/pb/voice/bot",
    "src/backend/voice/pb/voice/calls",
    "src/backend/voice/pb/voice/notification",
    "src/backend/voice/pb/voice/space",
    "src/backend/voice/pb/voice/subscription",
}
generated_module_mods = {
    root + "/go.mod"
    for root in generated_module_roots
    if any(path.startswith(root + "/") for path in committed_targets)
}
if len(generated_module_mods) != len(generated_module_roots):
    raise SystemExit("R23 generated-module go.mod derivation is incomplete")

allowed = {
    ".github/workflows/ci.yml",
    "Makefile",
    "protos/voice/r23_contract_manifest.json",
    "protos/voice/r23_contract_test/RED_EVIDENCE.md",
    "protos/voice/r23_contract_test/generated_targets.json",
    "protos/voice/r23_contract_test/go.mod",
    "protos/voice/r23_contract_test/r23_contract_test.go",
    "protos/voice/r23_contract_test/testdata/deterministic_contract_vectors.json",
    "scripts/ci/voice-db-runtime-provisioning-contract_test.sh",
    "src/backend/bot/Dockerfile",
    "src/backend/bot/go.mod",
    "src/backend/chat/Dockerfile",
    "src/backend/gateway/rest_transcoding_integration_test.go",
    "src/backend/matchmaking/Dockerfile",
    "src/backend/matchmaking/go.mod",
    "src/backend/role/internal/grpcsvc/ordinary_scope_test.go",
    "src/backend/search/Dockerfile",
    "src/backend/search/go.mod",
    "src/backend/social/Dockerfile",
    "src/backend/social/go.mod",
    "src/backend/space/Dockerfile",
    "src/backend/space/go.mod",
    "src/backend/subscription/Dockerfile",
    "src/backend/subscription/go.mod",
    "src/backend/user/Dockerfile",
    "src/backend/voice/go.mod",
}
allowed.update(proto_paths)
allowed.update(committed_targets)
allowed.update(generated_module_mods)
print("\n".join(sorted(allowed)))
PY

python3 "${TMP_DIR}/r23-allowlist.py" \
  "${ROOT}/protos/voice/r23_contract_manifest.json" \
  "${ROOT}/protos/voice/r23_contract_test/generated_targets.json" \
  "${accepted_r23_base}" \
  >"${TMP_DIR}/r23-allowed-paths"

r23_contract_path_authorized_in_delta() {
  local path="$1"
  local delta="$2"
  grep -Fxq -- "${path}" "${TMP_DIR}/r23-allowed-paths" &&
    grep -Fxq -- "${path}" "${delta}"
}

r23_contract_path_allowed_for_window() {
  local enabled="$1"
  local path="$2"
  local delta="$3"
  [[ "${enabled}" == 'true' ]] &&
    r23_contract_path_authorized_in_delta "${path}" "${delta}"
}

r23_contract_path_allowed() {
  r23_contract_path_allowed_for_window \
    "${r23_allowlist_enabled}" \
    "$1" \
    "${TMP_DIR}/r23-fixed-delta"
}

# This R22.2 contract normally rejects Role and Voice runtime changes. These
# exact publisher files are the bounded exception for central JetStream
# bootstrap: they remove publisher-side stream administration and cannot grow
# into a directory-level runtime allowance.
identity_publisher_path_allowed() {
  case "$1" in
    src/backend/role/internal/roleevents/jetstream.go|\
    src/backend/role/internal/roleevents/jetstream_test.go|\
    src/backend/voice/internal/voiceevents/jetstream.go)
      return 0
      ;;
  esac
  return 1
}

accepted_voice_role_status_checkpoint='5a8976ae968c586ef97a90fc1a6ee6459e93850b'
voice_role_status_path_authorized_in_delta() {
  local path="$1" delta="$2" actual_blob="$3" repo="${4:-${ROOT}}" target="${5:-HEAD}"
  local expected_blob
  [[ "${path}" == 'src/backend/voice/internal/grpcsvc/role_guard.go' ]] || return 1
  grep -Fxq -- "${path}" "${delta}" || return 1
  git -C "${repo}" rev-parse --verify "${accepted_voice_role_status_checkpoint}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" merge-base --is-ancestor "${accepted_voice_role_status_checkpoint}" "${target}" 2>/dev/null || return 1
  expected_blob="$(git -C "${repo}" ls-tree "${accepted_voice_role_status_checkpoint}" -- "${path}" 2>/dev/null | awk '{print $3}')"
  [[ -n "${expected_blob}" && "${actual_blob}" == "${expected_blob}" ]]
}

accepted_account_voice_admission_checkpoint='95bc7468fd98bafe6e3b059d7e4a3fad723e9f24'
account_voice_admission_path='src/backend/voice/internal/grpcsvc/account_voice_admission.go'
account_voice_admission_path_authorized_in_delta() {
  local path="$1" delta="$2" actual_blob="$3" repo="${4:-${ROOT}}" target="${5:-HEAD}"
  local expected_blob
  [[ "${path}" == "${account_voice_admission_path}" ]] || return 1
  grep -Fxq -- "${path}" "${delta}" || return 1
  git -C "${repo}" rev-parse --verify "${accepted_account_voice_admission_checkpoint}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" merge-base --is-ancestor "${accepted_account_voice_admission_checkpoint}" "${target}" 2>/dev/null || return 1
  expected_blob="$(git -C "${repo}" ls-tree "${accepted_account_voice_admission_checkpoint}" -- "${path}" 2>/dev/null | awk '{print $3}')"
  [[ -n "${expected_blob}" && "${actual_blob}" == "${expected_blob}" ]]
}

account_voice_admission_blob="$(git -C "${ROOT}" rev-parse "${accepted_account_voice_admission_checkpoint}:${account_voice_admission_path}")"
printf '%s\n' "${account_voice_admission_path}" >"${TMP_DIR}/account-voice-admission-delta"
account_voice_admission_path_authorized_in_delta "${account_voice_admission_path}" \
  "${TMP_DIR}/account-voice-admission-delta" "${account_voice_admission_blob}" || {
  printf '%s\n' 'F13 oracle bug: exact reviewed account admission checkpoint was rejected' >&2
  exit 2
}
if account_voice_admission_path_authorized_in_delta "${account_voice_admission_path}" \
  "${TMP_DIR}/account-voice-admission-delta" '0000000000000000000000000000000000000000' || \
  account_voice_admission_path_authorized_in_delta "${account_voice_admission_path}.near-match" \
    "${TMP_DIR}/account-voice-admission-delta" "${account_voice_admission_blob}"; then
  printf '%s\n' 'F13 oracle bug: changed blob or near-match account admission path was accepted' >&2
  exit 2
fi
account_voice_admission_nonancestor_repo="${TMP_DIR}/account-admission-nonancestor"
mkdir -p "${account_voice_admission_nonancestor_repo}"
git -C "${account_voice_admission_nonancestor_repo}" init -q
git -C "${account_voice_admission_nonancestor_repo}" config user.name 'Voice CI fixture'
git -C "${account_voice_admission_nonancestor_repo}" config user.email 'voice-ci@example.invalid'
git -C "${account_voice_admission_nonancestor_repo}" fetch -q --no-tags "${ROOT}" \
  "${accepted_account_voice_admission_checkpoint}"
mkdir -p "${account_voice_admission_nonancestor_repo}/src/backend/voice/internal/grpcsvc"
git -C "${ROOT}" show "${accepted_account_voice_admission_checkpoint}:${account_voice_admission_path}" > \
  "${account_voice_admission_nonancestor_repo}/${account_voice_admission_path}"
git -C "${account_voice_admission_nonancestor_repo}" add "${account_voice_admission_path}"
git -C "${account_voice_admission_nonancestor_repo}" commit -qm 'F13 unrelated account admission checkpoint fixture'
if account_voice_admission_path_authorized_in_delta "${account_voice_admission_path}" \
  "${TMP_DIR}/account-voice-admission-delta" "${account_voice_admission_blob}" \
  "${account_voice_admission_nonancestor_repo}" HEAD; then
  printf '%s\n' 'F13 oracle bug: non-ancestor account admission checkpoint was accepted' >&2
  exit 2
fi

voice_role_status_path='src/backend/voice/internal/grpcsvc/role_guard.go'
voice_role_status_blob="$(git -C "${ROOT}" rev-parse "${accepted_voice_role_status_checkpoint}:${voice_role_status_path}")"
printf '%s\n' "${voice_role_status_path}" >"${TMP_DIR}/voice-role-status-delta"
voice_role_status_path_authorized_in_delta "${voice_role_status_path}" \
  "${TMP_DIR}/voice-role-status-delta" "${voice_role_status_blob}" || {
  printf '%s\n' 'F13 oracle bug: exact reviewed Voice Role status checkpoint was rejected' >&2
  exit 2
}
if voice_role_status_path_authorized_in_delta "${voice_role_status_path}" \
  "${TMP_DIR}/voice-role-status-delta" '0000000000000000000000000000000000000000' || \
  voice_role_status_path_authorized_in_delta "${voice_role_status_path}.near-match" \
    "${TMP_DIR}/voice-role-status-delta" "${voice_role_status_blob}"; then
  printf '%s\n' 'F13 oracle bug: changed blob or near-match Voice Role path was accepted' >&2
  exit 2
fi
voice_role_status_nonancestor_repo="${TMP_DIR}/voice-role-status-nonancestor"
git init -q "${voice_role_status_nonancestor_repo}"
git -C "${voice_role_status_nonancestor_repo}" config user.name 'Voice CI fixture'
git -C "${voice_role_status_nonancestor_repo}" config user.email 'voice-ci@example.invalid'
git -C "${voice_role_status_nonancestor_repo}" fetch -q --no-tags "${ROOT}" \
  "${accepted_voice_role_status_checkpoint}"
mkdir -p "${voice_role_status_nonancestor_repo}/src/backend/voice/internal/grpcsvc"
git -C "${voice_role_status_nonancestor_repo}" show \
  "${accepted_voice_role_status_checkpoint}:${voice_role_status_path}" > \
  "${voice_role_status_nonancestor_repo}/${voice_role_status_path}"
git -C "${voice_role_status_nonancestor_repo}" add "${voice_role_status_path}"
git -C "${voice_role_status_nonancestor_repo}" commit -qm 'F13 unrelated Voice Role checkpoint fixture'
if voice_role_status_path_authorized_in_delta "${voice_role_status_path}" \
  "${TMP_DIR}/voice-role-status-delta" "${voice_role_status_blob}" \
  "${voice_role_status_nonancestor_repo}" HEAD; then
  printf '%s\n' 'F13 oracle bug: non-ancestor Voice Role checkpoint was accepted' >&2
  exit 2
fi

f13_voice_root_source_file() {
  [[ "${1%/*}" == 'src/backend/voice' && "${1##*.}" == go ]]
}
f13_voice_root_source_file 'src/backend/voice/main.go' || {
  printf '%s\n' 'F13 oracle bug: root Voice production source was not classified' >&2
  exit 2
}
for f13_nested_voice_source in \
  'src/backend/voice/internal/spaceprincipalruntime/config.go' \
  'src/backend/voice/pb/voice/bot/v1/bot.pb.go'; do
  if f13_voice_root_source_file "${f13_nested_voice_source}"; then
    printf 'F13 oracle bug: nested Voice source was classified as root production: %s\n' \
      "${f13_nested_voice_source}" >&2
    exit 2
  fi
done

# BE-255 adds the bounded Space voice-room media vertical while R22.2 room
# lifecycle remains source-disabled. Keep this admission file-by-file: it must
# not grant neighboring Voice runtime, lifecycle, MatchFound, proto, or data
# store work.
space_media_path_allowed() {
  case "$1" in
    src/backend/role/go.mod|\
    src/backend/role/internal/outboxdelivery/voice_policy.go|\
    src/backend/role/internal/outboxdelivery/voice_policy_test.go|\
    src/backend/role/internal/roleevents/jetstream.go|\
    src/backend/role/internal/roleevents/jetstream_test.go|\
    src/backend/role/internal/store/voice_policy_invalidation_outbox.go|\
    src/backend/role/main.go|\
    src/backend/space/internal/outboxdelivery/voice_invalidation.go|\
    src/backend/space/internal/outboxdelivery/voice_invalidation_test.go|\
    src/backend/space/internal/spaceevents/jetstream.go|\
    src/backend/space/internal/spaceevents/jetstream_test.go|\
    src/backend/space/internal/spaceevents/jetstream_transport_test.go|\
    src/backend/space/internal/store/voice_access_invalidation_outbox.go|\
    src/backend/space/main.go|\
    src/backend/voice/internal/grpcsvc/space_media_join.go|\
    src/backend/voice/internal/grpcsvc/space_media_join_test.go|\
    src/backend/voice/internal/grpcsvc/voice_grpc.go|\
    src/backend/voice/internal/grpcsvc/voice_room.go|\
    src/backend/voice/internal/grpcsvc/voice_room_access.go|\
    src/backend/voice/internal/grpcsvc/voice_room_access_canonical_test.go|\
    src/backend/voice/internal/grpcsvc/voice_room_integration_test.go|\
    src/backend/voice/internal/livekit/space_token.go|\
    src/backend/voice/internal/livekit/space_token_test.go|\
    src/backend/voice/internal/s2s/role_voice_room_grants.go|\
    src/backend/voice/internal/s2s/voice_room_access.go|\
    src/backend/voice/internal/spacemedia/consumer.go|\
    src/backend/voice/internal/spacemedia/consumer_test.go|\
    src/backend/voice/internal/spacemedia/coordinator.go|\
    src/backend/voice/internal/spacemedia/coordinator_test.go|\
    src/backend/voice/internal/store/call_store.go|\
    src/backend/voice/internal/store/redis_store.go|\
    src/backend/voice/internal/store/redis_store_test.go|\
    src/backend/voice/main.go|\
    src/backend/user/pb/voice/auth/v1/auth.pb.go|\
  src/backend/user/pb/voice/auth/v1/auth_grpc.pb.go|\
  src/backend/voice/pb/voice/auth/v1/auth.pb.go|\
    src/backend/voice/pb/voice/auth/v1/auth_grpc.pb.go|\
    protos/voice/auth/v1/auth.proto|\
    src/backend/voice/internal/principalgrpc/interceptor.go|\
    src/backend/voice/internal/sessionfloor/client.go|\
    src/backend/voice/internal/voiceuserprincipalruntime/config.go|\
    src/backend/voice/internal/voiceuserprincipalruntime/interceptor.go|\
    src/backend/voice/internal/voiceuserprincipalruntime/runtime.go|\
    src/backend/role/Dockerfile|\
    src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go|\
    src/backend/voice/internal/gameprovision/account_voice_fence.go|\
    src/backend/voice/internal/spacemedia/outbox.go|\
    src/backend/voice/internal/spacemedia/postgres_admission.go)
      return 0
      ;;
  esac
  return 1
}

for be255_path in \
  src/backend/role/Dockerfile \
  src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go \
  src/backend/voice/internal/gameprovision/account_voice_fence.go \
  src/backend/voice/internal/spacemedia/outbox.go \
  src/backend/voice/internal/spacemedia/postgres_admission.go \
  src/backend/role/go.mod \
  src/backend/role/internal/outboxdelivery/voice_policy.go \
  src/backend/role/internal/outboxdelivery/voice_policy_test.go \
  src/backend/role/internal/roleevents/jetstream.go \
  src/backend/role/internal/roleevents/jetstream_test.go \
  src/backend/role/internal/store/voice_policy_invalidation_outbox.go \
  src/backend/role/main.go \
  src/backend/space/internal/outboxdelivery/voice_invalidation.go \
  src/backend/space/internal/outboxdelivery/voice_invalidation_test.go \
  src/backend/space/internal/spaceevents/jetstream.go \
  src/backend/space/internal/spaceevents/jetstream_test.go \
  src/backend/space/internal/spaceevents/jetstream_transport_test.go \
  src/backend/space/internal/store/voice_access_invalidation_outbox.go \
  src/backend/space/main.go \
  src/backend/voice/internal/grpcsvc/space_media_join.go \
  src/backend/voice/internal/grpcsvc/space_media_join_test.go \
  src/backend/voice/internal/grpcsvc/voice_grpc.go \
  src/backend/voice/internal/grpcsvc/voice_room.go \
  src/backend/voice/internal/grpcsvc/voice_room_access.go \
  src/backend/voice/internal/grpcsvc/voice_room_access_canonical_test.go \
  src/backend/voice/internal/grpcsvc/voice_room_integration_test.go \
  src/backend/voice/internal/livekit/space_token.go \
  src/backend/voice/internal/livekit/space_token_test.go \
  src/backend/voice/internal/s2s/role_voice_room_grants.go \
  src/backend/voice/internal/s2s/voice_room_access.go \
  src/backend/voice/internal/spacemedia/consumer.go \
  src/backend/voice/internal/spacemedia/consumer_test.go \
  src/backend/voice/internal/spacemedia/coordinator.go \
  src/backend/voice/internal/spacemedia/coordinator_test.go \
  src/backend/voice/internal/store/call_store.go \
  src/backend/voice/internal/store/redis_store.go \
  src/backend/voice/internal/store/redis_store_test.go \
  src/backend/voice/main.go \
  src/backend/user/pb/voice/auth/v1/auth.pb.go \
  src/backend/user/pb/voice/auth/v1/auth_grpc.pb.go \
  src/backend/voice/pb/voice/auth/v1/auth.pb.go \
  src/backend/voice/pb/voice/auth/v1/auth_grpc.pb.go \
  protos/voice/auth/v1/auth.proto \
  src/backend/voice/internal/principalgrpc/interceptor.go \
  src/backend/voice/internal/sessionfloor/client.go \
  src/backend/voice/internal/voiceuserprincipalruntime/config.go \
  src/backend/voice/internal/voiceuserprincipalruntime/interceptor.go \
  src/backend/voice/internal/voiceuserprincipalruntime/runtime.go; do
  space_media_path_allowed "${be255_path}" || {
    printf 'F13 oracle bug: approved BE-255 path was rejected: %s\n' "${be255_path}" >&2
    exit 2
  }
done
# These five files were independently reviewed as part of the accepted BE-255
# checkpoint. Keep their content exception narrower than the static path list.
accepted_be255_checkpoint='80540ee4f2043c7fa2c89b7bb78b38e7c010a68c'
be255_checkpoint_is_commit_ancestor() {
  local repo="$1" checkpoint="$2" target="$3"
  git -C "${repo}" rev-parse --verify "${checkpoint}^{commit}" >/dev/null 2>&1 &&
    git -C "${repo}" merge-base --is-ancestor "${checkpoint}" "${target}"
}
if ! be255_checkpoint_is_commit_ancestor "${ROOT}" "${accepted_be255_checkpoint}" HEAD; then
  printf '%s\n' 'F13: accepted BE-255 checkpoint must be an ancestor commit' >&2
  exit 1
fi
git -C "${ROOT}" ls-tree -r "${accepted_be255_checkpoint}" >"${TMP_DIR}/be255-checkpoint-tree"
be255_checkpoint_path_authorized_in_delta() {
  local path="$1" delta="$2" actual_blob="$3" expected_blob checkpoint_blob
  case "${path}" in
    src/backend/role/Dockerfile)
      expected_blob='a8810b9df3e6ee09b250ce1e0b9b052b2853ad32' ;;
    src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go)
      expected_blob='507374dffe11b1f1efac5cda6ad9f302fd34ba6c' ;;
    src/backend/voice/internal/gameprovision/account_voice_fence.go)
      expected_blob='901fb1952369f1093fa23baed913c7648cdf11c8' ;;
    src/backend/voice/internal/spacemedia/outbox.go)
      expected_blob='e53dd810e2d769565dbfab77f2cecbe78b9cef2f' ;;
    src/backend/voice/internal/spacemedia/postgres_admission.go)
      expected_blob='832cd554232c6d77299ae26c9afa7ed36771d220' ;;
    *) return 1 ;;
  esac
  grep -Fxq -- "${path}" "${delta}" || return 1
  checkpoint_blob="$(awk -F '\t' -v path="${path}" '$2 == path { split($1, fields, " "); print fields[3] }' "${TMP_DIR}/be255-checkpoint-tree")"
  [[ "${checkpoint_blob}" == "${expected_blob}" && "${actual_blob}" == "${expected_blob}" ]]
}

be255_checkpoint_fixture_repo="${TMP_DIR}/be255-checkpoint-fixture.git"
git init -q --bare "${be255_checkpoint_fixture_repo}"
be255_checkpoint_fixture_tree="$(printf '' | git -C "${be255_checkpoint_fixture_repo}" mktree)"
be255_ancestor_fixture="$(printf '%s\n' 'ancestor fixture' | git -C "${be255_checkpoint_fixture_repo}" -c user.name=fixture -c user.email=fixture@example.invalid commit-tree "${be255_checkpoint_fixture_tree}")"
be255_nonancestor_fixture="$(printf '%s\n' 'unrelated fixture' | git -C "${be255_checkpoint_fixture_repo}" -c user.name=fixture -c user.email=fixture@example.invalid commit-tree "${be255_checkpoint_fixture_tree}")"
be255_child_fixture="$(printf '%s\n' 'child fixture' | git -C "${be255_checkpoint_fixture_repo}" -c user.name=fixture -c user.email=fixture@example.invalid commit-tree "${be255_checkpoint_fixture_tree}" -p "${be255_ancestor_fixture}")"
git -C "${be255_checkpoint_fixture_repo}" update-ref refs/heads/main "${be255_child_fixture}"
git -C "${be255_checkpoint_fixture_repo}" update-ref refs/heads/unrelated "${be255_nonancestor_fixture}"
be255_checkpoint_is_commit_ancestor "${be255_checkpoint_fixture_repo}" "${be255_ancestor_fixture}" refs/heads/main || {
  printf '%s\n' 'F13 oracle bug: accepted checkpoint ancestor fixture was rejected' >&2
  exit 2
}
if be255_checkpoint_is_commit_ancestor "${be255_checkpoint_fixture_repo}" "${be255_nonancestor_fixture}" refs/heads/main; then
  printf '%s\n' 'F13 oracle bug: non-ancestor checkpoint fixture was accepted' >&2
  exit 2
fi
if be255_checkpoint_is_commit_ancestor "${be255_checkpoint_fixture_repo}" '0000000000000000000000000000000000000000' refs/heads/main; then
  printf '%s\n' 'F13 oracle bug: missing checkpoint fixture was accepted' >&2
  exit 2
fi

be255_fixture_paths=(
  'src/backend/role/Dockerfile'
  'src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go'
  'src/backend/voice/internal/gameprovision/account_voice_fence.go'
  'src/backend/voice/internal/spacemedia/outbox.go'
  'src/backend/voice/internal/spacemedia/postgres_admission.go'
)
be255_fixture_blobs=(
  'a8810b9df3e6ee09b250ce1e0b9b052b2853ad32'
  '507374dffe11b1f1efac5cda6ad9f302fd34ba6c'
  '901fb1952369f1093fa23baed913c7648cdf11c8'
  'e53dd810e2d769565dbfab77f2cecbe78b9cef2f'
  '832cd554232c6d77299ae26c9afa7ed36771d220'
)
: >"${TMP_DIR}/be255-empty-delta"
for index in "${!be255_fixture_paths[@]}"; do
  path="${be255_fixture_paths[${index}]}"
  blob="${be255_fixture_blobs[${index}]}"
  printf '%s\n' "${path}" >"${TMP_DIR}/be255-fixture-delta"
  be255_checkpoint_path_authorized_in_delta "${path}" "${TMP_DIR}/be255-fixture-delta" "${blob}" || {
    printf 'F13 oracle bug: exact approved BE-255 path/blob was rejected: %s\n' "${path}" >&2
    exit 2
  }
  if be255_checkpoint_path_authorized_in_delta "${path}" "${TMP_DIR}/be255-empty-delta" "${blob}"; then
    printf 'F13 oracle bug: BE-255 path absent from delta was accepted: %s\n' "${path}" >&2
    exit 2
  fi
  if be255_checkpoint_path_authorized_in_delta "${path}" "${TMP_DIR}/be255-fixture-delta" '0000000000000000000000000000000000000000'; then
    printf 'F13 oracle bug: changed BE-255 blob was accepted: %s\n' "${path}" >&2
    exit 2
  fi
done
for near_match_path in \
  src/backend/role/Dockerfile.near-match \
  src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go.near-match \
  src/backend/voice/internal/gameprovision/account_voice_fence.go.near-match \
  src/backend/voice/internal/spacemedia/outbox.go.near-match \
  src/backend/voice/internal/spacemedia/postgres_admission.go.near-match; do
  printf '%s\n' "${near_match_path}" >"${TMP_DIR}/be255-near-match-delta"
  if be255_checkpoint_path_authorized_in_delta "${near_match_path}" "${TMP_DIR}/be255-near-match-delta" 'a8810b9df3e6ee09b250ce1e0b9b052b2853ad32'; then
    printf 'F13 oracle bug: near-match BE-255 path was accepted: %s\n' "${near_match_path}" >&2
    exit 2
  fi
done
for unrelated_space_media_path in \
  src/backend/role/Dockerfile.near-match \
  src/backend/space/internal/spaceevents/jetstream_bootstrap_test.go.near-match \
  src/backend/voice/internal/gameprovision/account_voice_fence.go.near-match \
  src/backend/voice/internal/spacemedia/outbox.go.near-match \
  src/backend/voice/internal/spacemedia/postgres_admission.go.near-match \
  src/backend/role/internal/outboxdelivery/unrelated.go \
  src/backend/space/internal/store/voice_access_invalidation_outbox.go.near-match \
  src/backend/voice/internal/spacemedia/unrelated.go \
  src/backend/voice/internal/roomlifecycle/lifecycle_worker.go \
  src/backend/voice/internal/livekit/match_found.go \
  src/backend/voice/pb/voice/auth/v1/auth.pb.go.near-match \
  src/backend/user/pb/voice/auth/v1/unrelated.pb.go \
  src/backend/voice/internal/sessionfloor/unrelated.go \
  src/backend/voice/internal/voiceuserprincipalruntime/unrelated.go \
  src/backend/voice/internal/principalgrpc/interceptor.go.near-match \
  protos/voice/auth/v1/unrelated.proto \
  src/backend/voice/pb/voice/events/v1/events.pb.go \
  deploy/nats/operator-owned-stream.yaml; do
  if space_media_path_allowed "${unrelated_space_media_path}"; then
    printf 'F13 oracle bug: unrelated or forbidden BE-255 near-match was accepted: %s\n' "${unrelated_space_media_path}" >&2
    exit 2
  fi
done

t31_runtime_path_allowed() {
  local path="$1"
  grep -Fxq -- "${path}" "${ROOT}/scripts/ci/t31-r22-scope-allowlist.txt"
}

# The T31 cross-service vertical is explicitly approved for its exact paths;
# this must stay a file-by-file exception, not a directory or suffix pattern.
for t31_path in \
  protos/voice/role/v1/role.proto \
  src/backend/role/internal/store/game_session_grants.go \
  src/backend/voice/main.go; do
  t31_runtime_path_allowed "${t31_path}" || {
    printf 'F13 oracle bug: approved T31 path was rejected: %s\n' "${t31_path}" >&2
    exit 2
  }
done
for unrelated_path in \
  src/backend/role/internal/store/unrelated.go \
  src/backend/role/internal/store/game_session_grants.go.near-match \
  src/backend/voice/main.go.near-match \
  protos/voice/role/v1/unrelated.proto; do
  if t31_runtime_path_allowed "${unrelated_path}"; then
    printf 'F13 oracle bug: unrelated or near-match T31 path was accepted: %s\n' "${unrelated_path}" >&2
    exit 2
  fi
done
if grep -Fq -e '*' -e '?' -e '[' -e ']' "${ROOT}/scripts/ci/t31-r22-scope-allowlist.txt"; then
  printf '%s\n' 'F13 oracle bug: T31 path exception contains a glob' >&2
  exit 2
fi

for identity_publisher_path in \
  src/backend/role/internal/roleevents/jetstream.go \
  src/backend/role/internal/roleevents/jetstream_test.go \
  src/backend/voice/internal/voiceevents/jetstream.go; do
  identity_publisher_path_allowed "${identity_publisher_path}" || {
    printf 'F13 oracle bug: exact identity publisher path was rejected: %s\n' "${identity_publisher_path}" >&2
    exit 2
  }
done
for identity_publisher_path in \
  src/backend/role/internal/roleevents/jetstream.go.near-match \
  src/backend/role/internal/roleevents/publisher.go \
  src/backend/voice/internal/voiceevents/jetstream_test.go \
  src/backend/voice/internal/voiceevents/other.go; do
  if identity_publisher_path_allowed "${identity_publisher_path}"; then
    printf 'F13 oracle bug: non-publisher identity path was accepted: %s\n' "${identity_publisher_path}" >&2
    exit 2
  fi
done

r23_contract_path_authorized_in_delta \
  'protos/voice/auth/v1/auth.proto' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: canonical R23 proto was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/backend/auth/src/main/proto/voice/auth/v1/auth.proto' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: committed R23 Auth mirror was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/backend/voice/pb/voice/space/v1/space.pb.go' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: committed R23 Go target was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/frontend/lib/gen/voice/space/v1/space.pb.dart' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: committed R23 Dart target was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/backend/voice/pb/voice/space/go.mod' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: derived generated-module go.mod was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/backend/space/Dockerfile' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: exact R23 CI-closure path was rejected' >&2
  exit 2
}
r23_contract_path_authorized_in_delta \
  'src/backend/role/internal/grpcsvc/ordinary_scope_test.go' \
  "${TMP_DIR}/r23-fixed-delta" || {
  printf '%s\n' 'F13 oracle bug: exact R23 Role contract-test path was rejected' >&2
  exit 2
}
if r23_contract_path_authorized_in_delta \
  'protos/voice/auth/v1/auth.proto.near-match' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: near-match R23 path was accepted' >&2
  exit 2
fi
if r23_contract_path_authorized_in_delta \
  'src/backend/auth/target/generated-sources/protobuf/java/app/voice/auth/v1/Auth.java' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: uncommitted Java target was accepted' >&2
  exit 2
fi
if r23_contract_path_authorized_in_delta \
  'src/backend/space/Dockerfile.near-match' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: near-match CI-closure path was accepted' >&2
  exit 2
fi
if r23_contract_path_authorized_in_delta \
  'src/backend/role/internal/grpcsvc/ordinary_scope_test.go.near-match' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: near-match Role contract-test path was accepted' >&2
  exit 2
fi
if r23_contract_path_authorized_in_delta \
  'src/backend/space/internal/grpcsvc/r23_rogue.go' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: unlisted R23 path was accepted' >&2
  exit 2
fi
grep -Fxv -- 'protos/voice/auth/v1/auth.proto' \
  "${TMP_DIR}/r23-fixed-delta" >"${TMP_DIR}/r23-fixed-delta-without-auth"
if r23_contract_path_authorized_in_delta \
  'protos/voice/auth/v1/auth.proto' \
  "${TMP_DIR}/r23-fixed-delta-without-auth"; then
  printf '%s\n' 'F13 oracle bug: authorized path absent from fixed_delta was accepted' >&2
  exit 2
fi
if r23_contract_path_allowed_for_window \
  false \
  'protos/voice/auth/v1/auth.proto' \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: manifest-present window still exempted an R23 path' >&2
  exit 2
fi
ordinary_disabled_window_path='docs/PLAN.md'
if r23_contract_path_allowed_for_window \
  false \
  "${ordinary_disabled_window_path}" \
  "${TMP_DIR}/r23-fixed-delta"; then
  printf '%s\n' 'F13 oracle bug: disabled window unexpectedly exempted an ordinary path' >&2
  exit 2
fi

fixed_tree="$(git -C "${ROOT}" rev-parse "${accepted_r23_base}^{tree}")"
r23_fixture_commit() {
  local message="$1"
  shift
  printf '%s\n' "${message}" | \
    GIT_AUTHOR_NAME='Voice CI' \
    GIT_AUTHOR_EMAIL='voice-ci@example.invalid' \
    GIT_COMMITTER_NAME='Voice CI' \
    GIT_COMMITTER_EMAIL='voice-ci@example.invalid' \
    git -C "${ROOT}" -c commit.gpgSign=false commit-tree "$@"
}
prerequisite_commit="$(r23_fixture_commit 'F13 prerequisite fixture' "${fixed_tree}" -p "${accepted_r23_base}")"
merge_side_commit="$(r23_fixture_commit 'F13 merge-side fixture' "${fixed_tree}" -p "${accepted_r23_base}")"
merge_commit="$(r23_fixture_commit 'F13 prerequisite merge fixture' "${fixed_tree}" -p "${prerequisite_commit}" -p "${merge_side_commit}")"
post_merge_commit="$(r23_fixture_commit 'F13 post-merge push fixture' "${fixed_tree}" -p "${merge_commit}")"
non_descendant_commit="$(git -C "${ROOT}" rev-list --max-parents=0 "${accepted_r23_base}" | sed -n '1p')"
r23_contract_base_allowed "${accepted_r23_base}" || {
  printf '%s\n' 'F13 oracle bug: accepted R23 base was rejected' >&2
  exit 2
}
r23_contract_base_allowed "${prerequisite_commit}" || {
  printf '%s\n' 'F13 oracle bug: descendant prerequisite base was rejected' >&2
  exit 2
}
r23_contract_base_allowed "${merge_commit}" || {
  printf '%s\n' 'F13 oracle bug: prerequisite merge base was rejected' >&2
  exit 2
}
r23_contract_base_allowed "${post_merge_commit}" || {
  printf '%s\n' 'F13 oracle bug: post-merge push base was rejected' >&2
  exit 2
}
if r23_contract_base_allowed "${non_descendant_commit}"; then
  printf '%s\n' 'F13 oracle bug: non-descendant base was accepted' >&2
  exit 2
fi
if r23_contract_base_allowed HEAD; then
  printf '%s\n' 'F13 oracle bug: descendant base containing the R23 manifest was accepted' >&2
  exit 2
fi

printf '%s\n' '{' >"${TMP_DIR}/malformed-r23-manifest.json"
if python3 "${TMP_DIR}/r23-allowlist.py" \
  "${TMP_DIR}/malformed-r23-manifest.json" \
  "${ROOT}/protos/voice/r23_contract_test/generated_targets.json" \
  "${accepted_r23_base}" \
  >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle bug: malformed manifest enabled the R23 allowlist' >&2
  exit 2
fi
python3 - \
  "${ROOT}/protos/voice/r23_contract_manifest.json" \
  "${TMP_DIR}/mismatched-r23-manifest.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    manifest = json.load(stream)
manifest["metadata"]["base_sha"] = "0000000000000000000000000000000000000000"
with open(sys.argv[2], "w", encoding="utf-8") as stream:
    json.dump(manifest, stream)
PY
if python3 "${TMP_DIR}/r23-allowlist.py" \
  "${TMP_DIR}/mismatched-r23-manifest.json" \
  "${ROOT}/protos/voice/r23_contract_test/generated_targets.json" \
  "${accepted_r23_base}" \
  >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle bug: mismatched manifest base_sha enabled the R23 allowlist' >&2
  exit 2
fi

source "${ROOT}/scripts/ci/voice-r22-runtime-scope.sh"

expect_fixed '.github/workflows/ci.yml' \
  'echo "VOICE_CI_PUSH_BEFORE_SHA=${VOICE_CI_PUSH_BEFORE_SHA}" >> "${GITHUB_ENV}"' F13

f13_develop_aggregate_enabled() {
  [[ "${1:-}" == 'push' && "${2:-}" == 'refs/heads/develop' ]]
}

# Only the named release branch uses per-contribution analysis on a master PR.
# Ordinary pull requests and all branch pushes retain the aggregate path below.
f13_master_release_pr_enabled() {
  [[ "${1:-}" == 'pull_request' &&
    "${2:-}" == 'master' &&
    "${3:-}" == codex/voice-master-release-* &&
    "${4:-}" =~ ^refs/pull/[0-9]+/merge$ ]]
}

# A pinned Game checkpoint selects only a verified GitHub PR merge graph; it is routing, not authority.
f13_master_checkpoint_pr_enabled() {
  local repo="$1" event="${2:-}" base_ref="${3:-}" head_ref="${4:-}"
  local merge_ref="${5:-}" event_sha="${6:-}" event_base="${7:-}" target="${8:-}"
  local target_commit base_commit target_line source_head source_delta
  local -a target_fields
  [[ "$event" == pull_request && "$base_ref" == master &&
    "$merge_ref" =~ ^refs/pull/[0-9]+/merge$ &&
    -n "$head_ref" && -n "$event_sha" && -n "$event_base" && -n "$target" ]] || return 1
  target_commit="$(git -C "$repo" rev-parse --verify "$target^{commit}" 2>/dev/null)" || return 1
  [[ "$target_commit" == "$event_sha" ]] || return 1
  base_commit="$(git -C "$repo" rev-parse --verify "$event_base^{commit}" 2>/dev/null)" || return 1
  target_line="$(git -C "$repo" rev-list --parents -n 1 "$target_commit" 2>/dev/null)" || return 1
  read -r -a target_fields <<<"$target_line"
  (("${#target_fields[@]}" == 3)) || return 1
  [[ "${target_fields[1]}" == "$base_commit" ]] || return 1
  source_head="${target_fields[2]}"
  source_delta="${TMP_DIR}/f13-checkpoint-pr-source-delta"
  git -C "$repo" diff --name-only --no-renames "$base_commit" "$source_head" >"$source_delta" 2>/dev/null || return 1
  game_checkpoint_projection_authorized_in_delta "$repo" "$target_commit" "$base_commit" "$source_head"     "$game_checkpoint_snapshot_base" "$game_checkpoint_snapshot_head" "$game_checkpoint_snapshot_record" "$source_delta"
}

f13_master_pr_contribution_enabled() {
  local repo="$1" event="${2:-}" base_ref="${3:-}" head_ref="${4:-}"
  local merge_ref="${5:-}" event_sha="${6:-}" event_base="${7:-}" target="${8:-}"
  f13_master_release_pr_enabled "$event" "$base_ref" "$head_ref" "$merge_ref" ||
    f13_master_checkpoint_pr_enabled "$repo" "$event" "$base_ref" "$head_ref" "$merge_ref" "$event_sha" "$event_base" "$target"
}

# Master pushes must be analyzed as a merge contribution. The merge collector
# validates the event-before first parent, exact two-parent shape, and result
# tree; malformed pushes must fail there rather than fall back to aggregate.
f13_master_merge_push_enabled() {
  [[ "${1:-}" == push && "${2:-}" == refs/heads/master ]]
}

f13_master_push_before_matches() {
  local event_before="${1:-}" resolved_base="${2:-}"
  [[ "${event_before}" =~ ^[0-9a-fA-F]{40}$ &&
    "${event_before,,}" != 0000000000000000000000000000000000000000 &&
    "${resolved_base}" == "${event_before}" ]]
}

f13_path_is_guarded() {
  case "$1" in
    protos/*|*/pb/*|*.pb.go|src/backend/space/*|src/backend/role/*|\
    src/backend/voice/pb/*|src/backend/voice/*_test.go|\
    src/backend/voice/*.go|src/backend/voice/go.mod|src/backend/voice/go.sum|\
    src/backend/voice/Dockerfile|src/backend/migrations/voice_db/*|\
    src/backend/voice/internal/grpcsvc/*.go|src/backend/voice/internal/store/*.go|\
    src/backend/voice/internal/livekit/*.go|src/backend/voice/internal/s2s/*.go|\
    src/backend/voice/internal/voiceevents/*.go|src/backend/voice/internal/roomlifecycle/*.go)
      return 0 ;;
  esac
  return 1
}

# Criss-cross topic merges can have several best merge bases even when their
# guarded/runtime inputs have one unambiguous shared state. In that narrow case
# use the common ancestor of all best bases, but only after proving every path
# governed by this collector has the same complete tree entry at each best
# base. Differences in those paths remain ambiguous and fail closed.
f13_f54_capability_file=''
f13_f54_3071_capability_file=''
f13_f54_I_operation=''
f13_f54_I_ledger=''
f13_f54_I_ledger_sha=''
f13_f54_J_operation=''
f13_f54_J_ledger=''
f13_f54_J_ledger_sha=''
f13_f54_collection_file=''
f13_f54_collection_sha=''
f13_f54_router_seal=''
f13_f54_router_seal_sha=''
f13_f54_router_receipt=''
f13_f54_router_observation=''

f13_f54_trace_event() {
  local kind="$1" trace_repo="$2"
  shift 2
  [[ -n "${f13_f54_trace_file:-}" ]] || return 0
  python3 "${f13_f54_observer}" append "${f13_f54_trace_file}" "${kind}" \
    "${f13_f54_frame:-}" "${f13_f54_operation:-}" "${trace_repo}" "$@"
}

f13_f54_observe_child() {
  local repo="$1" merge="$2" first="$3" source="$4" output="$5" observation child_status=0 scalar_status=0 scalar_entry
  [[ -n "${f13_f54_trace_file:-}" && -n "${f13_f54_observer:-}" &&
    "${merge}" == f54bcadab4aa19969281d7d35d76ebad91553d4d &&
    "${first}" == 0a0024900b5bb3ba2f6fee391615f0f4fa0864e3 &&
    "${source}" == f15d718b2d20837ae5503dc049134b66592b3675 ]] || return 0
  observation="$(python3 "${f13_f54_observer}" boundary "${repo}" "${merge}" "${first}" "${source}")" || return 1
  f13_f54_trace_event boundary_observation "${repo}" "${output}" \
    4d943fac27fd247d136d84ae145440e54485d3ab || return 1
  scalar_entry="$(git -C "${repo}" ls-tree "${merge}" -- protos/voice/auth/v1/auth.proto)" || return 1
  f13_checkpoint_merge_tree_entry_matches "${repo}" 4d943fac27fd247d136d84ae145440e54485d3ab \
    "${first}" "${source}" protos/voice/auth/v1/auth.proto "${scalar_entry}" || scalar_status=$?
  f13_f54_trace_event scalar_seam_observation "${repo}" "${scalar_status}" || return 1
  [[ "${scalar_status}" == 1 ]] || return 1
  printf '%s\n' 'F54 prototype child begin: supplemental partitiontrue, original outer prefix ends here' >&2
  f13_collect_develop_contributions "${repo}" 4d943fac27fd247d136d84ae145440e54485d3ab \
    "${source}" "${TMP_DIR}/f54-observed-child-map" "${TMP_DIR}/f54-observed-child-category" true true || child_status=$?
  # The original resolver caller still FAILS. This is supplementary real child
  # provenance; it neither supplies the resolver output nor replaces outer rows.
  printf 'F54 prototype child observation exit=%s boundary=%s\n' "${child_status}" "${observation}" >&2
  return 0
}

f13_f54_observe_span() {
  local repo="$1" original_trace="$2" original_native="$3" current_native="$4"
  local trace="$5" map="$6" category="$7" binding
  local origin_command=span-origin
  local -a origin_author_args=()
  if [[ -n "${f13_f54_span_author_capability:-}" ]]; then
    origin_command=span-author-origin
    origin_author_args=("${f13_f54_span_author_capability}")
  fi
  local f13_f54_trace_file="${trace}" f13_f54_frame='' f13_f54_operation=''
  [[ -n "${f13_f54_observer:-}" && ! -e "${trace}" && "${map}" != "${TMP_DIR}/f13-contributions" ]] || return 1
  binding="$(python3 "${f13_f54_observer}" "${origin_command}" "${repo}" "${original_trace}" \
    "${original_native}" "${current_native}" "${origin_author_args[@]}")" || return 1
  f13_f54_trace_event span_link "${repo}" "${binding}" || return 1
  # This is new supplemental evidence, not an original recursive call. Keep
  # the observed integration owner/map and propagate every genuine failure.
  f13_collect_develop_contributions "${repo}" 2445e81de07681cfd5f1491e4c793aaac747f571 \
    23551116ac5e741d3ec4ff7b412b7c9e248539ff "${map}" "${category}" false true
}

f13_f54_collect_exact_unit() {
  local repo="$1" base="$2" target="$3" output="$4" category="$5" unit="$6"
  local f13_f54_original_trace="$7" f13_f54_original_native="$8" f13_f54_current_native="$9"
  local f13_f54_capability_file="${TMP_DIR}/f54-exact-source-capabilities.json"
  local f13_f54_M_operation='' f13_f54_M_child_map="${TMP_DIR}/f54-actual-M-child-map"
  local f13_f54_M_span_trace="${TMP_DIR}/f54-actual-M-span.jsonl" f13_f54_M_span_map="${TMP_DIR}/f54-actual-M-span-map"
  local f13_f54_M_ledger="${TMP_DIR}/f54-actual-M-ledger.json" f13_f54_M_ledger_sha=''
  [[ "${repo}" == "${ROOT}" && "${f13_f54_observer:-}" == "${ROOT}/scripts/ci/f13_f54_history.py" &&
    -n "${f13_f54_trace_file:-}" && ! -e "${f13_f54_trace_file}" && ! -e "${f13_f54_capability_file}" ]] || return 1
  # Explicit finite author entrypoint; the ordinary production entrypoint does
  # not initialize a capability from environment or prototype constants.
  python3 "${f13_f54_observer}" capabilities "${repo}" "${unit}" >"${f13_f54_capability_file}" || return 1
  python3 "${f13_f54_observer}" cap-entry "${repo}" "${f13_f54_capability_file}" "${base}" "${target}" >/dev/null || return 1
  f13_collect_develop_contributions "${repo}" "${base}" "${target}" "${output}" "${category}" true false || return 1
  f13_f54_collection_file="${TMP_DIR}/f54-completed-collection.json"
  python3 "${f13_f54_observer}" collection-seal "${repo}" "${f13_f54_trace_file}" "${output}" >"${f13_f54_collection_file}" || return 1
  f13_f54_collection_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_collection_file}")"
}

f13_f54_collect_exact_master_unit() {
  f13_f54_collect_master_author_impl capabilities "$@"
}

f13_f54_collect_current_author_master_unit() {
  f13_f54_collect_master_author_impl current-author-capabilities "$@"
}

f13_f54_production_applicable() {
  # Ancestry selects a mandatory verifier, never a permission or fallback.
  git -C "${ROOT}" merge-base --is-ancestor d1727a21dbe7aa72385228b21be6465eac66da50 HEAD
}

f13_f54_collect_production_master_unit() {
  local repo="$1" base="$2" target="$3" output="$4" category="$5"
  local inputs="${TMP_DIR}/f13-production-inputs"
  [[ "${repo}" == "${ROOT}" && "${native_p3_active:-false}" == true && ! -e "${inputs}" ]] || return 1
  f13_f54_observer="${ROOT}/scripts/ci/f13_f54_history.py"
  f13_f54_trace_file="${TMP_DIR}/f13-production-trace.jsonl"
  [[ ! -e "${f13_f54_trace_file}" ]] || return 1
  python3 "${ROOT}/scripts/ci/f13_production_source.py" materialize "${repo}" "${inputs}" \
    >"${TMP_DIR}/f13-production-applicability.json" || return 1
  f13_f54_collect_master_author_impl production-capabilities "${repo}" "${base}" "${target}" \
    "${output}" "${category}" "${inputs}/unit" "${inputs}/original/observed/trace.jsonl" \
    "${inputs}/original/baseline/f13-native-p3-binding.json" "${native_p3_binding}" "${inputs}/fixed3071-unit"
}

f13_f54_collect_master_author_impl() {
  local capability_command="$1"
  shift
  [[ "${capability_command}" == capabilities || "${capability_command}" == current-author-capabilities ||
     "${capability_command}" == production-capabilities ]] || return 1
  local J_capability_command=3071-capabilities
  local F_capability_command=corrected-F-capability
  if [[ "${capability_command}" == current-author-capabilities ]]; then
    J_capability_command=3071-current-author-capabilities
  fi
  if [[ "${capability_command}" == production-capabilities ]]; then
    J_capability_command=3071-production-capabilities
    F_capability_command=corrected-F-production-capability
  fi
  local repo="$1" base="$2" target="$3" output="$4" category="$5" unit="$6"
  local f13_f54_original_trace="$7" f13_f54_original_native="$8" f13_f54_current_native="$9"
  local fixed3071_unit="${10:-}" f13_f54_3071_capability_file=''
  local f13_f54_F_capability_file='' f13_f54_F_deletion_ruling="${unit}/rudeus410-F-reviewed-deletion-decision.md"
  local f13_f54_I_operation='' f13_f54_I_ledger="${TMP_DIR}/fixed3071-I-ledger.json" f13_f54_I_ledger_sha=''
  local f13_f54_J_operation='' f13_f54_J_ledger="${TMP_DIR}/fixed3071-J-ledger.json" f13_f54_J_ledger_sha=''
  local f13_f54_capability_file="${TMP_DIR}/f54-exact-source-capabilities.json"
  local f13_f54_span_author_capability=''
  if [[ "${capability_command}" == current-author-capabilities || "${capability_command}" == production-capabilities ]]; then
    f13_f54_span_author_capability="${f13_f54_capability_file}"
  fi
  local f13_f54_M_operation='' f13_f54_M_child_map="${TMP_DIR}/f54-actual-M-child-map"
  local f13_f54_M_span_trace="${TMP_DIR}/f54-actual-M-span.jsonl" f13_f54_M_span_map="${TMP_DIR}/f54-actual-M-span-map"
  local f13_f54_M_ledger="${TMP_DIR}/f54-actual-M-ledger.json" f13_f54_M_ledger_sha=''
  local entry="${TMP_DIR}/f54-master-entry.json" result="${TMP_DIR}/f54-master-wrapper-result.json"
  [[ "${repo}" == "${ROOT}" && "${f13_f54_observer:-}" == "${ROOT}/scripts/ci/f13_f54_history.py" &&
    "${native_p3_active:-false}" == true && -n "${f13_f54_trace_file:-}" && ! -e "${f13_f54_trace_file}" &&
    ! -e "${f13_f54_capability_file}" ]] || return 1
  # Same original wrapper and native B/S/T rules. This explicit author caller
  # only supplies finite M evidence to its genuine nested H53 source frame.
  python3 "${f13_f54_observer}" "${capability_command}" "${repo}" "${unit}" >"${f13_f54_capability_file}" || return 1
  python3 "${f13_f54_observer}" master-entry "${repo}" "${f13_f54_capability_file}" "${native_p3_binding}" >"${entry}" || return 1
  local history_binding='' entry_type
  entry_type="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["type"])' "${entry}")" || return 1
  if [[ "${entry_type}" == F54_EXPLICIT_MASTER_PRODUCTION_HISTORY_ENTRY_v1 ]]; then
    history_binding="${f13_f54_current_native}"
  fi
  if [[ -n "${fixed3071_unit}" ]]; then
    f13_f54_3071_capability_file="${TMP_DIR}/fixed3071-structural-capabilities.json"
    [[ ! -e "${f13_f54_3071_capability_file}" ]] || return 1
    python3 "${f13_f54_observer}" "${J_capability_command}" "${repo}" "${fixed3071_unit}" \
      "${native_p3_binding}" >"${f13_f54_3071_capability_file}" || return 1
    if [[ -f "${fixed3071_unit}/corrected-F/source-unit.json" ]]; then
      f13_f54_F_capability_file="${TMP_DIR}/corrected-F-capability.json"
      [[ ! -e "${f13_f54_F_capability_file}" ]] || return 1
      python3 "${f13_f54_observer}" "${F_capability_command}" "${repo}" \
        "${fixed3071_unit}/corrected-F/source-unit.json" >"${f13_f54_F_capability_file}" || return 1
    fi
  fi
  f13_f54_trace_event master_entry "${repo}" "${entry}" || return 1
  case "${GITHUB_EVENT_NAME:-}:${GITHUB_REF:-}" in
    pull_request:refs/pull/*/merge)
      f13_collect_master_release_pr_contributions "${repo}" "${base}" "${target}" "${output}" "${category}" "${history_binding}" || return 1 ;;
    push:refs/heads/master)
      f13_collect_master_push_contributions "${repo}" "${GITHUB_EVENT_NAME}" "${GITHUB_REF}" \
        "${VOICE_CI_PUSH_BEFORE_SHA:-}" "${base}" "${target}" "${output}" "${category}" "${history_binding}" || return 1 ;;
    *) return 1 ;;
  esac
  python3 "${f13_f54_observer}" master-result "${entry}" "${output}.source-map" "${output}" >"${result}" || return 1
  f13_f54_trace_event master_wrapper_result "${repo}" "${result}" || return 1
  f13_f54_collection_file="${TMP_DIR}/f54-completed-collection.json"
  python3 "${f13_f54_observer}" collection-seal "${repo}" "${f13_f54_trace_file}" "${output}" >"${f13_f54_collection_file}" || return 1
  f13_f54_collection_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_collection_file}")"
}

f13_f54_policy_context() {
  local output="$1" name
  local -a values=()
  while IFS= read -r name; do
    case "${name}" in
      ROOT|TMP_DIR|base_sha|r22_runtime_delta|accepted_*|game_checkpoint_*|native_p3_*|r23_*) values+=("${name}" "${!name-}") ;;
    esac
  done < <(compgen -v | LC_ALL=C sort -u)
  python3 "${f13_f54_observer}" policy-context "${ROOT}" "${TMP_DIR}/f13-contributions" "${values[@]}" >"${output}"
}

f13_f54_manual_gate_context() {
  local output="$1" name
  local -a values=()
  while IFS= read -r name; do
    case "${name}" in
      ROOT|TMP_DIR|base_sha|accepted_*|game_checkpoint_*|native_p3_*|r23_*) values+=("${name}" "${!name-}") ;;
    esac
  done < <(compgen -v | LC_ALL=C sort -u)
  python3 "${f13_f54_observer}" manual-F-context "${ROOT}" "${TMP_DIR}/f13-contributions" "${values[@]}" >"${output}"
}

f13_f54_router_prepare() {
  [[ -n "${f13_f54_collection_file:-}" ]] || return 0
  local context="${TMP_DIR}/f54-original-policy-context.json"
  f13_f54_policy_context "${context}" || return 1
  f13_f54_router_seal="${TMP_DIR}/f54-original-router-seal.json"
  python3 "${f13_f54_observer}" router-seal "${ROOT}" "${f13_f54_collection_file}" "${f13_f54_collection_sha}" \
    "${TMP_DIR}/f13-contributions" "${context}" >"${f13_f54_router_seal}" || return 1
  f13_f54_router_seal_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_router_seal}")" || return 1
  f13_f54_router_observation="${TMP_DIR}/f54-original-router.jsonl"
  [[ ! -e "${f13_f54_router_observation}" ]] || return 1
  f13_router_worker_socket="${TMP_DIR}/router.sock"
  f13_router_worker_session="$(python3 -c 'import secrets;print(secrets.token_hex(32))')" || return 1
  python3 "${f13_f54_observer}" router-worker "${f13_router_worker_socket}" "${f13_router_worker_session}" \
    "${ROOT}" "${f13_f54_router_seal}" "${f13_f54_router_seal_sha}" "${TMP_DIR}/f13-contributions" \
    "${TMP_DIR}/f54-current-policy-context.json" &
  f13_router_worker_pid=$!
  local attempt
  for ((attempt=0; attempt<100; attempt++)); do
    if [[ -f "${f13_router_worker_socket}.ready" ]]; then
      [[ "$(<"${f13_router_worker_socket}.ready")" == "${f13_router_worker_session}" ]] || return 1
      return 0
    fi
    kill -0 "${f13_router_worker_pid}" 2>/dev/null || return 1
    sleep 0.05
  done
  return 1
}

f13_f54_router_request() {
  [[ -n "${f13_router_worker_pid:-}" && -S "${f13_router_worker_socket:-}" ]] || return 1
  python3 "${f13_f54_observer}" router-request "${f13_router_worker_socket}" "${f13_router_worker_session}" "$1" "$2"
}

f13_f54_router_stop() {
  [[ -n "${f13_f54_collection_file:-}" ]] || return 0
  f13_f54_router_request stop '' || return 1
  wait "${f13_router_worker_pid}" || return 1
  f13_router_worker_pid=''
}

f13_f54_router_begin() {
  local path="$1"
  [[ -n "${f13_f54_collection_file:-}" ]] || return 0
  [[ -n "${f13_f54_router_seal:-}" ]] || return 1
  f13_f54_policy_context "${TMP_DIR}/f54-current-policy-context.json" || return 1
  f13_f54_router_receipt="${TMP_DIR}/f54-current-router-receipt.json"
  f13_f54_router_request bind "${path}" >"${f13_f54_router_receipt}" || return 1
  python3 "${f13_f54_observer}" router-append "${f13_f54_router_observation}" before "${f13_f54_router_receipt}"
}

f13_f54_router_gate() {
  local gate="$1" status="$2"
  shift 2
  [[ -n "${f13_f54_collection_file:-}" ]] || return 0
  [[ -s "${f13_f54_router_receipt:-}" ]] || return 1
  python3 "${f13_f54_observer}" router-append "${f13_f54_router_observation}" gate "${f13_f54_router_receipt}" "${gate}" "${status}" "$@"
}

f13_f54_execute_M_history() {
  local repo="$1" merge="$2" first="$3" source="$4" output="$5" receipt="$6" boundary
  local collector_base="${7:-}" collector_target="${8:-}"
  [[ -n "${f13_f54_capability_file:-}" && -s "${receipt}" ]] || return 1
  boundary="$(python3 "${f13_f54_observer}" cap-boundary "${repo}" "${f13_f54_capability_file}" \
    "${merge}" "${first}" "${source}" "${collector_base}" "${collector_target}")" || return 1
  boundary="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["history_boundary"])' "${boundary}")" || return 1
  f13_f54_M_operation="${f13_f54_operation}"
  f13_f54_trace_event boundary_observation "${repo}" "${output}" "${boundary}" || return 1
  # The child supplies actual history/flags, never replacement outer owners.
  f13_collect_develop_contributions "${repo}" "${boundary}" "${source}" \
    "${f13_f54_M_child_map}" "${TMP_DIR}/f54-actual-M-child-category" true true || return 1
  f13_f54_observe_span "${repo}" "${f13_f54_original_trace}" "${f13_f54_original_native}" \
    "${f13_f54_current_native}" "${f13_f54_M_span_trace}" "${f13_f54_M_span_map}" \
    "${TMP_DIR}/f54-actual-M-span-category"
}

f13_f54_execute_3071_history() {
  local repo="$1" merge="$2" first="$3" source="$4" output="$5" receipt="$6" diagnostic_file="$7" boundary
  [[ -n "${f13_f54_3071_capability_file:-}" && -s "${receipt}" ]] || return 1
  boundary="$(python3 "${f13_f54_observer}" 3071-boundary "${repo}" "${f13_f54_3071_capability_file}" "${merge}" "${first}" "${source}")" || return 1
  boundary="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["history_boundary"])' "${boundary}")" || return 1
  f13_f54_trace_event boundary_3071_observation "${repo}" "${output}" "${boundary}" || return 1
  # This genuine child adds evidence; it never supplies replacement outer owners.
  # Original failure propagates unchanged. No M supersession is extended here.
  f13_collect_develop_contributions "${repo}" "${boundary}" "${source}" \
    "${TMP_DIR}/fixed3071-child-map" "${diagnostic_file}" true true
}

f13_f54_fixed_J_prior_side() {
  local repo="$1" origin="$2" trace="$3" output="$4"
  local f13_f54_trace_file="${trace}" f13_f54_frame='' f13_f54_operation='' f13_f54_running=false
  [[ ! -e "${trace}" && ! -e "${output}" ]] || return 1
  f13_f54_trace_event J_prior_span_link "${repo}" "${origin}" || return 1
  # New source-history observation only. No Realtime/product executable runs,
  # and this map never replaces or concatenates the real dc0/J outer map.
  f13_collect_develop_contributions "${repo}" d4abb829622e9bd91b26842b61522e6840840867 \
    645d9c54966c1270a1e1dbe9d767ddad6aac40ff "${output}" "${TMP_DIR}/fixed3071-J-prior-category" true false
}

f13_f54_prepare_J_prior_history() {
  local repo="$1" output="$2" origin="${TMP_DIR}/fixed3071-J-prior-origin.json"
  local trace="${TMP_DIR}/fixed3071-J-prior.jsonl" map="${TMP_DIR}/fixed3071-J-prior-map"
  local receipt="${TMP_DIR}/fixed3071-J-prior-receipt.json"
  [[ -n "${f13_f54_3071_capability_file:-}" && ! -e "${origin}" && ! -e "${receipt}" ]] || return 1
  python3 "${f13_f54_observer}" 3071-J-prior-origin "${repo}" "${f13_f54_3071_capability_file}" \
    "${f13_f54_trace_file}" >"${origin}" || return 1
  f13_f54_fixed_J_prior_side "${repo}" "${origin}" "${trace}" "${map}" || return 1
  python3 "${f13_f54_observer}" 3071-J-prior-receipt "${repo}" "${origin}" "${trace}" "${map}" "${output}" >"${receipt}" || return 1
  f13_f54_trace_event J_prior_history_ready "${repo}" "${receipt}"
}

f13_f54_prepare_J_ledger() {
  local repo="$1" wanted="${f13_f54_frame:-}/3071ddad390443de4119c796cdad9899ff08f7fd/2"
  [[ -n "${f13_f54_3071_capability_file:-}" && "${f13_f54_operation:-}" == "${wanted}" ]] || return 0
  [[ -s "${TMP_DIR}/fixed3071-J-prior-receipt.json" && -n "${f13_f54_J_ledger:-}" && ! -e "${f13_f54_J_ledger}" ]] || return 1
  python3 "${f13_f54_observer}" 3071-J-ledger "${repo}" "${f13_f54_3071_capability_file}" "${f13_f54_trace_file}" >"${f13_f54_J_ledger}" || return 1
  f13_f54_J_operation="${wanted}"
  f13_f54_J_ledger_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_J_ledger}")" || return 1
  f13_f54_trace_event range_J_ledger_ready "${repo}" "${f13_f54_J_ledger}"
}

f13_f54_consume_J_before_mutation() {
  local repo="$1" path="$2" output="$3" incoming="$4" transition="$5" receipt="${TMP_DIR}/fixed3071-J-consumed.json"
  [[ -n "${f13_f54_J_operation:-}" && "${f13_f54_operation:-}" == "${f13_f54_J_operation}" ]] || return 0
  [[ -n "${f13_f54_J_ledger_sha:-}" ]] || return 1
  if python3 "${f13_f54_observer}" finite-J-path "${path}"; then
    [[ "${transition}" == replace && -n "${f13_finite_J_worker_pid:-}" ]] || return 1
    python3 "${f13_f54_observer}" finite-J-request "${f13_finite_J_socket}" "${f13_finite_J_session}" \
      consume "${path}" "${incoming}" >"${receipt}" || return 1
  else
    python3 "${f13_f54_observer}" 3071-J-consume "${f13_f54_J_ledger}" "${f13_f54_J_ledger_sha}" \
      "${f13_f54_trace_file}" "${path}" "${output}" "${incoming}" "${transition}" >"${receipt}" || return 1
  fi
  f13_f54_trace_event range_J_ledger_conserved "${repo}" "${path}" "${receipt}"
}

f13_f54_fixed3071_range_observe() {
  local repo="$1" base="$2" target="$3" allow="$4" partition="$5"
  local origin="${TMP_DIR}/fixed3071-range-origin.json" receipt="${TMP_DIR}/fixed3071-range-branch.json"
  local side_trace="${TMP_DIR}/fixed3071-range-side.jsonl" side_map="${TMP_DIR}/fixed3071-range-side-map"
  [[ -n "${f13_f54_3071_capability_file:-}" && -n "${f13_f54_trace_file:-}" &&
    "${f13_f54_observer:-}" == "${repo}/scripts/ci/f13_f54_history.py" &&
    "${base}" == d4abb829622e9bd91b26842b61522e6840840867 &&
    "${target}" == 909090a34743902799306974d6f3ebabe3c02f10 &&
    "${allow}:${partition}" == true:false && ! -e "${side_trace}" ]] || return 1
  python3 "${f13_f54_observer}" 3071-range-origin "${repo}" "${f13_f54_3071_capability_file}" \
    "${f13_f54_trace_file}" >"${origin}" || return 1
  f13_f54_fixed3071_range_side "${repo}" "${origin}" "${side_trace}" "${side_map}" || return 1
  python3 "${f13_f54_observer}" 3071-range-branch "${repo}" "${origin}" "${side_trace}" "${side_map}" >"${receipt}" || return 1
  # The ordinary range predicate is still false. A distinct fixed witness and
  # successful actual side frame permit only observation of the same engine.
  f13_f54_trace_event range_3071_branch "${repo}" "${receipt}"
}

f13_f54_fixed3071_range_side() {
  local repo="$1" origin="$2" trace="$3" output="$4"
  local f13_f54_trace_file="${trace}" f13_f54_frame='' f13_f54_operation='' f13_f54_running=false
  f13_f54_trace_event range_side_link "${repo}" "${origin}" || return 1
  # Separate evidence. Original operation body/gates, flags and owners stay raw.
  # Never concatenate this map into the three-spine persistent output map.
  f13_collect_develop_contributions "${repo}" d4abb829622e9bd91b26842b61522e6840840867 \
    a80f649cbe0a03671c25531ace4ac1bd6bf54756 "${output}" "${TMP_DIR}/fixed3071-range-side-category" true false
}

f13_f54_prepare_range_I_ledger() {
  local repo="$1" branch="${TMP_DIR}/fixed3071-range-branch.json" wanted
  [[ -n "${f13_f54_3071_capability_file:-}" && -s "${branch}" ]] || return 0
  wanted="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["origin"]["state"]["integration_operation"])' "${branch}")" || return 1
  [[ "${wanted}" == "${f13_f54_operation:-}" ]] || return 0
  [[ -n "${f13_f54_I_ledger:-}" && ! -e "${f13_f54_I_ledger}" ]] || return 1
  f13_f54_I_operation="${wanted}"
  python3 "${f13_f54_observer}" 3071-I-ledger "${repo}" "${f13_f54_3071_capability_file}" \
    "${f13_f54_trace_file}" "${branch}" >"${f13_f54_I_ledger}" || return 1
  f13_f54_I_ledger_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_I_ledger}")" || return 1
  f13_f54_trace_event range_I_ledger_ready "${repo}" "${f13_f54_I_ledger}"
}

f13_f54_consume_I_before_mutation() {
  local repo="$1" path="$2" output="$3" incoming="$4" transition="$5" receipt="${TMP_DIR}/fixed3071-I-consumed.json"
  [[ -n "${f13_f54_I_operation:-}" && "${f13_f54_operation:-}" == "${f13_f54_I_operation}" ]] || return 0
  [[ -n "${f13_f54_I_ledger_sha:-}" ]] || return 1
  python3 "${f13_f54_observer}" 3071-I-consume "${f13_f54_I_ledger}" "${f13_f54_I_ledger_sha}" \
    "${f13_f54_trace_file}" "${path}" "${output}" "${incoming}" "${transition}" >"${receipt}" || return 1
  f13_f54_trace_event range_I_ledger_conserved "${repo}" "${path}" "${receipt}"
}

f13_f54_consume_M_before_mutation() {
  local repo="$1" path="$2" output="$3" incoming="$4" category="$5" transition="$6" receipt
  [[ -n "${f13_f54_M_operation:-}" && "${f13_f54_operation}" == "${f13_f54_M_operation}" ]] || return 0
  receipt="${TMP_DIR}/f54-actual-M-consumed.json"
  python3 "${f13_f54_observer}" M-consume "${f13_f54_M_ledger}" "${f13_f54_M_ledger_sha}" \
    "${f13_f54_trace_file}" "${path}" "${output}" "${incoming}" "${transition}" >"${receipt}" || return 1
  # Conservation precedes mutation. The original full policy router stays at
  # its later phase; a raw Boolean is neither rewritten nor an admission grant.
  f13_f54_trace_event complete_ledger_conserved "${repo}" "${path}" "${receipt}"
}

f13_f54_exact_supersession() {
  local repo="$1" path="$2" output="$3" incoming="$4" composition receipt result
  local collector_base="${5:-}" collector_target="${6:-}"
  [[ -n "${f13_f54_M_operation:-}" && "${f13_f54_operation:-}" == "${f13_f54_M_operation}" &&
    -n "${f13_f54_capability_file:-}" && -n "${f13_f54_M_ledger_sha:-}" ]] || return 1
  result="$(git -C "${repo}" ls-tree f54bcadab4aa19969281d7d35d76ebad91553d4d -- ":(literal)${path}")" || return 1
  composition="$(python3 "${f13_f54_observer}" cap-composition "${repo}" "${f13_f54_capability_file}" \
    f54bcadab4aa19969281d7d35d76ebad91553d4d 4d943fac27fd247d136d84ae145440e54485d3ab \
    0a0024900b5bb3ba2f6fee391615f0f4fa0864e3 f15d718b2d20837ae5503dc049134b66592b3675 "${path}" "${result}" \
    "${collector_base}" "${collector_target}")" || return 1
  f13_f54_trace_event composition_capability "${repo}" "${composition}" || return 1
  receipt="${TMP_DIR}/f54-exact-supersession.json"
  python3 "${f13_f54_observer}" M-supersession "${repo}" "${f13_f54_capability_file}" \
    "${f13_f54_M_ledger}" "${f13_f54_M_ledger_sha}" "${f13_f54_trace_file}" "${path}" "${output}" "${incoming}" >"${receipt}" || return 1
  f13_f54_trace_event exact_unit_supersession "${repo}" "${path}" "${receipt}"
}

f13_f54_prepare_M_ledger() {
  local repo="$1"
  [[ -n "${f13_f54_M_operation:-}" && "${f13_f54_operation}" == "${f13_f54_M_operation}" ]] || return 0
  [[ ! -e "${f13_f54_M_ledger}" ]] || return 1
  python3 "${f13_f54_observer}" M-ledger "${repo}" "${f13_f54_capability_file}" \
    "${f13_f54_trace_file}" "${f13_f54_original_trace}" "${f13_f54_M_span_trace}" \
    "${f13_f54_original_native}" "${f13_f54_current_native}" >"${f13_f54_M_ledger}" || return 1
  f13_f54_M_ledger_sha="$(python3 -c 'import hashlib,sys;print(hashlib.sha256(open(sys.argv[1],"rb").read()).hexdigest())' "${f13_f54_M_ledger}")" || return 1
  f13_f54_trace_event M_ledger_ready "${repo}" "${f13_f54_M_ledger}"
}

f13_resolve_source_base() {
  local repo="$1" first_parent="$2" source_head="$3" output_file="$4"
  local exact_merge="${5:-}" exact_capability="${6:-}" exact_observer="${7:-}" exact_receipt exact_3071_capability="${8:-}"
  local collector_base="${9:-}" collector_target="${10:-}"
  local bases canonical base path first_entry entry first_entry_set
  local paths="${TMP_DIR}/f13-ambiguous-base-paths-${first_parent}-${source_head}"
  bases="$(git -C "${repo}" merge-base --all "${first_parent}" "${source_head}" 2>/dev/null)" || return 1
  [[ -n "${bases}" ]] || return 1
  if [[ "$(wc -l <<<"${bases}" | tr -d '[:space:]')" == 1 ]]; then
    canonical="${bases}"
  else
    if [[ "${exact_merge}" == 132437c43f6f0625fae8954542f8d9c2f6aa3037 &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]]; then
      local -a exact_132_trace=()
      if [[ "${collector_base}" == 2c8bdf9e2f420b724575096beb4fcf5cb4ea00be &&
        "${collector_target}" == 011c53f4a2be3c4e8f660fa9dc6ce40492da20e2 ]]; then
        exact_132_trace=("${f13_f54_trace_file:?}")
      fi
      exact_receipt="$(python3 "${exact_observer}" 132-boundary "${repo}" "${exact_merge}" \
        "${first_parent}" "${source_head}" "${collector_base}" "${collector_target}" "${exact_132_trace[@]}")" || return 1
      canonical="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["history_boundary"])' "${exact_receipt}")" || return 1
      printf '%s\n' "${canonical}" >"${output_file}" || return 1
      printf '%s\n' "${bases}" >"${output_file}.candidates" || return 1
      printf '%s\n' "${exact_receipt}" >"${output_file}.132-capability" || return 1
      f13_f54_trace_event history_132_capability "${repo}" "${exact_receipt}" || return 1
      return 0
    fi
    if [[ -n "${exact_3071_capability}" && "${exact_merge}" == 3071ddad390443de4119c796cdad9899ff08f7fd &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]] &&
      exact_receipt="$(python3 "${exact_observer}" 3071-boundary "${repo}" "${exact_3071_capability}" \
        "${exact_merge}" "${first_parent}" "${source_head}")"; then
      canonical="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["history_boundary"])' "${exact_receipt}")" || return 1
      printf '%s\n' "${canonical}" >"${output_file}" || return 1
      printf '%s\n' "${bases}" >"${output_file}.candidates" || return 1
      printf '%s\n' "${exact_receipt}" >"${output_file}.3071-capability" || return 1
      f13_f54_trace_event history_3071_capability "${repo}" "${exact_3071_capability}" "${exact_receipt}" || return 1
      return 0
    fi
    if [[ -n "${exact_merge}" && -n "${exact_capability}" &&
      "${exact_observer}" == "${repo}/scripts/ci/f13_f54_history.py" ]] &&
      exact_receipt="$(python3 "${exact_observer}" cap-boundary "${repo}" "${exact_capability}" \
        "${exact_merge}" "${first_parent}" "${source_head}" "${collector_base}" "${collector_target}")"; then
      # This value is an exact history boundary, never a selected best base.
      canonical="$(python3 -c 'import json,sys;print(json.loads(sys.argv[1])["history_boundary"])' "${exact_receipt}")" || return 1
      printf '%s\n' "${canonical}" >"${output_file}" || return 1
      printf '%s\n' "${bases}" >"${output_file}.candidates" || return 1
      printf '%s\n' "${exact_receipt}" >"${output_file}.capability" || return 1
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event history_capability "${repo}" "${exact_capability}" "${exact_receipt}" || return 1
      fi
      return 0
    fi
    canonical="$(git -C "${repo}" merge-base --octopus "${first_parent}" "${source_head}" 2>/dev/null)" || return 1
    [[ "${canonical}" =~ ^[0-9a-f]{40}$ ]] || return 1
    git -C "${repo}" diff --name-only "${canonical}" "${first_parent}" >"${paths}" || return 1
    git -C "${repo}" diff --name-only "${canonical}" "${source_head}" >>"${paths}" || return 1
    while IFS= read -r base; do
      [[ -n "${base}" ]] || continue
      git -C "${repo}" diff --name-only "${canonical}" "${base}" >>"${paths}" || return 1
    done <<<"${bases}"

    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      if ! f13_path_is_guarded "${path}" && ! printf '%s\n' "${path}" | voice_r22_runtime_changed; then
        continue
      fi
      first_entry=""
      first_entry_set=false
      while IFS= read -r base; do
        [[ -n "${base}" ]] || continue
        entry="$(git -C "${repo}" ls-tree "${base}" -- ":(literal)${path}" 2>/dev/null)" || return 1
        if [[ "${first_entry_set}" != true ]]; then
          first_entry="${entry}"
          first_entry_set=true
        elif [[ "${entry}" != "${first_entry}" ]]; then
          return 1
        fi
      done <<<"${bases}"
    done < <(LC_ALL=C sort -u "${paths}")
  fi
  [[ "${canonical}" =~ ^[0-9a-f]{40}$ ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${canonical}" "${first_parent}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${canonical}" "${source_head}" 2>/dev/null || return 1
  printf '%s\n' "${canonical}" >"${output_file}"
  printf '%s\n' "${bases}" >"${output_file}.candidates"
}

f13_cross_scope_path_allowed() {
  local path="$1" runtime_trigger="$2"
  [[ "${runtime_trigger}" == true ]] || return 0
  case "${path}" in
    protos/*|*/pb/*|*.pb.go)
      return 1 ;;
    src/backend/space/*|src/backend/role/*)
      identity_publisher_path_allowed "${path}" ;;
    *)
      return 0 ;;
  esac
}

# Build an exact per-contribution map for an aggregate develop push. For merge
# commits, only source-base-to-source-head paths belong to the contribution;
# verify the merge tree retained each source path's full tree entry.
f13_collect_failure() {
  local diagnostic_file="$1" category="$2"
  if [[ -n "${f13_f54_trace_file:-}" ]]; then
    f13_f54_trace_event failure "${repo:-${ROOT}}" "${category}" "${output:-}" || return 1
  fi
  if [[ -n "${diagnostic_file}" ]]; then
    printf '%s\n' "${category}" >"${diagnostic_file}" 2>/dev/null || true
  fi
  return 1
}

f13_guarded_entry_matches_final_target() {
  local stored_entry="${1-}" target_entry="${2-}"
  # Empty is a legitimate exact ls-tree result for a deleted path. Callers
  # establish map-key presence separately before comparing entries.
  [[ "${stored_entry}" == "${target_entry}" ]]
}

f13_tree_entry_identity() {
  local entry="${1:-}" mode type oid
  if [[ -z "${entry}" ]]; then
    printf '%s' absent
    return 0
  fi
  read -r mode type oid _ <<<"${entry}"
  if [[ -z "${mode:-}" || -z "${type:-}" || -z "${oid:-}" ]]; then
    printf '%s' malformed
    return 0
  fi
  printf '%s:%s:%s' "${mode}" "${type}" "${oid}"
}

# Failure-only diagnostics stay on stderr and expose Git identities/tree entry
# metadata, never source contents, credentials, or the collector's stdout map.
f13_emit_guarded_failure_diagnostic() {
  local category="$1" path="$2" commit="$3" aggregate_head="$4"
  local owner="$5" source_head="$6" stored_entry="$7" target_entry="$8" predicate="$9"
  printf 'CI_DIAG stage=f13-collector category=%s path=%q commit=%s aggregate_head=%s owner=%s source_head=%s stored=%s target=%s predicate=%s\n' \
    "${category}" "${path}" "${commit:-unknown}" "${aggregate_head:-unknown}" \
    "${owner:-unknown}" "${source_head:-unknown}" \
    "$(f13_tree_entry_identity "${stored_entry}")" \
    "$(f13_tree_entry_identity "${target_entry}")" "${predicate}" >&2
}

f13_validate_sync_merge_resolution() {
  local repo="$1" commit="$2" diagnostic_file="${3:-}"
  local path
  local resolution_paths="${TMP_DIR}/f13-sync-merge-resolution-${commit}"
  git -C "${repo}" show --remerge-diff --format= --name-only "${commit}" 2>/dev/null >"${resolution_paths}" || {
    f13_collect_failure "${diagnostic_file}" sync_merge_resolution_inspection
    return 1
  }
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    if f13_path_is_guarded "${path}"; then
      f13_collect_failure "${diagnostic_file}" sync_merge_guarded_resolution
      return 1
    fi
  done <"${resolution_paths}"
}

# Retire a stale guarded row only when an exact base-sync merge imports the
# same full tree entry that the old, already-base-owned source row represented.
f13_sync_boundary_reconciles_guarded_path() {
  local repo="$1" base="$2" commit="$3" path="$4"
  local content_base="$5" owner="$6" source_head="$7"
  local saved_guarded_entry="$8" saved_source_entry="$9"
  local parents first_parent base_entry first_parent_entry result_entry owner_entry source_head_entry resolved_content_base
  parents="$(git -C "${repo}" rev-list --parents -n 1 "${commit}" 2>/dev/null)" || return 1
  read -r -a parents <<<"${parents}"
  ((${#parents[@]} == 3)) && [[ "${parents[2]}" == "${base}" ]] || return 1
  first_parent="${parents[1]}"
  [[ -n "${content_base}" ]] || return 1
  resolved_content_base="$(git -C "${repo}" rev-parse --verify "${content_base}^{commit}" 2>/dev/null)" || return 1
  [[ "${resolved_content_base}" == "${content_base}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${owner}" "${base}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${source_head}" "${base}" 2>/dev/null || return 1
  owner_entry="$(git -C "${repo}" ls-tree "${owner}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  source_head_entry="$(git -C "${repo}" ls-tree "${source_head}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  first_parent_entry="$(git -C "${repo}" ls-tree "${first_parent}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  base_entry="$(git -C "${repo}" ls-tree "${base}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  result_entry="$(git -C "${repo}" ls-tree "${commit}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  [[ "${saved_guarded_entry}" == "${owner_entry}" &&
    "${saved_source_entry}" == "${source_head_entry}" &&
    "${owner_entry}" == "${source_head_entry}" &&
    "${saved_guarded_entry}" == "${first_parent_entry}" &&
    "${saved_source_entry}" == "${first_parent_entry}" &&
    "${result_entry}" == "${base_entry}" ]]
}

# Separately typed complete native source authority. These variables are
# initialized here, never inherited as environment/caller proof. Game/R1/R2
# and guarded-correction records retain their own fixed identities.
native_p3_active=false
native_p3_paths=''
native_p3_paths_hash=''
native_p3_binding=''
native_p3_binding_hash=''
native_p3_source=''
native_p3_target=''
native_p3_snapshot=''

native_p3_path_bound() {
  local repo="$1" path="$2"
  [[ "${native_p3_active}" == true && "${repo}" == "${ROOT}" &&
    -n "${native_p3_paths}" && -n "${native_p3_binding}" &&
    "$(git -C "${ROOT}" hash-object -- "${native_p3_paths}")" == "${native_p3_paths_hash}" &&
    "$(git -C "${ROOT}" hash-object -- "${native_p3_binding}")" == "${native_p3_binding_hash}" ]] || return 1
  grep -Fxq -- "${path}" "${native_p3_paths}"
}

native_p3_prepare() {
  local source target parent_line event="${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}"
  local verifier="${ROOT}/scripts/ci/f13_native_p3_source.py"
  [[ ! -f "${ROOT}/scripts/ci/f13_native_p3_authority.py" ]] || verifier="${ROOT}/scripts/ci/f13_native_p3_authority.py"
  local -a parents
  target="$(git -C "${ROOT}" rev-parse HEAD)" || return 1
  source="${target}"
  if [[ "${event}" != push || "${GITHUB_REF:-}" != refs/heads/develop ]]; then
    parent_line="$(git -C "${ROOT}" rev-list --parents -n 1 "${target}")" || return 1
    read -r -a parents <<<"${parent_line}"
    ((${#parents[@]} == 3)) || return 1
    source="${parents[2]}"
  fi
  native_p3_paths="${TMP_DIR}/f13-native-p3-paths"
  native_p3_binding="${TMP_DIR}/f13-native-p3-binding.json"
  python3 "${verifier}" \
    --repo "${ROOT}" --base "${base_sha}" --source "${source}" --target "${target}" \
    --event "${event}" --ref="${GITHUB_REF:-}" --event-sha="${GITHUB_SHA:-}" \
    --before="${VOICE_CI_PUSH_BEFORE_SHA:-}" --base-ref="${GITHUB_BASE_REF:-}" \
    --output "${native_p3_binding}" >"${native_p3_paths}" || return 1
  native_p3_paths_hash="$(git -C "${ROOT}" hash-object -- "${native_p3_paths}")" || return 1
  native_p3_binding_hash="$(git -C "${ROOT}" hash-object -- "${native_p3_binding}")" || return 1
  native_p3_source="${source}"
  native_p3_target="${target}"
  native_p3_snapshot="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["snapshot"])' "${native_p3_binding}")" || return 1
  native_p3_active=true
}

native_p3_pre_h_owner() {
  local repo="$1" path="$2" owner="$3"
  native_p3_path_bound "${repo}" "${path}" &&
    git -C "${repo}" merge-base --is-ancestor "${owner}" "${native_p3_snapshot}" 2>/dev/null
}

native_p3_duplicate_owner_qualified() {
  local repo="$1" path="$2" prior_owner="$3" prior_source="$4" current_owner="$5"
  local current_source="$6" integration="$7" outcome="$8" candidate
  native_p3_path_bound "${repo}" "${path}" || return 1
  [[ "${outcome}" == clean || "${outcome}" == guarded_conflict || "${outcome}" == direct ]] || return 1
  for candidate in "${prior_owner}" "${prior_source}" "${current_owner}" "${current_source}" "${integration}"; do
    [[ -n "${candidate}" ]] &&
      git -C "${repo}" merge-base --is-ancestor "${candidate}" "${native_p3_snapshot}" 2>/dev/null || return 1
  done
  # Qualification retains all producer history and taint. Only an actual H
  # entry restoration may clear the intermediate guarded-entry obligation.
  if [[ "$(f13_git_tree_entry_record "${repo}" "${integration}" "${path}")" == \
        "$(f13_git_tree_entry_record "${repo}" "${native_p3_snapshot}" "${path}")" ]]; then
    return 0
  fi
  return 2
}

native_p3_audit_map() {
  [[ "${native_p3_active}" == true ]] || return 0
  local event="${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}"
  local verifier="${ROOT}/scripts/ci/f13_native_p3_source.py"
  [[ ! -f "${ROOT}/scripts/ci/f13_native_p3_authority.py" ]] || verifier="${ROOT}/scripts/ci/f13_native_p3_authority.py"
  python3 "${verifier}" \
    --repo "${ROOT}" --base "${base_sha}" --source "${native_p3_source}" --target "${native_p3_target}" \
    --event "${event}" --ref="${GITHUB_REF:-}" --event-sha="${GITHUB_SHA:-}" \
    --before="${VOICE_CI_PUSH_BEFORE_SHA:-}" --base-ref="${GITHUB_BASE_REF:-}" \
    --map "${TMP_DIR}/f13-contributions" --history "${TMP_DIR}/f13-native-p3-history.tsv" \
    --output "${TMP_DIR}/f13-native-p3-approval.json"
}

native_p3_path_allowed_for_provenance() {
  local repo="$1" map="$2" path="$3"
  native_p3_path_bound "${repo}" "${path}" || return 1
  [[ "${map}" == "${TMP_DIR}/f13-contributions" ]] || return 1
  python3 - "${TMP_DIR}/f13-native-p3-approval.json" "${map}" "${path}" \
    "${native_p3_binding}" "${TMP_DIR}/f13-native-p3-history.tsv" <<'PY'
import hashlib, json, pathlib, sys
try:
    proof = json.loads(pathlib.Path(sys.argv[1]).read_text())
    data = pathlib.Path(sys.argv[2]).read_bytes()
    valid = (hashlib.sha256(data).hexdigest() == proof['map_sha256']
        and hashlib.sha256(pathlib.Path(sys.argv[5]).read_bytes()).hexdigest() == proof['history_sha256']
        and proof['binding'] == json.loads(pathlib.Path(sys.argv[4]).read_text())
        and sum(line.split('\t')[0] == sys.argv[3] for line in data.decode().splitlines()) == 1
        and proof['rows'][sys.argv[3]] in data.decode().splitlines())
except (OSError, ValueError, KeyError):
    valid = False
sys.exit(0 if valid else 1)
PY
}

native_p3_state_fingerprint() {
  local state_file
  printf '%s\n' "${native_p3_active}" "${native_p3_source}" "${native_p3_target}" \
    "${native_p3_snapshot}" "${native_p3_paths_hash}" "${native_p3_binding_hash}"
  for state_file in "${native_p3_paths}" "${native_p3_binding}" \
    "${TMP_DIR}/f13-native-p3-history.tsv" "${TMP_DIR}/f13-native-p3-approval.json"; do
    if [[ -f "${state_file}" ]]; then
      git -C "${ROOT}" hash-object -- "${state_file}" || return 1
    else
      printf '%s\n' absent
    fi
  done
}

f13_reviewed_test_merge_qualified() {
  local repo="$1" merge="$2" first="$3" source_base="$4" source="$5" path="$6" receipt target
  [[ "${repo}" == "${ROOT}" ]] || return 1
  target="$(git -C "${ROOT}" rev-parse HEAD)" || return 1
  receipt="$(python3 "${ROOT}/scripts/ci/f13_reviewed_test_merge.py" \
    --repo "${repo}" --target "${target}" --merge "${merge}" --first "${first}" \
    --source-base "${source_base}" --source "${source}" --path "${path}" 2>/dev/null)" || return 1
  printf '%s\n' "${receipt}" >>"${TMP_DIR}/f13-reviewed-test-merges.jsonl" || return 1
  printf 'CI_DIAG stage=f13-reviewed-test-merge qualification=%s\n' "${receipt}" >&2
}

f13_historical_composition_qualified() {
  local repo="$1" merge="$2" first="$3" source_base="$4" source="$5" path="$6" receipt target
  [[ "${repo}" == "${ROOT}" ]] || return 1
  target="$(git -C "${ROOT}" rev-parse HEAD)" || return 1
  receipt="$(python3 "${ROOT}/scripts/ci/f13_historical_composition.py" \
    --repo "${repo}" --target "${target}" --merge "${merge}" --first "${first}" \
    --source-base "${source_base}" --source "${source}" --path "${path}" 2>/dev/null)" || return 1
  printf '%s\n' "${receipt}" >>"${TMP_DIR}/f13-historical-composition.jsonl" || return 1
  printf 'CI_DIAG stage=f13-historical-composition qualification=%s\n' "${receipt}" >&2
}

f13_role_conditional_qualified() {
  local repo="$1" map="$2" path="$3" incoming_row="$4" prior_row receipt target qualifier
  [[ "${repo}" == "${ROOT}" ]] || return 1
  [[ "$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${map}")" == 1 ]] || return 1
  prior_row="$(awk -F '\t' -v p="${path}" '$1 == p { print $0 }' "${map}")" || return 1
  target="$(git -C "${ROOT}" rev-parse HEAD)" || return 1
  qualifier="${ROOT}/scripts/ci/f13_role_conditional.py"
  if [[ -f "${ROOT}/scripts/ci/f13_role_conditional_successor.py" ]]; then
    qualifier="${ROOT}/scripts/ci/f13_role_conditional_successor.py"
  fi
  receipt="$(python3 "${qualifier}" --repo "${repo}" \
    --target "${target}" --prior-row="${prior_row}" --incoming-row="${incoming_row}" 2>/dev/null)" || return 1
  # Retain actual producer arguments and both original runtime flags before
  # normal replacement. The final map still retains the strongest trigger.
  printf 'prior\t%s\nincoming\t%s\n' "${prior_row}" "${incoming_row}" \
    >>"${TMP_DIR}/f13-role-conditional-history.tsv" || return 1
  printf '%s\n' "${receipt}" >>"${TMP_DIR}/f13-role-conditional.jsonl" || return 1
  printf 'CI_DIAG stage=f13-role-conditional qualification=%s\n' "${receipt}" >&2
}

f13_space_audit_qualified() {
  local repo="$1" map="$2" path="$3" incoming_row="$4" prior_row receipt target
  [[ "${repo}" == "${ROOT}" ]] || return 1
  [[ "$(awk -F '\t' -v p="${path}" '$1 == p { n++ } END { print n+0 }' "${map}")" == 1 ]] || return 1
  prior_row="$(awk -F '\t' -v p="${path}" '$1 == p { print $0 }' "${map}")" || return 1
  target="$(git -C "${ROOT}" rev-parse HEAD)" || return 1
  receipt="$(python3 "${ROOT}/scripts/ci/f13_space_audit.py" --repo "${repo}" --target "${target}" \
    --prior-row="${prior_row}" --incoming-row="${incoming_row}" 2>/dev/null)" || return 1
  printf 'prior\t%s\nincoming\t%s\n' "${prior_row}" "${incoming_row}" \
    >>"${TMP_DIR}/f13-space-audit-history.tsv" || return 1
  printf '%s\n' "${receipt}" >>"${TMP_DIR}/f13-space-audit.jsonl" || return 1
  printf 'CI_DIAG stage=f13-space-audit qualification=%s\n' "${receipt}" >&2
}

f13_guarded_merge_state_fingerprint() {
  local state_file
  if [[ -f "${ROOT}/scripts/ci/f13_role_conditional_successor.py" ]]; then
    git -C "${ROOT}" hash-object -- "${ROOT}/scripts/ci/f13_role_conditional_successor.py" || return 1
  fi
  for state_file in "${ROOT}/scripts/ci/f13_reviewed_test_merge.py" \
    "${ROOT}/scripts/ci/f13_historical_composition.py" "${TMP_DIR}/f13-historical-composition.jsonl" \
    "${ROOT}/scripts/ci/f13_role_conditional.py" "${TMP_DIR}/f13-role-conditional-history.tsv" \
    "${TMP_DIR}/f13-role-conditional.jsonl" \
    "${ROOT}/scripts/ci/f13_space_audit.py" "${TMP_DIR}/f13-space-audit-history.tsv" \
    "${TMP_DIR}/f13-space-audit.jsonl" \
    "${TMP_DIR}/f13-guarded-merge-history.tsv" "${TMP_DIR}/f13-reviewed-test-merges.jsonl"; do
    if [[ -f "${state_file}" ]]; then
      git -C "${ROOT}" hash-object -- "${state_file}" || return 1
    else
      printf '%s\n' absent
    fi
  done
}

f13_guarded_correction_qualified() {
  local repo="$1" base="$2" target="$3" path="$4" correction="$5" owner_map="$6" receipt
  f13_develop_aggregate_enabled "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_REF:-}" || return 1
  receipt="$(python3 "${ROOT}/scripts/ci/f13_guarded_correction.py" \
    --repo "${repo}" --base "${base}" --target "${target}" --path "${path}" \
    --correction "${correction}" --owner-map "${owner_map}" \
    --event "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" --ref "${GITHUB_REF:-}" 2>/dev/null)" || return 1
  printf '%s\n' "${receipt}" >>"${TMP_DIR}/f13-guarded-corrections.jsonl"
  printf 'CI_DIAG stage=f13-guarded-correction qualification=%s\n' "${receipt}" >&2
}

f13_f54_r23_132_owner_permission() {
  local repo="$1" path="$2" output="$3" incoming="$4" receipt
  [[ "${path}" == protos/voice/r23_contract_manifest.json ||
     "${path}" == protos/voice/r23_contract_test/r23_contract_test.go ]] || return 1
  [[ -n "${f13_f54_frame:-}" && "${f13_f54_operation:-}" == "${f13_f54_frame}/132437c43f6f0625fae8954542f8d9c2f6aa3037/2" ]] || return 1
  receipt="${TMP_DIR}/f54-r23-132-owner-permission.json"
  python3 "${f13_f54_observer}" 132-owner-permission "${repo}" "${f13_f54_trace_file}" \
    "${path}" "${output}" "${incoming}" >"${receipt}" || return 1
  f13_f54_trace_event r23_132_owner_permission "${repo}" "${path}" "${receipt}"
}

f13_f54_exact_J_owner_permission() {
  local repo="$1" path="$2" output="$3" incoming="$4" receipt
  [[ "${path}" == src/backend/space/internal/grpcsvc/audit.go &&
    -n "${f13_f54_frame:-}" && "${f13_f54_operation:-}" == "${f13_f54_frame}/3071ddad390443de4119c796cdad9899ff08f7fd/2" &&
    "${f13_f54_operation}" == "${f13_f54_J_operation:-}" && -n "${f13_f54_3071_capability_file:-}" ]] || return 1
  receipt="${TMP_DIR}/f54-exact-J-owner-permission.json"
  python3 "${f13_f54_observer}" 3071-J-owner-permission "${repo}" \
    "${f13_f54_3071_capability_file}" "${f13_f54_J_ledger}" "${f13_f54_J_ledger_sha}" \
    "${f13_f54_trace_file}" "${path}" "${output}" "${incoming}" >"${receipt}" || return 1
  f13_f54_trace_event exact_J_owner_permission "${repo}" "${path}" "${receipt}"
}

f13_f54_finite_J_owner_permission() {
  local repo="$1" path="$2" output="$3" incoming="$4" receipt attempt
  [[ -n "${f13_f54_frame:-}" && "${f13_f54_operation:-}" == "${f13_f54_frame}/3071ddad390443de4119c796cdad9899ff08f7fd/2" &&
     "${f13_f54_operation}" == "${f13_f54_J_operation:-}" && -n "${f13_f54_3071_capability_file:-}" ]] || return 1
  python3 "${f13_f54_observer}" finite-J-path "${path}" || return 1
  if [[ -z "${f13_finite_J_worker_pid:-}" ]]; then
    f13_finite_J_socket="${TMP_DIR}/finite-j.sock"
    f13_finite_J_session="$(python3 -c 'import secrets;print(secrets.token_hex(32))')" || return 1
    python3 "${f13_f54_observer}" finite-J-worker "${f13_finite_J_socket}" "${f13_finite_J_session}" \
      "${repo}" "${f13_f54_3071_capability_file}" "${f13_f54_J_ledger}" "${f13_f54_J_ledger_sha}" \
      "${f13_f54_trace_file}" "${output}" &
    f13_finite_J_worker_pid=$!
    # Whole historical reconstruction completes before readiness is published.
    for ((attempt=0; attempt<3600; attempt++)); do
      if [[ -f "${f13_finite_J_socket}.ready" ]]; then
        [[ "$(<"${f13_finite_J_socket}.ready")" == "${f13_finite_J_session}" ]] || return 1
        break
      fi
      kill -0 "${f13_finite_J_worker_pid}" 2>/dev/null || return 1
      sleep 1
    done
    [[ -f "${f13_finite_J_socket}.ready" ]] || return 1
  fi
  receipt="${TMP_DIR}/f54-finite-J-owner-permission.json"
  python3 "${f13_f54_observer}" finite-J-request "${f13_finite_J_socket}" "${f13_finite_J_session}" \
    permission "${path}" "${incoming}" >"${receipt}" || return 1
  f13_f54_trace_event finite_J_owner_permission "${repo}" "${path}" "${receipt}"
}

f13_f54_finite_J_finish() {
  [[ -n "${f13_finite_J_worker_pid:-}" && "$1" == src/backend/space/internal/lifecycleprincipal/interceptor.go ]] || return 0
  python3 "${f13_f54_observer}" finite-J-request "${f13_finite_J_socket}" "${f13_finite_J_session}" finish '' '' || return 1
  wait "${f13_finite_J_worker_pid}" || return 1
  f13_finite_J_worker_pid=''
}

f13_evaluate_guarded_owner_qualifiers() {
  local repo="$1" path="$2" output="$3"
  incremental_merge=false
  checkpoint_supersession_status=1
  previous_source_head="${owner_source_heads[${path}]:-}"
  previous_source_entry="${owner_source_entries[${path}]:-}"
  if [[ -n "${previous_source_head}" && -n "${previous_source_entry}" ]] && \
    git -C "${repo}" merge-base --is-ancestor "${previous_source_head}" "${nested_base}" 2>/dev/null; then
    base_entry="$(git -C "${repo}" ls-tree "${nested_base}" -- "${path}" 2>/dev/null)" || {
      f13_collect_failure "${diagnostic_file}" source_tree_entry
      return 1
    }
    [[ "${previous_source_entry}" == "${base_entry}" ]] && source_lineage=true
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true ]]; then
    [[ "${ordinary_guarded_merge}" == true ]] && incremental_merge=true
    if git -C "${repo}" merge-base --is-ancestor "${owners[${path}]}" "${source_base}" 2>/dev/null; then
      incremental_merge=true
    fi
    [[ "${source_lineage}" == true ]] && incremental_merge=true
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true && \
    "${incremental_merge}" != true ]]; then
    if game_checkpoint_duplicate_owner_supersession_allowed \
      "${repo}" "${path}" "${owners[${path}]}" "${previous_source_head}" \
      "${nested_owner_commit}" "${nested_head}" "${commit}" \
      "${checkpoint_merge_outcomes[${path}]:-unverified}"; then
      incremental_merge=true
      checkpoint_supersession_status=0
    else
      checkpoint_supersession_status=$?
      [[ "${checkpoint_supersession_status}" == 2 ]] && incremental_merge=true
    fi
  fi
  if [[ "${allow_direct_followup}" != true && "${is_direct}" == true ]] && \
    f13_guarded_correction_qualified "${repo}" "${base}" "${target}" "${path}" "${commit}" "${output}"; then
    guarded_correction_allowed=true
  fi
  if [[ "${guarded_correction_allowed}" != true ]] &&
    [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; then
    native_merge_outcome="${checkpoint_merge_outcomes[${path}]:-unverified}"
    [[ "${is_direct}" == true ]] && native_merge_outcome=direct
    if native_p3_duplicate_owner_qualified "${repo}" "${path}" \
      "${owners[${path}]}" "${previous_source_head}" "${nested_owner_commit}" \
      "${nested_head}" "${commit}" "${native_merge_outcome}"; then
      native_supersession_status=0
    else
      native_supersession_status=$?
    fi
    if [[ "${native_supersession_status}" == 0 || "${native_supersession_status}" == 2 ]]; then
      guarded_correction_allowed=true
      # This is the actual producer history, not replacement proof rows.
      awk -F '\t' -v candidate="${path}" '$1 == candidate' "${output}" \
        >>"${TMP_DIR}/f13-native-p3-history.tsv" || return 1
      printf 'supersession\t%s\t%s\t%s\t%s\t%s\n' "${path}" "${commit}" \
        "${nested_head}" "${native_merge_outcome}" "${native_supersession_status}" \
        >>"${TMP_DIR}/f13-native-p3-history.tsv" || return 1
      if [[ "${native_supersession_status}" == 0 ]]; then
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_trace_event native_rebind_gate "${repo}" "${path}" \
            "${native_supersession_status}" "${owners[${path}]}" "${previous_source_head}" \
            "${nested_owner_commit}" "${nested_head}" "${commit}" "${native_merge_outcome}" \
            "${native_p3_binding}" || return 1
        fi
        source_entry="$(git -C "${repo}" ls-tree "${commit}" -- ":(literal)${path}")" || return 1
        nested_owner_commit="${commit}"
        nested_base="${first_parent}"
        nested_head="${commit}"
        unset 'guarded_entries[$path]' 'guarded_entry_commits[$path]'
      fi
    fi
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true &&
    "${incremental_merge}" != true && "${guarded_correction_allowed}" != true ]] &&
    declare -F f13_role_conditional_qualified >/dev/null; then
    role_incoming_row="$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' \
      "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" \
      "${nested_head}" "${content_base}" "${source_base_candidates}")" || return 1
    if f13_role_conditional_qualified "${repo}" "${output}" "${path}" "${role_incoming_row}"; then
      role_duplicate_allowed=true
    fi
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true &&
    "${incremental_merge}" != true && "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true ]] &&
    declare -F f13_space_audit_qualified >/dev/null; then
    role_incoming_row="$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' \
      "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" \
      "${nested_head}" "${content_base}" "${source_base_candidates}")" || return 1
    if f13_space_audit_qualified "${repo}" "${output}" "${path}" "${role_incoming_row}"; then
      space_audit_allowed=true
    fi
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true && "${incremental_merge}" != true &&
    "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true && "${space_audit_allowed}" != true &&
    -n "${f13_f54_capability_file:-}" ]]; then
    if f13_f54_exact_supersession "${repo}" "${path}" "${output}" \
      "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" "${base}" "${target}"; then
      exact_unit_supersession=true
    fi
  fi
  r23_132_owner_permission=false
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true && "${incremental_merge}" != true &&
    "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true &&
    "${space_audit_allowed}" != true && "${exact_unit_supersession}" != true ]]; then
    if f13_f54_r23_132_owner_permission "${repo}" "${path}" "${output}" \
      "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")"; then
      r23_132_owner_permission=true
    fi
  fi
  exact_J_owner_permission=false
  finite_J_owner_permission=false
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true && "${incremental_merge}" != true &&
    "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true &&
    "${space_audit_allowed}" != true && "${exact_unit_supersession}" != true ]]; then
    if f13_f54_exact_J_owner_permission "${repo}" "${path}" "${output}" \
      "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")"; then
      exact_J_owner_permission=true
    fi
  fi
  if [[ "${allow_direct_followup}" == true && "${is_direct}" != true && "${incremental_merge}" != true &&
    "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true &&
    "${space_audit_allowed}" != true && "${exact_unit_supersession}" != true && "${exact_J_owner_permission}" != true ]]; then
    if f13_f54_finite_J_owner_permission "${repo}" "${path}" "${output}" \
      "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")"; then
      finite_J_owner_permission=true
    fi
  fi
  return 0
}

f13_guarded_owner_transition_allowed() {
  local guarded_correction_allowed="$1" role_duplicate_allowed="$2" space_audit_allowed="$3"
  local exact_unit_supersession="$4" allow_direct_followup="$5" is_direct="$6" incremental_merge="$7"
  local exact_J_owner_permission="${8:-false}"
  local finite_J_owner_permission="${9:-false}"
  local r23_132_owner_permission="${10:-false}"
  if [[ "${r23_132_owner_permission}" != true && "${finite_J_owner_permission}" != true && "${exact_J_owner_permission}" != true && "${guarded_correction_allowed}" != true && "${role_duplicate_allowed}" != true && "${space_audit_allowed}" != true && "${exact_unit_supersession}" != true && \
    ( "${allow_direct_followup}" != true || \
    ( "${is_direct}" != true && "${incremental_merge}" != true ) ) ]]; then
    return 1
  fi
  return 0
}

f13_collect_develop_contributions() {
  if [[ -n "${f13_f54_trace_file:-}" && "${f13_f54_running:-false}" != true ]]; then
    local f13_f54_parent="${f13_f54_frame:-}" f13_f54_parent_operation="${f13_f54_operation:-}"
    local f13_f54_frame='' f13_f54_operation=''
    local f13_f54_running=true f13_f54_status=0
    f13_f54_frame="$(f13_f54_trace_event frame_enter "$1" "${f13_f54_parent}" "$2" "$3" "${6:-false}" "${7:-false}" "$4" "${f13_f54_parent_operation}")" || return 1
    f13_collect_develop_contributions "$@" || f13_f54_status=$?
    f13_f54_trace_event frame_exit "$1" "${f13_f54_status}" "$4" "${5:-}" || return 1
    return "${f13_f54_status}"
  fi
  local f13_f54_running=false f13_f54_operation=''
  local repo="$1" base="$2" target="$3" output="$4" diagnostic_file="${5:-}"
  local allow_direct_followup="${6:-false}"
  local partition_source_history="${7:-false}"
  local commit parent_count first_parent source_head source_base delta first_parent_commits is_direct
  local source_parent_index contribution_id source_base_file
  local sync_boundary boundary_commit
  local path source_entry result_entry target_entry blob trigger incremental_merge checkpoint_owner_commit checkpoint_owner_row
  local previous_trigger content_base nested_map nested_row nested_trigger nested_base nested_head
  local previous_source_head previous_source_entry base_entry source_lineage source_base_candidates nested_owner_commit
  local game_snapshot_contribution_allowed merge_tree_status checkpoint_supersession_status sync_row_count guarded_correction_allowed native_supersession_status native_merge_outcome ordinary_guarded_merge role_duplicate_allowed role_incoming_row space_audit_allowed f13_f54_path_consumed
  local -a commit_line
  local -A owners=() owner_triggers=() owner_content_bases=() owner_source_heads=() owner_source_entries=() guarded_entries=() guarded_entry_commits=() checkpoint_merge_outcomes=() ordinary_merge_entries=()
  : >"${output}"
  git -C "${repo}" rev-parse --verify "${base}^{commit}" >/dev/null 2>&1 || {
    f13_collect_failure "${diagnostic_file}" base_object
    return 1
  }
  git -C "${repo}" rev-parse --verify "${target}^{commit}" >/dev/null 2>&1 || {
    f13_collect_failure "${diagnostic_file}" target_object
    return 1
  }
  git -C "${repo}" merge-base --is-ancestor "${base}" "${target}" 2>/dev/null || {
    f13_collect_failure "${diagnostic_file}" base_ancestry
    return 1
  }
  first_parent_commits="$(git -C "${repo}" rev-list --first-parent "${target}" 2>/dev/null)" || {
    f13_collect_failure "${diagnostic_file}" first_parent_range
    return 1
  }
  sync_boundary=""
  if ! grep -Fxq -- "${base}" <<<"${first_parent_commits}"; then
    while IFS= read -r boundary_commit; do
      [[ -n "${boundary_commit}" ]] || continue
      read -r -a commit_line <<<"$(git -C "${repo}" rev-list --parents -n 1 "${boundary_commit}" 2>/dev/null)"
      if ((${#commit_line[@]} == 3)) && [[ "${commit_line[2]}" == "${base}" ]]; then
        sync_boundary="${boundary_commit}"
        break
      fi
    done <<<"${first_parent_commits}"
  fi
  if [[ -z "${sync_boundary}" && "$(grep -Fx -- "${base}" <<<"${first_parent_commits}")" != "${base}" ]]; then
    # Default refusal and sync semantics stay unchanged. This typed author-only
    # branch leaves base/sync/state untouched and preserves the failed predicate.
    if [[ -z "${f13_f54_3071_capability_file:-}" ]] ||
      ! declare -F f13_f54_fixed3071_range_observe >/dev/null ||
      ! f13_f54_fixed3071_range_observe "${repo}" "${base}" "${target}" \
      "${allow_direct_followup}" "${partition_source_history}"; then
      f13_collect_failure "${diagnostic_file}" first_parent_range
      return 1
    fi
  fi

  while IFS= read -r commit; do
    [[ -n "${commit}" ]] || continue
    read -r -a commit_line <<<"$(git -C "${repo}" rev-list --parents -n 1 "${commit}" 2>/dev/null)"
    parent_count=$((${#commit_line[@]} - 1))
    ((parent_count >= 1)) || {
      f13_collect_failure "${diagnostic_file}" parent_count
      return 1
    }
    if ((parent_count == 2)) && [[ "${commit_line[2]}" == "${base}" ]]; then
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        : >"${TMP_DIR}/f54-sync-empty-delta"
        f13_f54_operation="$(f13_f54_trace_event operation_attempt "${repo}" "${commit}" 2 \
          "${commit_line[1]}" "${base}" sync)" || return 1
        f13_f54_operation="$(f13_f54_trace_event operation_begin "${repo}" "${commit}" 2 \
          "${commit_line[1]}" "${base}" "${base}" "${base}" sync "${TMP_DIR}/f54-sync-empty-delta")" || return 1
      fi
      # This merge only syncs the already-owned base into the source lane.
      # Keep the novel first-parent commits from base..target, but do not
      # re-attribute base-side history as a source contribution.
      f13_validate_sync_merge_resolution "${repo}" "${commit}" "${diagnostic_file}" || return 1
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event sync_gate_passed "${repo}" || return 1
      fi
      for path in "${!guarded_entries[@]}"; do
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_trace_event sync_gate_before "${repo}" "${path}" "${output}" || return 1
        fi
        [[ -n "${owners[${path}]+present}" &&
          -n "${owner_triggers[${path}]+present}" &&
          -n "${owner_content_bases[${path}]+present}" &&
          -n "${owner_source_heads[${path}]+present}" &&
          -n "${owner_source_entries[${path}]+present}" &&
          -n "${guarded_entry_commits[${path}]+present}" ]] || continue
        if { [[ "${game_checkpoint_aggregate_active}" == true ]] &&
          grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}"; } ||
          { [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; }; then
          continue
        fi
        if f13_sync_boundary_reconciles_guarded_path "${repo}" "${base}" "${commit}" \
          "${path}" "${owner_content_bases[${path}]}" "${guarded_entry_commits[${path}]}" \
          "${owner_source_heads[${path}]}" \
          "${guarded_entries[${path}]}" "${owner_source_entries[${path}]}"; then
          if [[ -n "${f13_f54_trace_file:-}" ]]; then
            f13_f54_trace_event disposition "${repo}" "${path}" verified_original_sync_reconciliation "${output}" || return 1
          fi
          sync_row_count="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { count++ } END { print count + 0 }' "${output}")"
          [[ "${sync_row_count}" == 1 ]] || continue
          awk -F '\t' -v candidate="${path}" '$1 != candidate' "${output}" >"${TMP_DIR}/f13-sync-map-rewrite" || {
            f13_collect_failure "${diagnostic_file}" sync_map_write
            return 1
          }
          cat "${TMP_DIR}/f13-sync-map-rewrite" >"${output}" || {
            f13_collect_failure "${diagnostic_file}" sync_map_write
            return 1
          }
          unset 'owners[$path]' 'owner_triggers[$path]' 'owner_content_bases[$path]'
          unset 'owner_source_heads[$path]' 'owner_source_entries[$path]'
          unset 'guarded_entries[$path]' 'guarded_entry_commits[$path]'
        fi
      done
      continue
    fi
    first_parent="${commit_line[1]}"
    # Every non-first parent is an independent source contribution. Do not
    # collapse an octopus into its aggregate delta or ignore later parents.
    for ((source_parent_index = parent_count == 1 ? 1 : 2; source_parent_index <= parent_count; source_parent_index++)); do
    contribution_id="${commit}-${source_parent_index}"
    is_direct=false
    if ((parent_count >= 2)); then
      source_head="${commit_line[${source_parent_index}]}"
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_operation="$(f13_f54_trace_event operation_attempt "${repo}" "${commit}" "${source_parent_index}" "${first_parent}" "${source_head}" merge)" || return 1
      fi
      source_base_file="${TMP_DIR}/f13-source-base-${contribution_id}"
      f13_resolve_source_base "${repo}" "${first_parent}" "${source_head}" "${source_base_file}" \
        "${commit}" "${f13_f54_capability_file:-}" "${f13_f54_observer:-}" "${f13_f54_3071_capability_file:-}" "${base}" "${target}" || {
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_observe_child "${repo}" "${commit}" "${first_parent}" "${source_head}" "${output}" || return 1
        fi
        printf 'CI_DIAG stage=f13-source-base repo=%s commit=%s first_parent=%s source_head=%s base=%s target=%s parent_index=%s\n' \
          "${repo}" "${commit}" "${first_parent}" "${source_head}" "${base}" "${target}" "${source_parent_index}" >&2
        f13_collect_failure "${diagnostic_file}" source_base
        return 1
      }
      source_base="$(<"${source_base_file}")"
      source_base_candidates="$(tr '\n' ',' <"${source_base_file}.candidates" | sed 's/,$//')"
      git -C "${repo}" diff --name-only "${source_base}" "${source_head}" 2>/dev/null >"${TMP_DIR}/f13-source-delta-${contribution_id}" || {
        f13_collect_failure "${diagnostic_file}" source_delta
        return 1
      }
    else
      is_direct=true
      source_head="${commit}"
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_operation="$(f13_f54_trace_event operation_attempt "${repo}" "${commit}" "${source_parent_index}" "${first_parent}" "${source_head}" direct)" || return 1
      fi
      source_base="${first_parent}"
      source_base_candidates="${source_base}"
      git -C "${repo}" diff --name-only "${source_base}" "${source_head}" 2>/dev/null >"${TMP_DIR}/f13-source-delta-${contribution_id}" || {
        f13_collect_failure "${diagnostic_file}" source_delta
        return 1
      }
    fi
    delta="${TMP_DIR}/f13-source-delta-${contribution_id}"
    if [[ -n "${f13_f54_trace_file:-}" ]]; then
      f13_f54_operation="$(f13_f54_trace_event operation_begin "${repo}" "${commit}" "${source_parent_index}" \
        "${first_parent}" "${source_base}" "${source_head}" "${source_base_candidates}" \
        "${is_direct}" "${delta}")" || return 1
    fi
    if [[ "${is_direct}" != true && -n "${f13_f54_capability_file:-}" && -s "${source_base_file:-}.capability" ]]; then
      f13_f54_execute_M_history "${repo}" "${commit}" "${first_parent}" "${source_head}" "${output}" "${source_base_file}.capability" "${base}" "${target}" || return 1
    fi
    if [[ "${is_direct}" != true && -n "${f13_f54_3071_capability_file:-}" && -s "${source_base_file:-}.3071-capability" ]]; then
      f13_f54_execute_3071_history "${repo}" "${commit}" "${first_parent}" "${source_head}" "${output}" "${source_base_file}.3071-capability" "${diagnostic_file}" || return 1
      # Prior aggregate internals are separate new evidence; the successful
      # child alone is insufficient. The full J constructor runs only after
      # all unchanged original pregates and actual J classification below.
      f13_f54_prepare_J_prior_history "${repo}" "${output}" || return 1
    fi

    game_snapshot_contribution_allowed=false
    if game_checkpoint_contribution_allowed "${repo}" "${commit}" "${source_base}" "${source_head}" "${delta}"; then
      game_snapshot_contribution_allowed=true
    fi

    nested_map=""
    if [[ "${partition_source_history}" == true && "${is_direct}" != true ]]; then
      nested_map="${TMP_DIR}/f13-nested-map-${contribution_id}"
      local nested_partition=false
      if [[ -n "${history_binding:-}" && "${commit}" == 69cc6054dd82406e1c3f7c925dbc3466d27ea7c5 ]]; then
        local history69_receipt="${TMP_DIR}/f54-exact69-history-capability.json"
        [[ ! -e "${history69_receipt}" ]] || return 1
        python3 "${f13_f54_observer}" 69-history-checkpoint "${repo}" "${f13_f54_trace_file}" >"${history69_receipt}" || return 1
        f13_f54_trace_event history_69_capability "${repo}" "${history69_receipt}" || return 1
        nested_partition=true
      fi
      f13_collect_develop_contributions "${repo}" "${source_base}" "${source_head}" \
        "${nested_map}" "${diagnostic_file}" true "${nested_partition}" || return 1
    fi

    : >"${TMP_DIR}/f13-unapproved-delta"
    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event path_gate_before "${repo}" "${path}" "${output}" || return 1
      fi
      source_entry="$(git -C "${repo}" ls-tree "${source_head}" -- "${path}" 2>/dev/null)" || {
        f13_collect_failure "${diagnostic_file}" source_tree_entry
        return 1
      }
      # Resolve this path's nested owner before any guarded merge check. The
      # defer decision must never inherit an empty or previous path's owner.
      checkpoint_owner_commit="${commit}"
      if [[ -n "${nested_map}" ]]; then
        checkpoint_owner_row="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { row = $0 } END { print row }' "${nested_map}")"
        if [[ -n "${checkpoint_owner_row}" ]]; then
          IFS=$'\t' read -r _ _ checkpoint_owner_commit _ _ _ _ _ <<<"${checkpoint_owner_row}"
        fi
      fi
      checkpoint_merge_outcomes["${path}"]=unverified
      unset 'ordinary_merge_entries[$path]'
      if ((parent_count >= 2)) && f13_path_is_guarded "${path}"; then
        result_entry="$(git -C "${repo}" ls-tree "${commit}" -- "${path}" 2>/dev/null)" || {
          f13_collect_failure "${diagnostic_file}" merge_tree_entry
          return 1
        }
        if { [[ "${game_checkpoint_aggregate_active}" == true ]] &&
          grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}"; } ||
          { [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; }; then
          if f13_checkpoint_merge_tree_entry_matches "${repo}" "${source_base}" \
            "${first_parent}" "${source_head}" "${path}" "${result_entry}" \
            "${commit}" "${f13_f54_capability_file:-}" "${f13_f54_observer:-}" "${f13_f54_3071_capability_file:-}" "${f13_f54_F_capability_file:-}" "${base}" "${target}"; then
            checkpoint_merge_outcomes["${path}"]=clean
          else
            merge_tree_status=$?
            if { [[ "${merge_tree_status}" == 2 && "${commit}" != "${game_checkpoint_aggregate_head}" ]] &&
              grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" &&
              git -C "${repo}" merge-base --is-ancestor "${commit}" \
                "${game_checkpoint_aggregate_head}" 2>/dev/null &&
              git -C "${repo}" merge-base --is-ancestor "${commit}" \
                "${game_checkpoint_snapshot_head}" 2>/dev/null &&
              git -C "${repo}" merge-base --is-ancestor "${checkpoint_owner_commit}" \
                "${game_checkpoint_snapshot_head}" 2>/dev/null; } ||
              { [[ "${merge_tree_status}" == 2 ]] &&
                [[ "${native_p3_active:-false}" == true ]] && native_p3_pre_h_owner "${repo}" "${path}" "${commit}" &&
                [[ "${native_p3_active:-false}" == true ]] && native_p3_pre_h_owner "${repo}" "${path}" "${checkpoint_owner_commit}"; }; then
              checkpoint_merge_outcomes["${path}"]=guarded_conflict
            else
              f13_collect_failure "${diagnostic_file}" merge_tree_mismatch
              return 1
            fi
          fi
        else
          if [[ "${source_entry}" != "${result_entry}" ]]; then
            # Ordinary guarded contributions may combine both parents. Prove
            # the exact Git three-way result; source-only equality would reject
            # clean disjoint edits. Conflicts/manual resolutions still refuse.
            if f13_checkpoint_merge_tree_entry_matches "${repo}" "${source_base}" \
              "${first_parent}" "${source_head}" "${path}" "${result_entry}" \
              "${commit}" "${f13_f54_capability_file:-}" "${f13_f54_observer:-}" "${f13_f54_3071_capability_file:-}" "${f13_f54_F_capability_file:-}"; then
              ordinary_merge_entries["${path}"]="${result_entry}"
            else
              merge_tree_status=$?
              if declare -F f13_historical_composition_qualified >/dev/null &&
                f13_historical_composition_qualified "${repo}" "${commit}" "${first_parent}" \
                  "${source_base}" "${source_head}" "${path}"; then
                ordinary_merge_entries["${path}"]="${result_entry}"
              elif [[ "${merge_tree_status}" == 2 ]] &&
                declare -F f13_reviewed_test_merge_qualified >/dev/null &&
                f13_reviewed_test_merge_qualified "${repo}" "${commit}" "${first_parent}" \
                  "${source_base}" "${source_head}" "${path}"; then
                ordinary_merge_entries["${path}"]="${result_entry}"
              else
                f13_collect_failure "${diagnostic_file}" merge_tree_mismatch
                return 1
              fi
            fi
          fi
        fi
      fi
      if [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}" &&
        ! native_p3_pre_h_owner "${repo}" "${path}" "${checkpoint_owner_commit}"; then
        f13_collect_failure "${diagnostic_file}" checkpoint_post_h_owner
        return 1
      fi
      if [[ "${game_checkpoint_aggregate_active}" == true ]] &&
        grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" &&
        ! git -C "${repo}" merge-base --is-ancestor "${checkpoint_owner_commit}" "${game_checkpoint_snapshot_head}" 2>/dev/null; then
        f13_collect_failure "${diagnostic_file}" checkpoint_post_h_owner
        return 1
      fi
      if game_checkpoint_aggregate_path_allowed "${repo}" "${checkpoint_owner_commit}" "${path}" "${source_entry}"; then
        unset 'guarded_entries[$path]'
        unset 'guarded_entry_commits[$path]'
        # When an already-emitted guarded row is later restored by a real
        # pre-H direct owner, keep processing this delta so the producer map
        # is rebound to that owner instead of retaining the temporary row.
        if [[ -z "${owners[${path}]+present}" || "${is_direct}" != true ]]; then
          if [[ -n "${f13_f54_trace_file:-}" ]]; then
            f13_f54_trace_event disposition "${repo}" "${path}" original_game_aggregate_path "${output}" || return 1
          fi
          continue
        fi
      fi
      if [[ "${game_snapshot_contribution_allowed}" == true ]] && game_checkpoint_domain_path "${path}"; then
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_trace_event disposition "${repo}" "${path}" original_game_snapshot_contribution "${output}" || return 1
        fi
        unset 'guarded_entries[$path]'
        unset 'guarded_entry_commits[$path]'
        continue
      fi
      if f13_path_is_guarded "${path}"; then
        guarded_entries["${path}"]="${ordinary_merge_entries[${path}]-${source_entry}}"
        guarded_entry_commits["${path}"]="${checkpoint_owner_commit}"
      fi
      blob="$(awk '{print $3}' <<<"${source_entry}")"
      if game_checkpoint_path_authorized_in_delta "${path}" "${delta}" "${blob}" || \
        be255_checkpoint_path_authorized_in_delta "${path}" "${delta}" "${blob}"; then
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_trace_event disposition "${repo}" "${path}" original_checkpoint_blob_delta "${output}" || return 1
        fi
        continue
      fi
      printf '%s\n' "${path}" >>"${TMP_DIR}/f13-unapproved-delta"
    done <"${delta}"

    trigger=false
    if voice_r22_runtime_changed <"${TMP_DIR}/f13-unapproved-delta"; then
      trigger=true
    fi
    if [[ -n "${f13_f54_trace_file:-}" ]]; then
      f13_f54_trace_event classification "${repo}" "${trigger}" "${TMP_DIR}/f13-unapproved-delta" || return 1
      f13_f54_prepare_range_I_ledger "${repo}" || return 1
      f13_f54_prepare_J_ledger "${repo}" || return 1
      f13_f54_prepare_M_ledger "${repo}" || return 1
    fi
    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      source_entry="$(git -C "${repo}" ls-tree "${source_head}" -- "${path}" 2>/dev/null)" || {
        f13_collect_failure "${diagnostic_file}" source_tree_entry
        return 1
      }
      content_base="${owner_content_bases[${path}]:-${first_parent}}"
      nested_base="${source_base}"
      nested_head="${source_head}"
      source_lineage=false
      f13_f54_path_consumed=false
      guarded_correction_allowed=false
      role_duplicate_allowed=false
      space_audit_allowed=false
      if [[ -n "${nested_map}" ]]; then
        nested_row="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { row = $0 } END { print row }' "${nested_map}")"
        [[ -n "${nested_row}" ]] || {
          printf 'CI_DIAG stage=f13-nested-source category=nested_contribution_missing commit=%s path=%s source_head=%s\n' \
            "${commit}" "${path}" "${source_head}" >&2
          f13_collect_failure "${diagnostic_file}" nested_contribution_missing
          return 1
        }
        IFS=$'\t' read -r _ nested_trigger nested_owner_commit _ nested_base nested_head _ _ <<<"${nested_row}"
        trigger="${nested_trigger}"
      else
        nested_owner_commit="${commit}"
      fi
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event row_before "${repo}" "${path}" "${output}" \
          "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" || return 1
      fi
      ordinary_guarded_merge=false
      if [[ -n "${ordinary_merge_entries[${path}]+present}" ]]; then
        ordinary_guarded_merge=true
        # Preserve the real incoming owner/source and the earlier path row in
        # a sidecar. The map still names the incoming delta, while its actual
        # integration commit owns the verified combined result/content history.
        if [[ "${repo}" == "${ROOT}" ]]; then
          awk -F '\t' -v candidate="${path}" '$1 == candidate {print "prior\t" $0}' "${output}" \
            >>"${TMP_DIR}/f13-guarded-merge-history.tsv" || return 1
          printf 'merge\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
            "${path}" "${commit}" "${first_parent}" "${source_base}" "${source_head}" \
            "${nested_owner_commit}" "${nested_base}" "${nested_head}" "${source_base_candidates}" \
            "${ordinary_merge_entries[${path}]%%$'\t'*}" \
            >>"${TMP_DIR}/f13-guarded-merge-history.tsv" || return 1
        fi
      fi
      native_supersession_status=1
      local exact_unit_supersession=false exact_J_owner_permission=false finite_J_owner_permission=false r23_132_owner_permission=false
      if [[ -n "${owners[${path}]+present}" ]] && f13_path_is_guarded "${path}"; then
        # A release PR may directly correct a guarded path imported by an
        # earlier merge, or re-import its exact descendant source in a later
        # merge. The latter is incremental only when the previous owner is an
        # ancestor of this merge's source base; independent merge owners remain
        # ambiguous and fail closed.
        f13_evaluate_guarded_owner_qualifiers "${repo}" "${path}" "${output}" || return 1
        if ! f13_guarded_owner_transition_allowed "${guarded_correction_allowed}" \
          "${role_duplicate_allowed}" "${space_audit_allowed}" "${exact_unit_supersession}" \
          "${allow_direct_followup}" "${is_direct}" "${incremental_merge}" "${exact_J_owner_permission}" "${finite_J_owner_permission}" "${r23_132_owner_permission}"; then
          f13_emit_guarded_failure_diagnostic duplicate_guarded_owner "${path}" \
            "${commit}" "${game_checkpoint_aggregate_head:-}" "${owners[${path}]:-}" \
            "${nested_head:-}" "${guarded_entries[${path}]:-}" "${source_entry}" \
            "incremental_or_qualified_supersession_required:${checkpoint_supersession_status}"
          f13_collect_failure "${diagnostic_file}" duplicate_guarded_owner
          return 1
        fi
        previous_trigger="${owner_triggers[${path}]:-false}"
        [[ "${previous_trigger}" == true && ( "${source_lineage}" == true || "${guarded_correction_allowed}" == true || "${ordinary_guarded_merge}" == true || "${role_duplicate_allowed}" == true || "${space_audit_allowed}" == true ) ]] && trigger=true
        if [[ "${checkpoint_supersession_status}" == 0 ]]; then
          unset 'guarded_entries[$path]'
          unset 'guarded_entry_commits[$path]'
          if [[ "${checkpoint_merge_outcomes[${path}]:-}" == guarded_conflict ]]; then
            # The exact path conflict was resolved to the immutable H entry.
            # Record the real integration commit and its result entry as the
            # owner; the conflicting nested parent is not the approved blob.
            result_entry="$(git -C "${repo}" ls-tree "${commit}" -- ":(literal)${path}" 2>/dev/null)" || {
              f13_collect_failure "${diagnostic_file}" merge_tree_entry
              return 1
            }
            source_entry="${result_entry}"
            nested_owner_commit="${commit}"
            nested_base="${source_base}"
            nested_head="${commit}"
          fi
        fi
        if [[ -n "${f13_f54_trace_file:-}" ]]; then
          f13_f54_consume_I_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" replace || return 1
          f13_f54_consume_J_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" replace || return 1
          f13_f54_consume_M_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" "${diagnostic_file}" replace || return 1
          f13_f54_path_consumed=true
          f13_f54_trace_event row_remove "${repo}" "${path}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" original_guarded_map_replacement || return 1
        fi
        awk -F '\t' -v candidate="${path}" '$1 != candidate' "${output}" >"${TMP_DIR}/f13-map-rewrite" || {
          f13_collect_failure "${diagnostic_file}" release_pr_map_write
          return 1
        }
        cat "${TMP_DIR}/f13-map-rewrite" >"${output}" || {
          f13_collect_failure "${diagnostic_file}" release_pr_map_write
          return 1
        }
      fi
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        if [[ "${f13_f54_path_consumed}" != true ]]; then
          f13_f54_consume_I_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" append || return 1
          f13_f54_consume_J_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" append || return 1
          f13_f54_consume_M_before_mutation "${repo}" "${path}" "${output}" \
            "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" "${diagnostic_file}" append || return 1
        fi
        f13_f54_trace_event row_after "${repo}" "${path}" \
          "$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}")" original_owner_and_row_assignment || return 1
      fi
      owners["${path}"]="${commit}"
      owner_triggers["${path}"]="${trigger}"
      owner_content_bases["${path}"]="${content_base}"
      owner_source_heads["${path}"]="${nested_head}"
      owner_source_entries["${path}"]="${source_entry}"
      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
        "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}" >>"${output}"
      if [[ -n "${f13_f54_trace_file:-}" ]]; then
        f13_f54_trace_event row_committed "${repo}" "${path}" "${output}" || return 1
        f13_f54_finite_J_finish "${path}" || return 1
      fi
      if [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; then
        printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
          "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}" \
          >>"${TMP_DIR}/f13-native-p3-history.tsv" || return 1
      fi
      if [[ "${guarded_correction_allowed}" == true ]]; then
        printf 'CI_DIAG stage=f13-guarded-correction owner_row=%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
          "${path}" "${trigger}" "${commit}" "${first_parent}" "${nested_base}" "${nested_head}" "${content_base}" "${source_base_candidates}" >&2
      fi
    done <"${delta}"
    done
    # One reviewed dc0 merge-authored test is absent from both source parents.
    # Reconstruct its complete source unit and actual prefix before adding a
    # distinct result-parent row. Arbitrary missing nested rows still reject.
    if [[ "${commit}" == dc0acdefec7e5c413988adf71f86d12bff4017bc &&
      "${capability_command:-}" == production-capabilities &&
      -n "${f13_f54_trace_file:-}" && "${base}" == 698d4f064f032ea74abf3ede2017100b26c8ee69 &&
      "${target}" == 53f75f74523735ac594e21582bea659f567a7288 ]]; then
      local dc0_spec="${TMP_DIR}/dc0-residual-spec.json" dc0_receipt="${TMP_DIR}/dc0-residual-receipt.json"
      local dc0_delta="${TMP_DIR}/dc0-residual-delta" dc0_status=0 dc0_trigger=false dc0_raw
      local dc0_context="${TMP_DIR}/dc0-residual-context.json"
      local dc0_first=6e0cd809884f9a2b80f33d00a85c55ff42077aab
      local dc0_base=d4abb829622e9bd91b26842b61522e6840840867
      path=src/backend/chat/internal/gisprincipal/method_union_test.go
      [[ -z "${owners[${path}]+present}" ]] || { f13_collect_failure "${diagnostic_file}" duplicate_guarded_owner; return 1; }
      f13_f54_manual_gate_context "${dc0_context}" || return 1
      python3 "${f13_f54_observer}" dc0-residual-checkpoint "${repo}" "${f13_f54_trace_file}" "${output}" begin "${dc0_context}" >"${dc0_spec}" || return 1
      f13_f54_operation="$(f13_f54_trace_event dc0_residual_begin "${repo}" "${dc0_spec}" "${output}")" || return 1
      f13_f54_trace_event path_gate_before "${repo}" "${path}" "${output}" || return 1
      # No checkpoint exemption is claimed: run the original classifier on the
      # complete singleton residual, retaining its actual result and taint.
      printf '%s\n' "${path}" >"${dc0_delta}" || return 1
      voice_r22_runtime_changed <"${dc0_delta}" || dc0_status=$?
      [[ "${dc0_status}" == 0 || "${dc0_status}" == 1 ]] || return 1
      [[ "${dc0_status}" != 0 ]] || dc0_trigger=true
      f13_f54_trace_event classification "${repo}" "${dc0_trigger}" "${dc0_delta}" "${dc0_status}" || return 1
      dc0_raw="$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${dc0_trigger}" "${commit}" \
        "${dc0_first}" "${dc0_first}" "${commit}" "${dc0_first}" "${dc0_base}")"
      f13_f54_trace_event row_before "${repo}" "${path}" "${output}" "${dc0_raw}" || return 1
      python3 "${f13_f54_observer}" dc0-residual-checkpoint "${repo}" "${f13_f54_trace_file}" "${output}" append >/dev/null || return 1
      f13_f54_trace_event row_after "${repo}" "${path}" "${dc0_raw}" actual_dc0_result_parent_assignment || return 1
      source_entry="$(git -C "${repo}" ls-tree "${commit}" -- ":(literal)${path}")" || return 1
      owners["${path}"]="${commit}"; owner_triggers["${path}"]="${dc0_trigger}"
      owner_content_bases["${path}"]="${dc0_first}"; owner_source_heads["${path}"]="${commit}"
      owner_source_entries["${path}"]="${source_entry}"
      if f13_path_is_guarded "${path}"; then guarded_entries["${path}"]="${source_entry}"; guarded_entry_commits["${path}"]="${commit}"; fi
      printf '%s\n' "${dc0_raw}" >>"${output}" || return 1
      f13_f54_trace_event row_committed "${repo}" "${path}" "${output}" || return 1
      python3 "${f13_f54_observer}" dc0-residual-checkpoint "${repo}" "${f13_f54_trace_file}" "${output}" end >"${dc0_receipt}" || return 1
      f13_f54_trace_event dc0_residual_end "${repo}" "${dc0_receipt}" "${output}" || return 1
    fi
    # Exact F authored merge-result residual in this SAME engine state.
    # Original parent operations above remain unchanged; no nested-map fallback.
    if [[ "${commit}" == fa1528a7c232a644218edf49811e2bb2e373105f &&
      -n "${f13_f54_trace_file:-}" && -n "${f13_f54_F_capability_file:-}" &&
      "${base}" == d4abb829622e9bd91b26842b61522e6840840867 &&
      "${target}" == 909090a34743902799306974d6f3ebabe3c02f10 ]]; then
      local f_manual_spec="${TMP_DIR}/manual-F-${f13_f54_frame}.json"
      local f_manual_receipt="${TMP_DIR}/manual-F-${f13_f54_frame}-completed.json"
      local f_manual_delta="${TMP_DIR}/manual-F-${f13_f54_frame}-delta"
      local f_manual_unapproved="${TMP_DIR}/manual-F-${f13_f54_frame}-unapproved"
      local f_manual_original_status f_manual_native_status f_manual_game_status f_manual_eligibility f_manual_kind f_manual_raw
      local f_manual_context="${TMP_DIR}/manual-F-${f13_f54_frame}-gate-context.json"
      f13_f54_manual_gate_context "${f_manual_context}" || return 1
      python3 "${f13_f54_observer}" manual-F-spec "${repo}" "${f13_f54_F_capability_file}" \
        "${f13_f54_trace_file}" "${output}" "${f_manual_context}" >"${f_manual_spec}" || return 1
      git -C "${repo}" diff --name-only ab7a4a4d4ed4cbf9663b4a752b95c9aeb21eac3f "${commit}" >"${f_manual_delta}" || return 1
      f13_f54_operation="$(f13_f54_trace_event manual_F_begin "${repo}" "${f_manual_spec}" "${output}")" || return 1
      : >"${f_manual_unapproved}"
      while IFS= read -r path; do
        source_entry="$(git -C "${repo}" ls-tree "${commit}" -- "${path}")" || return 1
        # Preserve the actual ordinary composition result; the independent
        # complete corrected-F capability above qualifies the reviewed residual.
        f_manual_original_status=0
        f13_checkpoint_merge_tree_entry_matches "${repo}" c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c \
          40afa80e6d1ca7faebf528b7cb78f32cdf707e5d a80f649cbe0a03671c25531ace4ac1bd6bf54756 \
          "${path}" "${source_entry}" "${commit}" '' "${f13_f54_observer}" '' '' || f_manual_original_status=$?
        f_manual_native_status=0; f_manual_game_status=0
        if [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; then
          native_p3_pre_h_owner "${repo}" "${path}" "${commit}" || f_manual_native_status=$?
        fi
        if [[ "${game_checkpoint_aggregate_active}" == true ]] && grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}"; then
          git -C "${repo}" merge-base --is-ancestor "${commit}" "${game_checkpoint_snapshot_head}" || f_manual_game_status=$?
        fi
        [[ "${f_manual_native_status}" == 0 && "${f_manual_game_status}" == 0 ]] || {
          f13_collect_failure "${diagnostic_file}" checkpoint_post_h_owner; return 1;
        }
        f_manual_eligibility=unapproved
        if game_checkpoint_aggregate_path_allowed "${repo}" "${commit}" "${path}" "${source_entry}"; then
          f_manual_eligibility=game_aggregate
        elif game_checkpoint_contribution_allowed "${repo}" "${commit}" \
          c08cf1ed3a18e54e86541c54fa731fc9b24e1a8c a80f649cbe0a03671c25531ace4ac1bd6bf54756 "${f_manual_delta}" &&
          game_checkpoint_domain_path "${path}"; then
          f_manual_eligibility=game_snapshot
        elif game_checkpoint_path_authorized_in_delta "${path}" "${f_manual_delta}" "$(awk '{print $3}' <<<"${source_entry}")" ||
          be255_checkpoint_path_authorized_in_delta "${path}" "${f_manual_delta}" "$(awk '{print $3}' <<<"${source_entry}")"; then
          f_manual_eligibility=checkpoint_blob
        fi
        f13_f54_trace_event manual_F_gate "${repo}" "${path}" "${output}" "${f_manual_original_status}" \
          "${f_manual_native_status}" "${f_manual_game_status}" "${f_manual_eligibility}" || return 1
        [[ "${f_manual_eligibility}" != unapproved ]] || printf '%s\n' "${path}" >>"${f_manual_unapproved}"
      done <"${f_manual_delta}"
      local f_manual_trigger=false f_manual_classifier_status=0
      voice_r22_runtime_changed <"${f_manual_unapproved}" || f_manual_classifier_status=$?
      [[ "${f_manual_classifier_status}" == 0 || "${f_manual_classifier_status}" == 1 ]] || {
        f13_collect_failure "${diagnostic_file}" manual_result_classifier_error; return 1;
      }
      [[ "${f_manual_classifier_status}" != 0 ]] || f_manual_trigger=true
      f13_f54_trace_event classification "${repo}" "${f_manual_trigger}" "${f_manual_unapproved}" "${f_manual_classifier_status}" || return 1
      while IFS= read -r path; do
        f_manual_kind="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["dispositions"][sys.argv[2]]["type"])' "${f_manual_spec}" "${path}")" || return 1
        if [[ "${f_manual_kind}" != NEW_RESULT_PARENT_ROW ]]; then
          f13_f54_trace_event manual_F_association "${repo}" "${path}" "${output}" "${f_manual_kind}" || return 1
          continue
        fi
        [[ -z "${owners[${path}]+present}" ]] || { f13_collect_failure "${diagnostic_file}" duplicate_guarded_owner; return 1; }
        f_manual_raw="$(printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s' "${path}" "${f_manual_trigger}" "${commit}" \
          40afa80e6d1ca7faebf528b7cb78f32cdf707e5d 40afa80e6d1ca7faebf528b7cb78f32cdf707e5d "${commit}" \
          40afa80e6d1ca7faebf528b7cb78f32cdf707e5d 40afa80e6d1ca7faebf528b7cb78f32cdf707e5d)"
        f13_f54_trace_event row_before "${repo}" "${path}" "${output}" "${f_manual_raw}" || return 1
        python3 "${f13_f54_observer}" manual-F-append-check "${repo}" "${f13_f54_trace_file}" \
          "${f13_f54_operation}" "${path}" "${output}" || return 1
        f13_f54_trace_event row_after "${repo}" "${path}" "${f_manual_raw}" actual_typed_result_parent_assignment || return 1
        owners["${path}"]="${commit}"; owner_triggers["${path}"]="${f_manual_trigger}"
        owner_content_bases["${path}"]=40afa80e6d1ca7faebf528b7cb78f32cdf707e5d
        owner_source_heads["${path}"]="${commit}"
        source_entry="$(git -C "${repo}" ls-tree "${commit}" -- "${path}")" || return 1
        owner_source_entries["${path}"]="${source_entry}"
        if f13_path_is_guarded "${path}"; then guarded_entries["${path}"]="${source_entry}"; guarded_entry_commits["${path}"]="${commit}"; fi
        printf '%s\n' "${f_manual_raw}" >>"${output}"
        f13_f54_trace_event row_committed "${repo}" "${path}" "${output}" || return 1
        if [[ "${native_p3_active:-false}" == true ]] && native_p3_path_bound "${repo}" "${path}"; then
          printf '%s\n' "${f_manual_raw}" >>"${TMP_DIR}/f13-native-p3-history.tsv" || return 1
        fi
      done <"${f_manual_delta}"
      python3 "${f13_f54_observer}" manual-F-end "${repo}" "${f13_f54_trace_file}" "${f13_f54_operation}" >"${f_manual_receipt}" || return 1
      f13_f54_trace_event manual_F_end "${repo}" "${f_manual_receipt}" "${output}" || return 1
    fi
    if ((parent_count > 2)); then
      # A multi-source merge may not invent a guarded result path that no
      # source parent contributed. Full-entry checks above cover owned paths.
      git -C "${repo}" diff --name-only "${first_parent}" "${commit}" \
        >"${TMP_DIR}/f13-octopus-result-${commit}" || {
        f13_collect_failure "${diagnostic_file}" merge_tree_entry
        return 1
      }
      while IFS= read -r path; do
        [[ -n "${path}" ]] || continue
        if f13_path_is_guarded "${path}" && [[ "${owners[${path}]:-}" != "${commit}" ]]; then
          f13_collect_failure "${diagnostic_file}" merge_tree_mismatch
          return 1
        fi
      done <"${TMP_DIR}/f13-octopus-result-${commit}"
    fi
  done < <(git -C "${repo}" rev-list --first-parent --reverse "${base}..${target}" 2>/dev/null)

  # Historical source versions may be superseded by later direct commits in
  # this PR. The final owner must still retain the exact guarded tree entry.
  f13_f54_operation=''
  for path in "${!guarded_entries[@]}"; do
    if [[ -n "${f13_f54_trace_file:-}" ]]; then
      f13_f54_trace_event final_gate_before "${repo}" "${path}" "${output}" || return 1
    fi
    target_entry="$(git -C "${repo}" ls-tree "${target}" -- "${path}" 2>/dev/null)" || {
      f13_collect_failure "${diagnostic_file}" target_tree_entry
      return 1
    }
    if [[ "${native_p3_active:-false}" == true && "${target}" != "${native_p3_source}" ]] &&
      native_p3_pre_h_owner "${repo}" "${path}" "${target}" &&
      native_p3_pre_h_owner "${repo}" "${path}" "${guarded_entry_commits[${path}]:-}"; then
      # Recursive historical source: retain its row/taint until the final
      # complete source boundary and map audit require the exact H entry.
      continue
    fi
    # Recursive source collectors can see a valid pre-H owner before a later
    # sync merge carries the path to H. Defer only this intermediate P check;
    # retain the owner row so the aggregate A boundary must still prove H.
    if [[ "${game_checkpoint_aggregate_active}" == true &&
      "${target}" != "${game_checkpoint_aggregate_head}" &&
      -n "${guarded_entry_commits[${path}]:-}" ]] &&
      grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" &&
      git -C "${repo}" merge-base --is-ancestor "${target}" \
        "${game_checkpoint_aggregate_head}" 2>/dev/null &&
      git -C "${repo}" merge-base --is-ancestor "${guarded_entry_commits[${path}]}" \
        "${game_checkpoint_snapshot_head}" 2>/dev/null; then
      continue
    fi
    if ! f13_guarded_entry_matches_final_target \
      "${guarded_entries[${path}]}" "${target_entry}"; then
      # Reviewed terminal ABSENT is distinct from a historical row or a
      # migration alias. This complete-unit consumer leaves map/owners,
      # content and raw flags unchanged and emits the actual typed seal.
      if [[ "${path}" == src/backend/voice/internal/store/space_lifecycle_test.go &&
        "${target}" == 909090a34743902799306974d6f3ebabe3c02f10 &&
        -n "${f13_f54_F_capability_file:-}" && -n "${f13_f54_trace_file:-}" &&
        "${f13_f54_observer:-}" == "${ROOT}/scripts/ci/f13_f54_history.py" ]]; then
        f13_f54_trace_event reviewed_F_deletion_guard "${repo}" "${path}" "${output}" "${target}" \
          "${guarded_entries[${path}]}" "${guarded_entry_commits[${path}]:-}" \
          "${owner_source_heads[${path}]:-}" "${target_entry}" || return 1
        if python3 "${f13_f54_observer}" reviewed-F-deletion "${repo}" "${f13_f54_F_capability_file}" \
          "${f13_f54_trace_file}" "${output}" "${f13_f54_F_deletion_ruling}" \
          >"${TMP_DIR}/reviewed-F-terminal-deletion.json"; then
          f13_f54_trace_event reviewed_F_deletion "${repo}" \
            "${TMP_DIR}/reviewed-F-terminal-deletion.json" "${output}" || return 1
          continue
        fi
      fi
      # The only approved non-P retirement is the versioned, complete
      # MatchSquad migration renumber recorded by R2. The helper validates the
      # exact source owner, integration, destination P entry, and competing
      # baseline migration; then remove exactly its stale source row.
      if f13_migration_alias_reconciles_guarded_path "${repo}" "${base}" \
        "${target}" "${path}" "${guarded_entry_commits[${path}]:-}" \
        "${owner_content_bases[${path}]:-}" "${owner_source_heads[${path}]:-}" \
        "${guarded_entries[${path}]}" "${owner_source_entries[${path}]:-}" "${output}"; then
        sync_row_count="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { count++ } END { print count + 0 }' "${output}")"
        [[ "${sync_row_count}" == 1 ]] || {
          f13_collect_failure "${diagnostic_file}" migration_alias_row_ambiguous
          return 1
        }
        awk -F '\t' -v candidate="${path}" '$1 != candidate' "${output}" >"${TMP_DIR}/f13-migration-alias-map-rewrite" || {
          f13_collect_failure "${diagnostic_file}" migration_alias_map_write
          return 1
        }
        cat "${TMP_DIR}/f13-migration-alias-map-rewrite" >"${output}" || {
          f13_collect_failure "${diagnostic_file}" migration_alias_map_write
          return 1
        }
        unset 'owners[$path]' 'owner_triggers[$path]' 'owner_content_bases[$path]'
        unset 'owner_source_heads[$path]' 'owner_source_entries[$path]'
        unset 'guarded_entries[$path]' 'guarded_entry_commits[$path]'
        continue
      fi
      f13_emit_guarded_failure_diagnostic guarded_target_mismatch "${path}" \
        "${target}" "${game_checkpoint_aggregate_head:-}" \
        "${guarded_entry_commits[${path}]:-}" "${owner_source_heads[${path}]:-}" \
        "${guarded_entries[${path}]}" "${target_entry}" "stored_entry_must_equal_final_target"
      f13_collect_failure "${diagnostic_file}" guarded_target_mismatch
      return 1
    fi
  done
}

# A release PR is GitHub's synthetic merge of its exact base and head. Partition
# the source head using the existing contribution collector, then verify the
# synthetic merge retained the source-side path set and every full tree entry.
f13_collect_master_release_pr_contributions() {
  local repo="$1" base="$2" target="$3" output="$4" diagnostic_file="${5:-}"
  local history_binding="${6:-}" collection_base
  local target_line base_commit target_first_parent source_head
  local source_delta="${output}.source-delta" result_delta="${output}.result-delta"
  local source_map="${output}.source-map" path source_entry result_entry
  local aggregate_paths="${output}.game-snapshot-paths" approval_map="${output}.game-snapshot-approvals"
  local alias_map="${output}.game-snapshot-alias-approvals" row row_count row_hash
  local -a target_line_fields
  : >"${output}"
  : >"${approval_map}"
  : >"${alias_map}"
  : >"${aggregate_paths}"
  game_checkpoint_aggregate_active=false
  game_checkpoint_aggregate_base=''
  game_checkpoint_aggregate_head=''
  game_checkpoint_aggregate_target=''
  game_checkpoint_aggregate_paths=''
  game_checkpoint_aggregate_map=''
  game_checkpoint_aggregate_record=''
  game_checkpoint_aggregate_alias_map="${alias_map}"

  [[ -n "${base}" ]] || {
    f13_collect_failure "${diagnostic_file}" release_pr_missing_base
    return 1
  }
  git -C "${repo}" rev-parse --verify "${base}^{commit}" >/dev/null 2>&1 || {
    f13_collect_failure "${diagnostic_file}" base_object
    return 1
  }
  git -C "${repo}" rev-parse --verify "${target}^{commit}" >/dev/null 2>&1 || {
    f13_collect_failure "${diagnostic_file}" target_object
    return 1
  }
  base_commit="$(git -C "${repo}" rev-parse --verify "${base}^{commit}" 2>/dev/null)" || {
    f13_collect_failure "${diagnostic_file}" base_object
    return 1
  }
  target_line="$(git -C "${repo}" rev-list --parents -n 1 "${target}" 2>/dev/null)" || {
    f13_collect_failure "${diagnostic_file}" release_pr_shape
    return 1
  }
  read -r -a target_line_fields <<<"${target_line}"
  ((${#target_line_fields[@]} == 3)) || {
    f13_collect_failure "${diagnostic_file}" release_pr_shape
    return 1
  }
  target_first_parent="${target_line_fields[1]}"
  source_head="${target_line_fields[2]}"
  [[ "${target_first_parent}" == "${base_commit}" ]] || {
    f13_collect_failure "${diagnostic_file}" release_pr_base_parent
    return 1
  }
  collection_base="${base_commit}"
  if [[ -n "${history_binding}" ]]; then
    [[ "$(git -C "${repo}" rev-parse --verify "${target}^{commit}")" == "$(git -C "${repo}" rev-parse --verify HEAD)" ]] || {
      f13_collect_failure "${diagnostic_file}" release_history_target
      return 1
    }
    collection_base="$(python3 "${ROOT}/scripts/ci/f13_release_source.py" collection-base "${repo}" "${history_binding}")" || {
      f13_collect_failure "${diagnostic_file}" release_history_authority
      return 1
    }
    [[ "${collection_base}" =~ ^[0-9a-f]{40}$ ]] && git -C "${repo}" merge-base --is-ancestor "${collection_base}" "${base_commit}" || {
      f13_collect_failure "${diagnostic_file}" release_history_base
      return 1
    }
  fi

  git -C "${repo}" diff --name-only "${base_commit}" "${source_head}" 2>/dev/null >"${source_delta}" || {
    f13_collect_failure "${diagnostic_file}" release_pr_source_delta
    return 1
  }
  git -C "${repo}" diff --name-only "${base_commit}" "${target}" 2>/dev/null >"${result_delta}" || {
    f13_collect_failure "${diagnostic_file}" release_pr_result_delta
    return 1
  }
  cmp -s "${source_delta}" "${result_delta}" || {
    f13_collect_failure "${diagnostic_file}" release_pr_delta_mismatch
    return 1
  }
  while IFS= read -r path; do
    [[ -n "${path}" ]] || continue
    source_entry="$(git -C "${repo}" ls-tree "${source_head}" -- "${path}" 2>/dev/null)" || {
      f13_collect_failure "${diagnostic_file}" source_tree_entry
      return 1
    }
    result_entry="$(git -C "${repo}" ls-tree "${target}" -- "${path}" 2>/dev/null)" || {
      f13_collect_failure "${diagnostic_file}" merge_tree_entry
      return 1
    }
    [[ "${source_entry}" == "${result_entry}" ]] || {
      f13_collect_failure "${diagnostic_file}" release_pr_merge_tree_mismatch
      return 1
    }
  done <"${source_delta}"
  if game_checkpoint_aggregate_authorized "${repo}" "${base_commit}" "${source_head}" \
    "${target}" "${source_delta}" "${aggregate_paths}"; then
    game_checkpoint_aggregate_active=true
    game_checkpoint_aggregate_base="${base_commit}"
    game_checkpoint_aggregate_head="${source_head}"
    game_checkpoint_aggregate_target="${target}"
    game_checkpoint_aggregate_paths="${aggregate_paths}"
    game_checkpoint_aggregate_record="${game_checkpoint_snapshot_record}"
  fi
  f13_collect_develop_contributions "${repo}" "${collection_base}" "${source_head}" \
    "${source_map}" "${diagnostic_file}" true true || {
    game_checkpoint_aggregate_active=false
    return 1
  }
  game_checkpoint_aggregate_active=false
  cat "${source_map}" >"${output}" || {
    f13_collect_failure "${diagnostic_file}" release_pr_map_write
    return 1
  }
  game_checkpoint_aggregate_map="${output}"
  if [[ -s "${aggregate_paths}" ]]; then
    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      row_count="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { count++ } END { print count+0 }' "${output}")"
      f13_release_projection_path_accounted "${repo}" "${output}" "${path}" || {
        f13_collect_failure "${diagnostic_file}" duplicate_guarded_owner
        : >"${approval_map}"
        return 1
      }
      [[ "${row_count}" == 0 ]] && continue
      row="$(awk -F '\t' -v candidate="${path}" '$1 == candidate { value=$0 } END { print value }' "${output}")"
      row_hash="$(printf '%s\n' "${row}" | git -C "${repo}" hash-object --stdin 2>/dev/null)" || {
        f13_collect_failure "${diagnostic_file}" release_pr_map_write
        : >"${approval_map}"
        return 1
      }
      printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
        "${path}" "${row_hash}" "${base_commit}" "${source_head}" "${target}" \
        "${game_checkpoint_snapshot_base}" "${game_checkpoint_snapshot_head}" \
        "${game_checkpoint_snapshot_record}" >>"${approval_map}" || {
        f13_collect_failure "${diagnostic_file}" release_pr_map_write
        : >"${approval_map}"
        return 1
      }
    done <"${aggregate_paths}"
    game_checkpoint_aggregate_base="${base_commit}"
    game_checkpoint_aggregate_head="${source_head}"
    game_checkpoint_aggregate_target="${target}"
    game_checkpoint_aggregate_paths="${aggregate_paths}"
    game_checkpoint_aggregate_map="${output}"
    game_checkpoint_aggregate_alias_map="${alias_map}"
    game_checkpoint_aggregate_record="${game_checkpoint_snapshot_record}"
  fi
}

# Share the full push-event gate with producer-shaped fixtures so master-push
# provenance is tested through the same event-before and PR-merge validation.
f13_collect_master_push_contributions() {
  local repo="$1" event_name="$2" event_ref="$3" event_before="$4"
  local resolved_base="$5" target="$6" output="$7" diagnostic_file="${8:-}" history_binding="${9:-}"
  f13_master_merge_push_enabled "${event_name}" "${event_ref}" || {
    f13_collect_failure "${diagnostic_file}" push_event
    return 1
  }
  f13_master_push_before_matches "${event_before}" "${resolved_base}" || {
    f13_collect_failure "${diagnostic_file}" push_before_mismatch
    return 1
  }
  f13_collect_master_release_pr_contributions \
    "${repo}" "${event_before}" "${target}" "${output}" "${diagnostic_file}" "${history_binding}"
}

 f13_base_origin_class() {
  if [[ -n "${VOICE_R22_BASE_SHA:-}" ]]; then
    printf '%s\n' push-before
  else
    printf '%s\n' fallback
  fi
}

f13_fixture_repo="${TMP_DIR}/f13-contribution-fixture"
mkdir -p "${f13_fixture_repo}"
git -C "${f13_fixture_repo}" init -q
git -C "${f13_fixture_repo}" config user.name 'Voice CI fixture'
git -C "${f13_fixture_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_fixture_repo}" config commit.gpgsign false
printf '%s\n' 'fixture base' >"${f13_fixture_repo}/README.md"
git -C "${f13_fixture_repo}" add README.md
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture base'
f13_fixture_base="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
git -C "${f13_fixture_repo}" checkout -qb f13-r22-source
mkdir -p "${f13_fixture_repo}/src/backend/voice"
printf '%s\n' 'package main' >"${f13_fixture_repo}/src/backend/voice/main.go"
git -C "${f13_fixture_repo}" add src/backend/voice/main.go
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture R22 contribution'
git -C "${f13_fixture_repo}" checkout -qb f13-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture merge R22' f13-r22-source
git -C "${f13_fixture_repo}" checkout -qb f13-bot-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/protos/voice/bot/v1"
printf '%s\n' 'syntax = "proto3";' >"${f13_fixture_repo}/protos/voice/bot/v1/bot.proto"
git -C "${f13_fixture_repo}" add protos/voice/bot/v1/bot.proto
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture unrelated Bot contribution'
git -C "${f13_fixture_repo}" checkout -q f13-integration
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture merge Bot' f13-bot-source
f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-fixture-map" || {
  printf '%s\n' 'F13 oracle bug: separate contribution fixture was rejected' >&2
  exit 2
}
grep -Fqx $'src/backend/voice/main.go\ttrue' <(cut -f1-2 "${TMP_DIR}/f13-fixture-map") || {
  printf '%s\n' 'F13 oracle bug: R22 contribution lost its own runtime trigger' >&2
  exit 2
}

f13_make_nested_crisscross_fixture() {
  local repo="$1" label="$2" guarded="$3" base left right left_merge right_merge first_bases
  local nested_base first_side second_side source tree release
  mkdir -p "${repo}"
  git -C "${repo}" init -q
  git -C "${repo}" config user.name 'Voice CI fixture'
  git -C "${repo}" config user.email 'voice-ci@example.invalid'
  git -C "${repo}" config commit.gpgsign false
  printf '%s\n' base >"${repo}/README.md"
  git -C "${repo}" add README.md
  git -C "${repo}" commit -qm "${label} base"
  base="$(git -C "${repo}" rev-parse HEAD)"

  git -C "${repo}" checkout -qb "${label}-left" "${base}"
  printf '%s\n' left >"${repo}/left.txt"
  git -C "${repo}" add left.txt
  git -C "${repo}" commit -qm "${label} left contribution"
  left="$(git -C "${repo}" rev-parse HEAD)"
  git -C "${repo}" checkout -qb "${label}-right" "${base}"
  printf '%s\n' right >"${repo}/right.txt"
  git -C "${repo}" add right.txt
  git -C "${repo}" commit -qm "${label} right contribution"
  right="$(git -C "${repo}" rev-parse HEAD)"

  git -C "${repo}" checkout -q "${label}-left"
  git -C "${repo}" merge --no-ff -qm "${label} left sync" "${label}-right"
  left_merge="$(git -C "${repo}" rev-parse HEAD)"
  git -C "${repo}" checkout -q "${label}-right"
  git -C "${repo}" merge --no-ff -qm "${label} right sync" "${label}-left^"
  right_merge="$(git -C "${repo}" rev-parse HEAD)"
  git -C "${repo}" checkout -q "${label}-left"
  git -C "${repo}" merge --no-ff -qm "${label} first cross merge" "${right_merge}"
  nested_base="$(git -C "${repo}" rev-parse HEAD)"
  first_bases="$(git -C "${repo}" merge-base --all "${left_merge}" "${right_merge}")"
  [[ "$(wc -l <<<"${first_bases}" | tr -d '[:space:]')" == 2 ]] || {
    printf '%s\n' "F13 fixture oracle bug: ${label} first source pair is not criss-cross" >&2
    exit 2
  }

  git -C "${repo}" checkout -qb "${label}-first" "${nested_base}"
  printf '%s\n' first >"${repo}/first.txt"
  git -C "${repo}" add first.txt
  git -C "${repo}" commit -qm "${label} first nested contribution"
  first_side="$(git -C "${repo}" rev-parse HEAD)"
  git -C "${repo}" checkout -qb "${label}-second" "${nested_base}"
  printf '%s\n' second >"${repo}/second.txt"
  git -C "${repo}" add second.txt
  if [[ "${guarded}" == true ]]; then
    mkdir -p "${repo}/src/backend/voice"
    printf '%s\n' 'package main' >"${repo}/src/backend/voice/main.go"
    git -C "${repo}" add src/backend/voice/main.go
  fi
  git -C "${repo}" commit -qm "${label} second nested contribution"
  second_side="$(git -C "${repo}" rev-parse HEAD)"

  git -C "${repo}" checkout -q "${label}-first"
  git -C "${repo}" merge --no-ff -qm "${label} first nested sync" "${label}-second"
  git -C "${repo}" checkout -q "${label}-second"
  git -C "${repo}" merge --no-ff -qm "${label} second nested sync" "${first_side}"
  git -C "${repo}" checkout -q "${label}-first"
  git -C "${repo}" merge --no-ff -qm "${label} nested criss-cross integration" "${label}-second"
  source="$(git -C "${repo}" rev-parse HEAD)"
  [[ "$(git -C "${repo}" merge-base --all "$(git -C "${repo}" rev-parse "${label}-first^")" "$(git -C "${repo}" rev-parse "${label}-second")" | wc -l | tr -d '[:space:]')" == 2 ]] || {
    printf '%s\n' "F13 fixture oracle bug: ${label} did not create two best merge bases" >&2
    exit 2
  }
  tree="$(git -C "${repo}" rev-parse "${source}^{tree}")"
  release="$(printf '%s\n' "${label} release merge" | git -C "${repo}" commit-tree "${tree}" -p "${base}" -p "${source}")"
  printf '%s\n' "${base}" "${release}" "${source}"
}

# The accepted graph mirrors the two nested criss-cross source merges present
# in the release PR: a shared historical source merge and a later refresh merge.
f13_nested_repo="${TMP_DIR}/f13-nested-crisscross-fixture"
mapfile -t f13_nested_fixture_values < <(f13_make_nested_crisscross_fixture "${f13_nested_repo}" f13-nested false)
f13_nested_base="${f13_nested_fixture_values[0]:-}"
f13_nested_release="${f13_nested_fixture_values[1]:-}"
f13_collect_master_push_contributions "${f13_nested_repo}" push refs/heads/master \
  "${f13_nested_base}" "${f13_nested_base}" "${f13_nested_release}" \
  "${TMP_DIR}/f13-nested-map" "${TMP_DIR}/f13-nested-category" || {
  printf 'F13 oracle bug: policy-equivalent nested criss-cross sources were rejected (category=%s)\n' \
    "$(<"${TMP_DIR}/f13-nested-category")" >&2
  exit 2
}
f13_nested_base_sets="$(cut -f8 "${TMP_DIR}/f13-nested-map" | LC_ALL=C sort -u | awk 'index($0, ",") > 0 { count++ } END { print count+0 }')"
[[ "${f13_nested_base_sets}" -ge 2 ]] || {
  printf '%s\n' 'F13 oracle bug: complete best-base sets were not retained as contribution evidence' >&2
  exit 2
}
if f13_collect_master_push_contributions "${f13_nested_repo}" pull_request refs/heads/master \
  "${f13_nested_base}" "${f13_nested_base}" "${f13_nested_release}" \
  "${TMP_DIR}/f13-nested-bad-map" "${TMP_DIR}/f13-nested-bad-category"; then
  printf '%s\n' 'F13 oracle bug: non-push event passed the master-push gate' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-nested-bad-category")" == push_event ]] || exit 2
if f13_collect_master_push_contributions "${f13_nested_repo}" push refs/heads/master \
  "${f13_nested_base}" 0000000000000000000000000000000000000000 "${f13_nested_release}" \
  "${TMP_DIR}/f13-nested-bad-map" "${TMP_DIR}/f13-nested-bad-category"; then
  printf '%s\n' 'F13 oracle bug: mismatched push-before base passed the master-push gate' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-nested-bad-category")" == push_before_mismatch ]] || exit 2

# A guarded source path that differs across best bases remains ambiguous.
f13_nested_bad_repo="${TMP_DIR}/f13-nested-guarded-crisscross-fixture"
mapfile -t f13_nested_bad_fixture_values < <(f13_make_nested_crisscross_fixture "${f13_nested_bad_repo}" f13-nested-guarded true)
f13_nested_bad_base="${f13_nested_bad_fixture_values[0]:-}"
f13_nested_bad_release="${f13_nested_bad_fixture_values[1]:-}"
if f13_collect_master_release_pr_contributions "${f13_nested_bad_repo}" "${f13_nested_bad_base}" \
  "${f13_nested_bad_release}" "${TMP_DIR}/f13-nested-bad-map" "${TMP_DIR}/f13-nested-bad-category"; then
  printf '%s\n' 'F13 oracle bug: guarded criss-cross provenance was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-nested-bad-category")" == source_base ]] || {
  printf '%s\n' 'F13 oracle bug: guarded criss-cross ambiguity lost its fixed failure category' >&2
  exit 2
}

# An ordinary octopus integration retains distinct source contributions. Voice
# runtime scope must not contaminate another parent's independent Bot delta.
f13_octopus_repo="${TMP_DIR}/f13-octopus-fixture"
mkdir -p "${f13_octopus_repo}"
git -C "${f13_octopus_repo}" init -q
git -C "${f13_octopus_repo}" config user.name 'Voice CI fixture'
git -C "${f13_octopus_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_octopus_repo}" config commit.gpgsign false
printf '%s\n' base >"${f13_octopus_repo}/README.md"
git -C "${f13_octopus_repo}" add README.md
git -C "${f13_octopus_repo}" commit -qm 'F13 octopus base'
f13_octopus_base="$(git -C "${f13_octopus_repo}" rev-parse HEAD)"
git -C "${f13_octopus_repo}" checkout -qb octopus-voice
mkdir -p "${f13_octopus_repo}/src/backend/voice"
printf '%s\n' 'package main' >"${f13_octopus_repo}/src/backend/voice/main.go"
git -C "${f13_octopus_repo}" add src/backend/voice/main.go
git -C "${f13_octopus_repo}" commit -qm 'F13 octopus Voice source'
f13_octopus_voice="$(git -C "${f13_octopus_repo}" rev-parse HEAD)"
git -C "${f13_octopus_repo}" checkout -qb octopus-bot "${f13_octopus_base}"
mkdir -p "${f13_octopus_repo}/protos/voice/bot/v1"
printf '%s\n' 'syntax = "proto3";' >"${f13_octopus_repo}/protos/voice/bot/v1/bot.proto"
git -C "${f13_octopus_repo}" add protos/voice/bot/v1/bot.proto
git -C "${f13_octopus_repo}" commit -qm 'F13 octopus independent Bot source'
f13_octopus_bot="$(git -C "${f13_octopus_repo}" rev-parse HEAD)"
git -C "${f13_octopus_repo}" checkout -qb octopus-ui "${f13_octopus_base}"
printf '%s\n' fixture >"${f13_octopus_repo}/ui-fixture.txt"
git -C "${f13_octopus_repo}" add ui-fixture.txt
git -C "${f13_octopus_repo}" commit -qm 'F13 octopus independent UI source'
f13_octopus_ui="$(git -C "${f13_octopus_repo}" rev-parse HEAD)"
git -C "${f13_octopus_repo}" checkout -qb octopus-integration "${f13_octopus_base}"
git -C "${f13_octopus_repo}" merge --no-ff -qm 'F13 legitimate three-source octopus' \
  octopus-voice octopus-bot octopus-ui
f13_octopus_head="$(git -C "${f13_octopus_repo}" rev-parse HEAD)"
f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  "${f13_octopus_head}" "${TMP_DIR}/f13-octopus-map" "${TMP_DIR}/f13-octopus-category" true true || {
  printf '%s\n' 'F13 oracle bug: legitimate octopus provenance rejected' >&2
  exit 2
}
for row in \
  $'src/backend/voice/main.go\ttrue\t'"${f13_octopus_voice}" \
  $'protos/voice/bot/v1/bot.proto\tfalse\t'"${f13_octopus_bot}" \
  $'ui-fixture.txt\tfalse\t'"${f13_octopus_ui}"; do
  grep -Fxq "${row}" <(cut -f1,2,6 "${TMP_DIR}/f13-octopus-map") || {
    printf '%s\n' 'F13 oracle bug: octopus lost distinct source identity/trigger' >&2
    exit 2
  }
done
[[ "$(wc -l <"${TMP_DIR}/f13-octopus-map")" -eq 3 ]] || exit 2
git -C "${f13_octopus_repo}" checkout -qb octopus-release "${f13_octopus_base}"
git -C "${f13_octopus_repo}" merge --no-ff -qm 'F13 octopus outer release' octopus-integration
f13_collect_master_release_pr_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  HEAD "${TMP_DIR}/f13-octopus-release-map" || exit 2
cmp "${TMP_DIR}/f13-octopus-map" "${TMP_DIR}/f13-octopus-release-map" || exit 2

# Forged result trees, independent owners even with identical blobs, unknown
# objects, unrelated roots, and multiple possible source bases stay fail-closed.
git -C "${f13_octopus_repo}" checkout -q octopus-integration
printf '%s\n' 'package main' 'func NewLifecycleWorker() {}' >"${f13_octopus_repo}/src/backend/voice/main.go"
git -C "${f13_octopus_repo}" add src/backend/voice/main.go
f13_octopus_bad_tree="$(git -C "${f13_octopus_repo}" write-tree)"
f13_octopus_tampered="$(printf '%s\n' tampered | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_bad_tree}" -p "${f13_octopus_base}" -p "${f13_octopus_voice}" \
  -p "${f13_octopus_bot}" -p "${f13_octopus_ui}")"
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  "${f13_octopus_tampered}" "${TMP_DIR}/f13-octopus-bad-map" "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == merge_tree_mismatch ]] || exit 2
git -C "${f13_octopus_repo}" restore --source=HEAD --staged --worktree src/backend/voice/main.go
printf '%s\n' 'package main' >"${f13_octopus_repo}/src/backend/voice/rogue.go"
git -C "${f13_octopus_repo}" add src/backend/voice/rogue.go
f13_octopus_bad_tree="$(git -C "${f13_octopus_repo}" write-tree)"
f13_octopus_invented="$(printf '%s\n' invented-guarded-path | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_bad_tree}" -p "${f13_octopus_base}" -p "${f13_octopus_voice}" \
  -p "${f13_octopus_bot}" -p "${f13_octopus_ui}")"
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  "${f13_octopus_invented}" "${TMP_DIR}/f13-octopus-bad-map" "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == merge_tree_mismatch ]] || exit 2
git -C "${f13_octopus_repo}" restore --source=HEAD --staged --worktree src/backend/voice/rogue.go
git -C "${f13_octopus_repo}" checkout -qb octopus-duplicate "${f13_octopus_base}"
mkdir -p "${f13_octopus_repo}/src/backend/voice"
printf '%s\n' 'package main' >"${f13_octopus_repo}/src/backend/voice/main.go"
printf '%s\n' independent >"${f13_octopus_repo}/independent.txt"
git -C "${f13_octopus_repo}" add src/backend/voice/main.go independent.txt
git -C "${f13_octopus_repo}" commit -qm 'F13 independent identical guarded owner'
git -C "${f13_octopus_repo}" checkout -qb octopus-ambiguous "${f13_octopus_base}"
git -C "${f13_octopus_repo}" merge --no-ff -qm 'F13 ambiguous octopus' octopus-voice octopus-duplicate octopus-bot
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  HEAD "${TMP_DIR}/f13-octopus-bad-map" "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == duplicate_guarded_owner ]] || exit 2
f13_octopus_tree="$(git -C "${f13_octopus_repo}" rev-parse "${f13_octopus_head}^{tree}")"
f13_octopus_orphan="$(printf '%s\n' orphan | git -C "${f13_octopus_repo}" commit-tree "${f13_octopus_tree}")"
f13_octopus_unrelated="$(printf '%s\n' unrelated | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_tree}" -p "${f13_octopus_base}" -p "${f13_octopus_orphan}" -p "${f13_octopus_bot}")"
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  "${f13_octopus_unrelated}" "${TMP_DIR}/f13-octopus-bad-map" "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == source_base ]] || exit 2
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_base}" \
  0000000000000000000000000000000000000000 "${TMP_DIR}/f13-octopus-bad-map" \
  "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == target_object ]] || exit 2
f13_octopus_left="$(printf '%s\n' left | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_tree}" -p "${f13_octopus_voice}" -p "${f13_octopus_bot}")"
f13_octopus_right="$(printf '%s\n' right | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_tree}" -p "${f13_octopus_bot}" -p "${f13_octopus_voice}")"
f13_octopus_crisscross="$(printf '%s\n' ambiguous-base | git -C "${f13_octopus_repo}" commit-tree \
  "${f13_octopus_tree}" -p "${f13_octopus_left}" -p "${f13_octopus_right}" -p "${f13_octopus_ui}")"
if f13_collect_develop_contributions "${f13_octopus_repo}" "${f13_octopus_left}" \
  "${f13_octopus_crisscross}" "${TMP_DIR}/f13-octopus-bad-map" "${TMP_DIR}/f13-octopus-bad-category" true true; then exit 2; fi
[[ "$(<"${TMP_DIR}/f13-octopus-bad-category")" == source_base ]] || exit 2
printf '%s\n' 'F13 octopus distinct-source and rejection fixtures: PASS'

# A quiet exact-line reader must not make rev-list fail with SIGPIPE under
# pipefail when the valid base is followed by a large first-parent output tail.
f13_pipefail_repo="${TMP_DIR}/f13-first-parent-pipefail"
mkdir -p "${f13_pipefail_repo}"
git -C "${f13_pipefail_repo}" init -q
git -C "${f13_pipefail_repo}" config user.name 'Voice CI fixture'
git -C "${f13_pipefail_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_pipefail_repo}" config commit.gpgsign false
printf '%s\n' 'first-parent root' >"${f13_pipefail_repo}/README.md"
git -C "${f13_pipefail_repo}" add README.md
git -C "${f13_pipefail_repo}" commit -qm 'F13 first-parent root'
printf '%s\n' 'valid base' >"${f13_pipefail_repo}/base.txt"
git -C "${f13_pipefail_repo}" add base.txt
git -C "${f13_pipefail_repo}" commit -qm 'F13 first-parent base'
f13_pipefail_base="$(git -C "${f13_pipefail_repo}" rev-parse HEAD)"
mkdir -p "${f13_pipefail_repo}/src/backend/voice"
printf '%s\n' 'package main' >"${f13_pipefail_repo}/src/backend/voice/main.go"
git -C "${f13_pipefail_repo}" add src/backend/voice/main.go
git -C "${f13_pipefail_repo}" commit -qm 'F13 first-parent target'
f13_pipefail_target="$(git -C "${f13_pipefail_repo}" rev-parse HEAD)"
f13_real_git="$(command -v git)"
f13_pipefail_marker="${TMP_DIR}/f13-first-parent-wrapper-called"
git() {
  if [[ "${1:-}" == -C && "${2:-}" == "${f13_pipefail_repo}" &&
    "${3:-}" == rev-list && "${4:-}" == --first-parent &&
    "${5:-}" == "${f13_pipefail_target}" && ! -e "${f13_pipefail_marker}" ]]; then
    : >"${f13_pipefail_marker}"
    "${f13_real_git}" "$@" || return $?
    for ((f13_padding_line = 0; f13_padding_line < 50000; f13_padding_line++)); do
      printf '%040d\n' 0
    done
  else
    "${f13_real_git}" "$@"
  fi
}
f13_collect_develop_contributions \
  "${f13_pipefail_repo}" "${f13_pipefail_base}" "${f13_pipefail_target}" \
  "${TMP_DIR}/f13-first-parent-pipefail-map" \
  "${TMP_DIR}/f13-first-parent-pipefail-category" || {
  unset -f git
  printf '%s\n' 'F13 oracle bug: valid first-parent base was rejected after a large output tail' >&2
  exit 2
}
unset -f git
[[ -e "${f13_pipefail_marker}" ]] || {
  printf '%s\n' 'F13 fixture setup bug: first-parent output wrapper was not exercised' >&2
  exit 2
}
grep -Fqx $'src/backend/voice/main.go\ttrue' <(cut -f1-2 "${TMP_DIR}/f13-first-parent-pipefail-map") || {
  printf '%s\n' 'F13 oracle bug: first-parent output tail lost the guarded R22 contribution' >&2
  exit 2
}
grep -Fqx $'protos/voice/bot/v1/bot.proto\tfalse' <(cut -f1-2 "${TMP_DIR}/f13-fixture-map") || {
  printf '%s\n' 'F13 oracle bug: unrelated Bot contribution inherited the R22 trigger' >&2
  exit 2
}
f13_cross_scope_path_allowed 'protos/voice/bot/v1/bot.proto' false || {
  printf '%s\n' 'F13 oracle bug: unrelated-only proto contribution was rejected' >&2
  exit 2
}

f13_master_release_pr_enabled pull_request master codex/voice-master-release-20261011 refs/pull/741/merge || {
  printf '%s\n' 'F13 oracle bug: identified master release PR was not selected' >&2
  exit 2
}
if f13_master_release_pr_enabled pull_request master codex/feature/example refs/pull/99/merge ||
  f13_master_release_pr_enabled pull_request develop codex/voice-master-release-20261011 refs/pull/741/merge ||
  f13_master_release_pr_enabled push refs/heads/master codex/voice-master-release-20261011 refs/heads/master ||
  f13_master_release_pr_enabled pull_request master codex/voice-master-release-20261011 refs/pull/741/head; then
  printf '%s\n' 'F13 oracle bug: non-release event or ref selected master release partitioning' >&2
  exit 2
fi

f13_master_merge_push_enabled push refs/heads/master || {
  printf '%s\n' 'F13 oracle bug: exact master merge push was not selected for contribution analysis' >&2
  exit 2
}
if f13_master_merge_push_enabled push refs/heads/develop || \
  f13_master_merge_push_enabled pull_request refs/heads/master || \
  f13_master_merge_push_enabled push refs/heads/feature/example; then
  printf '%s\n' 'F13 oracle bug: inexact master push context selected contribution analysis' >&2
  exit 2
fi
f13_master_push_before_matches "${f13_fixture_base}" "${f13_fixture_base}" || {
  printf '%s\n' 'F13 oracle bug: matching master push before/base pair was rejected' >&2
  exit 2
}
f13_fixture_wrong_before="$(git -C "${f13_fixture_repo}" rev-parse f13-integration)"
if f13_master_push_before_matches '' "${f13_fixture_base}" || \
  f13_master_push_before_matches '0000000000000000000000000000000000000000' "${f13_fixture_base}" || \
  f13_master_push_before_matches "${f13_fixture_wrong_before}" "${f13_fixture_base}" || \
  f13_master_push_before_matches "${f13_fixture_base}" "${f13_fixture_wrong_before}"; then
  printf '%s\n' 'F13 oracle bug: missing, zero, or mismatched master push before/base was accepted' >&2
  exit 2
fi

git -C "${f13_fixture_repo}" checkout -qb f13-master-release-pr "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 synthetic master release PR merge' f13-integration
f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-release-map" \
  "${TMP_DIR}/f13-release-category" || {
  printf '%s\n' 'F13 oracle bug: valid two-parent release PR provenance was rejected' >&2
  exit 2
}
grep -Fqx $'src/backend/voice/main.go\ttrue' <(cut -f1-2 "${TMP_DIR}/f13-release-map") || {
  printf '%s\n' 'F13 oracle bug: release PR lost its R22 source contribution' >&2
  exit 2
}
grep -Fqx $'protos/voice/bot/v1/bot.proto\tfalse' <(cut -f1-2 "${TMP_DIR}/f13-release-map") || {
  printf '%s\n' 'F13 oracle bug: unrelated Bot contribution inherited the R22 trigger in release PR' >&2
  exit 2
}

f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" f13-master-release-pr \
  "${TMP_DIR}/f13-master-push-map" "${TMP_DIR}/f13-master-push-category" || {
  printf '%s\n' 'F13 oracle bug: exact master merge push failed contribution analysis' >&2
  exit 2
}
cmp -s "${TMP_DIR}/f13-release-map" "${TMP_DIR}/f13-master-push-map" || {
  printf '%s\n' 'F13 oracle bug: master merge push did not retain the PR contribution partition' >&2
  exit 2
}
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" '' f13-master-release-pr \
  "${TMP_DIR}/f13-master-push-missing-before-map" "${TMP_DIR}/f13-master-push-missing-before-category"; then
  printf '%s\n' 'F13 oracle bug: master merge push with missing before SHA was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-master-push-missing-before-category")" == release_pr_missing_base ]] || {
  printf '%s\n' 'F13 oracle bug: missing master push before SHA did not fail closed' >&2
  exit 2
}
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" '0000000000000000000000000000000000000000' f13-master-release-pr \
  "${TMP_DIR}/f13-master-push-zero-before-map" "${TMP_DIR}/f13-master-push-zero-before-category"; then
  printf '%s\n' 'F13 oracle bug: zero master push before SHA was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-master-push-zero-before-category")" == base_object ]] || {
  printf '%s\n' 'F13 oracle bug: zero master push before SHA did not fail at base validation' >&2
  exit 2
}
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" f13-integration f13-master-release-pr \
  "${TMP_DIR}/f13-master-push-wrong-before-map" "${TMP_DIR}/f13-master-push-wrong-before-category"; then
  printf '%s\n' 'F13 oracle bug: master push with before SHA outside first parent was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-master-push-wrong-before-category")" == release_pr_base_parent ]] || {
  printf '%s\n' 'F13 oracle bug: mismatched master push before SHA did not fail at parent validation' >&2
  exit 2
}
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" f13-r22-source \
  "${TMP_DIR}/f13-master-push-nonmerge-map" "${TMP_DIR}/f13-master-push-nonmerge-category"; then
  printf '%s\n' 'F13 oracle bug: master push with non-merge target was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-master-push-nonmerge-category")" == release_pr_shape ]] || {
  printf '%s\n' 'F13 oracle bug: non-merge master push did not fail at exact shape validation' >&2
  exit 2
}

# A later master push may be a normal follow-up PR containing only the
# collector/test fix. It still uses exact merge provenance without inheriting
# the earlier merge's unrelated runtime-triggered scope.
git -C "${f13_fixture_repo}" checkout -qb f13-master-script-source f13-master-release-pr
mkdir -p "${f13_fixture_repo}/scripts/ci"
printf '%s\n' '# follow-up contract only' >"${f13_fixture_repo}/scripts/ci/f13-followup.sh"
git -C "${f13_fixture_repo}" add scripts/ci/f13-followup.sh
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture follow-up collector change'
git -C "${f13_fixture_repo}" checkout -qb f13-master-script-followup f13-master-release-pr
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 synthetic follow-up master merge' f13-master-script-source
f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" f13-master-release-pr f13-master-script-followup \
  "${TMP_DIR}/f13-master-followup-map" "${TMP_DIR}/f13-master-followup-category" || {
  printf '%s\n' 'F13 oracle bug: script-only follow-up master merge failed contribution analysis' >&2
  exit 2
}
grep -Fqx $'scripts/ci/f13-followup.sh\tfalse' <(cut -f1-2 "${TMP_DIR}/f13-master-followup-map") || {
  printf '%s\n' 'F13 oracle bug: script-only follow-up master merge inherited runtime trigger' >&2
  exit 2
}

# A source head is not enough: the synthetic PR merge must retain its exact tree entries.
git -C "${f13_fixture_repo}" checkout -q f13-master-release-pr
printf '%s\n' 'conflict-altered result' >"${f13_fixture_repo}/protos/voice/bot/v1/bot.proto"
git -C "${f13_fixture_repo}" add protos/voice/bot/v1/bot.proto
f13_release_bad_tree="$(git -C "${f13_fixture_repo}" write-tree)"
f13_release_bad_commit="$(printf '%s\n' 'F13 altered release PR result' | git -C "${f13_fixture_repo}" -c user.name='Voice CI fixture' -c user.email='voice-ci@example.invalid' commit-tree "${f13_release_bad_tree}" -p "${f13_fixture_base}" -p f13-integration)"
git -C "${f13_fixture_repo}" update-ref refs/heads/f13-release-mismatch "${f13_release_bad_commit}"
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" f13-release-mismatch \
  "${TMP_DIR}/f13-release-mismatch-map" "${TMP_DIR}/f13-release-mismatch-category"; then
  printf '%s\n' 'F13 oracle bug: conflict-altered release PR tree was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-release-mismatch-category")" == release_pr_merge_tree_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: altered release PR tree did not fail at exact tree retention' >&2
  exit 2
}

# The negative case stages a deliberately altered Bot proto in this disposable
# synthetic repository. Restore only that fixture-owned path before later
# cases switch branches; do not let its index/worktree state contaminate them.
git -C "${f13_fixture_repo}" checkout -q f13-master-release-pr -- protos/voice/bot/v1/bot.proto || {
  printf '%s\n' 'F13 fixture setup bug: altered Bot proto could not be restored to the known merge tree' >&2
  exit 2
}
if ! git -C "${f13_fixture_repo}" diff --quiet f13-master-release-pr -- protos/voice/bot/v1/bot.proto || \
  ! git -C "${f13_fixture_repo}" diff --cached --quiet f13-master-release-pr -- protos/voice/bot/v1/bot.proto; then
  printf '%s\n' 'F13 fixture setup bug: Bot proto index/worktree differs from the known merge tree' >&2
  exit 2
fi
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" f13-integration \
  "${TMP_DIR}/f13-release-wrong-parent-map" "${TMP_DIR}/f13-release-wrong-parent-category"; then
  printf '%s\n' 'F13 oracle bug: release PR with wrong synthetic base parent was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-release-wrong-parent-category")" == release_pr_base_parent ]] || {
  printf '%s\n' 'F13 oracle bug: wrong release PR base parent was not rejected precisely' >&2
  exit 2
}
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" '' f13-master-release-pr \
  "${TMP_DIR}/f13-release-missing-base-map" "${TMP_DIR}/f13-release-missing-base-category"; then
  printf '%s\n' 'F13 oracle bug: missing release PR base was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-release-missing-base-category")" == release_pr_missing_base ]] || {
  printf '%s\n' 'F13 oracle bug: missing release PR base did not fail closed' >&2
  exit 2
}

git -C "${f13_fixture_repo}" checkout -qb f13-mixed-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice" "${f13_fixture_repo}/protos/voice/bot/v1"
printf '%s\n' 'package main' >"${f13_fixture_repo}/src/backend/voice/main.go"
printf '%s\n' 'syntax = "proto3";' >"${f13_fixture_repo}/protos/voice/bot/v1/bot.proto"
git -C "${f13_fixture_repo}" add src/backend/voice/main.go protos/voice/bot/v1/bot.proto
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture mixed contribution'
git -C "${f13_fixture_repo}" checkout -qb f13-mixed-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture merge mixed contribution' f13-mixed-source
f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-mixed-map" || {
  printf '%s\n' 'F13 oracle bug: mixed contribution fixture was rejected before path checks' >&2
  exit 2
}
grep -Fqx $'protos/voice/bot/v1/bot.proto\ttrue' <(cut -f1-2 "${TMP_DIR}/f13-mixed-map") || {
  printf '%s\n' 'F13 oracle bug: same-contribution proto change escaped its R22 trigger' >&2
  exit 2
}
if f13_cross_scope_path_allowed 'protos/voice/bot/v1/bot.proto' true; then
  printf '%s\n' 'F13 oracle bug: proto change in the R22 contribution escaped the guard' >&2
  exit 2
fi

git -C "${f13_fixture_repo}" checkout -qb f13-conflict-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/protos/voice/bot/v1"
printf '%s\n' 'source version' >"${f13_fixture_repo}/protos/voice/bot/v1/conflict.proto"
git -C "${f13_fixture_repo}" add protos/voice/bot/v1/conflict.proto
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture conflicting source'
git -C "${f13_fixture_repo}" checkout -qb f13-conflict-integration "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/protos/voice/bot/v1"
printf '%s\n' 'target version' >"${f13_fixture_repo}/protos/voice/bot/v1/conflict.proto"
git -C "${f13_fixture_repo}" add protos/voice/bot/v1/conflict.proto
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture conflicting target'
if git -C "${f13_fixture_repo}" merge --no-ff -m 'F13 fixture conflict merge' f13-conflict-source >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle bug: conflicting fixture unexpectedly merged cleanly' >&2
  exit 2
fi
printf '%s\n' 'target version' >"${f13_fixture_repo}/protos/voice/bot/v1/conflict.proto"
git -C "${f13_fixture_repo}" add protos/voice/bot/v1/conflict.proto
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture resolved conflict'
if f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD \
  "${TMP_DIR}/f13-conflict-map" "${TMP_DIR}/f13-conflict-category"; then
  printf '%s\n' 'F13 oracle bug: source/merge blob mismatch was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-conflict-category")" == merge_tree_mismatch ]] || {
  printf '%s\n' 'F13 oracle bug: guarded conflict resolution did not fail at exact source/merge tree equality' >&2
  exit 2
}

git -C "${f13_fixture_repo}" checkout -qb f13-doc-conflict-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/docs"
printf '%s\n' 'source documentation' >"${f13_fixture_repo}/docs/DEPLOYMENT.md"
git -C "${f13_fixture_repo}" add docs/DEPLOYMENT.md
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture conflicting unguarded documentation source'
git -C "${f13_fixture_repo}" checkout -qb f13-doc-conflict-integration "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/docs"
printf '%s\n' 'target documentation' >"${f13_fixture_repo}/docs/DEPLOYMENT.md"
git -C "${f13_fixture_repo}" add docs/DEPLOYMENT.md
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture conflicting unguarded documentation target'
if git -C "${f13_fixture_repo}" merge --no-ff -m 'F13 fixture documentation conflict merge' f13-doc-conflict-source >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle bug: documentation conflict fixture unexpectedly merged cleanly' >&2
  exit 2
fi
printf '%s\n' 'target documentation' >"${f13_fixture_repo}/docs/DEPLOYMENT.md"
git -C "${f13_fixture_repo}" add docs/DEPLOYMENT.md
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture resolved unguarded documentation conflict'
f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-doc-conflict-map" || {
  printf '%s\n' 'F13 oracle bug: unguarded documentation merge resolution was rejected' >&2
  exit 2
}
grep -Fqx $'docs/DEPLOYMENT.md\tfalse' <(cut -f1-2 "${TMP_DIR}/f13-doc-conflict-map") || {
  printf '%s\n' 'F13 oracle bug: unguarded documentation conflict was not recorded' >&2
  exit 2
}

# A merge can restore an activation line absent from its first parent when a
# conflict is resolved with the source tree. Provenance uses the source delta;
# activation scanning must use the first-parent-to-result delta.
f13_activation_fixture="${TMP_DIR}/f13-activation-fixture"
mkdir -p "${f13_activation_fixture}"
git -C "${f13_activation_fixture}" init -q
git -C "${f13_activation_fixture}" config user.name 'Voice CI fixture'
git -C "${f13_activation_fixture}" config user.email 'voice-ci@example.invalid'
git -C "${f13_activation_fixture}" config commit.gpgsign false
mkdir -p "${f13_activation_fixture}/src/backend/voice"
cat >"${f13_activation_fixture}/src/backend/voice/main.go" <<'EOF'
package main

func main() {
	NewLifecycleWorker()

	label := "base"
	_ = label
}
EOF
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 activation fixture source base'
f13_activation_source_base="$(git -C "${f13_activation_fixture}" rev-parse HEAD)"
git -C "${f13_activation_fixture}" checkout -qb f13-activation-source
python3 - "${f13_activation_fixture}/src/backend/voice/main.go" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
path.write_text(path.read_text().replace('label := "base"', 'label := "source benign edit"'))
PY
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 source benign edit preserves activation'
git -C "${f13_activation_fixture}" checkout -qb f13-activation-integration "${f13_activation_source_base}"
python3 - "${f13_activation_fixture}/src/backend/voice/main.go" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
source = path.read_text().replace('\tNewLifecycleWorker()\n\n', '').replace('label := "base"', 'label := "target edit"')
path.write_text(source)
PY
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 first parent removes activation'
f13_activation_base="$(git -C "${f13_activation_fixture}" rev-parse HEAD)"
if git -C "${f13_activation_fixture}" merge --no-ff -m 'F13 activation conflict merge' f13-activation-source >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle bug: activation fixture unexpectedly merged without conflict' >&2
  exit 2
fi
git -C "${f13_activation_fixture}" show f13-activation-source:src/backend/voice/main.go >"${f13_activation_fixture}/src/backend/voice/main.go"
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 resolve conflict with source tree'
f13_collect_develop_contributions \
  "${f13_activation_fixture}" "${f13_activation_base}" HEAD "${TMP_DIR}/f13-activation-map" || {
  printf '%s\n' 'F13 oracle bug: source-tree conflict resolution lost contribution provenance' >&2
  exit 2
}
f13_changed_content() {
  local repo="$1" map="$2" path="$3" commit first_parent source_base source_head content_base row
  if [[ -s "${map}" ]]; then
    row="$(awk -F '\t' -v path="${path}" '$1 == path { value=$0 } END { if (value != "") print value }' "${map}")"
    if [[ -z "${row}" ]]; then
      row="$(f13_retained_history_row "${repo}" "${map}" "${path}")" || return 1
    fi
    IFS=$'\t' read -r _ _ commit first_parent source_base source_head content_base _ <<<"${row}"
    [[ -n "${commit}" && -n "${first_parent}" && -n "${source_base}" && -n "${source_head}" ]] || return 1
    git -C "${repo}" diff --unified=0 "${content_base:-${first_parent}}" "${commit}" -- "${path}" | sed -n '/^+++ /d; /^+/s/^+//p'
  else
    changed_content "${base_sha}" "${path}"
  fi
}

f13_changed_content \
  "${f13_activation_fixture}" "${TMP_DIR}/f13-activation-map" 'src/backend/voice/main.go' \
  >"${TMP_DIR}/f13-activation-content"
if ! grep -Fq 'NewLifecycleWorker()' "${TMP_DIR}/f13-activation-content" || \
  ! forbidden_activation_content "${TMP_DIR}/f13-activation-content"; then
  printf '%s\n' 'F13 oracle bug: first-parent activation restoration was not detected' >&2
  exit 2
fi

# In a master release PR, a later direct correction may supersede an imported
# guarded path. The map points to that exact latest delta and keeps the earlier
# content-history base so the activation oracle sees all additions to the path.
# The earlier runtime-trigger bit remains true as well.
git -C "${f13_fixture_repo}" checkout -qb f13-release-followup-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
cat >"${f13_fixture_repo}/src/backend/voice/main.go" <<'EOF'
package main

func main() {}
EOF
printf '%s\n' 'package voice' 'const fixture = "initial"' >"${f13_fixture_repo}/src/backend/voice/followup_test.go"
git -C "${f13_fixture_repo}" add src/backend/voice/main.go src/backend/voice/followup_test.go
git -C "${f13_fixture_repo}" commit -qm 'F13 release fixture imported guarded paths'
git -C "${f13_fixture_repo}" checkout -qb f13-release-followup-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 release fixture imports guarded source' f13-release-followup-source
cat >"${f13_fixture_repo}/src/backend/voice/main.go" <<'EOF'
package main

func main() {
	NewLifecycleWorker()
}
EOF
printf '%s\n' 'package voice' 'const newDirectPath = true' >"${f13_fixture_repo}/src/backend/voice/z_new_direct.go"
git -C "${f13_fixture_repo}" add src/backend/voice/main.go src/backend/voice/z_new_direct.go
git -C "${f13_fixture_repo}" commit -qm 'F13 release fixture direct guarded correction'
f13_followup_activation_commit="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
printf '%s\n' 'package voice' 'const fixture = "corrected"' >"${f13_fixture_repo}/src/backend/voice/followup_test.go"
git -C "${f13_fixture_repo}" add src/backend/voice/followup_test.go
git -C "${f13_fixture_repo}" commit -qm 'F13 release fixture direct test correction'
f13_followup_head="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
git -C "${f13_fixture_repo}" checkout -qb f13-release-followup-pr "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 synthetic release PR with direct correction' f13-release-followup-integration
f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD \
  "${TMP_DIR}/f13-release-followup-map" "${TMP_DIR}/f13-release-followup-category" || {
  printf '%s\n' 'F13 oracle bug: release PR direct correction of imported guarded paths was rejected' >&2
  exit 2
}
grep -Fqx $'src/backend/voice/main.go\ttrue\t'"${f13_followup_activation_commit}" <(cut -f1-3 "${TMP_DIR}/f13-release-followup-map") || {
  printf '%s\n' 'F13 oracle bug: corrected guarded path lost its latest owner or prior runtime trigger' >&2
  exit 2
}
grep -Fqx $'src/backend/voice/followup_test.go\ttrue\t'"${f13_followup_head}" <(cut -f1-3 "${TMP_DIR}/f13-release-followup-map") || {
  printf '%s\n' 'F13 oracle bug: test path correction did not retain the imported contribution runtime trigger' >&2
  exit 2
}
f13_new_direct_row="$(awk -F '\t' '$1 == "src/backend/voice/z_new_direct.go" { row=$0 } END { print row }' \
  "${TMP_DIR}/f13-release-followup-map")"
IFS=$'\t' read -r _ _ _ f13_new_direct_parent _ _ f13_new_direct_content_base _ <<<"${f13_new_direct_row}"
[[ -n "${f13_new_direct_parent}" && \
  "${f13_new_direct_content_base}" == "${f13_new_direct_parent}" ]] || {
  printf '%s\n' 'F13 oracle bug: new path in mixed direct follow-up inherited another path content history' >&2
  exit 2
}
f13_changed_content "${f13_fixture_repo}" "${TMP_DIR}/f13-release-followup-map" \
  'src/backend/voice/main.go' >"${TMP_DIR}/f13-release-followup-content"
grep -Fq 'NewLifecycleWorker()' "${TMP_DIR}/f13-release-followup-content" && \
  forbidden_activation_content "${TMP_DIR}/f13-release-followup-content" || {
  printf '%s\n' 'F13 oracle bug: latest direct activation escaped its exact path/delta guard' >&2
  exit 2
}
f13_followup_row="$(awk -F '\t' '$1 == "src/backend/voice/followup_test.go" { row=$0 } END { print row }' \
  "${TMP_DIR}/f13-release-followup-map")"
IFS=$'\t' read -r _ _ _ _ f13_followup_source_base f13_followup_source_head _ _ <<<"${f13_followup_row}"
[[ "${f13_followup_source_base}" == "${f13_followup_activation_commit}" && \
  "${f13_followup_source_head}" == "${f13_followup_head}" ]] && \
  git -C "${f13_fixture_repo}" diff --name-only \
    "${f13_followup_source_base}" "${f13_followup_source_head}" | \
    grep -Fxq 'src/backend/voice/followup_test.go' || {
  printf 'F13 oracle bug: latest guarded path delta did not isolate follow-up test (base=%s head=%s expected_base=%s expected_head=%s)\n' \
    "${f13_followup_source_base}" "${f13_followup_source_head}" \
    "${f13_followup_activation_commit}" "${f13_followup_head}" >&2
  exit 2
}

# A harmless direct follow-up must not hide forbidden activation content that
# was already introduced by the imported merge and remains in the final file.
git -C "${f13_fixture_repo}" checkout -qb f13-release-retained-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
cat >"${f13_fixture_repo}/src/backend/voice/database.go" <<'EOF'
package voice

func StartDatabaseLifecycle() {
	NewLifecycleWorker()
}
EOF
git -C "${f13_fixture_repo}" add src/backend/voice/database.go
git -C "${f13_fixture_repo}" commit -qm 'F13 retained activation fixture contribution'
git -C "${f13_fixture_repo}" checkout -qb f13-release-retained-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 retained activation fixture merge' f13-release-retained-source
sed -i 's/func StartDatabaseLifecycle/func StartDatabaseLifecycle \/\/ corrected/' \
  "${f13_fixture_repo}/src/backend/voice/database.go"
git -C "${f13_fixture_repo}" add src/backend/voice/database.go
git -C "${f13_fixture_repo}" commit -qm 'F13 retained activation harmless direct correction'
git -C "${f13_fixture_repo}" checkout -qb f13-release-retained-pr "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 retained activation synthetic PR merge' f13-release-retained-integration
f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD \
  "${TMP_DIR}/f13-release-retained-map" "${TMP_DIR}/f13-release-retained-category" || {
  printf '%s\n' 'F13 oracle bug: harmless direct correction of retained activation was rejected' >&2
  exit 2
}
f13_changed_content "${f13_fixture_repo}" "${TMP_DIR}/f13-release-retained-map" \
  'src/backend/voice/database.go' >"${TMP_DIR}/f13-release-retained-content"
grep -Fq 'NewLifecycleWorker()' "${TMP_DIR}/f13-release-retained-content" && \
  forbidden_activation_content "${TMP_DIR}/f13-release-retained-content" || {
  printf '%s\n' 'F13 oracle bug: direct follow-up hid forbidden activation retained from imported merge' >&2
  exit 2
}

# A second merge is incremental only when its exact source base descends from
# the prior owner. This models a follow-up branch created from the prior merge.
git -C "${f13_fixture_repo}" checkout -qb f13-release-incremental-source "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
mkdir -p "${f13_fixture_repo}/src/backend/voice/internal/grpcsvc"
printf '%s\n' 'FROM scratch' >"${f13_fixture_repo}/src/backend/voice/Dockerfile"
printf '%s\n' 'package grpcsvc' 'const incremental = "first"' >"${f13_fixture_repo}/src/backend/voice/internal/grpcsvc/incremental.go"
git -C "${f13_fixture_repo}" add src/backend/voice/Dockerfile src/backend/voice/internal/grpcsvc/incremental.go
git -C "${f13_fixture_repo}" commit -qm 'F13 incremental merge first contribution'
f13_incremental_initial_head="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
git -C "${f13_fixture_repo}" checkout -qb f13-release-incremental-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 incremental merge first import' f13-release-incremental-source
f13_incremental_owner="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
git -C "${f13_fixture_repo}" checkout -qb f13-release-incremental-followup "${f13_incremental_initial_head}"
printf '%s\n' 'package grpcsvc' 'const incremental = "corrected"' >"${f13_fixture_repo}/src/backend/voice/internal/grpcsvc/incremental.go"
git -C "${f13_fixture_repo}" add src/backend/voice/internal/grpcsvc/incremental.go
git -C "${f13_fixture_repo}" commit -qm 'F13 incremental merge follow-up source'
f13_incremental_source_head="$(git -C "${f13_fixture_repo}" rev-parse HEAD)"
git -C "${f13_fixture_repo}" checkout -q f13-release-incremental-integration
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 incremental merge follow-up import' f13-release-incremental-followup
git -C "${f13_fixture_repo}" checkout -qb f13-release-incremental-pr "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 incremental synthetic release PR merge' f13-release-incremental-integration
f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD \
  "${TMP_DIR}/f13-release-incremental-map" "${TMP_DIR}/f13-release-incremental-category" || {
  printf '%s\n' 'F13 oracle bug: exact descendant-source merge correction was rejected' >&2
  exit 2
}
f13_incremental_row="$(awk -F '\t' '$1 == "src/backend/voice/internal/grpcsvc/incremental.go" { row=$0 } END { print row }' \
  "${TMP_DIR}/f13-release-incremental-map")"
IFS=$'\t' read -r _ f13_incremental_trigger f13_incremental_latest _ f13_incremental_source_base \
  f13_incremental_mapped_head _ _ <<<"${f13_incremental_row}"
[[ "${f13_incremental_trigger}" == true && \
  "${f13_incremental_source_base}" == "${f13_incremental_initial_head}" && \
  "${f13_incremental_mapped_head}" == "${f13_incremental_source_head}" && \
  "${f13_incremental_latest}" != "${f13_incremental_owner}" ]] || {
  printf '%s\n' 'F13 oracle bug: incremental merge lost latest source provenance' >&2
  exit 2
}

# A normal author-branch sync may import the current PR base as the second
# parent of a merge. Keep novel first-parent commits on both sides of that
# sync, exclude paths owned only by the imported base, and reject guarded
# manual merge resolutions separately.
f13_sync_fixture_repo="${TMP_DIR}/f13-sync-boundary-fixture"
mkdir -p "${f13_sync_fixture_repo}"
git -C "${f13_sync_fixture_repo}" init -q
git -C "${f13_sync_fixture_repo}" config user.name 'Voice CI fixture'
git -C "${f13_sync_fixture_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_sync_fixture_repo}" config commit.gpgsign false
printf '%s\n' 'sync fixture base' >"${f13_sync_fixture_repo}/README.md"
git -C "${f13_sync_fixture_repo}" add README.md
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture shared base'
f13_sync_shared_base="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
git -C "${f13_sync_fixture_repo}" checkout -qb f13-sync-release-base
mkdir -p "${f13_sync_fixture_repo}/src/backend/voice/internal/grpcsvc"
printf '%s\n' 'package grpcsvc' 'const releaseOwned = true' >"${f13_sync_fixture_repo}/src/backend/voice/internal/grpcsvc/release_owned_test.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/internal/grpcsvc/release_owned_test.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture release-owned guarded path'
f13_sync_release_ancestor="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
printf '%s\n' 'release base advanced' >>"${f13_sync_fixture_repo}/README.md"
git -C "${f13_sync_fixture_repo}" add README.md
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture release base'
f13_sync_base="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
git -C "${f13_sync_fixture_repo}" checkout -qb f13-sync-source "${f13_sync_shared_base}"
mkdir -p "${f13_sync_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' 'func beforeSync() {}' >"${f13_sync_fixture_repo}/src/backend/voice/sync_pre.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/sync_pre.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture guarded source before sync'
f13_sync_pre_commit="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
git -C "${f13_sync_fixture_repo}" merge --no-ff -qm 'F13 sync fixture imports exact release base' f13-sync-release-base
printf '%s\n' 'package voice' 'func afterSync() {}' >"${f13_sync_fixture_repo}/src/backend/voice/sync_post.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/sync_post.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture guarded source after sync'
f13_sync_post_commit="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
f13_sync_source_head="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
git -C "${f13_sync_fixture_repo}" checkout -qb f13-sync-release-pr "${f13_sync_base}"
git -C "${f13_sync_fixture_repo}" merge --no-ff -qm 'F13 sync fixture synthetic PR merge' f13-sync-source
f13_collect_master_release_pr_contributions \
  "${f13_sync_fixture_repo}" "${f13_sync_base}" HEAD \
  "${TMP_DIR}/f13-sync-boundary-map" "${TMP_DIR}/f13-sync-boundary-category" || {
  printf '%s\n' 'F13 oracle bug: normal second-parent base sync was rejected' >&2
  exit 2
}
printf '%s\n' \
  'src/backend/voice/sync_post.go' \
  'src/backend/voice/sync_pre.go' >"${TMP_DIR}/f13-sync-boundary-expected-paths"
cut -f1 "${TMP_DIR}/f13-sync-boundary-map" | sort >"${TMP_DIR}/f13-sync-boundary-actual-paths"
cmp -s "${TMP_DIR}/f13-sync-boundary-expected-paths" "${TMP_DIR}/f13-sync-boundary-actual-paths" || {
  printf '%s\n' 'F13 oracle bug: sync boundary reattributed imported base history or lost a source commit' >&2
  exit 2
}
f13_sync_pre_row="$(awk -F '\t' '$1 == "src/backend/voice/sync_pre.go" { row=$0 } END { print row }' \
  "${TMP_DIR}/f13-sync-boundary-map")"
f13_sync_post_row="$(awk -F '\t' '$1 == "src/backend/voice/sync_post.go" { row=$0 } END { print row }' \
  "${TMP_DIR}/f13-sync-boundary-map")"
IFS=$'\t' read -r _ _ f13_sync_pre_owner _ _ f13_sync_pre_head _ _ <<<"${f13_sync_pre_row}"
IFS=$'\t' read -r _ _ f13_sync_post_owner _ _ f13_sync_post_head _ _ <<<"${f13_sync_post_row}"
[[ "${f13_sync_pre_owner}" == "${f13_sync_pre_commit}" && \
  "${f13_sync_pre_head}" == "${f13_sync_pre_commit}" && \
  "${f13_sync_post_owner}" == "${f13_sync_post_commit}" && \
  "${f13_sync_post_head}" == "${f13_sync_post_commit}" ]] || {
  printf '%s\n' 'F13 oracle bug: sync-boundary map lost exact pre/post source ownership' >&2
  exit 2
}

# An exact later import of B reconciles only source rows whose owner and source
# head were already in B. This mirrors a PR branch that carried an older base
# version through a nested merge before syncing the current base.
f13_sync_reconcile_repo="${TMP_DIR}/f13-sync-reconcile-fixture"
mkdir -p "${f13_sync_reconcile_repo}"
git -C "${f13_sync_reconcile_repo}" init -q
git -C "${f13_sync_reconcile_repo}" config user.name 'Voice CI fixture'
git -C "${f13_sync_reconcile_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_sync_reconcile_repo}" config commit.gpgsign false
f13_sync_reconcile_path='src/backend/user/pb/sync_reconcile.pb.go'
mkdir -p "${f13_sync_reconcile_repo}/$(dirname "${f13_sync_reconcile_path}")"
printf '%s\n' 'baseline entry' >"${f13_sync_reconcile_repo}/${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" add .
git -C "${f13_sync_reconcile_repo}" commit -qm 'F13 reconciliation shared source'
f13_sync_reconcile_shared="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
git -C "${f13_sync_reconcile_repo}" checkout -qb f13-reconcile-base
printf '%s\n' 'pre-base stale entry' >"${f13_sync_reconcile_repo}/${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" add "${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" commit -qm 'F13 reconciliation base-owned source'
f13_sync_reconcile_owner="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
printf '%s\n' 'baseline entry' >"${f13_sync_reconcile_repo}/${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" add "${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" commit -qm 'F13 reconciliation authoritative base entry'
f13_sync_reconcile_base="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
git -C "${f13_sync_reconcile_repo}" checkout -qb f13-reconcile-source "${f13_sync_reconcile_shared}"
printf '%s\n' 'source lane' >"${f13_sync_reconcile_repo}/README.md"
git -C "${f13_sync_reconcile_repo}" add README.md
git -C "${f13_sync_reconcile_repo}" commit -qm 'F13 reconciliation source lane'
git -C "${f13_sync_reconcile_repo}" merge --no-ff -qm 'F13 reconciliation imports base-owned row' f13-reconcile-base~1
f13_sync_reconcile_stale="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
git -C "${f13_sync_reconcile_repo}" merge --no-ff -qm 'F13 reconciliation syncs exact current base' f13-reconcile-base
f13_sync_reconcile_merge="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
f13_sync_reconcile_head="${f13_sync_reconcile_merge}"
f13_sync_reconcile_tree="$(git -C "${f13_sync_reconcile_repo}" rev-parse "${f13_sync_reconcile_head}^{tree}")"
f13_sync_reconcile_target="$(printf '%s\n' 'F13 exact synthetic PR result' | \
  git -C "${f13_sync_reconcile_repo}" commit-tree "${f13_sync_reconcile_tree}" \
    -p "${f13_sync_reconcile_base}" -p "${f13_sync_reconcile_head}")" || {
  printf '%s\n' 'F13 fixture setup failed: could not create exact synthetic PR result' >&2
  exit 2
}
f13_collect_master_release_pr_contributions \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_target}" \
  "${TMP_DIR}/f13-sync-reconcile-map" "${TMP_DIR}/f13-sync-reconcile-category" || {
  printf '%s\n' 'F13 oracle bug: exact current-base sync did not reconcile a stale pre-base owner' >&2
  exit 2
}
if awk -F '\t' -v candidate="${f13_sync_reconcile_path}" '$1 == candidate { found=1 } END { exit !found }' \
  "${TMP_DIR}/f13-sync-reconcile-map"; then
  printf '%s\n' 'F13 oracle bug: exact current-base sync retained a stale guarded owner row' >&2
  exit 2
fi
f13_sync_reconcile_post_base="${TMP_DIR}/f13-sync-reconcile-post-base-owner"
git -C "${f13_sync_reconcile_repo}" checkout -qb f13-reconcile-post-base "${f13_sync_reconcile_base}"
printf '%s\n' 'pre-base stale entry' >"${f13_sync_reconcile_repo}/${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" add "${f13_sync_reconcile_path}"
git -C "${f13_sync_reconcile_repo}" commit -qm 'F13 post-base source proposal with matching bytes'
f13_sync_reconcile_post_base_owner="$(git -C "${f13_sync_reconcile_repo}" rev-parse HEAD)"
f13_sync_reconcile_first_entry="$(git -C "${f13_sync_reconcile_repo}" ls-tree "${f13_sync_reconcile_stale}" -- "${f13_sync_reconcile_path}")"
if f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
  "${f13_sync_reconcile_path}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_post_base_owner}" \
  "${f13_sync_reconcile_post_base_owner}" "${f13_sync_reconcile_first_entry}" \
  "${f13_sync_reconcile_first_entry}"; then
  printf '%s\n' 'F13 oracle bug: exact base sync retired a post-base owner with matching bytes' >&2
  exit 2
fi

# The comparison origin is metadata, not ownership authority: a valid commit
# outside B is acceptable when the actual owner and source head are in B.
f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
  "${f13_sync_reconcile_path}" "${f13_sync_reconcile_post_base_owner}" \
  "${f13_sync_reconcile_owner}" "${f13_sync_reconcile_owner}" \
  "${f13_sync_reconcile_first_entry}" "${f13_sync_reconcile_first_entry}" || {
  printf '%s\n' 'F13 oracle bug: valid non-base comparison origin rejected base-owned reconciliation' >&2
  exit 2
}
if f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
  "${f13_sync_reconcile_path}" '' "${f13_sync_reconcile_owner}" \
  "${f13_sync_reconcile_owner}" "${f13_sync_reconcile_first_entry}" \
  "${f13_sync_reconcile_first_entry}" ||
  f13_sync_boundary_reconciles_guarded_path \
    "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
    "${f13_sync_reconcile_path}" '0000000000000000000000000000000000000000' \
    "${f13_sync_reconcile_owner}" "${f13_sync_reconcile_owner}" \
    "${f13_sync_reconcile_first_entry}" "${f13_sync_reconcile_first_entry}"; then
  printf '%s\n' 'F13 oracle bug: missing or invalid comparison origin qualified base reconciliation' >&2
  exit 2
fi

# Missing provenance and altered mode metadata must never qualify, even when
# the content blob is otherwise identical to the first-parent/base entry.
if f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
  "${f13_sync_reconcile_path}" "${f13_sync_reconcile_base}" "" \
  "${f13_sync_reconcile_owner}" "${f13_sync_reconcile_first_entry}" \
  "${f13_sync_reconcile_first_entry}"; then
  printf '%s\n' 'F13 oracle bug: missing owner provenance qualified base reconciliation' >&2
  exit 2
fi
if [[ "${f13_sync_reconcile_first_entry}" == 100644* ]]; then
  f13_sync_reconcile_wrong_mode="${f13_sync_reconcile_first_entry/100644/100755}"
else
  f13_sync_reconcile_wrong_mode="${f13_sync_reconcile_first_entry/100755/100644}"
fi
if f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_reconcile_repo}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_merge}" \
  "${f13_sync_reconcile_path}" "${f13_sync_reconcile_base}" "${f13_sync_reconcile_owner}" \
  "${f13_sync_reconcile_owner}" "${f13_sync_reconcile_wrong_mode}" \
  "${f13_sync_reconcile_first_entry}"; then
  printf '%s\n' 'F13 oracle bug: mismatched mode metadata qualified base reconciliation' >&2
  exit 2
fi

# Git pathspec magic must not redirect a lookup away from a literal tracked
# filename that itself starts with a pathspec expression.
f13_sync_literal_repo="${TMP_DIR}/f13-sync-literal-path-fixture"
mkdir -p "${f13_sync_literal_repo}"
git -C "${f13_sync_literal_repo}" init -q
git -C "${f13_sync_literal_repo}" config user.name 'Voice CI fixture'
git -C "${f13_sync_literal_repo}" config user.email 'voice-ci@example.invalid'
git -C "${f13_sync_literal_repo}" config commit.gpgsign false
f13_sync_literal_path=':(glob)f13-reconcile.txt'
printf '%s\n' 'baseline' >"${f13_sync_literal_repo}/${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" add -- ":(literal)${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" commit -qm 'F13 literal path shared source'
f13_sync_literal_shared="$(git -C "${f13_sync_literal_repo}" rev-parse HEAD)"
git -C "${f13_sync_literal_repo}" checkout -qb f13-literal-base
printf '%s\n' 'stale' >"${f13_sync_literal_repo}/${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" add -- ":(literal)${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" commit -qm 'F13 literal path base owner'
f13_sync_literal_owner="$(git -C "${f13_sync_literal_repo}" rev-parse HEAD)"
printf '%s\n' 'baseline' >"${f13_sync_literal_repo}/${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" add -- ":(literal)${f13_sync_literal_path}"
git -C "${f13_sync_literal_repo}" commit -qm 'F13 literal path authoritative base'
f13_sync_literal_base="$(git -C "${f13_sync_literal_repo}" rev-parse HEAD)"
git -C "${f13_sync_literal_repo}" checkout -qb f13-literal-source "${f13_sync_literal_shared}"
printf '%s\n' 'source lane' >"${f13_sync_literal_repo}/README.md"
git -C "${f13_sync_literal_repo}" add README.md
git -C "${f13_sync_literal_repo}" commit -qm 'F13 literal path source lane'
git -C "${f13_sync_literal_repo}" merge --no-ff -qm 'F13 literal path imports owner' f13-literal-base~1
git -C "${f13_sync_literal_repo}" merge --no-ff -qm 'F13 literal path syncs base' f13-literal-base
f13_sync_literal_merge="$(git -C "${f13_sync_literal_repo}" rev-parse HEAD)"
f13_sync_literal_entry="$(git -C "${f13_sync_literal_repo}" ls-tree "${f13_sync_literal_owner}" -- ":(literal)${f13_sync_literal_path}")"
f13_sync_literal_result="$(git -C "${f13_sync_literal_repo}" ls-tree "${f13_sync_literal_merge}" -- ":(literal)${f13_sync_literal_path}")"
f13_sync_literal_base_entry="$(git -C "${f13_sync_literal_repo}" ls-tree "${f13_sync_literal_base}" -- ":(literal)${f13_sync_literal_path}")"
[[ -n "${f13_sync_literal_entry}" && "${f13_sync_literal_result}" == "${f13_sync_literal_base_entry}" ]] || {
  printf '%s\n' 'F13 fixture setup failed: literal path merge entries are not as expected' >&2
  exit 2
}
f13_sync_boundary_reconciles_guarded_path \
  "${f13_sync_literal_repo}" "${f13_sync_literal_base}" "${f13_sync_literal_merge}" \
  "${f13_sync_literal_path}" "${f13_sync_literal_shared}" "${f13_sync_literal_owner}" \
  "${f13_sync_literal_owner}" "${f13_sync_literal_entry}" "${f13_sync_literal_entry}" || {
  printf '%s\n' 'F13 oracle bug: literal path lookup failed exact base reconciliation' >&2
  exit 2
}

# A base that is merely an ancestor of the sync merge's second parent is not
# the exact imported boundary and must not gain the normal-sync exception.
if f13_collect_develop_contributions \
  "${f13_sync_fixture_repo}" "${f13_sync_release_ancestor}" "${f13_sync_source_head}" \
  "${TMP_DIR}/f13-sync-nonboundary-map" "${TMP_DIR}/f13-sync-nonboundary-category" true true; then
  printf '%s\n' 'F13 oracle bug: non-exact second-parent ancestor was accepted as a sync boundary' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-sync-nonboundary-category")" == first_parent_range ]] || {
  printf '%s\n' 'F13 oracle bug: non-exact sync boundary failed for an unexpected reason' >&2
  exit 2
}

# A manual guarded conflict resolution is not owned by either side of a sync
# merge, even though the merge imports the exact base.
git -C "${f13_sync_fixture_repo}" checkout -qb f13-sync-conflict-release "${f13_sync_shared_base}"
mkdir -p "${f13_sync_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' 'const syncConflict = "release"' >"${f13_sync_fixture_repo}/src/backend/voice/sync_conflict.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/sync_conflict.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture release conflict side'
f13_sync_conflict_base="$(git -C "${f13_sync_fixture_repo}" rev-parse HEAD)"
git -C "${f13_sync_fixture_repo}" checkout -qb f13-sync-conflict-source "${f13_sync_shared_base}"
mkdir -p "${f13_sync_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' 'const syncConflict = "source"' >"${f13_sync_fixture_repo}/src/backend/voice/sync_conflict.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/sync_conflict.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture source conflict side'
if git -C "${f13_sync_fixture_repo}" merge --no-ff -m 'F13 sync fixture conflicting base sync' f13-sync-conflict-release >/dev/null 2>&1; then
  printf '%s\n' 'F13 oracle fixture error: expected guarded sync conflict did not occur' >&2
  exit 2
fi
printf '%s\n' 'package voice' 'const syncConflict = "unowned-resolution"' >"${f13_sync_fixture_repo}/src/backend/voice/sync_conflict.go"
git -C "${f13_sync_fixture_repo}" add src/backend/voice/sync_conflict.go
git -C "${f13_sync_fixture_repo}" commit -qm 'F13 sync fixture unowned guarded resolution'
if f13_collect_develop_contributions \
  "${f13_sync_fixture_repo}" "${f13_sync_conflict_base}" HEAD \
  "${TMP_DIR}/f13-sync-conflict-map" "${TMP_DIR}/f13-sync-conflict-category" true true; then
  printf '%s\n' 'F13 oracle bug: guarded manual sync resolution was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-sync-conflict-category")" == sync_merge_guarded_resolution ]] || {
  printf '%s\n' 'F13 oracle bug: guarded sync resolution failed for an unexpected reason' >&2
  exit 2
}

git -C "${f13_fixture_repo}" checkout -qb f13-repeat-source-one "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' >"${f13_fixture_repo}/src/backend/voice/repeated.go"
git -C "${f13_fixture_repo}" add src/backend/voice/repeated.go
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture first repeated guarded path'
git -C "${f13_fixture_repo}" checkout -qb f13-repeat-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture first repeated merge' f13-repeat-source-one
git -C "${f13_fixture_repo}" checkout -qb f13-repeat-source-two "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' >"${f13_fixture_repo}/src/backend/voice/repeated.go"
git -C "${f13_fixture_repo}" add src/backend/voice/repeated.go
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture second repeated guarded path'
git -C "${f13_fixture_repo}" checkout -q f13-repeat-integration
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture second repeated merge' f13-repeat-source-two
if f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-repeat-map"; then
  printf '%s\n' 'F13 oracle bug: guarded path with multiple contribution owners was accepted' >&2
  exit 2
fi
git -C "${f13_fixture_repo}" checkout -qb f13-release-duplicate-pr "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 synthetic duplicate release PR merge' f13-repeat-integration
if f13_collect_master_release_pr_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD \
  "${TMP_DIR}/f13-release-duplicate-map" "${TMP_DIR}/f13-release-duplicate-category"; then
  printf '%s\n' 'F13 oracle bug: duplicate guarded release contribution was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-release-duplicate-category")" == duplicate_guarded_owner ]] || {
  printf '%s\n' 'F13 oracle bug: duplicate release provenance did not fail closed' >&2
  exit 2
}

git -C "${f13_fixture_repo}" checkout -q --orphan f13-unrelated-source
git -C "${f13_fixture_repo}" rm -rf -q .
mkdir -p "${f13_fixture_repo}/src/backend/voice"
printf '%s\n' 'package main' >"${f13_fixture_repo}/src/backend/voice/orphan.go"
git -C "${f13_fixture_repo}" add src/backend/voice/orphan.go
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture unrelated root'
git -C "${f13_fixture_repo}" checkout -qb f13-unrelated-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --allow-unrelated-histories --no-ff -qm 'F13 fixture unrelated merge' f13-unrelated-source
if f13_collect_develop_contributions \
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-unrelated-map"; then
  printf '%s\n' 'F13 oracle bug: non-ancestor contribution source was accepted' >&2
  exit 2
fi

if f13_collect_develop_contributions \
  "${f13_fixture_repo}" '0000000000000000000000000000000000000000' HEAD \
  "${TMP_DIR}/f13-invalid-map" "${TMP_DIR}/f13-invalid-category"; then
  printf '%s\n' 'F13 oracle bug: missing aggregate baseline was accepted' >&2
  exit 2
fi
[[ "$(<"${TMP_DIR}/f13-invalid-category")" == base_object ]] || {
  printf '%s\n' 'F13 oracle bug: invalid baseline did not produce its fixed failure category' >&2
  exit 2
}
f13_saved_base_was_set=false
f13_saved_base_value="${VOICE_R22_BASE_SHA:-}"
if [[ "${VOICE_R22_BASE_SHA+x}" == x ]]; then
  f13_saved_base_was_set=true
fi
unset VOICE_R22_BASE_SHA
[[ "$(f13_base_origin_class)" == fallback ]] || {
  printf '%s\n' 'F13 oracle bug: absent baseline did not classify as fallback' >&2
  exit 2
}
VOICE_R22_BASE_SHA='fixture-value-never-emitted'
[[ "$(f13_base_origin_class)" == push-before ]] || {
  printf '%s\n' 'F13 oracle bug: supplied baseline did not classify as push-before' >&2
  exit 2
}
if [[ "${f13_saved_base_was_set}" == true ]]; then
  VOICE_R22_BASE_SHA="${f13_saved_base_value}"
else
  unset VOICE_R22_BASE_SHA
fi
if f13_develop_aggregate_enabled push refs/heads/feature/example || \
  f13_develop_aggregate_enabled push refs/heads/master || \
  f13_develop_aggregate_enabled pull_request refs/heads/develop; then
  printf '%s\n' 'F13 oracle bug: contribution partition activated outside develop push' >&2
  exit 2
fi
f13_develop_aggregate_enabled push refs/heads/develop || {
  printf '%s\n' 'F13 oracle bug: develop push did not enable contribution partition' >&2
  exit 2
}

f13_master_release_pr=false
native_p3_master_pr=false
if native_p3_prepare; then
  [[ "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" == pull_request ]] && native_p3_master_pr=true
else
  # Missing/incomplete/unreviewed native authority never changes old outcomes.
  native_p3_active=false
fi
f13_master_merge_push=false
f13_collect_status=0
if f13_master_pr_contribution_enabled   "$ROOT" "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_BASE_REF:-}"   "${GITHUB_HEAD_REF:-}" "${GITHUB_REF:-}" "${GITHUB_SHA:-}"   "${VOICE_R22_BASE_SHA:-}" HEAD; then
  f13_master_release_pr=true
fi
if [[ "${f13_master_release_pr}" == true ]]; then
  if ! f13_migration_alias_record_load "${ROOT}" "${game_checkpoint_snapshot_record}" \
    "${TMP_DIR}/f13-active-alias-record.tsv" "${TMP_DIR}/f13-active-collision-record.tsv"; then
    printf '%s\n' 'F13 oracle bug: active immutable R2 migration alias record failed exact table validation' >&2
    exit 2
  fi
  f13_migration_alias_record_negative_fixtures \
    "${TMP_DIR}/f13-active-alias-record.tsv" "${TMP_DIR}/f13-active-collision-record.tsv" || exit 2
  if ! f13_migration_alias_reconciliation_execution_fixture "${ROOT}" \
    "${VOICE_R22_BASE_SHA:-}" HEAD "$(git -C "${ROOT}" rev-parse HEAD^2)"; then
    printf '%s\n' 'F13 oracle bug: exact R2 migration alias helper rejected its complete producer row' >&2
    exit 2
  fi
  if f13_extract_versioned_alias_section "${ROOT}" \
    'd97e8acd6824c5dd08d4f70be93fc7f91d6f6fce' \
    '<!-- F13-MIGRATION-ALIASES-V2-BEGIN -->' '<!-- F13-MIGRATION-ALIASES-V2-END -->' \
    "${TMP_DIR}/f13-old-record-aliases"; then
    printf '%s\n' 'F13 oracle bug: old R1 unexpectedly contains the R2 migration alias section' >&2
    exit 2
  fi
  f13_active_alias_record="${game_checkpoint_snapshot_record}"
  game_checkpoint_snapshot_record='d97e8acd6824c5dd08d4f70be93fc7f91d6f6fce'
  if f13_collect_master_release_pr_contributions "${ROOT}" "${VOICE_R22_BASE_SHA:-}" HEAD \
    "${TMP_DIR}/f13-wrong-alias-record-map" "${TMP_DIR}/f13-wrong-alias-record-category"; then
    printf '%s\n' 'F13 oracle bug: release-PR producer accepted the prior R1 tuple for the R2 alias source' >&2
    exit 2
  fi
  game_checkpoint_snapshot_record="${f13_active_alias_record}"
fi
if f13_master_merge_push_enabled \
  "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_REF:-}"; then
  f13_master_merge_push=true
fi
if f13_f54_production_applicable || [[ "${f13_master_release_pr}" == true || "${f13_master_merge_push}" == true || "${native_p3_master_pr}" == true ]] || \
  f13_develop_aggregate_enabled "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_REF:-}"; then
  if f13_f54_production_applicable; then
    f13_f54_collect_production_master_unit "${ROOT}" "${VOICE_R22_BASE_SHA:-}" HEAD \
      "${TMP_DIR}/f13-contributions" "${TMP_DIR}/f13-failure-category" || f13_collect_status=$?
  elif [[ "${f13_master_release_pr}" == true || "${f13_master_merge_push}" == true || "${native_p3_master_pr}" == true ]]; then
    if [[ "${f13_master_merge_push}" == true ]]; then
      f13_collect_master_push_contributions \
        "${ROOT}" "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_REF:-}" \
        "${VOICE_CI_PUSH_BEFORE_SHA:-}" "${VOICE_R22_BASE_SHA:-}" HEAD \
        "${TMP_DIR}/f13-contributions" "${TMP_DIR}/f13-failure-category" || f13_collect_status=$?
    else
      f13_collect_master_release_pr_contributions \
        "${ROOT}" "${VOICE_R22_BASE_SHA:-}" HEAD "${TMP_DIR}/f13-contributions" "${TMP_DIR}/f13-failure-category" || f13_collect_status=$?
    fi
  else
    f13_collect_develop_contributions \
      "${ROOT}" "${base_sha}" HEAD "${TMP_DIR}/f13-contributions" "${TMP_DIR}/f13-failure-category" || f13_collect_status=$?
  fi
  if [[ "${f13_collect_status}" != 0 ]]; then
    f13_failure_category=unknown
    if [[ -s "${TMP_DIR}/f13-failure-category" ]]; then
      case "$(<"${TMP_DIR}/f13-failure-category")" in
        base_object|target_object|base_ancestry|first_parent_range|parent_count|source_base|source_delta|\
        push_before_mismatch|\
        sync_merge_resolution_inspection|sync_merge_guarded_resolution|\
        source_tree_entry|merge_tree_entry|merge_tree_mismatch|target_tree_entry|guarded_target_mismatch|checkpoint_post_h_owner|\
        duplicate_guarded_owner|nested_contribution_missing|release_pr_missing_base|release_pr_shape|release_pr_base_parent|\
        release_pr_source_delta|release_pr_result_delta|release_pr_delta_mismatch|\
        release_pr_merge_tree_mismatch|release_pr_map_write)
          f13_failure_category="$(<"${TMP_DIR}/f13-failure-category")" ;;
      esac
    fi
    printf 'F13 collector failure: category=%s base_origin=%s\n' \
      "${f13_failure_category}" "$(f13_base_origin_class)" >&2
    printf '%s\n' 'F13: develop contribution provenance is missing or ambiguous' >&2
    exit 1
  fi
  native_p3_audit_map || {
    printf '%s\n' 'F13: complete native source producer map rejected' >&2
    exit 1
  }
  if [[ "${f13_collect_status}" == 0 && "${f13_master_release_pr}" == true ]] &&
    ! f13_migration_alias_wrapper_result_fixture "${ROOT}" "${TMP_DIR}/f13-contributions"; then
    printf '%s\n' 'F13 oracle bug: release-PR alias ledger failed wrapper/downstream provenance checks' >&2
    exit 2
  fi
fi

{
  git -C "${ROOT}" diff --name-only "${base_sha}" --
  git -C "${ROOT}" ls-files --others --exclude-standard
} | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u >"${TMP_DIR}/changed-files"

while IFS= read -r file; do
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    continue
  fi
  game_checkpoint_path_allowed "${file}" "${TMP_DIR}/changed-files" || \
    be255_checkpoint_path_authorized_in_delta "${file}" "${TMP_DIR}/changed-files" "$(git -C "${ROOT}" hash-object --path="${file}" "${ROOT}/${file}" 2>/dev/null || true)" || \
    voice_room_grants_checkpoint_allowed "${file}" "${TMP_DIR}/changed-files" || \
    printf '%s\n' "${file}"
done <"${TMP_DIR}/changed-files" >"${TMP_DIR}/unapproved-runtime-delta"
if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
  awk -F '\t' '$2 == "true" { print $1 }' "${TMP_DIR}/f13-contributions" \
    | LC_ALL=C sort -u >"${TMP_DIR}/unapproved-runtime-delta"
fi
if [[ -n "${f13_f54_collection_file:-}" ]]; then
  python3 "${f13_f54_observer}" collection-runtime-paths "${ROOT}" "${f13_f54_collection_file}" "${f13_f54_collection_sha}" \
    "${TMP_DIR}/f13-contributions" >"${TMP_DIR}/unapproved-runtime-delta" || exit 2
fi
r22_runtime_delta=false
if voice_r22_runtime_changed <"${TMP_DIR}/unapproved-runtime-delta"; then
  r22_runtime_delta=true
fi

f13_runtime_for_path() {
  local path="$1" f54_status=0 f54_value
  if [[ -n "${f13_f54_collection_file:-}" ]]; then
    f54_value="$(f13_f54_router_request runtime "${path}")" || f54_status=$?
    if [[ "${f54_status}" == 0 ]]; then printf '%s\n' "${f54_value}"; return 0; fi
    [[ "${f54_status}" == 3 ]] || return 2
  fi
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    awk -F '\t' -v path="${path}" '$1 == path { value=$2; found=1 } END { if (!found) exit 1; print value }' \
      "${TMP_DIR}/f13-contributions"
  else
    printf '%s\n' "${r22_runtime_delta}"
  fi
}

f13_retained_history_row() {
  local repo="$1" map="$2" path="$3"
  [[ -n "${f13_f54_collection_file:-}" && "${repo}" == "${ROOT}" &&
    "${map}" == "${TMP_DIR}/f13-contributions" ]] || return 1
  f13_f54_router_request history "${path}"
}

f13_f54_manual_delta_for_path() {
  local repo="$1" map="$2" path="$3"
  [[ -n "${f13_f54_collection_file:-}" && "${map}" == "${TMP_DIR}/f13-contributions" ]] || return 2
  [[ "${repo}" == "${ROOT}" ]] || return 2
  f13_f54_router_request manual "${path}" >"${TMP_DIR}/f13-current-delta" || return 2
  printf '%s\n' "${TMP_DIR}/f13-current-delta"
}

f13_delta_for_path() {
  local repo="$1" map="$2" path="$3" row commit source_base source_head
  if [[ -s "${map}" ]]; then
    row="$(awk -F '\t' -v path="${path}" '$1 == path { value=$0 } END { if (value != "") print value }' "${map}")"
    if [[ -z "${row}" ]]; then
      row="$(f13_retained_history_row "${repo}" "${map}" "${path}")" || return 1
    fi
    IFS=$'\t' read -r _ _ commit _ source_base source_head _ _ <<<"${row}"
    [[ -n "${commit}" && -n "${source_base}" && -n "${source_head}" ]] || return 1
    if [[ "${source_head}" == fa1528a7c232a644218edf49811e2bb2e373105f ]]; then
      f13_f54_manual_delta_for_path "${repo}" "${map}" "${path}"
      return $?
    fi
    git -C "${repo}" diff --name-only "${source_base}" "${source_head}" >"${TMP_DIR}/f13-current-delta" || return 1
    grep -Fxq -- "${path}" "${TMP_DIR}/f13-current-delta" || return 1
    printf '%s\n' "${TMP_DIR}/f13-current-delta"
  else
    printf '%s\n' "${TMP_DIR}/changed-files"
  fi
}

f13_source_head_for_path() {
  local repo="$1" map="$2" path="$3" row source_head
  if [[ -s "${map}" ]]; then
    row="$(awk -F '\t' -v path="${path}" '$1 == path { value=$0 } END { if (value != "") print value }' "${map}")"
    if [[ -z "${row}" ]]; then
      row="$(f13_retained_history_row "${repo}" "${map}" "${path}")" || return 1
    fi
    IFS=$'\t' read -r _ _ _ _ _ source_head _ _ <<<"${row}"
    [[ -n "${source_head}" ]] || return 1
    printf '%s\n' "${source_head}"
  else
    printf '%s\n' HEAD
  fi
}

game_checkpoint_path_allowed_for_provenance() {
  local repo="$1" map="$2" path="$3" row approval_map approval_row row_count approval_count
  local row_hash approved_hash approved_path approved_base approved_head approved_target
  local approved_snapshot_base approved_snapshot_head approved_record checkpoint_entry target_entry
  local owner_commit owner_base owner_head
  game_checkpoint_domain_path "${path}" || return 1
  [[ -s "${map}" ]] || return 1
  [[ "${game_checkpoint_aggregate_map}" == "${map}" &&
    -n "${game_checkpoint_aggregate_base}" && -n "${game_checkpoint_aggregate_head}" &&
    -n "${game_checkpoint_aggregate_target}" && -n "${game_checkpoint_aggregate_record}" &&
    -s "${game_checkpoint_aggregate_paths}" ]] || return 1
  grep -Fxq -- "${path}" "${game_checkpoint_aggregate_paths}" || return 1
  row="$(awk -F '\t' -v path="${path}" '$1 == path { value = $0 } END { if (value != "") print value }' "${map}")"
  [[ -n "${row}" ]] || return 1
  row_count="$(awk -F '\t' -v path="${path}" '$1 == path { count++ } END { print count+0 }' "${map}")"
  [[ "${row_count}" == 1 ]] || return 1
  IFS=$'\t' read -r _ _ owner_commit _ owner_base owner_head _ _ <<<"${row}"
  [[ -n "${owner_commit}" && -n "${owner_base}" && -n "${owner_head}" ]] || return 1
  git -C "${repo}" merge-base --is-ancestor "${owner_commit}" "${game_checkpoint_aggregate_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${owner_base}" "${owner_head}" 2>/dev/null || return 1
  git -C "${repo}" merge-base --is-ancestor "${owner_head}" "${game_checkpoint_snapshot_head}" 2>/dev/null || return 1
  row_hash="$(printf '%s\n' "${row}" | git -C "${repo}" hash-object --stdin 2>/dev/null)" || return 1
  approval_map="${map}.game-snapshot-approvals"
  [[ -s "${approval_map}" ]] || return 1
  approval_row="$(awk -F '\t' -v path="${path}" '$1 == path { value = $0 } END { if (value != "") print value }' "${approval_map}")"
  approval_count="$(awk -F '\t' -v path="${path}" '$1 == path { count++ } END { print count+0 }' "${approval_map}")"
  [[ "${approval_count}" == 1 && -n "${approval_row}" ]] || return 1
  IFS=$'\t' read -r approved_path approved_hash approved_base approved_head approved_target \
    approved_snapshot_base approved_snapshot_head approved_record <<<"${approval_row}"
  [[ "${approved_path}" == "${path}" && "${approved_hash}" == "${row_hash}" &&
    "${approved_base}" == "${game_checkpoint_aggregate_base}" &&
    "${approved_head}" == "${game_checkpoint_aggregate_head}" &&
    "${approved_target}" == "${game_checkpoint_aggregate_target}" &&
    "${approved_snapshot_base}" == "${game_checkpoint_snapshot_base}" &&
    "${approved_snapshot_head}" == "${game_checkpoint_snapshot_head}" &&
    "${approved_record}" == "${game_checkpoint_aggregate_record}" &&
    "${approved_record}" == "${game_checkpoint_snapshot_record}" ]] || return 1
  checkpoint_entry="$(git -C "${repo}" ls-tree "${approved_snapshot_head}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  target_entry="$(git -C "${repo}" ls-tree "${approved_target}" -- ":(literal)${path}" 2>/dev/null)" || return 1
  [[ "${checkpoint_entry}" == "${target_entry}" ]]
}

# Drive the status-2 taint and later exact-H owner through the real release-PR
# wrapper. This protects the producer map, not just the supersession predicate.
f13_checkpoint_duplicate_owner_producer_fixture() {
  local repo path deleted_path base left_owner right_owner integration checkpoint record source_head target
  local missing_head missing_target wrong_head wrong_target map category row_count row row_owner row_head
  repo="${TMP_DIR}/f13-checkpoint-conflict-owner-producer"
  path='protos/voice/fixture/v1/fixture.proto'
  mkdir -p "${repo}"
  git -C "${repo}" init -q || return 1
  git -C "${repo}" config user.name 'Voice CI fixture'
  git -C "${repo}" config user.email 'voice-ci@example.invalid'
  git -C "${repo}" config commit.gpgsign false
  mkdir -p "${repo}/$(dirname "${path}")"
  deleted_path='src/backend/role/internal/fixture/obsolete.go'
  mkdir -p "${repo}/$(dirname "${deleted_path}")"
  printf '%s\n' 'producer baseline' >"${repo}/${path}"
  printf '%s\n' 'deleted by approved checkpoint' >"${repo}/${deleted_path}"
  git -C "${repo}" add -- "${path}" "${deleted_path}" &&
    git -C "${repo}" commit -qm 'F13 conflict-owner producer base' || return 1
  base="$(git -C "${repo}" rev-parse HEAD)" || return 1

  git -C "${repo}" checkout -qb producer-left "${base}" || return 1
  printf '%s\n' 'left contribution' >"${repo}/${path}"
  git -C "${repo}" add -- "${path}" &&
    git -C "${repo}" commit -qm 'F13 conflict-owner left contribution' || return 1
  left_owner="$(git -C "${repo}" rev-parse HEAD)" || return 1
  git -C "${repo}" checkout -qb producer-right "${base}" || return 1
  printf '%s\n' 'right contribution' >"${repo}/${path}"
  git -C "${repo}" add -- "${path}" &&
    git -C "${repo}" commit -qm 'F13 conflict-owner right contribution' || return 1
  right_owner="$(git -C "${repo}" rev-parse HEAD)" || return 1

  git -C "${repo}" checkout -q producer-left || return 1
  if git -C "${repo}" merge --no-commit producer-right >/dev/null 2>&1; then
    printf '%s\n' 'F13 oracle bug: guarded owner fixture merge unexpectedly had no conflict' >&2
    return 1
  fi
  printf '%s\n' 'temporary conflict resolution' >"${repo}/${path}"
  git -C "${repo}" add -- "${path}" &&
    git -C "${repo}" commit -qm 'F13 pre-H guarded conflict integration' || return 1
  integration="$(git -C "${repo}" rev-parse HEAD)" || return 1

  # H is a later real source commit which restores the approved path entry.
  printf '%s\n' 'approved checkpoint entry' >"${repo}/${path}"
  rm "${repo}/${deleted_path}"
  git -C "${repo}" add -A -- "${path}" "${deleted_path}" &&
    git -C "${repo}" commit -qm 'F13 exact-H guarded owner restore' || return 1
  checkpoint="$(git -C "${repo}" rev-parse HEAD)" || return 1
  record="$(git -C "${repo}" commit-tree "${checkpoint}^{tree}" -p "${checkpoint}" \
    <<< 'F13 immutable checkpoint record')" || return 1
  git -C "${repo}" checkout -qb producer-approved "${record}" || return 1
  printf '%s\n' 'ordinary metadata follow-up' >"${repo}/README.md"
  git -C "${repo}" add -- README.md &&
    git -C "${repo}" commit -qm 'F13 approved source head after checkpoint record' || return 1
  source_head="$(git -C "${repo}" rev-parse HEAD)" || return 1
  target="$(git -C "${repo}" commit-tree "${source_head}^{tree}" -p "${base}" \
    -p "${source_head}" <<< 'F13 producer-shaped release PR result')" || return 1

  game_checkpoint_snapshot_base="${base}"
  game_checkpoint_snapshot_head="${checkpoint}"
  game_checkpoint_snapshot_record="${record}"
  map="${TMP_DIR}/f13-checkpoint-conflict-owner-producer-map"
  category="${TMP_DIR}/f13-checkpoint-conflict-owner-producer-category"
  f13_collect_master_release_pr_contributions "${repo}" "${base}" "${target}" \
    "${map}" "${category}" || {
    printf 'F13 oracle bug: producer rejected qualified conflict followed by exact-H owner (category=%s)\n' \
      "$(<"${category}")" >&2
    return 1
  }
  row_count="$(awk -F '\t' -v candidate="${path}" \
    '$1 == candidate { count++ } END { print count+0 }' "${map}")"
  row="$(awk -F '\t' -v candidate="${path}" \
    '$1 == candidate { value=$0 } END { print value }' "${map}")"
  IFS=$'\t' read -r _ _ row_owner _ _ row_head _ _ <<<"${row}"
  [[ "${row_count}" == 1 && "${row_owner}" == "${checkpoint}" &&
    "${row_head}" == "${checkpoint}" ]] || {
    printf '%s\n' 'F13 oracle bug: producer map did not retain the actual exact-H owner row' >&2
    return 1
  }
  game_checkpoint_path_allowed_for_provenance "${repo}" "${map}" "${path}" || {
    printf '%s\n' 'F13 oracle bug: producer H-restored row failed pinned provenance authorization' >&2
    return 1
  }
  row_count="$(awk -F '\t' -v candidate="${deleted_path}" \
    '$1 == candidate { count++ } END { print count+0 }' "${map}")"
  [[ "${row_count}" == 1 ]] &&
    [[ -z "$(git -C "${repo}" ls-tree "${checkpoint}" -- ":(literal)${deleted_path}")" ]] &&
    [[ -z "$(git -C "${repo}" ls-tree "${target}" -- ":(literal)${deleted_path}")" ]] &&
    game_checkpoint_path_allowed_for_provenance "${repo}" "${map}" "${deleted_path}" || {
    printf '%s\n' 'F13 oracle bug: producer rejected the exact checkpoint deletion/empty tree entry' >&2
    return 1
  }
  cp "${map}" "${TMP_DIR}/f13-checkpoint-conflict-owner-producer-map-saved"
  awk -F '\t' -v candidate="${deleted_path}" '$1 != candidate' \
    "${TMP_DIR}/f13-checkpoint-conflict-owner-producer-map-saved" >"${map}" || return 1
  if game_checkpoint_path_allowed_for_provenance "${repo}" "${map}" "${deleted_path}"; then
    printf '%s\n' 'F13 oracle bug: missing deletion owner row was treated as an absent tree entry' >&2
    return 1
  fi
  cp "${TMP_DIR}/f13-checkpoint-conflict-owner-producer-map-saved" "${map}" || return 1

  # Without the later H owner, the same temporary conflict must remain tainted
  # and the release-PR producer must reject it.
  git -C "${repo}" checkout -qb producer-missing-restore "${integration}" || return 1
  printf '%s\n' 'missing-restore metadata' >"${repo}/README.md"
  git -C "${repo}" add -- README.md &&
    git -C "${repo}" commit -qm 'F13 source without exact-H restoration' || return 1
  missing_head="$(git -C "${repo}" rev-parse HEAD)" || return 1
  missing_target="$(git -C "${repo}" commit-tree "${missing_head}^{tree}" -p "${base}" \
    -p "${missing_head}" <<< 'F13 missing-restore PR result')" || return 1
  if f13_collect_master_release_pr_contributions "${repo}" "${base}" "${missing_target}" \
    "${TMP_DIR}/f13-checkpoint-missing-owner-map" \
    "${TMP_DIR}/f13-checkpoint-missing-owner-category"; then
    printf '%s\n' 'F13 oracle bug: missing H restoration passed the release-PR producer' >&2
    return 1
  fi

  # A wrong value after the conflict also remains rejected, even though it is
  # a real pre-H owner and the synthetic PR tree matches that source head.
  git -C "${repo}" checkout -qb producer-wrong-restore "${integration}" || return 1
  printf '%s\n' 'wrong checkpoint restoration' >"${repo}/${path}"
  git -C "${repo}" add -- "${path}" &&
    git -C "${repo}" commit -qm 'F13 wrong guarded restoration' || return 1
  wrong_head="$(git -C "${repo}" rev-parse HEAD)" || return 1
  wrong_target="$(git -C "${repo}" commit-tree "${wrong_head}^{tree}" -p "${base}" \
    -p "${wrong_head}" <<< 'F13 wrong-restore PR result')" || return 1
  if f13_collect_master_release_pr_contributions "${repo}" "${base}" "${wrong_target}" \
    "${TMP_DIR}/f13-checkpoint-wrong-owner-map" \
    "${TMP_DIR}/f13-checkpoint-wrong-owner-category"; then
    printf '%s\n' 'F13 oracle bug: wrong H restoration passed the release-PR producer' >&2
    return 1
  fi
}

f13_run_synthetic_provenance_fixture_body() {
  # Bash locals are dynamically scoped, so the synthetic collectors below see
  # isolated checkpoint state. No fixture may target the real producer output
  # or its authorization sidecars.
  local game_checkpoint_snapshot_base="${game_checkpoint_snapshot_base}"
  local game_checkpoint_snapshot_head="${game_checkpoint_snapshot_head}"
  local game_checkpoint_snapshot_record="${game_checkpoint_snapshot_record}"
  local game_checkpoint_aggregate_active="${game_checkpoint_aggregate_active}"
  local game_checkpoint_aggregate_base="${game_checkpoint_aggregate_base}"
  local game_checkpoint_aggregate_head="${game_checkpoint_aggregate_head}"
  local game_checkpoint_aggregate_target="${game_checkpoint_aggregate_target}"
  local game_checkpoint_aggregate_paths="${game_checkpoint_aggregate_paths}"
  local game_checkpoint_aggregate_map="${game_checkpoint_aggregate_map}"
  local game_checkpoint_aggregate_record="${game_checkpoint_aggregate_record}"
  local game_checkpoint_aggregate_alias_map="${game_checkpoint_aggregate_alias_map}"
  local f13_bot_only_delta

  f13_game_checkpoint_projection_fixtures
  f13_checkpoint_duplicate_owner_producer_fixture || exit 2

  git -C "${f13_fixture_repo}" checkout -qb f13-bot-only-integration "${f13_fixture_base}"
  git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture unrelated-only merge' f13-bot-source
  f13_collect_develop_contributions \
    "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-bot-only-map" || {
    printf '%s\n' 'F13 oracle bug: unrelated-only contribution fixture was rejected' >&2
    exit 2
  }
  f13_bot_only_delta="$(f13_delta_for_path \
    "${f13_fixture_repo}" "${TMP_DIR}/f13-bot-only-map" 'protos/voice/bot/v1/bot.proto')" || {
    printf '%s\n' 'F13 oracle bug: unrelated contribution lost its own delta' >&2
    exit 2
  }
  [[ "$(cat "${f13_bot_only_delta}")" == 'protos/voice/bot/v1/bot.proto' ]] || {
    printf '%s\n' 'F13 oracle bug: contribution delta included paths from another contribution' >&2
    exit 2
  }
  git -C "${f13_fixture_repo}" checkout -qb f13-direct-contribution "${f13_fixture_base}"
  mkdir -p "${f13_fixture_repo}/src/backend/voice"
  printf '%s\n' 'package main' >"${f13_fixture_repo}/src/backend/voice/direct.go"
  git -C "${f13_fixture_repo}" add src/backend/voice/direct.go
  git -C "${f13_fixture_repo}" commit -qm 'F13 fixture direct contribution'
  f13_collect_develop_contributions \
    "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-direct-map" || {
    printf '%s\n' 'F13 oracle bug: direct commit contribution fixture was rejected' >&2
    exit 2
  }
  grep -Fqx $'src/backend/voice/direct.go\ttrue' <(cut -f1-2 "${TMP_DIR}/f13-direct-map") || {
    printf '%s\n' 'F13 oracle bug: direct commit delta lost its runtime trigger' >&2
    exit 2
  }

}

f13_run_synthetic_provenance_fixtures() {
  local expected_guarded_merge_state
  expected_guarded_merge_state="$(f13_guarded_merge_state_fingerprint)" || return 1
  local expected_native_state=''
  if [[ "${native_p3_active:-false}" == true ]]; then
    expected_native_state="$(native_p3_state_fingerprint)" || return 1
  fi
  local expected_snapshot_base="${game_checkpoint_snapshot_base}"
  local expected_snapshot_head="${game_checkpoint_snapshot_head}"
  local expected_snapshot_record="${game_checkpoint_snapshot_record}"
  local expected_aggregate_active="${game_checkpoint_aggregate_active}"
  local expected_aggregate_base="${game_checkpoint_aggregate_base}"
  local expected_aggregate_head="${game_checkpoint_aggregate_head}"
  local expected_aggregate_target="${game_checkpoint_aggregate_target}"
  local expected_aggregate_paths="${game_checkpoint_aggregate_paths}"
  local expected_aggregate_map="${game_checkpoint_aggregate_map}"
  local expected_aggregate_record="${game_checkpoint_aggregate_record}"
  local expected_aggregate_alias_map="${game_checkpoint_aggregate_alias_map}"
  local expected_paths_hash expected_map_hash expected_row_approvals_hash expected_alias_approvals_hash
  local expected_paths_present=false expected_map_present=false
  local expected_row_approvals_present=false expected_alias_approvals_present=false
  local f13_state_path f13_real_alias_context=false

  if [[ -n "${game_checkpoint_aggregate_paths}" && -f "${game_checkpoint_aggregate_paths}" ]]; then
    expected_paths_present=true
    expected_paths_hash="$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_paths}")" || return 1
  fi
  if [[ -n "${game_checkpoint_aggregate_map}" && -f "${game_checkpoint_aggregate_map}" ]]; then
    expected_map_present=true
    expected_map_hash="$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_map}")" || return 1
  fi
  if [[ -n "${game_checkpoint_aggregate_map}" && -f "${game_checkpoint_aggregate_map}.game-snapshot-approvals" ]]; then
    expected_row_approvals_present=true
    expected_row_approvals_hash="$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_map}.game-snapshot-approvals")" || return 1
  fi
  if [[ -n "${game_checkpoint_aggregate_alias_map}" && -f "${game_checkpoint_aggregate_alias_map}" ]]; then
    expected_alias_approvals_present=true
    expected_alias_approvals_hash="$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_alias_map}")" || return 1
  fi

  # The five production-consumer assertions apply only when the current event
  # actually produced the complete two-row R2 alias ledger for this aggregate.
  if [[ "${f13_master_release_pr:-false}" == true &&
    "${game_checkpoint_aggregate_map}" == "${TMP_DIR}/f13-contributions" &&
    "${game_checkpoint_aggregate_alias_map}" == "${game_checkpoint_aggregate_map}.game-snapshot-alias-approvals" &&
    -f "${game_checkpoint_aggregate_paths}" && -f "${game_checkpoint_aggregate_map}" &&
    -f "${game_checkpoint_aggregate_map}.game-snapshot-approvals" &&
    -f "${game_checkpoint_aggregate_alias_map}" &&
    "${game_checkpoint_snapshot_base}" == "${game_checkpoint_aggregate_base}" &&
    "${game_checkpoint_snapshot_record}" == "${game_checkpoint_aggregate_record}" &&
    "$(awk 'END { print NR+0 }' "${game_checkpoint_aggregate_alias_map}")" == 2 ]] &&
    grep -Fxq -- 'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' "${game_checkpoint_aggregate_paths}" &&
    grep -Fxq -- 'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql' "${game_checkpoint_aggregate_paths}" &&
    awk -F '\t' '
      $1 == "src/backend/migrations/voice_db/000011_match_squad_operations.up.sql" { up++ }
      $1 == "src/backend/migrations/voice_db/000011_match_squad_operations.down.sql" { down++ }
      $1 != "src/backend/migrations/voice_db/000011_match_squad_operations.up.sql" &&
      $1 != "src/backend/migrations/voice_db/000011_match_squad_operations.down.sql" { bad=1 }
      END { exit !(up == 1 && down == 1 && !bad) }
    ' "${game_checkpoint_aggregate_alias_map}"; then
    f13_real_alias_context=true
    for f13_state_path in \
      'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
      'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql'; do
      game_checkpoint_alias_path_allowed_for_provenance \
        "${ROOT}" "${game_checkpoint_aggregate_map}" "${f13_state_path}" || {
        printf 'F13 oracle bug: production alias authorization failed before synthetic fixtures: %s\n' \
          "${f13_state_path}" >&2
        return 1
      }
    done
    for f13_state_path in \
      'src/backend/voice/internal/grpcsvc/matchsquad_member.go' \
      'src/backend/voice/internal/grpcsvc/matchsquad_voice.go' \
      'src/backend/voice/internal/livekit/token.go'; do
      game_checkpoint_path_allowed_for_provenance \
        "${ROOT}" "${game_checkpoint_aggregate_map}" "${f13_state_path}" || {
        printf 'F13 oracle bug: production ordinary Game provenance failed before synthetic fixtures: %s\n' \
          "${f13_state_path}" >&2
        return 1
      }
    done
  fi

  f13_run_synthetic_provenance_fixture_body

  [[ "$(f13_guarded_merge_state_fingerprint)" == "${expected_guarded_merge_state}" ]] || {
    printf '%s\n' 'F13 oracle bug: fixtures changed guarded merge authority/history' >&2
    return 1
  }

  if [[ -n "${expected_native_state}" ]]; then
    [[ "$(native_p3_state_fingerprint)" == "${expected_native_state}" ]] || {
      printf '%s\n' 'F13 oracle bug: fixtures changed native source authority/history' >&2
      return 1
    }
  fi

  [[ "${game_checkpoint_snapshot_base}" == "${expected_snapshot_base}" &&
    "${game_checkpoint_snapshot_head}" == "${expected_snapshot_head}" &&
    "${game_checkpoint_snapshot_record}" == "${expected_snapshot_record}" &&
    "${game_checkpoint_aggregate_active}" == "${expected_aggregate_active}" &&
    "${game_checkpoint_aggregate_base}" == "${expected_aggregate_base}" &&
    "${game_checkpoint_aggregate_head}" == "${expected_aggregate_head}" &&
    "${game_checkpoint_aggregate_target}" == "${expected_aggregate_target}" &&
    "${game_checkpoint_aggregate_paths}" == "${expected_aggregate_paths}" &&
    "${game_checkpoint_aggregate_map}" == "${expected_aggregate_map}" &&
    "${game_checkpoint_aggregate_record}" == "${expected_aggregate_record}" &&
    "${game_checkpoint_aggregate_alias_map}" == "${expected_aggregate_alias_map}" ]] || {
    printf '%s\n' 'F13 oracle bug: synthetic fixtures changed the active release checkpoint context' >&2
    return 1
  }
  if [[ "${expected_paths_present}" == true ]]; then
    [[ -f "${game_checkpoint_aggregate_paths}" &&
      "$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_paths}")" == "${expected_paths_hash}" ]] || {
      printf '%s\n' 'F13 oracle bug: synthetic fixtures changed the production projection path list' >&2
      return 1
    }
  elif [[ -n "${game_checkpoint_aggregate_paths}" && -e "${game_checkpoint_aggregate_paths}" ]]; then
    printf '%s\n' 'F13 oracle bug: synthetic fixtures created a production projection path list' >&2
    return 1
  fi
  if [[ "${expected_map_present}" == true ]]; then
    [[ -f "${game_checkpoint_aggregate_map}" &&
      "$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_map}")" == "${expected_map_hash}" ]] || {
      printf '%s\n' 'F13 oracle bug: synthetic fixtures changed the production contribution map' >&2
      return 1
    }
  elif [[ -n "${game_checkpoint_aggregate_map}" && -e "${game_checkpoint_aggregate_map}" ]]; then
    printf '%s\n' 'F13 oracle bug: synthetic fixtures created a production contribution map' >&2
    return 1
  fi
  if [[ "${expected_row_approvals_present}" == true ]]; then
    [[ -f "${game_checkpoint_aggregate_map}.game-snapshot-approvals" &&
      "$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_map}.game-snapshot-approvals")" == "${expected_row_approvals_hash}" ]] || {
      printf '%s\n' 'F13 oracle bug: synthetic fixtures changed the production row approval map' >&2
      return 1
    }
  elif [[ -n "${game_checkpoint_aggregate_map}" && -e "${game_checkpoint_aggregate_map}.game-snapshot-approvals" ]]; then
    printf '%s\n' 'F13 oracle bug: synthetic fixtures created a production row approval map' >&2
    return 1
  fi
  if [[ "${expected_alias_approvals_present}" == true ]]; then
    [[ -f "${game_checkpoint_aggregate_alias_map}" &&
      "$(git -C "${ROOT}" hash-object -- "${game_checkpoint_aggregate_alias_map}")" == "${expected_alias_approvals_hash}" ]] || {
      printf '%s\n' 'F13 oracle bug: synthetic fixtures changed the production alias ledger' >&2
      return 1
    }
  elif [[ -n "${game_checkpoint_aggregate_alias_map}" && -e "${game_checkpoint_aggregate_alias_map}" ]]; then
    printf '%s\n' 'F13 oracle bug: synthetic fixtures created a production alias ledger' >&2
    return 1
  fi
  if [[ "${f13_real_alias_context}" == true ]]; then
    [[ "${game_checkpoint_aggregate_map}" == "${TMP_DIR}/f13-contributions" &&
      "${game_checkpoint_aggregate_alias_map}" == "${game_checkpoint_aggregate_map}.game-snapshot-alias-approvals" &&
      -f "${game_checkpoint_aggregate_map}" && -f "${game_checkpoint_aggregate_alias_map}" &&
      "${game_checkpoint_snapshot_base}" == "${game_checkpoint_aggregate_base}" &&
      "${game_checkpoint_snapshot_record}" == "${game_checkpoint_aggregate_record}" ]] || {
      printf '%s\n' 'F13 oracle bug: active release output/alias ledger was not preserved after synthetic fixtures' >&2
      return 1
    }
    for f13_state_path in \
      'src/backend/migrations/voice_db/000011_match_squad_operations.up.sql' \
      'src/backend/migrations/voice_db/000011_match_squad_operations.down.sql'; do
      game_checkpoint_alias_path_allowed_for_provenance \
        "${ROOT}" "${game_checkpoint_aggregate_map}" "${f13_state_path}" || {
        printf 'F13 oracle bug: preserved release alias authorization failed after fixtures: %s\n' \
          "${f13_state_path}" >&2
        return 1
      }
    done
    for f13_state_path in \
      'src/backend/voice/internal/grpcsvc/matchsquad_member.go' \
      'src/backend/voice/internal/grpcsvc/matchsquad_voice.go' \
      'src/backend/voice/internal/livekit/token.go'; do
      game_checkpoint_path_allowed_for_provenance \
        "${ROOT}" "${game_checkpoint_aggregate_map}" "${f13_state_path}" || {
        printf 'F13 oracle bug: preserved ordinary Game provenance failed after fixtures: %s\n' \
          "${f13_state_path}" >&2
        return 1
      }
    done
  fi
  if [[ "${f13_real_alias_context}" == true ]]; then
    printf '%s\n' 'F13 producer checkpoint context survives synthetic fixtures: PASS' >&2
  fi
}

# The producer-shaped fixtures also call this provenance helper. Invoke them
# only after every collector and path-authorization function is defined.
f13_f54_router_prepare || exit 2
f13_run_synthetic_provenance_fixtures

f13_f54_run_original_router() {
  local f13_original_path_list="$1"
while IFS= read -r file; do
  f13_f54_router_begin "${file}" || exit 2
  f13_f54_router_failures_before="${failures}"
  if native_p3_path_allowed_for_provenance "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}"; then
    f13_f54_router_gate native_p3_path_allowed_for_provenance 0 "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}" || exit 2
    continue
  fi
  if game_checkpoint_alias_path_allowed_for_provenance \
    "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}"; then
    f13_f54_router_gate game_checkpoint_alias_path_allowed_for_provenance 0 "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}" || exit 2
    continue
  fi
  f13_path_delta="${TMP_DIR}/changed-files"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    f13_path_delta="$(f13_delta_for_path "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}")" || {
      if f13_path_is_guarded "${file}"; then
        fail "F13: changed guarded path is absent from its contribution delta: ${file}"
      fi
      f13_path_delta="${TMP_DIR}/changed-files"
    }
  fi
  f13_path_target="HEAD"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    f13_path_target="$(f13_source_head_for_path "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}")" || {
      if f13_path_is_guarded "${file}"; then
        fail "F13: changed guarded path has no source revision: ${file}"
      fi
      f13_path_target=HEAD
    }
  fi
  if game_checkpoint_path_allowed_for_provenance "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}"; then
    f13_f54_router_gate game_checkpoint_path_allowed_for_provenance 0 "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}" || exit 2
    continue
  fi
  file_r22_runtime_delta="${r22_runtime_delta}"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    file_r22_runtime_delta="$(f13_runtime_for_path "${file}")" || {
      [[ -z "${f13_f54_collection_file:-}" ]] || exit 2
      if f13_path_is_guarded "${file}"; then
        fail "F13: changed guarded path has no unique contribution provenance: ${file}"
      fi
      file_r22_runtime_delta=false
    }
  fi
  if game_checkpoint_path_allowed "${file}" "${f13_path_delta}"; then
    f13_f54_router_gate game_checkpoint_path_allowed 0 "${file}" "${f13_path_delta}" || exit 2
    continue
  fi
  actual_blob="$(git -C "${ROOT}" hash-object --path="${file}" "${ROOT}/${file}" 2>/dev/null || true)"
  if voice_room_grants_checkpoint_authorized_in_delta "${ROOT}" "${f13_path_target}" "${file}" "${f13_path_delta}" "${actual_blob}"; then
    f13_f54_router_gate voice_room_grants_checkpoint_authorized_in_delta 0 "${ROOT}" "${f13_path_target}" "${file}" "${f13_path_delta}" "${actual_blob}" || exit 2
    continue
  fi
  if be255_checkpoint_path_authorized_in_delta "${file}" "${f13_path_delta}" "${actual_blob}"; then
    f13_f54_router_gate be255_checkpoint_path_authorized_in_delta 0 "${file}" "${f13_path_delta}" "${actual_blob}" || exit 2
    continue
  fi
  if r23_contract_path_allowed_for_window "${r23_allowlist_enabled}" "${file}" "${f13_path_delta}"; then
    f13_f54_router_gate r23_contract_path_allowed_for_window 0 "${r23_allowlist_enabled}" "${file}" "${f13_path_delta}" || exit 2
    continue
  fi
  if t31_runtime_path_allowed "${file}"; then
    f13_f54_router_gate t31_runtime_path_allowed 0 "${file}" || exit 2
    continue
  fi
  if [[ "${file_r22_runtime_delta}" == true ]] && space_media_path_allowed "${file}"; then
    f13_f54_router_gate space_media_path_allowed 0 "${file}" || exit 2
    continue
  fi
  if ! f13_cross_scope_path_allowed "${file}" "${file_r22_runtime_delta}"; then
    case "${file}" in
      protos/*|*/pb/*|*.pb.go)
        fail "F13: proto/generated change is outside R22.2: ${file}" ;;
      src/backend/space/*|src/backend/role/*)
        fail "F13: Space/Role change is outside R22.2: ${file}" ;;
    esac
  fi
  case "${file}" in
    src/backend/voice/internal/grpcsvc/*.go|src/backend/voice/internal/store/*.go|src/backend/voice/internal/livekit/*.go|src/backend/voice/internal/s2s/*.go|src/backend/voice/internal/voiceevents/*.go)
      [[ "${file}" == *_test.go ]] || identity_publisher_path_allowed "${file}" || \
        voice_role_status_path_authorized_in_delta "${file}" "${f13_path_delta}" "${actual_blob}" || \
        account_voice_admission_path_authorized_in_delta "${file}" "${f13_path_delta}" "${actual_blob}" || \
        fail "F13: handler/Redis/external adapter change activates forbidden scope: ${file}"
      ;;
    src/backend/voice/internal/roomlifecycle/*.go)
      if [[ "${file}" != *_test.go ]]; then
        case "$(basename "${file}")" in
          lifecycle_store.go|postgres_store.go|postgres_decision.go|postgres_delivery.go) ;;
          *) fail "F13: unapproved lifecycle production surface: ${file}" ;;
        esac
      fi
      ;;
  esac
  if f13_voice_root_source_file "${file}"; then
      if [[ "${file}" != *_test.go ]]; then
        case "$(basename "${file}")" in
          main.go|health.go|database.go) ;;
          *) fail "F13: unapproved Voice runtime production file: ${file}" ;;
        esac
      fi
  fi
  f13_f54_router_status=0
  [[ "${failures}" == "${f13_f54_router_failures_before}" ]] || f13_f54_router_status=1
  f13_f54_router_gate original_remaining_path_policy "${f13_f54_router_status}" "${file}" "${file_r22_runtime_delta}" "${f13_path_target}" "${f13_path_delta}" "${actual_blob}" || exit 2
done <"${f13_original_path_list}"
}
f13_f54_run_original_router "${TMP_DIR}/changed-files"
if [[ -n "${f13_f54_collection_file:-}" ]]; then
  # Reverted/net-absent paths remain actual history obligations. Qualify them
  # explicitly with the same original policy/map/context; invent no endpoint row.
  python3 "${f13_f54_observer}" collection-retained-paths "${ROOT}" "${f13_f54_collection_file}" "${f13_f54_collection_sha}" \
    "${TMP_DIR}/f13-contributions" "${TMP_DIR}/changed-files" >"${TMP_DIR}/f54-retained-history-paths" || exit 2
  f13_f54_run_original_router "${TMP_DIR}/f54-retained-history-paths"
fi

while IFS= read -r file; do
  if game_checkpoint_path_allowed "${file}" "${TMP_DIR}/changed-files"; then
    continue
  fi
  if voice_room_grants_checkpoint_allowed "${file}" "${TMP_DIR}/changed-files"; then
    continue
  fi
  if t31_runtime_path_allowed "${file}"; then
    continue
  fi
  case "${file}" in
    src/backend/voice/main.go|src/backend/voice/health.go|src/backend/voice/database.go|src/backend/voice/internal/roomlifecycle/*.go)
      [[ "${file}" == *_test.go ]] && continue
      f13_changed_content "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}" >"${TMP_DIR}/changed-content"
      if forbidden_activation_content "${TMP_DIR}/changed-content"; then
        fail "F13: coordinator/bridge/publisher/handler activation added in ${file}"
      fi
      if [[ "${file}" == 'src/backend/voice/database.go' || "${file}" == src/backend/voice/internal/roomlifecycle/*.go ]] &&
        grep -Ein '(go-redis|nats\.go|internal/(grpcsvc|livekit|s2s|store|voiceevents))' "${TMP_DIR}/changed-content" >/dev/null; then
        fail "F13: storage/runtime factory adds a Redis/NATS/LiveKit/handler adapter in ${file}"
      fi
      ;;
  esac
done <"${TMP_DIR}/changed-files"

f13_f54_router_stop || exit 2

if ((failures > 0)); then
  printf 'Voice DB RED-F contract: %d failure(s)\n' "${failures}" >&2
  exit 1
fi

f13_wait_standalone_suites
echo 'Voice DB RED-F runtime/provisioning/docs contract: PASS'
