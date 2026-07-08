#!/usr/bin/env bash
# Smoke: provisioned voice-analytics-* Grafana dashboards + ClickHouse datasource queries.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE=(docker compose -f "${ROOT}/docker-compose.yml")

GRAFANA_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_PASS="${GRAFANA_ADMIN_PASSWORD:-changeme-voice-local}"
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"

auth_header() {
  printf 'Authorization: Basic %s' "$(printf '%s:%s' "${GRAFANA_USER}" "${GRAFANA_PASS}" | base64 | tr -d '\n')"
}

wait_grafana() {
  echo "Waiting for Grafana at ${GRAFANA_URL}..."
  for attempt in 1 2 3 4 5 6 7 8 9 10 11 12; do
    code="$(curl -sS -o /dev/null -w "%{http_code}" -H "$(auth_header)" "${GRAFANA_URL}/api/health" || echo "000")"
    if [ "${code}" = "200" ]; then
      return 0
    fi
    echo "grafana attempt ${attempt}/12: HTTP ${code}"
    sleep 5
  done
  echo "Grafana not healthy" >&2
  exit 1
}

assert_dashboard_uid() {
  local uid="$1"
  local code
  code="$(curl -sS -o /dev/null -w "%{http_code}" -H "$(auth_header)" \
    "${GRAFANA_URL}/api/dashboards/uid/${uid}" || echo "000")"
  if [ "${code}" != "200" ]; then
    echo "dashboard uid=${uid} not found (HTTP ${code})" >&2
    exit 1
  fi
  echo "dashboard ok: ${uid}"
}

assert_clickhouse_query() {
  local sql="$1"
  local tmp
  tmp="$(mktemp)"
  local code
  code="$(curl -sS -o "${tmp}" -w "%{http_code}" -X POST \
    -H "$(auth_header)" \
    -H "Content-Type: application/json" \
    "${GRAFANA_URL}/api/ds/query" \
    -d "$(jq -nc \
      --arg sql "${sql}" \
      '{
        queries: [{
          refId: "A",
          datasource: {type: "grafana-clickhouse-datasource", uid: "clickhouse"},
          rawSql: $sql,
          format: 1
        }],
        from: "now-24h",
        to: "now"
      }')" || echo "000")"
  if [ "${code}" != "200" ]; then
    echo "ClickHouse query failed (HTTP ${code}): ${sql}" >&2
    cat "${tmp}" >&2 || true
    rm -f "${tmp}"
    exit 1
  fi
  if ! jq -e '.results.A.frames | length > 0' "${tmp}" >/dev/null 2>&1; then
    echo "ClickHouse query returned no frames: ${sql}" >&2
    cat "${tmp}" >&2 || true
    rm -f "${tmp}"
    exit 1
  fi
  rm -f "${tmp}"
  echo "clickhouse query ok"
}

echo "Starting observability + ClickHouse + analytics for Grafana smoke..."
"${COMPOSE[@]}" --profile app --profile observability up -d \
  clickhouse clickhouse-init prometheus loki grafana analytics

echo "Waiting for clickhouse-init to finish..."
"${COMPOSE[@]}" wait clickhouse-init

wait_grafana

for uid in voice-analytics-ingest voice-analytics-product voice-analytics-engagement; do
  assert_dashboard_uid "${uid}"
done

assert_clickhouse_query "SELECT 1"
assert_clickhouse_query "SELECT date, uniqMerge(unique_users) AS dau FROM voice.dau_mv GROUP BY date ORDER BY date LIMIT 1"
assert_clickhouse_query "SELECT date, sum(event_count) AS cnt FROM voice.events_by_type_mv GROUP BY date LIMIT 1"

echo "Grafana analytics dashboard smoke passed."
