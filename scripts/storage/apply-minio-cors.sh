#!/usr/bin/env bash
# State-preserving CORS update for selective app rollouts. MinIO's PVC is untouched.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:?VOICE_K8S_NAMESPACE required}"
WEB_HOST="${VOICE_WEB_INGRESS_HOST:?VOICE_WEB_INGRESS_HOST required}"

kubectl set env statefulset/voice-minio -n "${NS}" \
  "MINIO_API_CORS_ALLOW_ORIGIN=https://${WEB_HOST}"
kubectl rollout status statefulset/voice-minio -n "${NS}" --timeout=300s
