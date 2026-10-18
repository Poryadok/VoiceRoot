#!/usr/bin/env bash
# Exercise the real production wrapper/shared DDL path without a cluster.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "${fixture}"' EXIT
mkdir -p "${fixture}/scripts/prod" "${fixture}/scripts/staging/lib" "${fixture}/docker/clickhouse/init" "${fixture}/bin"
cp "${ROOT}/scripts/prod/apply-clickhouse-init.sh" "${fixture}/scripts/prod/"
cp "${ROOT}/scripts/staging/apply-clickhouse-init.sh" "${fixture}/scripts/staging/"
cp "${ROOT}/scripts/staging/lib/kubectl-configmap.sh" "${fixture}/scripts/staging/lib/"
cp "${ROOT}/docker/clickhouse/init/001_events.sql" "${fixture}/docker/clickhouse/init/"
# Match the tracked 100644 script mode on a clean Linux checkout.
chmod 644 "${fixture}/scripts/staging/apply-clickhouse-init.sh"
cat > "${fixture}/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${CASE_DIR}/calls"
case "$*" in
  'create configmap --help') echo '--dry-run=client';;
  'create configmap '*) printf 'kind: ConfigMap\n';;
  'apply -f '*) [[ "${SCENARIO}" != config-failure ]];;
  'get secret '*) printf 'Zml4dHVyZS1vbmx5';;
  *'--multiquery')
    cat > "${CASE_DIR}/applied.sql"
    [[ "${SCENARIO}" != ddl-failure ]]
    ;;
  *'EXISTS TABLE voice.events_logical')
    if [[ "${SCENARIO}" == missing-view ]]; then echo 0; else echo 1; fi
    ;;
  *'EXISTS TABLE voice.events')
    if [[ "${SCENARIO}" == existing || -f "${CASE_DIR}/applied.sql" ]]; then echo 1; else echo 0; fi
    ;;
  'get job '*) return_status=1; exit "${return_status}";;
  *'SELECT 1'*|*'ALTER USER '*) :;;
  *) echo "Unexpected kubectl: $*" >&2; exit 98;;
esac
MOCK
chmod 755 "${fixture}/bin/kubectl"
export PATH="${fixture}/bin:${PATH}"
export CASE_DIR SCENARIO
unset VOICE_K8S_NAMESPACE

run_case() {
  SCENARIO="$1"
  CASE_DIR="${fixture}/${SCENARIO}"
  mkdir -p "${CASE_DIR}"
  local result=0
  bash "${fixture}/scripts/prod/apply-clickhouse-init.sh" > "${CASE_DIR}/output" 2>&1 || result=$?
  if [[ "$2" == pass && "${result}" != 0 || "$2" == fail && "${result}" == 0 ]]; then
    cat "${CASE_DIR}/output" >&2
    echo "FAIL ${SCENARIO}: status=${result}, expected $2" >&2
    exit 1
  fi
  echo "PASS ${SCENARIO}"
}

for scenario in bootstrap existing; do
  run_case "${scenario}" pass
  cmp "${fixture}/docker/clickhouse/init/001_events.sql" "${CASE_DIR}/applied.sql"
  grep -q -- '-n voice-prod' "${CASE_DIR}/calls"
  if grep -q -- 'voice-staging' "${CASE_DIR}/calls"; then exit 1; fi
done
VOICE_K8S_NAMESPACE=custom-production run_case override pass
grep -q -- '-n custom-production' "${CASE_DIR}/calls"
run_case ddl-failure fail
run_case missing-view fail
run_case config-failure fail
test ! -e "${CASE_DIR}/applied.sql"
rm "${fixture}/docker/clickhouse/init/001_events.sql"
run_case missing-sql fail
test ! -e "${CASE_DIR}/calls"
echo 'Production ClickHouse init: 7 cases passed (isolated kubectl fixture).'
