#!/usr/bin/env bash
# Read-only preflight; never prints secret values or changes a deployment.
set -euo pipefail
namespace="${VOICE_K8S_NAMESPACE:-voice-staging}"
check_secret() {
  local name="$1" required="$2"
  if ! kubectl get secret "$name" -n "$namespace" -o json |
      jq -e --argjson required "$required" '.data as $data | all($required[]; (($data[.] // "") | length) > 0)' >/dev/null; then
    echo "Missing Social principal secret material: $namespace/$name; see docs/DEPLOYMENT.md" >&2
    return 1
  fi
}
check_secret voice-social-principal-signing '["current.pem","next.pem","active-kid"]'
check_secret voice-file-principal-signing '["current.pem","next.pem","active-kid"]'
check_secret voice-principal-ca '["ca.crt"]'
for service in social user space file user-file; do
  check_secret "voice-$service-principal-tls" '["tls.crt","tls.key"]'
done
