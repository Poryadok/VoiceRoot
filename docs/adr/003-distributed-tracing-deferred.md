# ADR 003: Distributed tracing deferred

## Status

Accepted (2026-08-13)

## Context

[MICROSERVICES.md](../MICROSERVICES.md) technology table mentions OpenTelemetry + Jaeger. [observability.md](../features/observability.md) defines **v1** observability as structured JSON logs with `request_id` correlation in Loki; distributed traces are explicitly deferred.

## Decision

**No OTel/Jaeger deployment in v1.** Cross-service debugging uses:

- `X-Request-Id` / `request_id` in Gateway access logs → service logs → NATS/WS fanout events
- Prometheus metrics at Gateway and services
- `make compose-logs-collect` locally ([debug-backend-logs rule](../.cursor/rules/debug-backend-logs.mdc))

Tracing (OTel Collector → Tempo/Jaeger) is a **separate phase** after log/metrics SLOs are stable on staging.

## Consequences

- No trace SDK stubs in services until phase 2; avoids half-wired instrumentation.
- `deploy/observability/` ships Prometheus/Loki/Grafana without Jaeger.
