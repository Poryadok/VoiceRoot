# grpcclient

Shared helpers for outbound gRPC from Go services.

## Current scope (Batch B)

| API | Purpose |
|-----|---------|
| `DialTarget(addr)` | DNS re-resolve for Compose service names after container recreate |
| `WaitForGRPC(ctx, addr, opts)` | Startup probe until target accepts connections |

## Deferred (not in this package yet)

`docs/MICROSERVICES.md` targets circuit breakers, retry with backoff, and mTLS on all S2S gRPC calls. Those belong here or in a sibling `pkg/grpcresilience` package once adopted service-by-service (Wave 2+ / cross-cutting batch S). Until then:

- Per-service dial options remain in each `main.go`.
- Do not assume breaker/retry from importing `grpcclient` alone.
