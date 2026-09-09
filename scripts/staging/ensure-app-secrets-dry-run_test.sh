#!/usr/bin/env bash
# Regression coverage for bootstrap Secret generation across kubectl dry-run APIs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ENSURE="${ROOT}/scripts/staging/ensure-app-secrets.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="${KUBECTL_STATE:?}"
printf '%s\n' "$*" >>"${state}/calls"

if [ "$1" = get ] && [ "$2" = secret ]; then
  exit 1
fi

if [ "$1" = create ] && [ "$2" = secret ] && [ "$3" = generic ] && [ "${4:-}" = --help ]; then
  if [ "${KUBECTL_DRY_RUN_MODE:?}" = legacy ]; then
    echo '  --dry-run: if true, only print the object'
  else
    echo '  --dry-run=client: generate a manifest locally'
  fi
  exit 0
fi

if [ "$1" = create ] && [ "$2" = secret ] && [ "$3" = generic ]; then
  if [ "${KUBECTL_DRY_RUN_MODE:?}" = empty ]; then
    exit 0
  fi
  printf '%s\n' 'apiVersion: v1' 'kind: Secret' 'metadata:' '  name: voice-app-secrets'
  exit 0
fi

if [ "$1" = apply ] && [ "$2" = -f ] && [ -f "${3:-}" ]; then
  cp "$3" "${state}/applied.yaml"
  exit 0
fi

echo "unexpected kubectl invocation: $*" >&2
exit 1
EOF

cat >"${TMP}/openssl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  'rand -base64 48') printf '%s\n' 'fixture-auth-secret-material-0123456789abcdef' ;;
  *) echo "unexpected openssl invocation: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "${TMP}/kubectl" "${TMP}/openssl"

run_case() {
  local mode="$1"
  local output="${TMP}/${mode}.out"
  rm -f "${TMP}/calls" "${TMP}/applied.yaml"
  if PATH="${TMP}:${PATH}" KUBECTL_STATE="${TMP}" KUBECTL_DRY_RUN_MODE="${mode}" \
    AUTH_JWT_PRIVATE_KEY_FILE="${ROOT}/src/backend/auth/src/test/resources/jwt-test-private.pem" \
    bash "${ENSURE}" >"${output}" 2>&1; then
    return 0
  fi
  cat "${output}" >&2
  return 1
}

assert_no_secret_leak() {
  local output="$1"
  if grep -Fq 'fixture-auth-secret-material-0123456789abcdef' "${output}"; then
    fail 'bootstrap output leaked generated secret material'
  fi
}

run_case modern
cmp <(printf '%s\n' 'apiVersion: v1' 'kind: Secret' 'metadata:' '  name: voice-app-secrets') "${TMP}/applied.yaml" \
  || fail 'modern bootstrap must apply generated Secret manifest'
grep -Fq -- '--dry-run=client -o yaml' "${TMP}/calls" \
  || fail 'modern kubectl must use --dry-run=client'
if grep -Fq -- 'apply -f -' "${TMP}/calls"; then
  fail 'modern bootstrap must not pipe its manifest to kubectl apply'
fi
assert_no_secret_leak "${TMP}/modern.out"

run_case legacy
grep -Fq -- '--dry-run -o yaml' "${TMP}/calls" \
  || fail 'legacy kubectl must use boolean --dry-run'
if grep -Fq -- '--dry-run=client' "${TMP}/calls"; then
  fail 'legacy kubectl must not receive --dry-run=client'
fi
if grep -Fq -- 'apply -f -' "${TMP}/calls"; then
  fail 'legacy bootstrap must not pipe its manifest to kubectl apply'
fi
[ -s "${TMP}/applied.yaml" ] || fail 'legacy bootstrap must apply a non-empty manifest file'
assert_no_secret_leak "${TMP}/legacy.out"

rm -f "${TMP}/calls" "${TMP}/applied.yaml"
if PATH="${TMP}:${PATH}" KUBECTL_STATE="${TMP}" KUBECTL_DRY_RUN_MODE=empty \
  AUTH_JWT_PRIVATE_KEY_FILE="${ROOT}/src/backend/auth/src/test/resources/jwt-test-private.pem" \
  bash "${ENSURE}" >"${TMP}/empty.out" 2>&1; then
  fail 'empty generated manifest must fail before kubectl apply'
fi
grep -Fq 'empty Secret manifest' "${TMP}/empty.out" \
  || fail 'empty manifest failure must identify generation as the root cause'
if grep -Fq -- 'apply -f' "${TMP}/calls"; then
  fail 'empty generated manifest must never reach kubectl apply'
fi
assert_no_secret_leak "${TMP}/empty.out"

echo 'kubectl Secret bootstrap compatibility tests passed.'
