#!/usr/bin/env bash
# Post-deploy smoke for voice-observability namespace (k3s-lite).
set -euo pipefail

NS="${VOICE_OBSERVABILITY_NAMESPACE:-voice-observability}"
GRAFANA_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_PASS="${GRAFANA_ADMIN_PASSWORD:-changeme-voice-observability}"
PROM_LOCAL="${PROMETHEUS_LOCAL_URL:-http://127.0.0.1:9090}"
GRAFANA_LOCAL="${GRAFANA_LOCAL_URL:-http://127.0.0.1:3000}"

auth_header() {
  printf 'Authorization: Basic %s' "$(printf '%s:%s' "${GRAFANA_USER}" "${GRAFANA_PASS}" | base64 | tr -d '\n')"
}

echo "Observability smoke: namespace ${NS}"

not_ready="$(kubectl get pods -n "${NS}" --no-headers 2>/dev/null | awk '$3!="Running" && $3!="Completed" {print}' || true)"
if [ -n "${not_ready}" ]; then
  echo "pods not Running/Completed:" >&2
  echo "${not_ready}" >&2
  kubectl get pods -n "${NS}" >&2 || true
  exit 1
fi
echo "all pods Running or Completed"

kubectl port-forward -n "${NS}" "svc/prometheus" 9090:9090 >/tmp/voice-prom-pf.log 2>&1 &
PF_PROM=$!
kubectl port-forward -n "${NS}" "svc/grafana" 3000:80 >/tmp/voice-graf-pf.log 2>&1 &
PF_GRAF=$!
cleanup() {
  kill "${PF_PROM}" "${PF_GRAF}" 2>/dev/null || true
}
trap cleanup EXIT

for attempt in 1 2 3 4 5 6 7 8 9 10; do
  if curl -sS -o /dev/null "${PROM_LOCAL}/-/ready" 2>/dev/null; then
    break
  fi
  sleep 2
done

up_count="$(curl -sS "${PROM_LOCAL}/api/v1/query" --data-urlencode 'query=up{job=~"voice-.*"}' \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for r in d.get("data",{}).get("result",[]) if float(r.get("value",[0,0])[1])>0))' 2>/dev/null || echo 0)"
if [ "${up_count}" -lt 1 ]; then
  echo "Prometheus: no voice app targets UP (count=${up_count})" >&2
  exit 1
fi
echo "Prometheus: ${up_count} voice target(s) UP"

for attempt in 1 2 3 4 5 6 7 8 9 10; do
  code="$(curl -sS -o /dev/null -w "%{http_code}" -H "$(auth_header)" "${GRAFANA_LOCAL}/api/health" || echo "000")"
  if [ "${code}" = "200" ]; then
    break
  fi
  sleep 2
done
if [ "${code}" != "200" ]; then
  echo "Grafana health failed: HTTP ${code}" >&2
  exit 1
fi
echo "Grafana health ok"

for uid in voice-overview tier0-paths infrastructure logs-request-id; do
  dash_code="$(curl -sS -o /dev/null -w "%{http_code}" -H "$(auth_header)" \
    "${GRAFANA_LOCAL}/api/dashboards/uid/${uid}" || echo "000")"
  if [ "${dash_code}" != "200" ]; then
    echo "Grafana dashboard ${uid} missing (HTTP ${dash_code})" >&2
    exit 1
  fi
  echo "Grafana dashboard ok: ${uid}"
done

echo "Observability smoke passed."
