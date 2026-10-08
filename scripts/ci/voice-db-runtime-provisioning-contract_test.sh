#!/usr/bin/env bash
# RED-F oracle for Voice durable-store runtime, provisioning, deployment, docs,
# CI routing, and the source-disabled R22.2 cut. No Docker or cluster required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

failures=0

fail() {
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
  for dependency in ${ci_script_tests_dependencies}; do
    dependency_block="$(make_target_block "${dependency}")"
    if grep -Eq '^[[:space:]]+[^#].*scripts/staging/apply-migrate-jobs_test\.sh' \
        <<<"${dependency_block}"; then
      apply_contract_reachable=true
      break
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
expect_regex 'docs/DATA_STORES.md' '\*\*16\*\*.*(provision|созда|разв[её]р)' F12
expect_fixed 'src/backend/migrations/README.md' 'voice_db' F12
expect_fixed 'src/backend/migrations/README.md' 'compose-migrate-voice' F12
expect_fixed 'src/backend/migrations/README.md' 'compose-db-init' F12
expect_regex 'src/backend/migrations/README.md' 'compose-db-init.*(кажд|every|startup|запуск)' F12
expect_regex 'src/backend/migrations/README.md' '(DOWN|down).*(refus|отказ|запрещ|не удал)' F12
compose_migration_docs="$(awk '
  /^\*\*Compose \(all Go-owned DBs\):\*\*/ { in_block=1; next }
  in_block && /^\*\*E2E encryption/ { exit }
  in_block { print }
' "${ROOT}/src/backend/migrations/README.md")"
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

printf '%s\n' '== F13/G7: source-disabled scope boundary =='
base_sha="${VOICE_R22_BASE_SHA:-$(git -C "${ROOT}" merge-base HEAD origin/master)}"
# The later Game sprint authorizes the verified opt-in runtime checkpoint, not
# arbitrary future runtime or deployment changes. Exempt only its exact paths
# and Git blobs; the original R22.2 oracle still checks every remaining delta.
accepted_game_checkpoint='5ff441e04422e3ff1a4c0e1e7e81e3d17b8470eb'
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

f13_develop_aggregate_enabled() {
  [[ "${1:-}" == 'push' && "${2:-}" == 'refs/heads/develop' ]]
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
f13_collect_develop_contributions() {
  local repo="$1" base="$2" target="$3" output="$4"
  local commit parent_count first_parent source_head source_base delta
  local path source_entry result_entry target_entry blob trigger
  local -a commit_line
  local -A owners=()
  : >"${output}"
  git -C "${repo}" rev-parse --verify "${base}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" rev-parse --verify "${target}^{commit}" >/dev/null 2>&1 || return 1
  git -C "${repo}" merge-base --is-ancestor "${base}" "${target}" || return 1
  git -C "${repo}" rev-list --first-parent "${target}" | grep -Fxq -- "${base}" || return 1

  while IFS= read -r commit; do
    [[ -n "${commit}" ]] || continue
    read -r -a commit_line <<<"$(git -C "${repo}" rev-list --parents -n 1 "${commit}")"
    parent_count=$((${#commit_line[@]} - 1))
    ((parent_count == 1 || parent_count == 2)) || return 1
    first_parent="${commit_line[1]}"
    if ((parent_count == 2)); then
      source_head="${commit_line[2]}"
      source_base="$(git -C "${repo}" merge-base "${first_parent}" "${source_head}")" || return 1
      [[ -n "${source_base}" ]] || return 1
      git -C "${repo}" diff --name-only "${source_base}" "${source_head}" >"${TMP_DIR}/f13-source-delta" || return 1
    else
      source_head="${commit}"
      source_base="${first_parent}"
      git -C "${repo}" diff --name-only "${source_base}" "${source_head}" >"${TMP_DIR}/f13-source-delta" || return 1
    fi
    delta="${TMP_DIR}/f13-source-delta"

    : >"${TMP_DIR}/f13-unapproved-delta"
    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      source_entry="$(git -C "${repo}" ls-tree "${source_head}" -- "${path}")" || return 1
      if ((parent_count == 2)); then
        result_entry="$(git -C "${repo}" ls-tree "${commit}" -- "${path}")" || return 1
        [[ "${source_entry}" == "${result_entry}" ]] || return 1
      fi
      if f13_path_is_guarded "${path}"; then
        target_entry="$(git -C "${repo}" ls-tree "${target}" -- "${path}")" || return 1
        [[ "${source_entry}" == "${target_entry}" ]] || return 1
      fi
      blob="$(awk '{print $3}' <<<"${source_entry}")"
      if game_checkpoint_path_authorized_in_delta "${path}" "${delta}" "${blob}" || \
        be255_checkpoint_path_authorized_in_delta "${path}" "${delta}" "${blob}"; then
        continue
      fi
      printf '%s\n' "${path}" >>"${TMP_DIR}/f13-unapproved-delta"
    done <"${delta}"

    trigger=false
    if voice_r22_runtime_changed <"${TMP_DIR}/f13-unapproved-delta"; then
      trigger=true
    fi
    while IFS= read -r path; do
      [[ -n "${path}" ]] || continue
      if [[ -n "${owners[${path}]+present}" ]] && f13_path_is_guarded "${path}"; then
        return 1
      fi
      owners["${path}"]="${commit}"
      printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
        "${path}" "${trigger}" "${commit}" "${first_parent}" "${source_base}" "${source_head}" >>"${output}"
    done <"${delta}"
  done < <(git -C "${repo}" rev-list --first-parent --reverse "${base}..${target}")
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
grep -Fqx $'protos/voice/bot/v1/bot.proto\tfalse' <(cut -f1-2 "${TMP_DIR}/f13-fixture-map") || {
  printf '%s\n' 'F13 oracle bug: unrelated Bot contribution inherited the R22 trigger' >&2
  exit 2
}
f13_cross_scope_path_allowed 'protos/voice/bot/v1/bot.proto' false || {
  printf '%s\n' 'F13 oracle bug: unrelated-only proto contribution was rejected' >&2
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
  "${f13_fixture_repo}" "${f13_fixture_base}" HEAD "${TMP_DIR}/f13-conflict-map"; then
  printf '%s\n' 'F13 oracle bug: source/merge blob mismatch was accepted' >&2
  exit 2
fi

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
git -C "${f13_activation_fixture}" commit -qm 'F13 activation fixture base'
f13_activation_base="$(git -C "${f13_activation_fixture}" rev-parse HEAD)"
git -C "${f13_activation_fixture}" checkout -qb f13-activation-source
python3 - "${f13_activation_fixture}/src/backend/voice/main.go" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
path.write_text(path.read_text().replace('label := "base"', 'label := "source benign edit"'))
PY
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 source benign edit preserves activation'
git -C "${f13_activation_fixture}" checkout -qb f13-activation-integration "${f13_activation_base}"
python3 - "${f13_activation_fixture}/src/backend/voice/main.go" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
source = path.read_text().replace('\tNewLifecycleWorker()\n\n', '').replace('label := "base"', 'label := "target edit"')
path.write_text(source)
PY
git -C "${f13_activation_fixture}" add src/backend/voice/main.go
git -C "${f13_activation_fixture}" commit -qm 'F13 first parent removes activation'
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
  local repo="$1" map="$2" path="$3" commit first_parent source_base source_head row
  if [[ -s "${map}" ]]; then
    row="$(awk -F '\t' -v path="${path}" '$1 == path { value=$0 } END { if (value != "") print value }' "${map}")"
    IFS=$'\t' read -r _ _ commit first_parent source_base source_head <<<"${row}"
    [[ -n "${commit}" && -n "${first_parent}" && -n "${source_base}" && -n "${source_head}" ]] || return 1
    git -C "${repo}" diff --unified=0 "${first_parent}" "${commit}" -- "${path}" | sed -n '/^+++ /d; /^+/s/^+//p'
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

git -C "${f13_fixture_repo}" checkout -qb f13-repeat-source-one "${f13_fixture_base}"
mkdir -p "${f13_fixture_repo}/src/backend/voice"
printf '%s\n' 'package voice' >"${f13_fixture_repo}/src/backend/voice/repeated.go"
git -C "${f13_fixture_repo}" add src/backend/voice/repeated.go
git -C "${f13_fixture_repo}" commit -qm 'F13 fixture first repeated guarded path'
git -C "${f13_fixture_repo}" checkout -qb f13-repeat-integration "${f13_fixture_base}"
git -C "${f13_fixture_repo}" merge --no-ff -qm 'F13 fixture first repeated merge' f13-repeat-source-one
git -C "${f13_fixture_repo}" checkout -qb f13-repeat-source-two "${f13_fixture_base}"
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
  "${f13_fixture_repo}" '0000000000000000000000000000000000000000' HEAD "${TMP_DIR}/f13-invalid-map"; then
  printf '%s\n' 'F13 oracle bug: missing aggregate baseline was accepted' >&2
  exit 2
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

if f13_develop_aggregate_enabled "${GITHUB_EVENT_NAME:-${VOICE_CI_EVENT_NAME:-}}" "${GITHUB_REF:-}"; then
  if ! f13_collect_develop_contributions "${ROOT}" "${base_sha}" HEAD "${TMP_DIR}/f13-contributions"; then
    printf '%s\n' 'F13: develop contribution provenance is missing or ambiguous' >&2
    exit 1
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
    printf '%s\n' "${file}"
done <"${TMP_DIR}/changed-files" >"${TMP_DIR}/unapproved-runtime-delta"
if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
  awk -F '\t' '$2 == "true" { print $1 }' "${TMP_DIR}/f13-contributions" \
    | LC_ALL=C sort -u >"${TMP_DIR}/unapproved-runtime-delta"
fi
r22_runtime_delta=false
if voice_r22_runtime_changed <"${TMP_DIR}/unapproved-runtime-delta"; then
  r22_runtime_delta=true
fi

f13_runtime_for_path() {
  local path="$1"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    awk -F '\t' -v path="${path}" '$1 == path { value=$2; found=1 } END { if (!found) exit 1; print value }' \
      "${TMP_DIR}/f13-contributions"
  else
    printf '%s\n' "${r22_runtime_delta}"
  fi
}

f13_delta_for_path() {
  local repo="$1" map="$2" path="$3" row commit source_base source_head
  if [[ -s "${map}" ]]; then
    row="$(awk -F '\t' -v path="${path}" '$1 == path { value=$0 } END { if (value != "") print value }' "${map}")"
    IFS=$'\t' read -r _ _ commit _ source_base source_head <<<"${row}"
    [[ -n "${commit}" && -n "${source_base}" && -n "${source_head}" ]] || return 1
    git -C "${repo}" diff --name-only "${source_base}" "${source_head}" >"${TMP_DIR}/f13-current-delta" || return 1
    grep -Fxq -- "${path}" "${TMP_DIR}/f13-current-delta" || return 1
    printf '%s\n' "${TMP_DIR}/f13-current-delta"
  else
    printf '%s\n' "${TMP_DIR}/changed-files"
  fi
}

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

while IFS= read -r file; do
  f13_path_delta="${TMP_DIR}/changed-files"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    f13_path_delta="$(f13_delta_for_path "${ROOT}" "${TMP_DIR}/f13-contributions" "${file}")" || {
      if f13_path_is_guarded "${file}"; then
        fail "F13: changed guarded path is absent from its contribution delta: ${file}"
      fi
      f13_path_delta="${TMP_DIR}/changed-files"
    }
  fi
  file_r22_runtime_delta="${r22_runtime_delta}"
  if [[ -s "${TMP_DIR}/f13-contributions" ]]; then
    file_r22_runtime_delta="$(f13_runtime_for_path "${file}")" || {
      if f13_path_is_guarded "${file}"; then
        fail "F13: changed guarded path has no unique contribution provenance: ${file}"
      fi
      file_r22_runtime_delta=false
    }
  fi
  if game_checkpoint_path_allowed "${file}" "${f13_path_delta}"; then
    continue
  fi
  actual_blob="$(git -C "${ROOT}" hash-object --path="${file}" "${ROOT}/${file}" 2>/dev/null || true)"
  if be255_checkpoint_path_authorized_in_delta "${file}" "${f13_path_delta}" "${actual_blob}"; then
    continue
  fi
  if r23_contract_path_allowed_for_window "${r23_allowlist_enabled}" "${file}" "${f13_path_delta}"; then
    continue
  fi
  if t31_runtime_path_allowed "${file}"; then
    continue
  fi
  if [[ "${file_r22_runtime_delta}" == true ]] && space_media_path_allowed "${file}"; then
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
    src/backend/voice/*.go)
      if [[ "${file}" != *_test.go ]]; then
        case "$(basename "${file}")" in
          main.go|health.go|database.go) ;;
          *) fail "F13: unapproved Voice runtime production file: ${file}" ;;
        esac
      fi
      ;;
  esac
done <"${TMP_DIR}/changed-files"

while IFS= read -r file; do
  if game_checkpoint_path_allowed "${file}" "${TMP_DIR}/changed-files"; then
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

if ((failures > 0)); then
  printf 'Voice DB RED-F contract: %d failure(s)\n' "${failures}" >&2
  exit 1
fi

echo 'Voice DB RED-F runtime/provisioning/docs contract: PASS'
