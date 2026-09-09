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
  PATH="${TMP}/bin:${PATH}" KUBECTL_STATE="${TMP}" \
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
  for value in \
    'totp-generated-fixture-material-0123456789abcdef' \
    'delete-generated-fixture-material-fedcba9876543210' \
    'existing-totp-fixture-material-0123456789abcdef' \
    'existing-delete-fixture-material-fedcba9876543210'; do
    if grep -Fq "${value}" "${output}"; then
      fail "script output leaked secret material"
    fi
  done
}

write_mocks

echo '== fresh secret bootstrap and required-key retention =='
fresh_output="${TMP}/fresh.out"
run_script "${ENSURE}" "${fresh_output}"
secret_value AUTH_TOTP_ENCRYPTION_KEY >/dev/null || fail 'fresh bootstrap must create AUTH_TOTP_ENCRYPTION_KEY'
secret_value ACCOUNT_DELETE_TOKEN_SECRET >/dev/null || fail 'fresh bootstrap must create ACCOUNT_DELETE_TOKEN_SECRET'
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
legacy_output="${TMP}/legacy-ensure.out"
run_script "${ENSURE}" "${legacy_output}"
[[ ! -f "${TMP}/secret/AUTH_TOTP_ENCRYPTION_KEY" ]] || fail 'ensure must retain an existing secret without overwriting it'
[[ ! -f "${TMP}/secret/ACCOUNT_DELETE_TOKEN_SECRET" ]] || fail 'ensure must retain an existing secret without overwriting it'
assert_no_value_leak "${legacy_output}"

legacy_patch_output="${TMP}/legacy-patch.out"
run_script "${PATCH}" "${legacy_patch_output}"
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

echo 'ensure-app-secrets required Auth secret regression tests passed.'
