# Voice observability stack (soft launch)

Prometheus, Grafana, Loki, Promtail, and Alertmanager for staging k3s. Spec: [docs/features/observability.md](../../docs/features/observability.md).

**Namespace:** `voice-observability` (separate from `voice-staging` app pods).

**Default profile:** `k3s-lite` — plain `kubectl apply`, no Helm, tuned for single-node k3s ([STAGING_SERVER.md](../../docs/STAGING_SERVER.md)).

## Profiles

| Profile | Path | When to use |
|---------|------|-------------|
| **k3s-lite** (default) | `profiles/k3s-lite/` | Staging k3s single-node; ~1–1.5 Gi RAM for obs pods |
| **full** (optional) | `profiles/full/values-kube-prometheus.yaml` | Cluster with ≥8 GB free RAM; Helm `kube-prometheus-stack` |

Do **not** apply both profiles to the same namespace.

## Resource estimate (k3s-lite)

| Component | Requests | Limits | PVC |
|-----------|----------|--------|-----|
| Prometheus | 256 Mi | 512 Mi | 20 Gi |
| Grafana | 128 Mi | 256 Mi | 5 Gi |
| Loki | 256 Mi | 512 Mi | 30 Gi |
| Promtail (×1 on single node) | 64 Mi | 128 Mi | — |
| Alertmanager | 64 Mi | 128 Mi | — |
| Exporters (voice-staging) | ~96 Mi | ~192 Mi | — |
| **Total (requests)** | **~864 Mi** | **~1.7 Gi** | **55 Gi** |

After apply, check actual usage:

```bash
kubectl top pods -n voice-observability
```

If the node is RAM-constrained, defer Loki and use `make compose-logs-collect` locally until hardware is upgraded (see [docs/TODO.md](../../docs/TODO.md)).

Storage class: `local-path` (k3s default).

## Apply order

```bash
# From repo root (bash on staging host or CI with kubeconfig):
scripts/staging/apply-observability.sh

# Override admin password before first Grafana login:
GRAFANA_ADMIN_PASSWORD='your-secret' scripts/staging/apply-observability.sh
```

The script:

1. Creates namespace `voice-observability`
2. Builds ConfigMaps from `config/`, `prometheus/`, `alertmanager/`, `grafana/provisioning/`, and `grafana/dashboards/` (JSON)
3. Applies Deployments/DaemonSet from `profiles/k3s-lite/`

**Optional notifications** (Telegram + email):

```bash
cp deploy/observability/alertmanager/secret.example.yaml /tmp/alertmanager-notifications.yaml
# edit values, then:
kubectl apply -f /tmp/alertmanager-notifications.yaml
NOTIFICATIONS_ENABLED=true scripts/staging/apply-observability.sh
```

Without the Secret, Alertmanager uses a **null** receiver — alerts evaluate in Prometheus but nothing is delivered.

## Access Grafana

Grafana is **ClusterIP only** — not on public ingress.

```bash
kubectl port-forward -n voice-observability svc/grafana 3000:80
# http://localhost:3000 — user admin (password from grafana-admin Secret)
```

## Directory layout

```
deploy/observability/
  README.md
  config/                  # Loki, Promtail, Prometheus base (built by apply script)
  profiles/
    k3s-lite/              # DEFAULT — namespace, Deployments, DaemonSet
    full/                  # Helm values + ServiceMonitors + PrometheusRule CRs
  alertmanager/
    config.yaml            # null receiver (default)
    config-notifications.yaml
    secret.example.yaml
    templates/
  exporters/               # Postgres, Redis, NATS (voice-staging namespace)
  prometheus/
    scrape/voice-apps.yaml
    scrape/infra-exporters.yaml
    rules/                 # recording + P1/P2 alerts
  grafana/
    provisioning/
    dashboards/            # JSON dashboards (file-storage (docs/features/file-storage.md))
```

## Scrape wiring

App pods in `voice-staging` are discovered via `prometheus.io/scrape` pod annotations (k3s-lite default). Exporters for Postgres/Redis/NATS/LiveKit are static targets in `prometheus/scrape/infra-exporters.yaml`; manifests in `exporters/` (see [exporters/README.md](exporters/README.md) for expected metric names).

### App scrape matrix (staging)

| Deployment | HTTP port | Metrics path |
|------------|-----------|--------------|
| voice-gateway | 8080 | `/metrics` |
| voice-auth | 8080 | `/actuator/prometheus` |
| voice-messaging, chat, user, social, space, role, voice, file, matchmaking, search, notification, realtime, bot | 8080 | `/metrics` |

Annotations live on pod templates in `deploy/staging/services.yaml` and `deploy/staging/gateway-deployment.yaml`. Re-apply staging after changes so pods pick up new metadata.

### Profiles

| Profile | App scrape mechanism | Rules |
|---------|---------------------|-------|
| **k3s-lite** (default) | Pod annotations + `kubernetes-pods-voice` job | ConfigMap from `prometheus/rules/` |
| **full** | `profiles/full/service-monitors.yaml` | `profiles/full/prometheus-rules.yaml` (PrometheusRule CR) |

Recording rules use post-migration Gateway metrics only: `gateway_http_requests_total`, `gateway_http_request_duration_seconds` (not legacy `gateway_request_count`).

Promtail collects stdout from namespaces `voice-staging` and `voice-observability`. LogQL example:

```logql
{namespace="voice-staging"} | json | request_id="<id>"
```

## Validate manifests (client dry-run)

```bash
for f in deploy/observability/profiles/k3s-lite/*.yaml; do
  kubectl apply --dry-run=client -f "$f"
done
```

## Validate Prometheus rules

Requires [promtool](https://prometheus.io/docs/prometheus/latest/command-line/promtool/) (Prometheus release tarball):

```bash
promtool check rules deploy/observability/prometheus/rules/*.yaml
```

PostgresDown inhibition: derivative alerts (`Tier0High5xx`, `GatewayLatencyHigh`, `PodCrashLooping`, `JetStreamLag`, `RedisDown`, `NATSDown`, `GatewayDown`) are suppressed when `PostgresDown` fires — see `alertmanager/config.yaml`.

## Smoke after deploy

Минимальный чеклист soft launch (см. также [observability.md § Smoke](../../docs/features/observability.md) и runbook [TESTING.md § Debug by request_id](../../docs/TESTING.md)):

### 1. Observability pods healthy

```bash
kubectl get pods -n voice-observability
```

Все поды `Running` / `Completed` (Promtail DaemonSet). При `CrashLoopBackOff` — `kubectl describe pod` и `kubectl top pods -n voice-observability` (RAM на k3s-lite).

### 2. Grafana — targets UP

```bash
kubectl port-forward -n voice-observability svc/grafana 3000:80
```

- **Connections → Data sources**: Prometheus и Loki — зелёные.
- **Dashboards → Voice**: Overview, Tier-0 Paths, Infrastructure, Logs — Request ID, Voice & LiveKit.
- **Explore → Prometheus**: запрос `up{namespace="voice-staging"}` — цели приложений и exporters в состоянии `1`.

### 3. E2E logs — DM и `request_id`

1. На staging: login → WS к Realtime → отправить DM (два аккаунта).
2. Скопировать **`X-Request-Id`** из ответа Gateway.
3. **Explore → Loki**:

```logql
{namespace="voice-staging"} | json | request_id="<id>"
```

Ожидаемая цепочка: `http_access` → `grpc_call` → `nats_publish` → `nats_consume` → `ws_fanout`. Или дашборд http://localhost:3000/d/logs-request-id/voice-logs-request-id .

### 4. Prometheus — Gateway traffic

```bash
kubectl port-forward -n voice-observability svc/prometheus 9090:9090
```

В UI или Explore: `increase(gateway_http_requests_total[5m])` — ненулевой рост после REST/WS на staging. `/targets` — job `kubernetes-pods-voice` и infra exporters **UP**.

### 5. Synthetic alert test (optional)

Только если настроен Secret уведомлений (`NOTIFICATIONS_ENABLED=true`, см. выше):

```bash
kubectl port-forward -n voice-observability svc/alertmanager 9093:9093
# amtool из tarball Prometheus:
amtool alert add test_voice_observability severity=critical --alertmanager.url=http://127.0.0.1:9093
```

Проверить доставку в Telegram/email; без Secret Alertmanager использует **null** receiver — правила в Prometheus считаются, уведомления не уходят (ожидаемо для первого apply).

## Local compose parity

Profile `observability` in [docker-compose.yml](../../docker-compose.yml) runs Prometheus, Grafana, Loki, and Promtail from [local/](local/).

```bash
make compose-observability-up
# Grafana http://localhost:3000 — admin / changeme-voice-local (override GRAFANA_ADMIN_PASSWORD)
# Prometheus http://localhost:9090
```

`make compose-logs-collect` is unchanged: it writes an offline `.local/dev.ndjson` snapshot and does not conflict with Loki. Both can be used together — see [local/README.md](local/README.md).

Validate: `docker compose --profile observability config`

## Security

- Do not expose `/metrics`, Prometheus, Loki, or Grafana on public ingress without auth.
- Change default Grafana password on first deploy.
- Notification secrets are not committed — use `secret.example.yaml` as template.
