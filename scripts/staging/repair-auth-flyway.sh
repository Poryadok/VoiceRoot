#!/usr/bin/env bash
# Reconcile auth_db Flyway checksums before Auth JVM starts (idempotent).
# Safe on every deploy: repair is a no-op when checksums already match.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
FLYWAY_TAG="${VOICE_FLYWAY_IMAGE_TAG:-flyway/flyway:11.3.0}"
MIGRATIONS_DIR="${ROOT}/src/backend/auth/src/main/resources/db/migration"
CM_NAME="voice-auth-flyway-migrations"
JOB_NAME="voice-repair-auth-flyway"
TEMPLATE="${ROOT}/deploy/templates/repair-auth-flyway-job.yaml"

dump_repair_job_logs() {
  echo "repair job ${JOB_NAME} status:" >&2
  kubectl get job "${JOB_NAME}" -n "${NS}" -o wide >&2 || true
  kubectl describe job "${JOB_NAME}" -n "${NS}" >&2 || true
  local pod
  pod="$(kubectl get pods -n "${NS}" -l "job-name=${JOB_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [ -n "${pod}" ]; then
    kubectl describe pod "${pod}" -n "${NS}" >&2 || true
    kubectl logs "${pod}" -n "${NS}" --all-containers=true --tail=120 >&2 || true
  fi
}

if [ ! -d "${MIGRATIONS_DIR}" ]; then
  echo "skip auth flyway repair: missing ${MIGRATIONS_DIR}"
  exit 0
fi

if ! kubectl get secret voice-app-secrets -n "${NS}" >/dev/null 2>&1; then
  echo "ERROR: secret voice-app-secrets missing in ${NS}; cannot repair auth flyway" >&2
  exit 1
fi

bash "${ROOT}/scripts/staging/sync-postgres-password.sh"

echo "Applying Flyway migrations ConfigMap ${CM_NAME} from ${MIGRATIONS_DIR}"
kubectl create configmap "${CM_NAME}" -n "${NS}" \
  --from-file="${MIGRATIONS_DIR}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl delete job "${JOB_NAME}" -n "${NS}" --ignore-not-found

echo "Applying Flyway repair job ${JOB_NAME}"
sed -e "s|__K_NAMESPACE__|${NS}|g" \
    -e "s|__FLYWAY_IMAGE_TAG__|${FLYWAY_TAG}|g" \
  "${TEMPLATE}" | kubectl apply -f -

if ! kubectl wait --for=condition=complete "job/${JOB_NAME}" -n "${NS}" --timeout=180s; then
  dump_repair_job_logs
  exit 1
fi

echo "Auth Flyway repair+migrate complete."
