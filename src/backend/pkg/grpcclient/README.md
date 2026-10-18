# grpcclient

Shared helpers for outbound gRPC from Go services.

## Current scope

| API | Purpose |
|-----|---------|
| `DialTarget(addr)` | DNS re-resolve for Compose service names after container recreate |
| `WaitForGRPC(ctx, addr, opts)` | Startup probe until target accepts connections |
| `NewUnaryBreaker(config)` | One connection-scoped unary admission breaker |
| `DefaultBreakerConfig()` | Immutable reviewed initial policy values |

The breaker interceptor keeps independent state for each exact full RPC method.
Create one instance per ClientConn; never share it across connections or
credential/authority policies. Gateway's ordinary clients use the default
policy: five qualifying failures within a 30-second first-failure window,
ten-second OPEN, one real caller as HALF_OPEN probe. Success resets history;
Unavailable/DeadlineExceeded count only while the caller context is live.
Other errors and caller cancellation/deadline are neutral. A neutral probe
reopens; it cannot prove recovery. Local expired contexts are rejected before
allocation or invocation. Health Check/List bypass business state.

The map defaults to 1024 entries, lazily evicts only idle CLOSED entries with no
outstanding calls after ten minutes, and rejects a new method when full. OPEN,
HALF_OPEN and in-flight entries are never evicted. Entry/generation fencing
prevents stale completions changing a newer cycle; panic cleanup is neutral
and propagates the panic. Config values must be positive and bounded by
threshold 100, failure window 5min, OPEN 1min, map 4096 and idle 1h.

No application retry, hedging, waiting queue, deadline or cached fallback is
introduced. The interceptor invokes at most once per admitted logical call.
Use `grpc.WithDisableRetry()` with the ordinary connection: it disables
configured retries, while grpc-go transparent retries remain possible for
unprocessed calls. This is not one-wire-attempt or exactly-once delivery.
State/rejection observation must use fixed reasons and method names, outside
the state mutex; never log request/response bodies or authority material.

## Remaining adoption

`docs/MICROSERVICES.md` requires circuit breakers on interservice gRPC; its
exponential-retry requirement names NATS/external APIs, not arbitrary gRPC
mutations. This slice activates ordinary Gateway unary clients only. Other Go
consumers, protected mTLS/principal Gateway clients, Java Auth and streaming
need their own reviewed adoption. Existing TLS/principal policies stay owned
by their transport constructors. Importing `grpcclient` alone installs no
interceptor and does not establish resilience or TLS compliance.
