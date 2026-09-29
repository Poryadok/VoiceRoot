#!/usr/bin/env bash
# Keep images-only File/User pods aligned with the public S3 signing origin.
set -euo pipefail

NS="${VOICE_K8S_NAMESPACE:?VOICE_K8S_NAMESPACE required}"
HOST="${VOICE_STORAGE_INGRESS_HOST:-${VOICE_GATEWAY_INGRESS_HOST:?VOICE_GATEWAY_INGRESS_HOST required}}"
ENDPOINT="${VOICE_S3_SIGNING_ENDPOINT:-https://${HOST}}"

kubectl set env deployment/voice-file -n "${NS}" "FILE_R2_SIGNING_ENDPOINT=${ENDPOINT}"
kubectl set env deployment/voice-user -n "${NS}" "USER_R2_SIGNING_ENDPOINT=${ENDPOINT}"
kubectl rollout status deployment/voice-file -n "${NS}" --timeout=300s
kubectl rollout status deployment/voice-user -n "${NS}" --timeout=300s
