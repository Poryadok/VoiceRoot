# Federation Service

Go scaffold for the Voice federation service (product deferred per [PLAN.md](../../../docs/PLAN.md)).

Current public surface:

- GET `/health` returns `{"service":"federation","status":"ok"}`
- GET `/metrics` — Prometheus scrape endpoint (k8s annotations expect this path)

Runtime wiring:

- **Compose** (`docker compose --profile app`): `federation` service builds and runs health/metrics only; intentionally **not** in `GATEWAY_GRPC_UPSTREAMS_JSON` (S2S-only per [federation-service.md](../../../docs/microservices/federation-service.md)).
- **k8s** (`deploy/staging|prod/services.yaml`): `voice-federation` exposes HTTP `:8080` and reserved gRPC `:9090` (`FEDERATION_GRPC_LISTEN`); gRPC server not implemented yet.

Domain behavior, gRPC handlers, database repositories, and migrations are intentionally out of scope for this initialization step.
