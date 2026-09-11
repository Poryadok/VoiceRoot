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
expect_regex 'docs/DEPLOYMENT.md' '(migrat|миграц).*(before|до).*(voice-voice|Voice)|(voice_db|000001_room_lifecycle).*(before|до).*(roll|запуск|депло)' F12
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
{
  git -C "${ROOT}" diff --name-only "${base_sha}" --
  git -C "${ROOT}" ls-files --others --exclude-standard
} | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u >"${TMP_DIR}/changed-files"

while IFS= read -r file; do
  case "${file}" in
    protos/*|*/pb/*|*.pb.go)
      fail "F13: proto/generated change is outside R22.2: ${file}"
      ;;
    src/backend/space/*|src/backend/role/*)
      fail "F13: Space/Role change is outside R22.2: ${file}"
      ;;
    src/backend/voice/internal/grpcsvc/*.go|src/backend/voice/internal/store/*.go|src/backend/voice/internal/livekit/*.go|src/backend/voice/internal/s2s/*.go|src/backend/voice/internal/voiceevents/*.go)
      [[ "${file}" == *_test.go ]] || fail "F13: handler/Redis/external adapter change activates forbidden scope: ${file}"
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
  case "${file}" in
    src/backend/voice/main.go|src/backend/voice/health.go|src/backend/voice/database.go|src/backend/voice/internal/roomlifecycle/*.go)
      [[ "${file}" == *_test.go ]] && continue
      changed_content "${base_sha}" "${file}" >"${TMP_DIR}/changed-content"
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
