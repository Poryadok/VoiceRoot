#!/usr/bin/env bash
# Exercise the actual production wrapper without network, Kubernetes or tokens.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
fixture="$(mktemp -d)"
trap 'rm -rf "${fixture}"' EXIT
mkdir -p "${fixture}/scripts/prod" "${fixture}/scripts/staging" "${fixture}/deploy/prod" "${fixture}/bin"
cp "${ROOT}/scripts/prod/"{smoke-prod,load-prod-domains,map-prod-env}.sh "${fixture}/scripts/prod/"
cp "${ROOT}/deploy/prod/domains.defaults" "${fixture}/deploy/prod/"

cat > "${fixture}/bin/curl" <<'CURL'
#!/usr/bin/env bash
set -euo pipefail
count=0
[ ! -f "${PROBE_DIR}/count" ] || read -r count < "${PROBE_DIR}/count"
count=$((count + 1))
printf '%s\n' "${count}" > "${PROBE_DIR}/count"
printf '%s\t' "$@" >> "${PROBE_DIR}/requests"
printf '\n' >> "${PROBE_DIR}/requests"
if [ "${count}" = "${FAIL_AT:-0}" ]; then
  [ "${NETWORK_FAILURE:-false}" != true ] || exit 7
  printf '%s' "${FAIL_STATUS}"
else
  printf 401
fi
CURL
cat > "${fixture}/scripts/staging/smoke-staging.sh" <<'SHARED'
#!/usr/bin/env bash
printf '%s\n' "${VOICE_STAGING_URL}|${VOICE_K8S_NAMESPACE}|${STAGING_STAFF_TOKEN:-}" > "${PROBE_DIR}/shared"
[ "${SHARED_FAILURE:-false}" != true ]
SHARED
chmod +x "${fixture}/bin/curl"

fail() { echo "FAIL: $*" >&2; exit 1; }
run_smoke() {
  local name="$1"
  shift
  mkdir -p "${fixture}/${name}"
  env PATH="${fixture}/bin:${PATH}" PROBE_DIR="${fixture}/${name}" \
    VOICE_GATEWAY_INGRESS_HOST=api.prod.test VOICE_K8S_NAMESPACE=voice-production \
    PROD_STAFF_TOKEN=production-fixture-token STAGING_STAFF_TOKEN=unrelated-fixture-token \
    "$@" bash "${fixture}/scripts/prod/smoke-prod.sh" > "${fixture}/${name}/output" 2>&1
}

run_smoke allowed || fail '401 security probes plus shared smoke must succeed'
[ -f "${fixture}/allowed/count" ] || fail 'production wrapper never checked protected routes'
[ "$(cat "${fixture}/allowed/count")" = 6 ] || fail 'each protected route must reject missing and invalid credentials'
for route in \
  '/api/v1/spaces/00000000-0000-0000-0000-000000000001' \
  '/api/v1/analytics/dashboard/product' \
  '/api/v1/admin/moderation/reports?status=pending&queue=content'; do
  [ "$(grep -Fc "https://api.prod.test${route}" "${fixture}/allowed/requests")" = 2 ] || fail "missing security probe for ${route}"
done
[ "$(grep -c 'Authorization: Bearer voice-production-smoke-invalid' "${fixture}/allowed/requests")" = 3 ] || fail 'invalid bearer must be tested separately'
if grep -Eq 'production-fixture-token|unrelated-fixture-token|(^|[[:space:]])(-k|--insecure|--location|-L|POST|PUT|DELETE)([[:space:]]|$)' "${fixture}/allowed/requests"; then
  fail 'negative probes must be read-only, unprivileged and verify TLS without redirect following'
fi
grep -Fxq 'https://api.prod.test|voice-production|production-fixture-token' "${fixture}/allowed/shared" || fail 'production mapping must reach shared smoke'
grep -Fq 'Production smoke passed.' "${fixture}/allowed/output" || fail 'production success must be distinguishable'

for spec in '1:200' '3:302' '3:403' '6:404' '6:500' '1:000'; do
  index="${spec%%:*}"
  status="${spec##*:}"
  name="reject-${index}-${status}"
  if run_smoke "${name}" FAIL_AT="${index}" FAIL_STATUS="${status}"; then
    fail "HTTP ${status} at probe ${index} must fail production smoke"
  fi
  [ ! -f "${fixture}/${name}/shared" ] || fail 'failed security probe must stop before shared smoke'
  [ "$(cat "${fixture}/${name}/count")" = "${index}" ] || fail 'failed security probe must stop immediately'
done
if run_smoke network FAIL_AT=1 FAIL_STATUS=000 NETWORK_FAILURE=true; then
  fail 'transport failure must fail production smoke'
fi
[ ! -f "${fixture}/network/shared" ] || fail 'transport failure must stop before shared smoke'
if run_smoke shared-failure SHARED_FAILURE=true; then
  fail 'shared smoke failure must propagate'
fi
echo 'Production smoke offline contract: 9 cases passed.'
