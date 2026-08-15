# ADR 002: gRPC mTLS scope

## Status

Accepted (2026-08-13)

## Context

[MICROSERVICES.md](../MICROSERVICES.md) lists gRPC with mTLS between services. Application code and staging/prod today use **plaintext gRPC** on port 9090 inside the cluster, with edge hardening via NetworkPolicy and `BOT_GRPC_GATEWAY_ONLY` for Bot.

## Decision

**v1 scope:**

| Layer | Mechanism |
|-------|-----------|
| Edge | HTTPS/WSS via Gateway ingress; JWT validation at Gateway |
| In-cluster gRPC | Plaintext on private network; NetworkPolicy restricts Bot gRPC to Gateway pods |
| mTLS | **Not wired in app code**; deferred to mesh (Linkerd/Istio) or per-service TLS when infra repo owns CA rotation |

Documented in [DEPLOYMENT.md](../DEPLOYMENT.md) § gRPC mTLS and NetworkPolicy.

## Consequences

- `pkg/grpcclient` provides dial/wait only; no mTLS helpers until ADR is superseded.
- Federation S2S and future admin control-plane remain separate ingress / mTLS when implemented.
