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

wait_clickhouse_native() {
  echo "Waiting for ClickHouse native protocol on voice-clickhouse-0..."
  local i
  for i in $(seq 1 30); do
    if kubectl exec -n "${NS}" voice-clickhouse-0 -- clickhouse-client --query 'SELECT 1' >/dev/null 2>&1; then
      echo "ClickHouse native ready (attempt ${i})"
      return 0
    fi
    sleep 2
  done
  echo "ERROR: ClickHouse native protocol not ready after 60s" >&2
  kubectl describe pod voice-clickhouse-0 -n "${NS}" >&2 || true
  return 1
}

wait_clickhouse_remote_auth() {
  echo "Waiting for ClickHouse remote auth on voice-clickhouse:9000..."
  local i
  for i in $(seq 1 30); do
    if clickhouse_remote_auth_ok; then
      echo "ClickHouse remote auth ready (attempt ${i})"
      return 0
    fi
    sleep 2
  done

  echo "Remote auth still failing; syncing default user password from voice-app-secrets..."
  if sync_clickhouse_password_from_secret; then
    for i in $(seq 1 15); do
      if clickhouse_remote_auth_ok; then
        echo "ClickHouse remote auth ready after password sync (attempt ${i})"
        return 0
      fi
      sleep 2
    done
  fi

  echo "ERROR: ClickHouse remote auth failed after 60s (check CLICKHOUSE_PASSWORD and StatefulSet env)" >&2
  kubectl logs -n "${NS}" voice-clickhouse-0 --tail=80 >&2 || true
  return 1
}

clickhouse_remote_auth_ok() {
  kubectl exec -n "${NS}" voice-clickhouse-0 -- \
    sh -c 'clickhouse-client --host voice-clickhouse --user "${CLICKHOUSE_USER}" --password "${CLICKHOUSE_PASSWORD}" --query "SELECT 1"' \
    >/dev/null 2>&1
}

sync_clickhouse_password_from_secret() {
  kubectl exec -n "${NS}" voice-clickhouse-0 -- \
    sh -c 'test -n "${CLICKHOUSE_PASSWORD}" && clickhouse-client --query "ALTER USER default IDENTIFIED BY '"'"'"${CLICKHOUSE_PASSWORD}"'"'"'"'
}

dump_clickhouse_init_job_logs() {
  echo "clickhouse init job status:" >&2
  kubectl get job "${JOB_NAME}" -n "${NS}" -o wide >&2 || true
  kubectl describe job "${JOB_NAME}" -n "${NS}" >&2 || true
  echo "clickhouse init pod logs:" >&2
  kubectl logs -n "${NS}" -l "app.kubernetes.io/name=${JOB_NAME}" --tail=200 >&2 || true
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

wait_clickhouse_native
wait_clickhouse_remote_auth

echo "Applying clickhouse init job ${JOB_NAME}"
substitute < "${TEMPLATE}" | kubectl apply -f -
if ! kubectl wait --for=condition=complete "job/${JOB_NAME}" -n "${NS}" --timeout=300s; then
  dump_clickhouse_init_job_logs
  exit 1
fi

echo "ClickHouse init complete."
