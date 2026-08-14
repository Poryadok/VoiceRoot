# ADR 001: Compose and k8s Postgres 16 / Redis 7 parity

## Status

Accepted (2026-08-13)

## Context

`docker-compose.yml`, `deploy/staging/infra.yaml`, and `deploy/prod/infra.yaml` use **PostgreSQL 16** (`postgres:16-alpine`) and **Redis 7** (`redis:7-alpine`). Newer major versions (PG 18, Redis 8) exist; some docs and upstream images advertise them.

## Decision

**Stay on Postgres 16 and Redis 7** across compose and in-cluster infra until a coordinated upgrade wave:

- All golang-migrate / Flyway paths and CI smokes are validated on PG 16.
- Redis 7 covers Pub/Sub, sliding-window rate limits, JWT blacklist, WS tickets, and Realtime fanout; Redis 8 brings no required product feature.
- Single version line reduces drift between local compose, staging k3s, and prod.

## Consequences

- Documented parity in [DEPLOYMENT.md](../DEPLOYMENT.md).
- Future upgrade: one PR bumping compose + infra manifests + soak on staging before prod; not part of routine feature work.
