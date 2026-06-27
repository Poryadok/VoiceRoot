# Infrastructure exporters (staging)

Prometheus exporters for Postgres, Redis, and NATS JetStream in `voice-staging`. LiveKit exposes native Prometheus metrics on port **6789** (configured in `deploy/staging/infra.yaml`).

Applied by `scripts/staging/apply-observability.sh` after the staging app stack (`voice-app-config`, `voice-app-secrets`, infra) exists.

## Apply

```bash
# Exporters are applied automatically with the observability stack:
scripts/staging/apply-observability.sh

# Or manually:
kubectl apply -f deploy/observability/exporters/
```

## Expected metrics (smoke)

| Component | Job | Key metrics | Notes |
|-----------|-----|-------------|-------|
| Postgres | `voice-postgres` | `pg_up`, `pg_stat_database_numbackends`, `pg_stat_database_xact_commit` | `pg_up == 1` when DB reachable |
| Redis | `voice-redis` | `redis_up`, `redis_connected_clients`, `redis_memory_used_bytes` | `redis_up == 1` when Redis reachable |
| NATS | `voice-nats-exporter` | `nats_varz_connections`, `nats_jetstream_consumer_num_pending`, recording `nats_up` | Exporter prefix `-prefix=nats`; JetStream lag per stream via `nats_jetstream_stream_messages_pending` recording rule |
| LiveKit | `voice-livekit` | `livekit_room_total`, `livekit_participant_total` | Native `/metrics` on `:6789` |
| Traefik (optional) | `traefik` | `traefik_entrypoint_requests_total` | Only if Traefik metrics enabled on `:9100` |

### PromQL examples

```promql
# Health
pg_up
redis_up
nats_up

# JetStream pending messages per stream (after recording rule)
nats_jetstream_stream_messages_pending

# Per-consumer pending (raw exporter)
nats_jetstream_consumer_num_pending

# LiveKit voice SLO panels
livekit_room_total
livekit_participant_total
```

### Verify targets

```bash
kubectl port-forward -n voice-observability svc/prometheus 9090:9090
# Open http://localhost:9090/targets — voice-postgres, voice-redis, voice-nats-exporter, voice-livekit should be UP
```

### Dry-run manifests

```bash
for f in deploy/observability/exporters/*.yaml; do
  kubectl apply --dry-run=client -f "$f"
done
```

## Resource footprint

Each exporter Deployment requests ~32 Mi RAM (three exporters ≈ 96 Mi on top of staging infra).
