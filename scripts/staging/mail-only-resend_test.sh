#!/usr/bin/env bash
# Every kubectl call is mocked: prove mail-only mode cannot mutate other resources.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
export TEST_STATE="$TMP"
mkdir -p "$TMP/bin"
cat >"$TMP/bin/kubectl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
case "$*" in
  'create --dry-run=client --validate=false -f - -o json')
    echo create >>"$TEST_STATE/calls"
    python3 -c '
import json, sys
fields = {}
section = None
for line in sys.stdin:
    if line.startswith(("metadata:", "stringData:", "data:")):
        section = line.split(":", 1)[0]
        fields[section] = {}
    elif line.startswith("  ") and section and ":" in line:
        name, value = line.strip().split(":", 1)
        value = value.strip()
        fields[section][name] = json.loads(value) if value.startswith("\"") else value
    elif not line.startswith(" "):
        section = None
print(json.dumps({"kind": "Secret", **fields}))
' ;;
  'get secret voice-app-secrets -n voice-staging -o name')
    echo get-secret >>"$TEST_STATE/calls"
    echo secret/voice-app-secrets ;;
  'get deployment voice-auth -n voice-staging -o name')
    echo get-auth >>"$TEST_STATE/calls"
    echo deployment.apps/voice-auth ;;
  patch\ secret\ voice-app-secrets\ -n\ voice-staging\ --type=merge\ --patch-file\ *)
    echo patch >>"$TEST_STATE/calls"
    python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); assert set(p)=={"data"}; assert set(p["data"])=={"AUTH_RESEND_API_KEY","AUTH_RESEND_FROM"}' "${!#}"
    echo secret/voice-app-secrets ;;
  'rollout restart deployment/voice-auth -n voice-staging')
    echo restart >>"$TEST_STATE/calls" ;;
  'rollout status deployment/voice-auth -n voice-staging --timeout=180s')
    echo status >>"$TEST_STATE/calls" ;;
  *) echo "unexpected kubectl command" >&2; exit 90 ;;
esac
MOCK
chmod +x "$TMP/bin/kubectl"

key="$(openssl rand -hex 24)"
encoded_key="$(printf '%s' "$key" | base64 | tr -d '\r\n')"
manifest() {
  printf 'apiVersion: v1\nkind: Secret\nmetadata:\n  name: voice-app-secrets\n  namespace: voice-staging\nstringData:\n'
  [ "$1" = missing_key ] || printf '  AUTH_RESEND_API_KEY: "%s"\n' "$key"
  [ "$1" = missing_from ] || printf '  AUTH_RESEND_FROM: "Voice <sender@example.invalid>"\n'
  printf '  POSTGRES_PASSWORD: "unrelated-placeholder"\n'
}

run_case() {
  local kind="$1" expected="$2" status
  rm -f "$TMP/calls"
  set +e
  PATH="$TMP/bin:$PATH" VOICE_K8S_NAMESPACE=voice-staging \
    STAGING_APP_SECRETS_YAML_B64="$(manifest "$kind" | base64 | tr -d '\r\n')" \
    bash "$ROOT/scripts/staging/mail-only-resend.sh" >"$TMP/output" 2>&1
  status=$?
  set -e
  ! grep -Fq "$key" "$TMP/output" || { echo 'FAIL: mail key leaked' >&2; exit 1; }
  ! grep -Fq "$encoded_key" "$TMP/output" || { echo 'FAIL: encoded mail key leaked' >&2; exit 1; }
  if [ "$expected" = reject ]; then
    [ "$status" -ne 0 ] || { echo 'FAIL: partial mail Secret accepted' >&2; exit 1; }
    [ "$(cat "$TMP/calls")" = create ] || { echo 'FAIL: partial mail Secret reached mutation' >&2; exit 1; }
  else
    [ "$status" -eq 0 ] || { echo 'FAIL: complete mail Secret rejected' >&2; cat "$TMP/calls" "$TMP/output" >&2; exit 1; }
    [ "$(tr '\n' ' ' <"$TMP/calls")" = 'create get-secret get-auth patch restart status ' ] ||
      { echo 'FAIL: mail-only kubectl call sequence changed' >&2; exit 1; }
  fi
}

run_case missing_key reject
run_case missing_from reject
run_case complete accept
echo 'Mail-only staging contract passed.'
