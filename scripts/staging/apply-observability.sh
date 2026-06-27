#!/usr/bin/env bash
# Apply Voice observability stack to k3s/Kubernetes.
#
# Usage (from repo root):
#   scripts/staging/apply-observability.sh
#   OBSERVABILITY_PROFILE=k3s-lite scripts/staging/apply-observability.sh
#   NOTIFICATIONS_ENABLED=true scripts/staging/apply-observability.sh
#   GRAFANA_ADMIN_PASSWORD='...' scripts/staging/apply-observability.sh
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OBS_DIR="${ROOT}/deploy/observability"
PROFILE="${OBSERVABILITY_PROFILE:-k3s-lite}"
NS="${VOICE_OBSERVABILITY_NAMESPACE:-voice-observability}"

if [ "${PROFILE}" != "k3s-lite" ]; then
  echo "ERROR: only k3s-lite profile is implemented via kubectl apply."
  echo "For full stack use Helm — see deploy/observability/profiles/full/values-kube-prometheus.yaml"
  exit 1
fi

PROFILE_DIR="${OBS_DIR}/profiles/k3s-lite"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

echo "Voice observability: profile=${PROFILE} namespace=${NS}"

# --- Validate Prometheus rules (optional; skip if promtool not installed) ---
if command -v promtool >/dev/null 2>&1; then
  echo "Validating Prometheus rules..."
  promtool check rules "${OBS_DIR}/prometheus/rules/"*.yaml
else
  echo "WARN: promtool not found — skip rule validation (install from prometheus release tarball)"
fi

# --- Prometheus config (base + scrape jobs) ---
cat "${OBS_DIR}/config/prometheus-base.yml" \
  "${OBS_DIR}/prometheus/scrape/voice-apps.yaml" \
  "${OBS_DIR}/prometheus/scrape/infra-exporters.yaml" \
  > "${TMP}/prometheus.yml"

kubectl apply -f "${PROFILE_DIR}/namespace.yaml"

kubectl -n "${NS}" create configmap prometheus-config \
  --from-file=prometheus.yml="${TMP}/prometheus.yml" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap prometheus-rules \
  --from-file="${OBS_DIR}/prometheus/rules/" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Loki / Promtail ---
kubectl -n "${NS}" create configmap loki-config \
  --from-file=loki.yaml="${OBS_DIR}/config/loki.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap promtail-config \
  --from-file=promtail.yaml="${OBS_DIR}/config/promtail.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Alertmanager ---
AM_CONFIG="${OBS_DIR}/alertmanager/config.yaml"
if [ "${NOTIFICATIONS_ENABLED:-false}" = "true" ]; then
  if ! kubectl -n "${NS}" get secret alertmanager-notifications >/dev/null 2>&1; then
    echo "ERROR: NOTIFICATIONS_ENABLED=true but secret alertmanager-notifications missing."
    echo "Copy deploy/observability/alertmanager/secret.example.yaml and apply it first."
    exit 1
  fi
  AM_CONFIG="${OBS_DIR}/alertmanager/config-notifications.yaml"
  echo "Alertmanager: using notification receivers (Telegram + email)"
else
  echo "Alertmanager: null receiver (set NOTIFICATIONS_ENABLED=true after applying notification Secret)"
fi

kubectl -n "${NS}" create configmap alertmanager-config \
  --from-file=alertmanager.yml="${AM_CONFIG}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap alertmanager-templates \
  --from-file="${OBS_DIR}/alertmanager/templates/" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Grafana provisioning ---
kubectl -n "${NS}" create configmap grafana-datasources \
  --from-file=datasources.yaml="${OBS_DIR}/grafana/provisioning/datasources.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "${NS}" create configmap grafana-dashboards-provisioning \
  --from-file=dashboards.yaml="${OBS_DIR}/grafana/provisioning/dashboards.yaml" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Grafana admin secret (override with GRAFANA_ADMIN_PASSWORD) ---
GRAFANA_PASS="${GRAFANA_ADMIN_PASSWORD:-changeme-voice-observability}"
kubectl -n "${NS}" create secret generic grafana-admin \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="${GRAFANA_PASS}" \
  --dry-run=client -o yaml | kubectl apply -f -

# --- Infra exporters in voice-staging (Postgres, Redis, NATS) ---
if ! kubectl get namespace voice-staging >/dev/null 2>&1; then
  echo "WARN: namespace voice-staging missing — skip exporters (apply staging stack first)"
else
  for manifest in "${OBS_DIR}/exporters/"*.yaml; do
    kubectl apply -f "${manifest}"
  done
fi

# --- Workloads (order: storage backends first, then collectors, then UI) ---
for manifest in prometheus.yaml loki.yaml alertmanager.yaml promtail.yaml grafana.yaml; do
  kubectl apply -f "${PROFILE_DIR}/${manifest}"
done

echo ""
echo "Waiting for core pods..."
kubectl rollout status "deployment/prometheus" -n "${NS}" --timeout=180s
kubectl rollout status "deployment/loki" -n "${NS}" --timeout=180s
kubectl rollout status "deployment/alertmanager" -n "${NS}" --timeout=120s
kubectl rollout status "deployment/grafana" -n "${NS}" --timeout=120s

echo ""
echo "Observability apply complete."
echo "  kubectl get pods -n ${NS}"
echo "  kubectl top pods -n ${NS}"
echo "  kubectl port-forward -n ${NS} svc/grafana 3000:80"
echo ""
echo "App scrape: ensure voice-staging Deployments have prometheus.io/scrape annotations"
echo "  (deploy/staging/services.yaml, gateway-deployment.yaml) and pods are restarted."
echo "  Verify targets: kubectl port-forward -n ${NS} svc/prometheus 9090:9090 → /targets"
