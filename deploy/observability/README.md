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
| **Total (requests)** | **~768 Mi** | **~1.5 Gi** | **55 Gi** |

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
2. Builds ConfigMaps from `config/`, `prometheus/`, `alertmanager/`, `grafana/provisioning/`
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
    full/                  # Helm values for kube-prometheus-stack
  alertmanager/
    config.yaml            # null receiver (default)
    config-notifications.yaml
    secret.example.yaml
    templates/
  prometheus/
    scrape/voice-apps.yaml
    rules/                 # recording + P1/P2 alerts
  grafana/
    provisioning/
    dashboards/            # JSON dashboards (Phase 3)
```

## Scrape wiring

App pods in `voice-staging` are discovered via `prometheus.io/scrape` pod annotations (Chunk 9.1). Exporters for Postgres/Redis/NATS are static targets (Chunk 8.1).

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

1. `kubectl get pods -n voice-observability` — all Running
2. Port-forward Grafana → check Prometheus/Loki datasources
3. Prometheus targets: `kubectl port-forward -n voice-observability svc/prometheus 9090:9090` → `/targets`
4. Send a staging DM → find `request_id` in Loki
5. Optional: `amtool` alert test when notifications Secret is configured

## Local compose parity

Compose profile `observability` is added in Phase 4 (`deploy/observability/local/`). `make compose-logs-collect` remains independent.

## Security

- Do not expose `/metrics`, Prometheus, Loki, or Grafana on public ingress without auth.
- Change default Grafana password on first deploy.
- Notification secrets are not committed — use `secret.example.yaml` as template.
