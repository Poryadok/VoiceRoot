#!/usr/bin/env bash
# Apply ClickHouse DDL on staging (idempotent IF NOT EXISTS in 001_events.sql).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS="${VOICE_K8S_NAMESPACE:-voice-staging}"
SQL_FILE="${ROOT}/docker/clickhouse/init/001_events.sql"
JOB_NAME="voice-clickhouse-init"
CM_NAME="voice-clickhouse-init"

clickhouse_secret_password() {
  kubectl get secret voice-app-secrets -n "${NS}" -o jsonpath='{.data.CLICKHOUSE_PASSWORD}' 2>/dev/null \
    | base64 -d 2>/dev/null || true
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

sync_clickhouse_password_from_secret() {
  local pass
  pass="$(clickhouse_secret_password)"
  if [ -z "${pass}" ]; then
    echo "WARN: CLICKHOUSE_PASSWORD missing in voice-app-secrets; skip password sync" >&2
    return 1
  fi
  echo "Syncing ClickHouse default user password from voice-app-secrets..."
  kubectl exec -n "${NS}" voice-clickhouse-0 -- env CLICKHOUSE_PASSWORD="${pass}" \
    sh -c 'clickhouse-client --query "ALTER USER default IDENTIFIED BY '"'"'"${CLICKHOUSE_PASSWORD}"'"'"'"'
}

clickhouse_remote_auth_ok() {
  local pass
  pass="$(clickhouse_secret_password)"
  if [ -z "${pass}" ]; then
    return 1
  fi
  kubectl exec -n "${NS}" voice-clickhouse-0 -- env CLICKHOUSE_PASSWORD="${pass}" \
    sh -c 'clickhouse-client --host voice-clickhouse --user default --password "${CLICKHOUSE_PASSWORD}" --query "SELECT 1"' \
    >/dev/null 2>&1
}

wait_clickhouse_remote_auth() {
  echo "Waiting for ClickHouse remote auth on voice-clickhouse:9000..."
  local i
  for i in $(seq 1 15); do
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

  echo "ERROR: ClickHouse remote auth failed after password sync (check CLICKHOUSE_PASSWORD and StatefulSet env)" >&2
  kubectl logs -n "${NS}" voice-clickhouse-0 --tail=80 >&2 || true
  return 1
}

clickhouse_schema_ready() {
  kubectl exec -n "${NS}" voice-clickhouse-0 -- \
    clickhouse-client --query "EXISTS TABLE voice.events" 2>/dev/null | grep -qx '1'
}

apply_clickhouse_init_sql() {
  echo "Applying ClickHouse init SQL via voice-clickhouse-0 (local client)..."
  kubectl exec -i -n "${NS}" voice-clickhouse-0 -- clickhouse-client --multiquery < "${SQL_FILE}"
}

if [ ! -f "${SQL_FILE}" ]; then
  echo "skip clickhouse init: missing ${SQL_FILE}"
  exit 0
fi

echo "Applying ConfigMap ${CM_NAME} from ${SQL_FILE}"
kubectl create configmap "${CM_NAME}" -n "${NS}" \
  --from-file=001_events.sql="${SQL_FILE}" \
  --dry-run=client -o yaml | kubectl apply -f -

if clickhouse_schema_ready; then
  echo "ClickHouse schema already present (voice.events); skipping DDL"
  kubectl delete job "${JOB_NAME}" -n "${NS}" --ignore-not-found >/dev/null 2>&1 || true
  exit 0
fi

if kubectl get job "${JOB_NAME}" -n "${NS}" >/dev/null 2>&1; then
  echo "deleting stale job ${JOB_NAME}"
  kubectl delete job "${JOB_NAME}" -n "${NS}" --ignore-not-found
fi

wait_clickhouse_native
sync_clickhouse_password_from_secret || true
wait_clickhouse_remote_auth
apply_clickhouse_init_sql

if ! clickhouse_schema_ready; then
  echo "ERROR: ClickHouse init SQL applied but voice.events is missing" >&2
  exit 1
fi

echo "ClickHouse init complete."
