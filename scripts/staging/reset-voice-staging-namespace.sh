#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
namespace="${VOICE_K8S_NAMESPACE:-voice-staging}"
storage_class="${VOICE_NATS_STORAGE_CLASS:-}"
storage_size="${VOICE_NATS_STORAGE_SIZE:-}"

fail() { echo "ERROR: Voice staging clean install blocked: $*" >&2; exit 1; }

[ "${VOICE_STAGING_WIPE_APPROVED:-false}" = true ] || fail 'explicit workflow wipe approval is required'
[ "$namespace" = voice-staging ] || fail 'only the dedicated voice-staging namespace may be reset'
[[ "$storage_class" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || fail 'a valid NATS storage class is required'
[[ "$storage_size" =~ ^[1-9][0-9]*(Mi|Gi|Ti)$ ]] || fail 'a valid NATS storage size is required'

kubectl get namespace voice-staging >/dev/null
echo 'Deleting only namespace voice-staging and its namespaced Voice data.'
kubectl delete namespace voice-staging --wait=true --timeout=300s
kubectl apply -f "${root}/deploy/staging/namespace.yaml"
kubectl wait --for=jsonpath='{.status.phase}'=Active namespace/voice-staging --timeout=120s
kubectl create configmap voice-nats-clean-install-state \
  --namespace voice-staging \
  --from-literal=mode=clean-install \
  --from-literal=storageClass="${storage_class}" \
  --from-literal=storageSize="${storage_size}"
