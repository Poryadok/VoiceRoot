#!/usr/bin/env bash
# Offline regression tests of the real observability entrypoint; no cluster access.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT
mkdir -p "${TMP}/bin"

fail() { echo "FAIL: $*" >&2; exit 1; }

cat >"${TMP}/bin/promtool" <<'MOCK'
#!/usr/bin/env bash
exit 0
MOCK
cat >"${TMP}/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
namespace=''
args=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    -n|--namespace) namespace="$2"; shift 2 ;;
    --namespace=*) namespace="${1#*=}"; shift ;;
    *) args+=("$1"); shift ;;
  esac
done
set -- "${args[@]}"
if [ "$1" = create ] && [[ " $* " == *' --help '* ]]; then
  if [ "${CLIENT}" = legacy ]; then
    echo '  --dry-run=false: If true, only print the object'
  else
    echo '  --dry-run="none": Must be "none", "server", or "client"'
  fi
  exit 0
fi
if [ "$1" = create ] && [ "$2" = configmap ]; then
  printf '%s\n' 'apiVersion: v1' 'kind: ConfigMap' 'metadata:' "  name: $3"
  exit 0
fi
if [ "$1" = create ] && [ "$2" = secret ] && [ "$3" = generic ]; then
  : >"${CASE_DIR}/secret-attempted"
  [ "$4" = grafana-admin ] || exit 81
  [ "${namespace}" = "${EXPECTED_NAMESPACE}" ] || exit 82
  user='' password='' dry_run='' output=''
  shift 4
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --from-literal=admin-user=*) user="${1#--from-literal=admin-user=}"; shift ;;
      --from-literal=admin-password=*) password="${1#--from-literal=admin-password=}"; shift ;;
      --dry-run|--dry-run=true|--dry-run=client) dry_run="$1"; shift ;;
      -o) output="$2"; shift 2 ;;
      *) echo "Unexpected secret argument" >&2; exit 83 ;;
    esac
  done
  [ "$user" = admin ] && [ "$password" = "${EXPECTED_PASSWORD}" ] && [ "$output" = yaml ] || exit 84
  if [ "${CLIENT}" = legacy ]; then
    if [ "$dry_run" != --dry-run ] && [ "$dry_run" != --dry-run=true ]; then
      echo 'legacy kubectl: invalid boolean value "client" for --dry-run' >&2
      exit 85
    fi
  else
    [ "$dry_run" = --dry-run=client ] || exit 86
  fi
  : >"${CASE_DIR}/contract-checked"
  case "${SCENARIO}" in
    failed) exit 23 ;;
    empty) exit 0 ;;
  esac
  printf '%s\n' 'apiVersion: v1' 'kind: Secret' 'metadata:' '  name: grafana-admin' \
    "  namespace: ${namespace}" 'type: Opaque' 'data:' \
    "  admin-user: $(printf %s "$user" | base64 | tr -d '\n')" \
    "  admin-password: $(printf %s "$password" | base64 | tr -d '\n')" >"${CASE_DIR}/generated.yaml"
  cat "${CASE_DIR}/generated.yaml"
  [ "${SCENARIO}" != partial-failure ] || exit 23
  exit 0
fi
if [ "$1" = apply ] && [ "$2" = -f ]; then
  if [ "$3" = - ]; then
    cat >"${CASE_DIR}/candidate.yaml"
  else
    cat "$3" >"${CASE_DIR}/candidate.yaml"
  fi
  # ConfigMap applies precede the Grafana Secret. Capture empty streams too:
  # failed generators must never invoke apply, even if kubectl would reject it.
  if grep -q '^kind: ConfigMap$' "${CASE_DIR}/candidate.yaml"; then exit 0; fi
  if [ "$3" = - ] || [[ "$3" != *'/deploy/observability/'* ]]; then
    echo apply >>"${CASE_DIR}/secret-applies"
    cp "${CASE_DIR}/candidate.yaml" "${CASE_DIR}/applied.yaml"
    [ "${SCENARIO}" != apply-failure ] || exit 37
  elif [ -e "${CASE_DIR}/secret-attempted" ]; then
    echo workload >>"${CASE_DIR}/after-secret"
  fi
  exit 0
fi
if [ "$1" = get ]; then exit 1; fi
if [ "$1" = rollout ]; then echo rollout >>"${CASE_DIR}/after-secret"; exit 0; fi
echo "Unexpected kubectl command: $1" >&2
exit 87
MOCK
chmod +x "${TMP}/bin/kubectl" "${TMP}/bin/promtool"

run_case() (
  export CLIENT="$1" SCENARIO="$2"
  local variant="${3:-override}"
  export CASE_DIR="${TMP}/${CLIENT}-${SCENARIO}-${variant}"
  mkdir -p "${CASE_DIR}"
  export EXPECTED_NAMESPACE=voice-observability EXPECTED_PASSWORD=changeme-voice-observability
  unset GRAFANA_ADMIN_PASSWORD VOICE_OBSERVABILITY_NAMESPACE
  if [ "$variant" = override ]; then
    export EXPECTED_NAMESPACE=observability-fixture
    # Literal test data, including shell metacharacters and spaces, never a real credential.
    export EXPECTED_PASSWORD='fixture password $dollar; "quotes" & / = end'
    export GRAFANA_ADMIN_PASSWORD="${EXPECTED_PASSWORD}"
    export VOICE_OBSERVABILITY_NAMESPACE="${EXPECTED_NAMESPACE}"
  fi
  local status=0
  PATH="${TMP}/bin:$PATH" OBSERVABILITY_PROFILE=k3s-lite NOTIFICATIONS_ENABLED=false \
    STAGING_CLICKHOUSE_PASSWORD=fixture-clickhouse \
    bash "${ROOT}/scripts/staging/apply-observability.sh" >"${CASE_DIR}/output" 2>&1 || status=$?
  if [ "$SCENARIO" = success ]; then
    if [ "$status" -ne 0 ]; then
      cat "${CASE_DIR}/output" >&2
      fail "$CLIENT/$variant: expected success, got $status"
    fi
    [ -f "${CASE_DIR}/contract-checked" ] || fail 'Secret identity/credentials were not checked'
    [ -f "${CASE_DIR}/applied.yaml" ] || fail 'Secret was not applied'
    cmp "${CASE_DIR}/generated.yaml" "${CASE_DIR}/applied.yaml" || fail 'Secret manifest changed'
    [ "$(wc -l <"${CASE_DIR}/secret-applies")" -eq 1 ] || fail 'Secret apply must occur once'
    [ -s "${CASE_DIR}/after-secret" ] || fail 'Workloads did not proceed after Secret success'
  else
    [ "$status" -ne 0 ] || fail "$CLIENT/$SCENARIO: expected failure"
    [ -e "${CASE_DIR}/contract-checked" ] || fail 'Failed before exercising requested generation behavior'
    [ ! -e "${CASE_DIR}/after-secret" ] || fail 'Workloads continued after Secret failure'
    if [ "$SCENARIO" = apply-failure ]; then
      [ "$status" -eq 37 ] || fail "Apply failure exit 37 was not preserved: $status"
      cmp "${CASE_DIR}/generated.yaml" "${CASE_DIR}/applied.yaml" || fail 'Apply did not receive generated Secret'
    else
      [ ! -e "${CASE_DIR}/secret-applies" ] || fail "$CLIENT/$SCENARIO: generation failure reached apply"
    fi
  fi
  echo "PASS: $CLIENT/$SCENARIO/$variant"
)

failures=0
for client in legacy modern; do
  for scenario in success failed empty partial-failure apply-failure; do
    # Keep errexit active inside each case while collecting all regression failures.
    set +e
    (set -e; run_case "$client" "$scenario")
    status=$?
    set -e
    if [ "$status" -ne 0 ]; then failures=$((failures + 1)); fi
  done
  set +e
  (set -e; run_case "$client" success default)
  status=$?
  set -e
  if [ "$status" -ne 0 ]; then failures=$((failures + 1)); fi
done
[ "$failures" -eq 0 ] || fail "$failures observability Secret regression cases failed"
echo 'Observability Secret compatibility tests passed.'
