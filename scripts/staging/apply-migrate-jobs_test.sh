#!/usr/bin/env bash
# Contract tests for migration Job reuse/rerun ordering and credential-safe specs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
TEST_TMP="$(mktemp -d)"
trap 'rm -rf "${TEST_TMP}"' EXIT

definitions="$(awk '/^if ! kubectl get secret/{exit} {print}' "${ROOT}/scripts/staging/apply-migrate-jobs.sh")"
eval "${definitions}"

CALLS="${TEST_TMP}/calls"
STORED_HASH="${TEST_TMP}/stored-hash"
JOB_EXISTS="${TEST_TMP}/job-exists"
JOB_SUCCEEDED="${TEST_TMP}/job-succeeded"
WAIT_FAIL="${TEST_TMP}/wait-fail"
RENDERED_JOB="${TEST_TMP}/rendered-job.yaml"
CAPTURED_DSN="${TEST_TMP}/captured-dsn"
DIAGNOSTIC_LOG="${TEST_TMP}/diagnostic-log"
MIGRATIONS="${TEST_TMP}/migrations"
TEST_BIN="${TEST_TMP}/bin"
mkdir -p "${MIGRATIONS}"
mkdir -p "${TEST_BIN}"
printf '%s\n' 'SELECT 1;' >"${MIGRATIONS}/000001_fixture.up.sql"
TEMPLATE="${ROOT}/deploy/templates/migrate-voice-db-job.yaml"

cat >"${TEST_BIN}/migrate" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
  case "${arg}" in
    -database=*) printf '%s' "${arg#*=}" >"${MIGRATE_DSN_CAPTURE:?}" ;;
  esac
done
EOF
chmod +x "${TEST_BIN}/migrate"

readonly RESERVED_USER='u@ser /%?#:'
readonly RESERVED_PASSWORD='R22 a@b/%?#: space END'
readonly ENCODED_USER='u%40ser%20%2F%25%3F%23%3A'
readonly ENCODED_PASSWORD='R22%20a%40b%2F%25%3F%23%3A%20space%20END'
readonly EXPECTED_RESERVED_DSN="postgres://${ENCODED_USER}:${ENCODED_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"
readonly JOB_STATUS_JSONPATH='jsonpath=job active={.status.active} failed={.status.failed} succeeded={.status.succeeded}{"\n"}{range .status.conditions[*]}job.condition type={.type} reason={.reason}{"\n"}{end}'
readonly POD_STATUS_JSONPATH='jsonpath=pod phase={.status.phase}{"\n"}{range .status.containerStatuses[*]}container name={.name} terminated.reason={.state.terminated.reason} waiting.reason={.state.waiting.reason} exitCode={.state.terminated.exitCode}{"\n"}{end}'
readonly SAFE_JOB_STATUS='job active=0 failed=1 succeeded=0
job.condition type=Failed reason=BackoffLimitExceeded'
readonly SAFE_POD_STATUS='pod phase=Failed
container name=migrate terminated.reason=Error waiting.reason= exitCode=1'
readonly EXPECTED_JOB_STATUS_REQUEST="kubectl.get.job.request voice-migrate-voice-db -n ${NS} -o ${JOB_STATUS_JSONPATH}"
readonly EXPECTED_POD_NAME_REQUEST="kubectl.get.pods.request -n ${NS} -l job-name=voice-migrate-voice-db -o jsonpath={.items[0].metadata.name}"
readonly EXPECTED_POD_STATUS_REQUEST="kubectl.get.pod.request voice-migrate-voice-db-pod -n ${NS} -o ${POD_STATUS_JSONPATH}"

reset_mock() {
  : >"${CALLS}" || return 1
  rm -f "${STORED_HASH}" "${JOB_EXISTS}" "${JOB_SUCCEEDED}" "${WAIT_FAIL}" "${RENDERED_JOB}" "${CAPTURED_DSN}" "${DIAGNOSTIC_LOG}" || return 1
}

kubectl_apply_configmap() {
  printf 'configmap.apply %s\n' "$1" >>"${CALLS}"
}

kubectl() {
  local command="${1:-}"
  shift || true
  case "${command}" in
    annotate)
      local arg
      for arg in "$@"; do
        case "${arg}" in
          voice.io/migration-content-hash=*)
            printf '%s' "${arg#*=}" >"${STORED_HASH}"
            ;;
        esac
      done
      printf '%s\n' 'configmap.annotate' >>"${CALLS}"
      ;;
    get)
      local resource="${1:-}"
      shift || true
      case "${resource}" in
        job)
          printf '%s\n' 'job.get' >>"${CALLS}"
          printf 'kubectl.get.job.request %s\n' "$*" >>"${CALLS}"
          [[ -f "${JOB_EXISTS}" ]] || return 1
          if [[ "$*" == *"${JOB_STATUS_JSONPATH}"* ]]; then
            printf '%s\n' 'kubectl.get.job.status.exact' >>"${CALLS}"
            printf '%s\n' "${SAFE_JOB_STATUS}"
          elif [[ "$*" == *'.status.succeeded'* ]]; then
            [[ -f "${JOB_SUCCEEDED}" ]] && cat "${JOB_SUCCEEDED}" || printf '0'
          fi
          ;;
        configmap)
          printf '%s\n' 'configmap.get' >>"${CALLS}"
          [[ -f "${STORED_HASH}" ]] && cat "${STORED_HASH}"
          ;;
        pods)
          printf 'kubectl.get.pods.request %s\n' "$*" >>"${CALLS}"
          printf 'voice-migrate-voice-db-pod'
          ;;
        pod)
          printf 'kubectl.get.pod.request %s\n' "$*" >>"${CALLS}"
          if [[ "$*" == *"${POD_STATUS_JSONPATH}"* ]]; then
            printf '%s\n' 'kubectl.get.pod.status.exact' >>"${CALLS}"
            printf '%s\n' "${SAFE_POD_STATUS}"
          fi
          ;;
      esac
      ;;
    delete)
      printf '%s\n' 'kubectl.delete.job' >>"${CALLS}"
      rm -f "${JOB_EXISTS}" "${JOB_SUCCEEDED}"
      ;;
    apply)
      cat >"${RENDERED_JOB}"
      printf '%s\n' 'kubectl.apply.job' >>"${CALLS}"
      : >"${JOB_EXISTS}"
      printf '0' >"${JOB_SUCCEEDED}"
      ;;
    wait)
      printf '%s\n' 'kubectl.wait.job' >>"${CALLS}"
      [[ ! -f "${WAIT_FAIL}" ]]
      ;;
    describe)
      printf '%s\n' 'kubectl.describe' >>"${CALLS}"
      [[ -f "${DIAGNOSTIC_LOG}" ]] && cat "${DIAGNOSTIC_LOG}"
      ;;
    logs)
      printf '%s\n' 'kubectl.logs' >>"${CALLS}"
      if [[ -f "${DIAGNOSTIC_LOG}" ]]; then
        cat "${DIAGNOSTIC_LOG}"
      else
        printf '%s\n' 'migration failed; inspect schema_migrations state'
      fi
      ;;
    *)
      printf 'unexpected kubectl command: %s %s\n' "${command}" "$*" >&2
      return 2
      ;;
  esac
}

first_call_line() {
  local pattern="$1"
  grep -n -m1 -E "${pattern}" "${CALLS}" | cut -d: -f1
}

require_call_before() {
  local earlier_pattern="$1"
  local later_pattern="$2"
  local message="$3"
  local earlier_line later_line
  earlier_line="$(first_call_line "${earlier_pattern}" || true)"
  later_line="$(first_call_line "${later_pattern}" || true)"
  if [[ -z "${earlier_line}" || -z "${later_line}" || "${earlier_line}" -ge "${later_line}" ]]; then
    echo "FAIL: ${message}" >&2
    return 1
  fi
}

assert_no_credential_fragments() {
  local text="$1"
  local fragment
  for fragment in \
    "${RESERVED_USER}" \
    'ser /%?#:' \
    "${RESERVED_PASSWORD}" \
    'b/%?#: space END' \
    '%?#: space END' \
    'space END' \
    "${ENCODED_USER}" \
    "${ENCODED_PASSWORD}" \
    '%40b%2F%25%3F%23%3A%20space%20END'; do
    [[ "${text}" != *"${fragment}"* ]] || return 1
  done
}

validate_actionable_failure_diagnostics() {
  local diagnostics="$1"
  local calls_file="$2"
  local required
  for required in \
    'job active=0 failed=1 succeeded=0' \
    'job.condition type=Failed reason=BackoffLimitExceeded' \
    'pod phase=Failed' \
    'container name=migrate terminated.reason=Error waiting.reason= exitCode=1'; do
    grep -Fq -- "${required}" <<<"${diagnostics}" || return 1
  done

  assert_no_credential_fragments "${diagnostics}" || return 1
  [[ "${diagnostics}" != *'postgres://'* &&
     "${diagnostics}" != *'postgresql://'* &&
     "${diagnostics}" != *'condition-message-secret'* &&
     "${diagnostics}" != *'pod-log-secret'* &&
     "${diagnostics}" != *'output omitted'* ]] || return 1

  local expected count line
  for expected in \
    "${EXPECTED_JOB_STATUS_REQUEST}" \
    "${EXPECTED_POD_NAME_REQUEST}" \
    "${EXPECTED_POD_STATUS_REQUEST}" \
    'kubectl.get.job.status.exact' \
    'kubectl.get.pod.status.exact'; do
    count="$({ grep -Fxc -- "${expected}" "${calls_file}" || true; } | tr -d '[:space:]')"
    [[ "${count}" == '1' ]] || return 1
  done

  while IFS= read -r line; do
    case "${line}" in
      kubectl.get.job.request\ *|kubectl.get.pods.request\ *|kubectl.get.pod.request\ *)
        if grep -Eq '(^|[[:space:]])(-o([^[:space:]]*)?|--output([^[:space:]]*)?)([[:space:]]|$)' \
            <<<"${line}"; then
          case "${line}" in
            "${EXPECTED_JOB_STATUS_REQUEST}"|"${EXPECTED_POD_NAME_REQUEST}"|"${EXPECTED_POD_STATUS_REQUEST}") ;;
            *) return 1 ;;
          esac
        fi
        ;;
    esac
  done <"${calls_file}"

  if grep -Eq '^kubectl\.(describe|logs)$|\.message|\.spec|\.args|\.env|(^|[[:space:]])-o[[:space:]]+(wide|yaml|json)([[:space:]]|$)' \
      "${calls_file}"; then
    return 1
  fi
}

validate_reserved_dsn() {
  local dsn="$1"
  [[ "${dsn}" == "${EXPECTED_RESERVED_DSN}" ]] || return 1
  node - "${dsn}" "${RESERVED_USER}" "${RESERVED_PASSWORD}" <<'NODE'
const [dsn, expectedUser, expectedPassword] = process.argv.slice(2);
const parsed = new URL(dsn);
if (decodeURIComponent(parsed.username) !== expectedUser ||
    decodeURIComponent(parsed.password) !== expectedPassword ||
    parsed.hostname !== 'voice-postgres' ||
    parsed.port !== '5432' ||
    parsed.pathname !== '/voice_db' ||
    parsed.searchParams.get('sslmode') !== 'disable') {
  process.exit(1);
}
NODE
}

voice_job_shell_script() {
  awk '
    /^            - \|[[:space:]]*$/ { in_script=1; next }
    in_script && /^          [^[:space:]]/ { exit }
    in_script {
      sub(/^              /, "")
      print
    }
  ' "${TEMPLATE}"
}

run_reserved_dsn_encoding_case() {
  reset_mock || return 1
  local shell_script captured template_text
  shell_script="$(voice_job_shell_script)" || return 1
  [[ -n "${shell_script}" ]] || {
    echo 'FAIL: Voice migration Job lacks an in-container DSN command' >&2
    return 1
  }
  template_text="$(cat "${TEMPLATE}")" || return 1
  assert_no_credential_fragments "${template_text}" || {
    echo 'FAIL: Voice migration Job spec contains reserved credential material' >&2
    return 1
  }

  PATH="${TEST_BIN}:${PATH}" MIGRATE_DSN_CAPTURE="${CAPTURED_DSN}" \
    POSTGRES_USER="${RESERVED_USER}" POSTGRES_PASSWORD="${RESERVED_PASSWORD}" \
    POSTGRES_HOST=voice-postgres POSTGRES_DATABASE=voice_db \
    /bin/sh -ec "${shell_script}" || return 1
  [[ -s "${CAPTURED_DSN}" ]] || {
    echo 'FAIL: Voice migration Job did not pass a database DSN to migrate' >&2
    return 1
  }
  captured="$(cat "${CAPTURED_DSN}")" || return 1
  validate_reserved_dsn "${captured}" || {
    echo 'FAIL: Voice migration Job must RFC3986-encode and correctly decode reserved username/password characters' >&2
    return 1
  }
}

run_reserved_diagnostics_case() {
  reset_mock || return 1
  local raw_dsn diagnostics status rendered
  raw_dsn="postgres://${RESERVED_USER}:${RESERVED_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"
  printf '%s\n' \
    "condition.message=condition-message-secret database_url=[${raw_dsn}]" \
    "pod-log-secret database_url=[${EXPECTED_RESERVED_DSN}]" \
    >"${DIAGNOSTIC_LOG}" || return 1
  PG_USER="${RESERVED_USER}"
  PG_PASS="${RESERVED_PASSWORD}"
  : >"${WAIT_FAIL}" || return 1

  set +e
  diagnostics="$(apply_migrate voice_db "${MIGRATIONS}" "${TEMPLATE}" voice-migrate-voice-db voice-voice-db-migrations 2>&1)"
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: reserved diagnostics fixture must reach failed-Job reporting' >&2
    return 1
  }
  [[ -s "${RENDERED_JOB}" ]] || {
    echo 'FAIL: reserved diagnostics fixture did not capture the rendered Job' >&2
    return 1
  }
  rendered="$(cat "${RENDERED_JOB}")" || return 1
  assert_no_credential_fragments "${rendered}" || {
    echo 'FAIL: rendered Voice migration Job retained reserved credential material' >&2
    return 1
  }
  assert_no_credential_fragments "${diagnostics}" || {
    echo 'FAIL: sanitized migration diagnostics retained a reserved credential fragment' >&2
    return 1
  }
  [[ "${diagnostics}" != *'postgres://'* && "${diagnostics}" != *'postgresql://'* ]] || {
    echo 'FAIL: sanitizer must redact the complete PostgreSQL DSN' >&2
    return 1
  }
  validate_actionable_failure_diagnostics "${diagnostics}" "${CALLS}" || {
    echo 'FAIL: failed migration diagnostics must expose only constrained actionable Job/Pod status fields' >&2
    return 1
  }
}

run_diagnostics_oracle_self_test() {
  local fixture_calls="${TEST_TMP}/diagnostics-selftest-calls"
  local diagnostics status raw_dsn
  raw_dsn="postgres://${RESERVED_USER}:${RESERVED_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"
  cat >"${fixture_calls}" <<EOF || return 1
${EXPECTED_JOB_STATUS_REQUEST}
kubectl.get.job.status.exact
${EXPECTED_POD_NAME_REQUEST}
${EXPECTED_POD_STATUS_REQUEST}
kubectl.get.pod.status.exact
EOF

  set +e
  validate_actionable_failure_diagnostics \
    '[migration diagnostic output omitted to protect database credentials]' \
    "${fixture_calls}" >/dev/null 2>&1
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle accepted a fixed non-actionable omission message' >&2
    return 1
  }

  diagnostics="${SAFE_JOB_STATUS}
${SAFE_POD_STATUS}
condition-message-secret ${raw_dsn}
pod-log-secret ${EXPECTED_RESERVED_DSN}"
  set +e
  validate_actionable_failure_diagnostics "${diagnostics}" "${fixture_calls}" >/dev/null 2>&1
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle surfaced a malicious condition message or pod log fixture' >&2
    return 1
  }

  printf '%s\n' 'kubectl.logs' >>"${fixture_calls}" || return 1
  set +e
  validate_actionable_failure_diagnostics \
    "${SAFE_JOB_STATUS}"$'\n'"${SAFE_POD_STATUS}" \
    "${fixture_calls}" >/dev/null 2>&1
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle accepted a pod logs invocation' >&2
    return 1
  }

  sed -i '/^kubectl\.logs$/d' "${fixture_calls}" || return 1

  printf '%s\n' \
    "kubectl.get.job.request voice-migrate-voice-db -n ${NS} -o=json" \
    >>"${fixture_calls}" || return 1
  set +e
  validate_actionable_failure_diagnostics \
    "${SAFE_JOB_STATUS}"$'\n'"${SAFE_POD_STATUS}" \
    "${fixture_calls}" >/dev/null 2>&1
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle accepted an additional broad -o=json Job request' >&2
    return 1
  }
  sed -i '/ -o=json$/d' "${fixture_calls}" || return 1

  printf '%s --output=jsonpath={.status.conditions[*].message}\n' \
    "${EXPECTED_JOB_STATUS_REQUEST}" >>"${fixture_calls}" || return 1
  set +e
  validate_actionable_failure_diagnostics \
    "${SAFE_JOB_STATUS}"$'\n'"${SAFE_POD_STATUS}" \
    "${fixture_calls}" >/dev/null 2>&1
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle accepted an extra unsafe selector beside the safe request' >&2
    return 1
  }
  sed -i '/ --output=jsonpath={\.status\.conditions\[\*\]\.message}$/d' \
    "${fixture_calls}" || return 1

  validate_actionable_failure_diagnostics \
    "${SAFE_JOB_STATUS}"$'\n'"${SAFE_POD_STATUS}" \
    "${fixture_calls}" || {
      echo 'FAIL: diagnostics oracle rejected its constrained safe fixture' >&2
      return 1
    }
}

run_security_oracle_self_test() {
  validate_reserved_dsn "${EXPECTED_RESERVED_DSN}" || {
    echo 'FAIL: DSN oracle rejected its valid encoded fixture' >&2
    return 1
  }
  local raw_dsn unsafe_status
  raw_dsn="postgres://${RESERVED_USER}:${RESERVED_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"
  set +e
  validate_reserved_dsn "${raw_dsn}" >/dev/null 2>&1
  unsafe_status=$?
  set -e
  [[ "${unsafe_status}" -ne 0 ]] || {
    echo 'FAIL: DSN oracle accepted raw reserved credentials' >&2
    return 1
  }
  assert_no_credential_fragments 'migration failed database_url=[REDACTED_DATABASE_URL]' || {
    echo 'FAIL: diagnostics oracle rejected its safe fixture' >&2
    return 1
  }
  set +e
  assert_no_credential_fragments "migration failed ${raw_dsn}" >/dev/null 2>&1
  unsafe_status=$?
  set -e
  [[ "${unsafe_status}" -ne 0 ]] || {
    echo 'FAIL: diagnostics oracle accepted raw reserved credentials' >&2
    return 1
  }
}

validate_changed_content_call_order() {
  require_call_before '^configmap\.get$' '^configmap\.apply ' \
    'stored ConfigMap hash must be read before applying changed migration content' || return 1
  require_call_before '^configmap\.get$' '^configmap\.annotate$' \
    'stored ConfigMap hash must be read before annotating changed migration content' || return 1
  require_call_before '^job\.get$' '^configmap\.apply ' \
    'stored Job state must be read before applying changed migration content' || return 1
  require_call_before '^job\.get$' '^configmap\.annotate$' \
    'stored Job state must be read before annotating changed migration content' || return 1
  require_call_before '^kubectl\.delete\.job$' '^kubectl\.apply\.job$' \
    'changed SQL content must delete the previous Job before applying its replacement' || return 1
  require_call_before '^kubectl\.apply\.job$' '^kubectl\.wait\.job$' \
    'changed SQL content must apply the replacement Job before waiting for it' || return 1
}

run_order_oracle_self_test() {
  reset_mock || return 1
  cat >"${CALLS}" <<'EOF' || return 1
job.get
configmap.apply voice-voice-db-migrations
configmap.get
configmap.annotate
kubectl.delete.job
kubectl.apply.job
kubectl.wait.job
EOF

  # The late Job assertions intentionally pass. The first ConfigMap ordering
  # assertion must still make the complete validator return nonzero.
  require_call_before '^kubectl\.delete\.job$' '^kubectl\.apply\.job$' \
    'self-test fixture must keep delete before apply' || return 1
  require_call_before '^kubectl\.apply\.job$' '^kubectl\.wait\.job$' \
    'self-test fixture must keep apply before wait' || return 1

  local diagnostics status
  set +e
  diagnostics="$(validate_changed_content_call_order 2>&1)"
  status=$?
  set -e
  if [[ "${status}" -eq 0 ]]; then
    echo 'FAIL: ordering validator masked an early failure after later assertions passed' >&2
    return 1
  fi
  grep -Fq 'stored ConfigMap hash must be read before applying changed migration content' \
    <<<"${diagnostics}" || {
      echo 'FAIL: ordering validator self-test did not reject the intended early violation' >&2
      return 1
    }
}

run_changed_content_case() {
  reset_mock || return 1
  printf 'old-content-hash' >"${STORED_HASH}" || return 1
  : >"${JOB_EXISTS}" || return 1
  printf '1' >"${JOB_SUCCEEDED}" || return 1

  apply_migrate voice_db "${MIGRATIONS}" "${TEMPLATE}" voice-migrate-voice-db voice-voice-db-migrations >/dev/null || return 1

  validate_changed_content_call_order || return 1
}

run_unchanged_content_case() {
  reset_mock || return 1
  current_hash="$(migration_content_hash "${MIGRATIONS}")" || return 1
  printf '%s' "${current_hash}" >"${STORED_HASH}" || return 1
  : >"${JOB_EXISTS}" || return 1
  printf '1' >"${JOB_SUCCEEDED}" || return 1

  apply_migrate voice_db "${MIGRATIONS}" "${TEMPLATE}" voice-migrate-voice-db voice-voice-db-migrations >/dev/null || return 1

  stored_hash_line="$(first_call_line '^configmap\.get$' || true)"
  first_mutation_line="$(first_call_line '^(configmap\.apply |configmap\.annotate$)' || true)"
  [[ -n "${stored_hash_line}" ]] || {
    echo 'FAIL: unchanged migration reuse must read the stored ConfigMap hash' >&2
    return 1
  }
  if [[ -n "${first_mutation_line}" && "${stored_hash_line}" -ge "${first_mutation_line}" ]]; then
    echo 'FAIL: unchanged migration reuse mutated the ConfigMap before reading its stored hash' >&2
    return 1
  fi
  if grep -Eq '^kubectl\.(delete|apply\.job|wait\.job)' "${CALLS}"; then
    echo 'FAIL: unchanged successful Voice migration Job must be reused' >&2
    return 1
  fi
}

run_secret_leak_case() {
  reset_mock || return 1
  readonly sentinel_password='R22_F_PASSWORD_MUST_NOT_APPEAR_7f4c'
  PG_PASS="${sentinel_password}"
  : >"${WAIT_FAIL}" || return 1

  set +e
  diagnostics="$(apply_migrate voice_db "${MIGRATIONS}" "${TEMPLATE}" voice-migrate-voice-db voice-voice-db-migrations 2>&1)"
  status=$?
  set -e
  [[ "${status}" -ne 0 ]] || {
    echo 'FAIL: fixture must reach failed-Job diagnostics' >&2
    return 1
  }
  [[ -s "${RENDERED_JOB}" ]] || {
    echo 'FAIL: fixture did not capture the rendered Voice migration Job' >&2
    return 1
  }
  if grep -Fq "${sentinel_password}" "${RENDERED_JOB}" ||
     grep -Fq "${sentinel_password}" <<<"${diagnostics}" ||
     grep -Fq "${sentinel_password}" "${CALLS}"; then
    echo 'FAIL: PostgreSQL password leaked into migration Job spec or diagnostics' >&2
    return 1
  fi
  grep -Fq 'secretKeyRef:' "${RENDERED_JOB}" || {
    echo 'FAIL: Voice migration Job must read database credentials through a Secret reference' >&2
    return 1
  }
  grep -Eq '(POSTGRES_PASSWORD|VOICE_DATABASE_URL)' "${RENDERED_JOB}" || {
    echo 'FAIL: Voice migration Job lacks a secret-backed database credential component' >&2
    return 1
  }
}

case "${1:-all}" in
  selftest)
    run_order_oracle_self_test
    run_security_oracle_self_test
    run_diagnostics_oracle_self_test
    ;;
  encoding)
    run_reserved_dsn_encoding_case
    ;;
  sanitize)
    run_reserved_diagnostics_case
    ;;
  rerun)
    run_changed_content_case
    run_unchanged_content_case
    ;;
  secret)
    run_secret_leak_case
    ;;
  all)
    failures=0
    run_order_oracle_self_test || failures=$((failures + 1))
    run_security_oracle_self_test || failures=$((failures + 1))
    run_diagnostics_oracle_self_test || failures=$((failures + 1))
    run_changed_content_case || failures=$((failures + 1))
    run_unchanged_content_case || failures=$((failures + 1))
    run_secret_leak_case || failures=$((failures + 1))
    run_reserved_dsn_encoding_case || failures=$((failures + 1))
    run_reserved_diagnostics_case || failures=$((failures + 1))
    ((failures == 0)) || exit 1
    ;;
  *)
    echo "usage: $0 [all|selftest|rerun|secret|encoding|sanitize]" >&2
    exit 2
    ;;
esac

echo 'apply-migrate-jobs contract tests passed.'
