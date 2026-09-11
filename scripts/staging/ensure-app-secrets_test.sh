#!/usr/bin/env bash
# Regression coverage for required Auth secret bootstrap and legacy-secret patching.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENSURE="${ROOT}/scripts/staging/ensure-app-secrets.sh"
PATCH="${ROOT}/scripts/staging/patch-app-secrets-database-urls.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

secret_value() {
  local key="$1"
  [ -f "${TMP}/secret/${key}" ] || return 1
  cat "${TMP}/secret/${key}"
}

write_mocks() {
  mkdir -p "${TMP}/bin" "${TMP}/secret"
  cat >"${TMP}/bin/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${KUBECTL_STATE:?}"
mkdir -p "${state}/secret"
printf '%s\n' "$*" >>"${state}/calls"

if [ "$1" = get ] && [ "$2" = secret ]; then
  if [ ! -f "${state}/exists" ]; then
    exit 1
  fi
  for arg in "$@"; do
    case "${arg}" in
      jsonpath={.data.*})
      key="${arg#jsonpath={.data.}"
      key="${key%\}}"
      [ -f "${state}/secret/${key}" ] || exit 0
      base64 <"${state}/secret/${key}" | tr -d '\n'
      exit 0
      ;;
    esac
  done
  exit 0
fi

if [ "$1" = create ] && [ "$2" = secret ] && [ "$3" = generic ] && [ "${4:-}" = --help ]; then
  echo '  --dry-run=client: generate a manifest locally'
  exit 0
fi

if [ "$1" = create ] && [ "$2" = secret ] && [ "$3" = generic ]; then
  touch "${state}/exists"
  for arg in "$@"; do
    case "${arg}" in
      --from-literal=*)
        pair="${arg#--from-literal=}"
        key="${pair%%=*}"
        value="${pair#*=}"
        printf '%s' "${value}" >"${state}/secret/${key}"
        ;;
      --from-file=AUTH_JWT_PRIVATE_KEY=*)
        cat "${arg#--from-file=AUTH_JWT_PRIVATE_KEY=}" >"${state}/secret/AUTH_JWT_PRIVATE_KEY"
        ;;
    esac
  done
  printf '%s\n' 'apiVersion: v1' 'kind: Secret'
  exit 0
fi

if [ "$1" = apply ] && [ "$2" = -f ] && [ -f "${3:-}" ]; then
  cat "$3" >/dev/null
  exit 0
fi

if [ "$1" = patch ] && [ "$2" = secret ]; then
  payload=''
  while [ "$#" -gt 0 ]; do
    if [ "$1" = -p ]; then
      payload="$2"
      break
    fi
    shift
  done
  printf '%s' "${payload}" >"${state}/patch-payload"
  node -e '
    const fs = require("fs");
    const data = JSON.parse(process.argv[1]).stringData;
    for (const [key, value] of Object.entries(data)) {
      fs.writeFileSync(`${process.env.KUBECTL_STATE}/secret/${key}`, value);
    }
  ' "${payload}"
  exit 0
fi

echo "unexpected kubectl invocation: $*" >&2
exit 1
EOF
  cat >"${TMP}/bin/openssl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${KUBECTL_STATE:?}"
if [ "$1" = rand ]; then
  counter_file="${state}/openssl-rand-count"
  count=0
  [ -f "${counter_file}" ] && count="$(cat "${counter_file}")"
  count=$((count + 1))
  printf '%s' "${count}" >"${counter_file}"
  if [ "${count}" -eq 1 ]; then
    printf '%s\n' 'totp-generated-fixture-material-0123456789abcdef'
  else
    printf '%s\n' 'delete-generated-fixture-material-fedcba9876543210'
  fi
  exit 0
fi
if [ "$1" = pkey ]; then
  exit 0
fi
echo "unexpected openssl invocation: $*" >&2
exit 1
EOF
  cat >"${TMP}/bin/jq" <<'EOF'
#!/usr/bin/env node
const args = process.argv.slice(2);
const value = (flag, name) => {
  const index = args.findIndex((arg, i) => arg === flag && args[i + 1] === name);
  return index < 0 ? undefined : args[index + 2];
};
const query = args.at(-1);
if (query.includes('{stringData:')) {
  process.stdout.write(JSON.stringify({stringData: JSON.parse(value('--argjson', 'stringData'))}) + '\n');
} else if (query.includes('$obj +')) {
  const object = JSON.parse(value('--argjson', 'obj'));
  object[value('--arg', 'key')] = value('--arg', 'value');
  process.stdout.write(JSON.stringify(object) + '\n');
} else if (query.includes('AUTH_JWT_PRIVATE_KEY')) {
  process.stdout.write(JSON.stringify({AUTH_JWT_PRIVATE_KEY: value('--arg', 'jwt')}) + '\n');
} else {
  process.stdout.write('{}\n');
}
EOF
  chmod +x "${TMP}/bin/kubectl" "${TMP}/bin/openssl" "${TMP}/bin/jq"
}

run_script() {
  local script="$1"
  local output="$2"
  local postgres_password="${3:-voice-f09-fresh-password}"
  PATH="${TMP}/bin:${PATH}" KUBECTL_STATE="${TMP}" \
    VOICE_STAGING_POSTGRES_PASSWORD="${postgres_password}" \
    AUTH_JWT_PRIVATE_KEY_FILE="${ROOT}/src/backend/auth/src/test/resources/jwt-test-private.pem" \
    timeout 30s bash "${script}" >"${output}" 2>&1 || {
      cat "${output}" >&2
      return 1
    }
}

assert_random_generation_count() {
  local expected="$1"
  local count=0
  [ -f "${TMP}/openssl-rand-count" ] && count="$(cat "${TMP}/openssl-rand-count")"
  [[ "${count}" == "${expected}" ]] || fail "expected ${expected} Auth secret generations, got ${count}"
}

assert_no_value_leak() {
  local output="$1"
  validate_no_value_leak "${output}" || fail 'script output leaked secret material'
}

validate_no_value_leak() {
  local output="$1"
  for value in \
    'totp-generated-fixture-material-0123456789abcdef' \
    'delete-generated-fixture-material-fedcba9876543210' \
    'existing-totp-fixture-material-0123456789abcdef' \
    'existing-delete-fixture-material-fedcba9876543210' \
    'voice-f09-fresh-password' \
    'voice-f09-missing-key-password' \
    'voice-f09-resynced-password' \
    'voice-f09-stale-password' \
    'R22 a@b/%?#: space END' \
    'b/%?#: space END' \
    '%?#: space END' \
    'space END' \
    "${ENCODED_PG_PASSWORD}" \
    "${EXPECTED_RESERVED_VOICE_DSN}" \
    '%40b%2F%25%3F%23%3A%20space%20END'; do
    if grep -Fq "${value}" "${output}"; then
      return 1
    fi
  done
}

readonly RESERVED_PG_PASSWORD='R22 a@b/%?#: space END'
readonly ENCODED_PG_PASSWORD='R22%20a%40b%2F%25%3F%23%3A%20space%20END'
readonly EXPECTED_RESERVED_VOICE_DSN="postgres://voice:${ENCODED_PG_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"

validate_reserved_voice_dsn() {
  local dsn="$1"
  [[ "${dsn}" == "${EXPECTED_RESERVED_VOICE_DSN}" ]] || return 1
  node - "${dsn}" "${RESERVED_PG_PASSWORD}" <<'NODE'
const [dsn, expectedPassword] = process.argv.slice(2);
const parsed = new URL(dsn);
if (decodeURIComponent(parsed.username) !== 'voice' ||
    decodeURIComponent(parsed.password) !== expectedPassword ||
    parsed.hostname !== 'voice-postgres' ||
    parsed.port !== '5432' ||
    parsed.pathname !== '/voice_db' ||
    parsed.searchParams.get('sslmode') !== 'disable') {
  process.exit(1);
}
NODE
}

reset_secret_state() {
  rm -rf "${TMP}/secret" "${TMP}/exists" "${TMP}/patch-payload" \
    "${TMP}/calls" "${TMP}/openssl-rand-count" || return 1
  mkdir -p "${TMP}/secret" || return 1
}

run_dsn_oracle_self_test() {
  validate_reserved_voice_dsn "${EXPECTED_RESERVED_VOICE_DSN}" || \
    fail 'Voice DSN oracle rejected its valid encoded fixture'
  local raw_dsn unsafe_status
  raw_dsn="postgres://voice:${RESERVED_PG_PASSWORD}@voice-postgres:5432/voice_db?sslmode=disable"
  set +e
  validate_reserved_voice_dsn "${raw_dsn}" >/dev/null 2>&1
  unsafe_status=$?
  set -e
  [[ "${unsafe_status}" -ne 0 ]] || fail 'Voice DSN oracle accepted raw reserved credentials'

  local leak_fixture
  leak_fixture="${TMP}/encoded-leak-selftest.out"
  printf '%s\n' "${EXPECTED_RESERVED_VOICE_DSN}" >"${leak_fixture}" || return 1
  set +e
  validate_no_value_leak "${leak_fixture}" >/dev/null 2>&1
  unsafe_status=$?
  set -e
  [[ "${unsafe_status}" -ne 0 ]] || fail 'output oracle accepted a full encoded Voice DSN leak'

  printf '%s\n' '%40b%2F%25%3F%23%3A%20space%20END' >"${leak_fixture}" || return 1
  set +e
  validate_no_value_leak "${leak_fixture}" >/dev/null 2>&1
  unsafe_status=$?
  set -e
  [[ "${unsafe_status}" -ne 0 ]] || fail 'output oracle accepted an encoded password suffix leak'

  printf '%s\n' 'safe secret update message' >"${leak_fixture}" || return 1
  validate_no_value_leak "${leak_fixture}" || fail 'output oracle rejected its safe fixture'
}

run_reserved_bootstrap_case() {
  reset_secret_state || return 1
  local output dsn
  output="${TMP}/reserved-bootstrap.out"
  run_script "${ENSURE}" "${output}" "${RESERVED_PG_PASSWORD}" || return 1
  dsn="$(secret_value VOICE_DATABASE_URL)" || return 1
  validate_reserved_voice_dsn "${dsn}" || {
    echo 'FAIL: fresh bootstrap must RFC3986-encode the reserved Voice database password' >&2
    return 1
  }
  assert_no_value_leak "${output}"
}

run_reserved_patch_case() {
  reset_secret_state || return 1
  touch "${TMP}/exists" || return 1
  printf '%s' "${RESERVED_PG_PASSWORD}" >"${TMP}/secret/POSTGRES_PASSWORD" || return 1
  printf '%s' 'postgres://voice:stale@voice-postgres:5432/voice_db?sslmode=disable' \
    >"${TMP}/secret/VOICE_DATABASE_URL" || return 1
  local output dsn
  output="${TMP}/reserved-patch.out"
  run_script "${PATCH}" "${output}" "${RESERVED_PG_PASSWORD}" || return 1
  dsn="$(secret_value VOICE_DATABASE_URL)" || return 1
  validate_reserved_voice_dsn "${dsn}" || {
    echo 'FAIL: patch-app-secrets-database-urls must RFC3986-encode the reserved Voice database password' >&2
    return 1
  }
  assert_no_value_leak "${output}"
}

write_mocks

case "${1:-all}" in
  security-selftest)
    run_dsn_oracle_self_test
    echo 'Voice database DSN oracle self-test passed.'
    exit 0
    ;;
  reserved-bootstrap)
    run_reserved_bootstrap_case
    echo 'Reserved bootstrap Voice DSN test passed.'
    exit 0
    ;;
  reserved-patch)
    run_reserved_patch_case
    echo 'Reserved patch Voice DSN test passed.'
    exit 0
    ;;
  all) ;;
  *)
    echo "usage: $0 [all|security-selftest|reserved-bootstrap|reserved-patch]" >&2
    exit 2
    ;;
esac

run_dsn_oracle_self_test

echo '== fresh secret bootstrap and required-key retention =='
fresh_output="${TMP}/fresh.out"
run_script "${ENSURE}" "${fresh_output}"
secret_value AUTH_TOTP_ENCRYPTION_KEY >/dev/null || fail 'fresh bootstrap must create AUTH_TOTP_ENCRYPTION_KEY'
secret_value ACCOUNT_DELETE_TOKEN_SECRET >/dev/null || fail 'fresh bootstrap must create ACCOUNT_DELETE_TOKEN_SECRET'
[[ "$(secret_value VOICE_DATABASE_URL)" == 'postgres://voice:voice-f09-fresh-password@voice-postgres:5432/voice_db?sslmode=disable' ]] \
  || fail 'fresh bootstrap must create the dedicated Voice lifecycle DSN'
totp="$(secret_value AUTH_TOTP_ENCRYPTION_KEY)"
delete_token="$(secret_value ACCOUNT_DELETE_TOKEN_SECRET)"
jwt="$(secret_value AUTH_JWT_PRIVATE_KEY)"
[[ "${totp}" != "${delete_token}" ]] || fail 'fresh bootstrap must generate distinct TOTP and account-delete keys'
[[ "${totp}" != "${jwt}" ]] || fail 'TOTP key must not reuse JWT material'
[[ "${delete_token}" != "${jwt}" ]] || fail 'account-delete key must not reuse JWT material'
assert_no_value_leak "${fresh_output}"
assert_random_generation_count 2

fresh_patch_output="${TMP}/fresh-patch.out"
run_script "${PATCH}" "${fresh_patch_output}"
[[ "$(secret_value AUTH_TOTP_ENCRYPTION_KEY)" == "${totp}" ]] \
  || fail 'fresh patch must retain the generated TOTP key'
[[ "$(secret_value ACCOUNT_DELETE_TOKEN_SECRET)" == "${delete_token}" ]] \
  || fail 'fresh patch must retain the generated account-delete key'
assert_no_value_leak "${fresh_patch_output}"
assert_random_generation_count 2

echo '== existing legacy secret patch =='
rm -rf "${TMP}/secret" "${TMP}/exists" "${TMP}/patch-payload" "${TMP}/calls" "${TMP}/openssl-rand-count"
mkdir -p "${TMP}/secret"
touch "${TMP}/exists"
printf '%s' 'voice-f09-resynced-password' >"${TMP}/secret/POSTGRES_PASSWORD"
printf '%s' 'postgres://voice:voice-f09-stale-password@voice-postgres:5432/voice_db?sslmode=disable' \
  >"${TMP}/secret/VOICE_DATABASE_URL"
legacy_output="${TMP}/legacy-ensure.out"
run_script "${ENSURE}" "${legacy_output}"
[[ ! -f "${TMP}/secret/AUTH_TOTP_ENCRYPTION_KEY" ]] || fail 'ensure must retain an existing secret without overwriting it'
[[ ! -f "${TMP}/secret/ACCOUNT_DELETE_TOKEN_SECRET" ]] || fail 'ensure must retain an existing secret without overwriting it'
assert_no_value_leak "${legacy_output}"

legacy_patch_output="${TMP}/legacy-patch.out"
run_script "${PATCH}" "${legacy_patch_output}"
[[ "$(secret_value VOICE_DATABASE_URL)" == 'postgres://voice:voice-f09-resynced-password@voice-postgres:5432/voice_db?sslmode=disable' ]] \
  || fail 'legacy secret patch must resync VOICE_DATABASE_URL to POSTGRES_PASSWORD'
secret_value AUTH_TOTP_ENCRYPTION_KEY >/dev/null || fail 'legacy secret patch must add AUTH_TOTP_ENCRYPTION_KEY'
secret_value ACCOUNT_DELETE_TOKEN_SECRET >/dev/null || fail 'legacy secret patch must add ACCOUNT_DELETE_TOKEN_SECRET'
totp="$(secret_value AUTH_TOTP_ENCRYPTION_KEY)"
delete_token="$(secret_value ACCOUNT_DELETE_TOKEN_SECRET)"
jwt="$(secret_value AUTH_JWT_PRIVATE_KEY)"
[[ "${totp}" != "${delete_token}" ]] || fail 'legacy patch must generate distinct TOTP and account-delete keys'
[[ "${totp}" != "${jwt}" ]] || fail 'legacy TOTP key must not reuse JWT material'
[[ "${delete_token}" != "${jwt}" ]] || fail 'legacy account-delete key must not reuse JWT material'
assert_no_value_leak "${legacy_patch_output}"
assert_random_generation_count 2

echo '== existing secret missing Voice DSN =='
rm -rf "${TMP}/secret" "${TMP}/exists" "${TMP}/patch-payload" "${TMP}/calls" "${TMP}/openssl-rand-count"
mkdir -p "${TMP}/secret"
touch "${TMP}/exists"
printf '%s' 'voice-f09-missing-key-password' >"${TMP}/secret/POSTGRES_PASSWORD"
printf '%s' 'existing-totp-fixture-material-0123456789abcdef' >"${TMP}/secret/AUTH_TOTP_ENCRYPTION_KEY"
printf '%s' 'existing-delete-fixture-material-fedcba9876543210' >"${TMP}/secret/ACCOUNT_DELETE_TOKEN_SECRET"
missing_voice_output="${TMP}/missing-voice.out"
run_script "${PATCH}" "${missing_voice_output}"
[[ "$(secret_value VOICE_DATABASE_URL)" == 'postgres://voice:voice-f09-missing-key-password@voice-postgres:5432/voice_db?sslmode=disable' ]] \
  || fail 'existing secret patch must add an exact dedicated Voice lifecycle DSN when the key is missing'
assert_no_value_leak "${missing_voice_output}"
assert_random_generation_count 0

echo '== existing required values are retained =='
rm -rf "${TMP}/secret" "${TMP}/exists" "${TMP}/patch-payload" "${TMP}/calls" "${TMP}/openssl-rand-count"
mkdir -p "${TMP}/secret"
touch "${TMP}/exists"
printf '%s' 'existing-totp-fixture-material-0123456789abcdef' >"${TMP}/secret/AUTH_TOTP_ENCRYPTION_KEY"
printf '%s' 'existing-delete-fixture-material-fedcba9876543210' >"${TMP}/secret/ACCOUNT_DELETE_TOKEN_SECRET"
retained_patch_output="${TMP}/retained-patch.out"
run_script "${PATCH}" "${retained_patch_output}"
[[ "$(secret_value AUTH_TOTP_ENCRYPTION_KEY)" == 'existing-totp-fixture-material-0123456789abcdef' ]] \
  || fail 'existing TOTP key must be retained'
[[ "$(secret_value ACCOUNT_DELETE_TOKEN_SECRET)" == 'existing-delete-fixture-material-fedcba9876543210' ]] \
  || fail 'existing account-delete key must be retained'
assert_no_value_leak "${retained_patch_output}"
assert_random_generation_count 0

echo '== reserved-character Voice DSN bootstrap =='
run_reserved_bootstrap_case

echo '== reserved-character Voice DSN patch =='
run_reserved_patch_case

echo 'ensure-app-secrets required Auth secret regression tests passed.'
