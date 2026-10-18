# Analytics Service

Go service for product analytics: JetStream ingest → in-memory batches →
ClickHouse, with ingest/query gRPC services and staff REST queries through Gateway.
Product analytics remains **partial**; HTTP health alone is not ingestion proof.

## Sources of truth

- [Service contract](../../../docs/microservices/analytics-service.md) and
  [feature scope](../../../docs/features/analytics.md).
- [Storage and configuration](../../../docs/DATA_STORES.md#clickhouse-analytics-service),
  [implementation status](../../../docs/PLAN.md), and
  [verification rules](../../../docs/TESTING.md).
- [ClickHouse DDL](../../../docker/clickhouse/init/001_events.sql) and
  [protobuf API](../../../protos/voice/analytics/v1/analytics.proto).

## Startup and configuration

| Variable | Current behavior |
| --- | --- |
| `LISTEN_ADDR` | HTTP health/metrics address; default `:8080`. |
| `ANALYTICS_GRPC_LISTEN` | Ingest/query gRPC address; default `:9090`. |
| `CLICKHOUSE_DSN` | Parsed and pinged before serving; a configured but unavailable store fails startup. |
| `ANALYTICS_REQUIRE_PERSISTENCE` | Set `true` in staging/prod. Requires DSN and an explicit non-development HMAC key. |
| `ANALYTICS_ID_HASH_KEY` | HMAC key for account/profile identifiers; the development fallback is local-only. |
| `NATS_URL` | Enables JetStream consumers when ClickHouse is configured. |
| `ANALYTICS_BATCH_MAX_EVENTS` | Flush threshold; default `1000` events. |
| `ANALYTICS_BATCH_FLUSH_INTERVAL` | Periodic flush; default `5s`. |

Without a DSN, optional local mode exposes only `GET /health` and `/metrics`;
it does not start gRPC ingest/query or NATS consumption. This mode is not a
persistence check. The strict persistence flag rejects missing DSN/key or the
development key before opening listeners.

With persistence configured, apply the canonical ClickHouse DDL and provision
the documented JetStream consumers through the deployment bootstrap. Analytics
binds and validates existing durable consumers; it does not create or modify them.

## Delivery and query boundaries

JetStream messages are ACKed after successful ClickHouse insertion; failed
insertion NAKs the messages for redelivery. Batches live in process memory,
not Redis. Direct gRPC ingest has no broker acknowledgement and is not
crash-durable. Official queries use the logical event view to deduplicate stable
event IDs while retaining raw events.

The current query implementation supports `product`, `engagement`, `revenue`,
`health` and `moderation` dashboards, the `registration` funnel, retention, and
CSV/JSON export. This API inventory does not establish complete metric coverage
or the feature's live acceptance criteria; consult the service contract and PLAN.

## Development checks

Use Go 1.26. From this directory, `go test -short ./...` runs the short suite;
`go test ./...` includes the integration tier and requires the dependencies
specified in [TESTING.md](../../../docs/TESTING.md). Short-suite success is not
ClickHouse/JetStream end-to-end proof. The Docker build context is `src/backend`,
using `analytics/Dockerfile`; Compose starts the service under profile `app`.
