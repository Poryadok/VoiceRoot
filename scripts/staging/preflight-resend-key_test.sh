#!/usr/bin/env bash
# Contract: staging rejects absent or blank mail credentials before any mutation.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
ACL_PROOF_SHA="$(sha256sum "${ROOT}/deploy/nats/acl-intent.yaml" | cut -d' ' -f1)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT
export TEST_STATE="${TMP}"
mkdir -p "${TMP}/bin"
cat >"${TMP}/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1:-}" = create ]; then
  [ "$*" = 'create --dry-run=client --validate=false -f - -o json' ] || exit 96
  python3 -c '
import json, sys
fields = {}
section = None
for line in sys.stdin:
    if line.startswith("metadata:"):
        section = "metadata"
    elif line.startswith("stringData:"):
        section = "stringData"
    elif line.startswith("data:"):
        section = "data"
    elif not line.startswith(" "):
        section = None
    if section and line.startswith("  ") and ":" in line:
        name, value = line.strip().split(":", 1)
        value = value.strip()
        fields.setdefault(section, {})[name] = json.loads(value) if value.startswith("\"") else value
print(json.dumps({"kind": "Secret", **fields}))
'
  exit 0
fi
if [ "${1:-}" = get ] && [ "${2:-}" = secret ]; then
  cat "${TEST_STATE}/existing.json"
  exit 0
fi
if [ "$*" = 'get configmap voice-nats-generation -n voice-staging -o json' ]; then
  printf 'Error from server (NotFound): configmaps "voice-nats-generation" not found\n' >&2
  exit 1
fi
printf '%s\n' mutation >>"${TEST_STATE}/mutations"
exit 97
MOCK
chmod +x "${TMP}/bin/kubectl"

key="$(openssl rand -hex 24)"
trap 'rm -rf "${TMP}"; unset key' EXIT
encoded_key="$(printf '%s' "$key" | base64 | tr -d '\r\n')"
fixture_keys="$(awk '/^---/{exit} /^  [A-Z][A-Z0-9_]*:/{sub(":", "", $1); if ($1 != "AUTH_RESEND_API_KEY") print $1}' "${ROOT}/deploy/staging/secret.example.yaml")"
fixture_encoded="$(printf fixture | base64 | tr -d '\r\n')"
printf '{"kind":"Secret","metadata":{"name":"voice-app-secrets","namespace":"voice-staging","resourceVersion":"42"},"data":{"AUTH_RESEND_API_KEY":"%s"}}\n' \
  "$encoded_key" >"${TMP}/existing.json"

manifest() {
  printf 'apiVersion: v1\nkind: Secret\nmetadata:\n  name: voice-app-secrets\n'
  if [ "$1" != missing_namespace ]; then printf '  namespace: voice-staging\n'; fi
  if [ "$1" = encoded ] || [ "$1" = invalid_data ]; then printf 'data:\n'; else printf 'stringData:\n'; fi
  local fixture_value=fixture
  if [ "$1" = encoded ] || [ "$1" = invalid_data ]; then fixture_value="$fixture_encoded"; fi
  local fixture_key
  for fixture_key in $fixture_keys; do
    [ "$1" = missing_required ] && [ "$fixture_key" = BOT_DATABASE_URL ] && continue
    printf '  %s: "%s"\n' "$fixture_key" "$fixture_value"
  done
  case "$1" in
    missing) ;;
    blank) printf '  AUTH_RESEND_API_KEY: ""\n' ;;
    spaces) printf '  AUTH_RESEND_API_KEY: "   "\n' ;;
    populated|missing_required) printf '  AUTH_RESEND_API_KEY: "%s"\n' "$key" ;;
    encoded) printf '  AUTH_RESEND_API_KEY: "%s"\n' "$encoded_key" ;;
    invalid_data) printf '  AUTH_RESEND_API_KEY: "!"\n' ;;
    missing_namespace) printf '  AUTH_RESEND_API_KEY: "%s"\n' "$key" ;;
  esac
}

run_manifest_case() {
  local kind="$1" expected_mutations="$2" status
  rm -f "${TMP}/mutations"
  manifest "$kind" | base64 | tr -d '\r\n' >"${TMP}/input.b64"
  set +e
  PATH="${TMP}/bin:${PATH}" VOICE_IMAGE_TAG=contract-test DEPLOY_MODE=full \
    VOICE_NATS_STORAGE_CLASS=local-path VOICE_NATS_STORAGE_SIZE=1Gi \
    VOICE_NATS_ACL_PROOF_SHA="${ACL_PROOF_SHA}" \
    STAGING_APP_SECRETS_YAML_B64="$(cat "${TMP}/input.b64")" \
    bash "${ROOT}/scripts/staging/render-and-apply.sh" >"${TMP}/output" 2>&1
  status=$?
  set -e
  ! grep -Fq "$key" "${TMP}/output" || { echo 'FAIL: key leaked to output' >&2; exit 1; }
  ! grep -Fq "$encoded_key" "${TMP}/output" || { echo 'FAIL: encoded key leaked to output' >&2; exit 1; }
  ! grep -Fq "$(cat "${TMP}/input.b64")" "${TMP}/output" || { echo 'FAIL: manifest leaked to output' >&2; exit 1; }
  if [ "$expected_mutations" = no ]; then
    [ "$status" -ne 0 ] || { echo "FAIL: ${kind} key was accepted" >&2; exit 1; }
    [ ! -f "${TMP}/mutations" ] || { echo "FAIL: ${kind} key reached mutation" >&2; exit 1; }
  else
    [ -f "${TMP}/mutations" ] || { echo "FAIL: ${kind} key did not pass preflight (status ${status})" >&2; exit 1; }
  fi
}

run_manifest_case missing yes
run_manifest_case blank no
run_manifest_case spaces no
run_manifest_case populated yes
run_manifest_case missing_required no
run_manifest_case encoded yes
run_manifest_case invalid_data no
run_manifest_case missing_namespace no

# The hosted validation-only job has no Kubernetes context, so YAML parsing
# must work without invoking kubectl or accessing a cluster.
manifest populated | python3 "${ROOT}/scripts/staging/check-resend-key.py" voice-staging --yaml >/dev/null ||
  { echo 'FAIL: complete YAML did not pass offline validation' >&2; exit 1; }
if manifest missing_required | python3 "${ROOT}/scripts/staging/check-resend-key.py" voice-staging --yaml >/dev/null; then
  echo 'FAIL: incomplete YAML passed offline validation' >&2
  exit 1
fi

run_existing_case() {
  local kind="$1" expected_mutations="$2" status
  rm -f "${TMP}/mutations"
  {
    printf '{"kind":"Secret","metadata":{"name":"voice-app-secrets","namespace":"voice-staging"},"data":{'
    local fixture_key
    for fixture_key in $fixture_keys; do
      [ "$kind" = missing_required ] && [ "$fixture_key" = BOT_DATABASE_URL ] && continue
      printf '"%s":"%s",' "$fixture_key" "$fixture_encoded"
    done
    case "$kind" in
      missing) printf '"AUTH_RESEND_API_KEY":null' ;;
      blank) printf '"AUTH_RESEND_API_KEY":""' ;;
      populated|missing_required) printf '"AUTH_RESEND_API_KEY":"%s"' "$encoded_key" ;;
    esac
    printf '}}\n'
  } >"${TMP}/existing.json"
  set +e
  PATH="${TMP}/bin:${PATH}" VOICE_IMAGE_TAG=contract-test DEPLOY_MODE=full \
    VOICE_NATS_STORAGE_CLASS=local-path VOICE_NATS_STORAGE_SIZE=1Gi \
    VOICE_NATS_ACL_PROOF_SHA="${ACL_PROOF_SHA}" \
    STAGING_APP_SECRETS_YAML_B64= \
    bash "${ROOT}/scripts/staging/render-and-apply.sh" >"${TMP}/output" 2>&1
  status=$?
  set -e
  ! grep -Fq "$key" "${TMP}/output" || { echo 'FAIL: key leaked to output' >&2; exit 1; }
  ! grep -Fq "$encoded_key" "${TMP}/output" || { echo 'FAIL: encoded key leaked to output' >&2; exit 1; }
  if [ "$expected_mutations" = no ]; then
    [ "$status" -ne 0 ] || { echo "FAIL: existing ${kind} key was accepted" >&2; exit 1; }
    [ ! -f "${TMP}/mutations" ] || { echo "FAIL: existing ${kind} key reached mutation" >&2; exit 1; }
  else
    [ -f "${TMP}/mutations" ] || { echo "FAIL: existing ${kind} key did not pass preflight" >&2; exit 1; }
  fi
}

run_existing_case missing no
run_existing_case blank no
run_existing_case populated yes
run_existing_case missing_required no
echo 'Staging Resend preflight contract passed.'
