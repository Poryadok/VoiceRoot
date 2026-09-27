#!/usr/bin/env bash
# Patch only Auth mail fields from the uploaded staging Secret; never apply it whole.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
if [ -z "${STAGING_APP_SECRETS_YAML_B64:-}" ]; then
  echo 'ERROR: staging app Secret upload is missing' >&2
  exit 1
fi

patch_file="$(mktemp)"
chmod 600 "$patch_file"
trap 'rm -f "$patch_file"' EXIT
if ! printf '%s' "$STAGING_APP_SECRETS_YAML_B64" |
  base64 -d 2>/dev/null |
  kubectl create --dry-run=client --validate=false -f - -o json 2>/dev/null |
  python3 "$ROOT/scripts/staging/mail-only-patch.py" "$NS" >"$patch_file" 2>/dev/null; then
  echo 'ERROR: uploaded Secret must contain nonblank Auth mail fields' >&2
  exit 1
fi

kubectl get secret voice-app-secrets -n "$NS" -o name >/dev/null
kubectl get deployment voice-auth -n "$NS" -o name >/dev/null
kubectl patch secret voice-app-secrets -n "$NS" --type=merge --patch-file "$patch_file" >/dev/null
kubectl rollout restart deployment/voice-auth -n "$NS" >/dev/null
kubectl rollout status deployment/voice-auth -n "$NS" --timeout=180s
echo 'Staging Auth mail fields updated; Auth rollout ready.'
