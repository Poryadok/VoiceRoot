#!/usr/bin/env bash
# Apply app manifests (services, gateway, frontends) with image tag substitution.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/staging/load-staging-domains.sh
source "${ROOT}/scripts/staging/load-staging-domains.sh"
REGISTRY="${VOICE_IMAGE_REGISTRY:?VOICE_IMAGE_REGISTRY required}"
TAG="${VOICE_IMAGE_TAG:?VOICE_IMAGE_TAG required}"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"

render() {
  sed -e "s|__IMAGE_REGISTRY__|${REGISTRY}|g" \
      -e "s|__IMAGE_TAG__|${TAG}|g" \
      -e "s|IMAGE_PLACEHOLDER|${REGISTRY}/gateway:${TAG}|g" \
      "$1"
}

patch_image_pull_secrets() {
  local secret_name="${VOICE_IMAGE_PULL_SECRET:-}"
  if [ -z "${secret_name}" ]; then
    return 0
  fi
  for dep in $(kubectl get deployment -n "${NS}" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
    kubectl patch deployment "${dep}" -n "${NS}" --type=strategic \
      -p="{\"spec\":{\"template\":{\"spec\":{\"imagePullSecrets\":[{\"name\":\"${secret_name}\"}]}}}}" \
      2>/dev/null || true
  done
}

auth_pre_scale_needed() {
  if [ "${NEEDS_FULL_ROLLOUT:-false}" = "true" ]; then
    return 0
  fi
  case ",${CHANGED_SERVICES:-}," in
    *,auth,* ) return 0 ;;
  esac
  return 1
}

scale_auth_down_if_needed() {
  if ! auth_pre_scale_needed; then
    echo "Skipping auth scale-down (selective deploy without auth change)"
    return 0
  fi
  if ! kubectl get deployment voice-auth -n "${NS}" >/dev/null 2>&1; then
    return 0
  fi
  kubectl scale deployment/voice-auth -n "${NS}" --replicas=0
  kubectl wait --for=delete pod -l app=voice-auth -n "${NS}" --timeout=180s 2>/dev/null || true
}

prepare_notification_recreate_transition() {
  if ! kubectl get deployment voice-notification -n "${NS}" >/dev/null 2>&1; then
    return 0
  fi
  # Remove server-defaulted rollingUpdate before applying the canonical Recreate manifest.
  kubectl patch deployment voice-notification -n "${NS}" --type=strategic \
    -p '{"spec":{"strategy":{"$retainKeys":["type"],"type":"Recreate"}}}'
}

prepare_singleton_nats_recreate_transitions() {
  local deployment
  for deployment in voice-bot voice-chat voice-matchmaking voice-space; do
    if ! kubectl get deployment "${deployment}" -n "${NS}" >/dev/null 2>&1; then
      continue
    fi
    # These fixed push durables have no queue group; stop the old subscriber
    # before the new one starts, and remove server-defaulted rollingUpdate.
    kubectl patch deployment "${deployment}" -n "${NS}" --type=strategic \
      -p '{"spec":{"strategy":{"$retainKeys":["type"],"type":"Recreate"}}}'
  done
}

require_nats_bootstrap() {
  for job in voice-nats-realtime-bootstrap voice-nats-notification-bootstrap voice-nats-search-bootstrap voice-nats-analytics-chat-bootstrap; do
    if ! kubectl wait --for=condition=complete "job/${job}" -n "${NS}" --timeout=5s; then
      echo "ERROR: required NATS bootstrap ${job} is incomplete; run apply-infra before app rollout" >&2
      exit 1
    fi
  done
}

bash "${ROOT}/scripts/staging/check-social-principal-secrets.sh"
require_nats_bootstrap
scale_auth_down_if_needed

prepare_notification_recreate_transition
prepare_singleton_nats_recreate_transitions
render "${ROOT}/deploy/staging/services.yaml" | kubectl apply -f -
sed "s|__K_NAMESPACE__|${NS}|g" \
  "${ROOT}/deploy/templates/network-policy-social-privacy-principal.yaml" | kubectl apply -f -
sed "s|__K_NAMESPACE__|${NS}|g" \
  "${ROOT}/deploy/templates/network-policy-file-user-principal.yaml" | kubectl apply -f -
sed "s|__K_NAMESPACE__|${NS}|g" \
  "${ROOT}/deploy/templates/network-policy-search-user-projection.yaml" | kubectl apply -f -
render "${ROOT}/deploy/staging/gateway-deployment.yaml" | kubectl apply -f -

if auth_pre_scale_needed; then
  scale_auth_down_if_needed
fi

patch_image_pull_secrets

if [ -f "${ROOT}/deploy/staging/developer-portal.yaml" ]; then
  render "${ROOT}/deploy/staging/developer-portal.yaml" | \
    sed -e "s|__DEVELOPER_PORTAL_INGRESS_HOST__|${VOICE_DEVELOPER_PORTAL_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi
if [ -f "${ROOT}/deploy/staging/flutter-web.yaml" ]; then
  render "${ROOT}/deploy/staging/flutter-web.yaml" | \
    sed -e "s|__WEB_INGRESS_HOST__|${VOICE_WEB_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi
if [ -f "${ROOT}/deploy/staging/admin.yaml" ]; then
  render "${ROOT}/deploy/staging/admin.yaml" | \
    sed -e "s|__ADMIN_INGRESS_HOST__|${VOICE_ADMIN_INGRESS_HOST}|g" | \
    kubectl apply -f -
fi

patch_image_pull_secrets
echo "Staging app manifests applied (tag ${TAG})."
