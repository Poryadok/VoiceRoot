#!/usr/bin/env bash
# Apply ClickHouse DDL on staging (idempotent IF NOT EXISTS in 001_events.sql).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SQL_FILE="${ROOT}/docker/clickhouse/init/001_events.sql"
JOB_NAME="voice-clickhouse-init"
CM_NAME="voice-clickhouse-init"
TEMPLATE="${ROOT}/deploy/templates/clickhouse-init-job.yaml"

substitute() {
  sed -e "s|__K_NAMESPACE__|${NS}|g"
}

if [ ! -f "${SQL_FILE}" ]; then
  echo "skip clickhouse init: missing ${SQL_FILE}"
  exit 0
fi

echo "Applying ConfigMap ${CM_NAME} from ${SQL_FILE}"
kubectl create configmap "${CM_NAME}" -n "${NS}" \
  --from-file=001_events.sql="${SQL_FILE}" \
  --dry-run=client -o yaml | kubectl apply -f -

if kubectl get job "${JOB_NAME}" -n "${NS}" >/dev/null 2>&1; then
  succeeded="$(kubectl get job "${JOB_NAME}" -n "${NS}" -o jsonpath='{.status.succeeded}' 2>/dev/null || echo 0)"
  if [ "${succeeded:-0}" = "1" ]; then
    echo "clickhouse init job ${JOB_NAME} already succeeded; skipping"
    exit 0
  fi
  echo "deleting incomplete job ${JOB_NAME}"
  kubectl delete job "${JOB_NAME}" -n "${NS}" --ignore-not-found
fi

echo "Applying clickhouse init job ${JOB_NAME}"
substitute < "${TEMPLATE}" | kubectl apply -f -
kubectl wait --for=condition=complete "job/${JOB_NAME}" -n "${NS}" --timeout=300s

echo "ClickHouse init complete."
