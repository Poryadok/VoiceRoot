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

{
  git -C "${ROOT}" diff --name-only "${base_sha}" --
  git -C "${ROOT}" ls-files --others --exclude-standard
} | sed '/^[[:space:]]*$/d' | LC_ALL=C sort -u >"${TMP_DIR}/changed-files"

while IFS= read -r file; do
  if r23_contract_path_allowed "${file}"; then
    continue
  fi
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
