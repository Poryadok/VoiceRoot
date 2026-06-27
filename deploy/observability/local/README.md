# Local compose observability profile

Prometheus, Grafana, Loki, and Promtail for `docker compose --profile observability`.

## Start

```bash
# App stack + observability (recommended)
make compose-observability-up

# Or manually:
docker compose --profile app --profile observability up -d
```

Grafana: http://localhost:3000 (default `admin` / `changeme-voice-local`, override with `GRAFANA_ADMIN_PASSWORD`).

Prometheus: http://localhost:9090

## Coexistence with `make compose-logs-collect`

| Tool | Output | Use case |
|------|--------|----------|
| `make compose-logs-collect` | `.local/dev.ndjson` (offline snapshot) | Debug by `request_id` without running Loki |
| Observability profile | Live Loki + Grafana dashboards | Metrics and LogQL while stack is up |

Both can run at the same time; log collection does not mount host paths or change container logging.

## LogQL (local)

Promtail sets `namespace="voice-local"` (staging dashboards use `voice-staging`):

```logql
{namespace="voice-local"} | json | request_id="<id>"
```

Filter by compose service: `{namespace="voice-local", service="gateway"}`.

## Validate config

```bash
docker compose --profile observability config
```
