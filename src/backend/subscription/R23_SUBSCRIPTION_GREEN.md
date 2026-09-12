# R23 Subscription participant and provider dedup — corrected GREEN evidence

Base commit: `9757f7c25a8b6ed736006624a5392a5849f28614`.

This corrected GREEN implements the frozen R23 Subscription RED contract and
the independent P0 review findings. Migration `000003` owns the three durable
evidence tables, their constraints and immutability triggers. Its DOWN path
takes all three `ACCESS EXCLUSIVE` locks before checking for evidence and
refuses destructive rollback with SQLSTATE `55000`.

Every governed Space entitlement path now takes the same transaction-scoped
per-Space advisory lock before its lifecycle lookup, including when no lifecycle
row exists yet. `ApplySpaceLifecycleFence`, `PurgeSpace`, activation, manual
cancellation and cancellation finalization take the exclusive lock. Entitlement
reads take the shared lock and return no entitlement for a non-`LIVE` fence.
Purchaser entitlement lookup collects candidate Spaces in stable order, locks
each, then revalidates the entitlement and fence in the same transaction. The
cancellation sweeper does the same stable lock-and-revalidate sequence before
selection, and finalization repeats the exclusive check. A real absent-row race
holds the first `FROZEN` apply transaction before commit and proves every public
read/mutation and worker selection/finalization path remains blocked, then
observes the committed `FROZEN` fence rather than committing stale access.

Permanent Paddle and CloudPayments dedup survives the documented `P90D` HMAC
rotation. Webhook handling now requires retained verification-key enumeration;
a current-only source fails closed. The current key is always the first digest
candidate, followed by distinct retained versions. In one transaction,
Subscription serializes the raw provider event identity, probes every candidate
digest, treats an earlier binding as replay or conflict, and inserts a new fence
only with the current key version. Integration tests complete real Space purge,
rotate from an old key to a new current key, then prove the old retained digest
still suppresses replay and preserves binding-conflict rejection for both
Paddle and CloudPayments.

Lifecycle replay now compares both the stored 32-byte request hash and the exact
stored deterministic request bytes for `ApplySpaceLifecycleFence` and
`PurgeSpace`. A mismatch is `ErrLifecycleBinding`. After the documented 30-day
compaction removes request bytes, exact byte equality can no longer be proved,
so an RPC replay fails closed with `ErrLifecycleBinding`; the compact `PURGED`
fence remains permanent authority and is never reopened by replay.

The terminal fence also rejects every newer lifecycle transition once the
stored state is `PURGE_DECIDED` or `PURGED`; only exact operation replay and a
lower-generation stale no-op retain their prior convergence behavior. A real
PostgreSQL purge regression attempts `PURGE_DECIDED -> FROZEN`, then after
successful purge attempts `PURGED -> FROZEN -> LIVE`. All transitions are
refused, the stored state remains `PURGED`, and governed activation and
entitlement reads remain denied.

The lifecycle RPCs have a production fail-closed verification path. The
ordinary gRPC listener always returns `Unavailable` for
`ApplySpaceLifecycleFence` and `PurgeSpace`. When the complete protected-runtime
configuration is present, a separate TLS listener accepts only those two RPCs,
extracts the canonical bearer transport fields, hashes the protobuf request,
and verifies the Space service JWT with issuer `space`, audience
`subscription`, exact RPC, request ID and request hash. It requires a complete
two-key HTTPS JWKS, enforces cache hard expiry, and records `(issuer,jti)` with
Redis `SET NX` until token expiry. Missing dependencies fail closed. Tests cover
current and next signing keys, every exact-binding failure, replay, TLS serving,
ordinary-listener denial, and rejection of raw identity metadata.

Runtime activation requires the shared `S2S_JWKS_*` cache/JWKS settings plus
`SUBSCRIPTION_PRINCIPAL_REPLAY_REDIS_ADDR`, optional Redis password,
`SUBSCRIPTION_PRINCIPAL_TLS_CERT_FILE`,
`SUBSCRIPTION_PRINCIPAL_TLS_KEY_FILE`, and
`SUBSCRIPTION_PRINCIPAL_GRPC_LISTEN` (default `:9091`). Partial configuration or
a configured listener without `DATABASE_URL` terminates startup. This packet
does not add deployment secrets or endpoints. Provider key/KMS,
CloudPayments-verification and renewal-cancellation implementations remain
injected production adapters; no secret or live provider call is in the
worktree.

## Changed paths

- `src/backend/migrations/subscription_db/000003_space_lifecycle_provider_dedup.down.sql`
- `src/backend/migrations/subscription_db/000003_space_lifecycle_provider_dedup.up.sql`
- `src/backend/subscription/R23_SUBSCRIPTION_GREEN.md`
- `src/backend/subscription/R23_SUBSCRIPTION_RED.md`
- `src/backend/subscription/R23_SUBSCRIPTION_RED.sha256`
- `src/backend/subscription/go.mod`
- `src/backend/subscription/go.sum`
- `src/backend/subscription/internal/grpcsvc/r23_correction_test.go`
- `src/backend/subscription/internal/grpcsvc/r23_space_lifecycle_red_test.go`
- `src/backend/subscription/internal/grpcsvc/space_lifecycle.go`
- `src/backend/subscription/internal/grpcsvc/space_pro_sync_test.go`
- `src/backend/subscription/internal/grpcsvc/subscription.go`
- `src/backend/subscription/internal/grpcsvc/subscription_handlers_test.go`
- `src/backend/subscription/internal/grpcsvc/subscription_test_helpers_test.go`
- `src/backend/subscription/internal/principalgrpc/interceptor.go`
- `src/backend/subscription/internal/principalruntime/config.go`
- `src/backend/subscription/internal/principalruntime/listener.go`
- `src/backend/subscription/internal/principalruntime/runtime.go`
- `src/backend/subscription/internal/principalruntime/runtime_test.go`
- `src/backend/subscription/internal/store/lifecycle.go`
- `src/backend/subscription/internal/store/r23_correction_test.go`
- `src/backend/subscription/internal/store/r23_space_lifecycle_migration_red_test.go`
- `src/backend/subscription/internal/store/space_lifecycle.go`
- `src/backend/subscription/internal/store/store.go`
- `src/backend/subscription/internal/store/store_integration_test.go`
- `src/backend/subscription/internal/store/sweeper.go`
- `src/backend/subscription/internal/sweeper/grace_reminder_test.go`
- `src/backend/subscription/internal/sweeper/sweeper.go`
- `src/backend/subscription/main.go`
- `src/backend/subscription/main_principal_test.go`
- `src/backend/subscription/principal_servers.go`

`R23_SUBSCRIPTION_GREEN.sha256` contains the digest of every path above and is
itself excluded because a file cannot contain its own stable digest. The frozen
RED artifacts remain byte-identical:

- `R23_SUBSCRIPTION_RED.md`: `d09cca654751e1416c358239227985892b4e2a5b045fa32c3ae9c4e874aecbdf`
- `internal/grpcsvc/r23_space_lifecycle_red_test.go`: `4cb1083b360d6351fe9999a190d0278a3d5ff7b413a0ac8d8679fcf2bc9f96e2`
- `internal/store/r23_space_lifecycle_migration_red_test.go`: `9908c3ba2047a9a85cbe007d8a0ccda4ef2da4ee78f9d60e0163906a834f958e`

The approved RED assertions were not edited. Existing Subscription fixtures
install migration `000003` and inject non-secret test keys. The sweeper fixture
also creates the lifecycle table now required by its queried schema.

## Verification

All Go commands ran from `src/backend/subscription`.

| Command | Exit | Result |
| --- | ---: | --- |
| `go test ./internal/grpcsvc -run '^TestR23CorrectionPurgedFenceRejectsFrozenThenLiveAndKeepsEntitlementsDenied$' -count=1 -v` before the production correction | 1 | genuine RED: after real purge, `PURGED -> FROZEN` returned gRPC `OK` instead of `FailedPrecondition`; 2.999s |
| the same focused command after the production correction | 0 | `PURGE_DECIDED -> FROZEN` and `PURGED -> FROZEN -> LIVE` are rejected; stored `PURGED`, activation denial and entitlement denial are asserted; 3.125s |
| `go test ./internal/store -run '^TestR23' -count=1` | 0 | full frozen and corrected store contract; 55.494s |
| `go test ./internal/grpcsvc -run '^TestR23' -count=1` | 0 | full frozen and corrected handler contract; 40.181s |
| focused first-FROZEN absent-row race | 0 | reads, purchaser query, legacy/provider mutations, sweeper selection and finalization; 8.502s |
| focused provider rotation-after-real-purge and current-only rejection | 0 | Paddle and CloudPayments replay/conflict plus mandatory enumeration; 8.079s |
| focused principal runtime and ordinary-listener tests | 0 | TLS/JWKS/binding/replay/listener behavior |
| `go test -race ./internal/store -run '^TestR23Correction' -count=1` | 0 | corrected store paths; 27.347s |
| `go test -race ./internal/grpcsvc -run '^TestR23CorrectionPurgedFenceRejectsFrozenThenLiveAndKeepsEntitlementsDenied$' -count=1` | 0 | real-purge terminal fence under the race detector; 3.784s |
| `go test -race ./internal/principalruntime -count=1` | 0 | protected verifier/runtime; 1.666s |
| `go test -race . -count=1` | 0 | listener construction/shutdown; 1.377s |
| `go test -short ./... -count=1` | 0 | complete module short suite |
| `go test -p 1 ./... -count=1` | 0 | clean final full module: grpcsvc 121.412s, principalruntime 0.344s, store 84.692s, subscriptionevents 0.331s, sweeper 2.938s |
| `go mod tidy -diff` | 0 | no dependency diff |
| `go vet ./...` | 0 | Go vet |
| `golangci-lint run ./...` | 0 | `0 issues` |
| `git diff --check` | 0 | whitespace validation |

The first lint attempt was deferred by another fleet process holding the global
golangci lock; the clean retry above completed with zero issues. No commit,
push, PR, generated output, PLAN/TODO edit, other-service edit, deployment edit,
credential or live provider call was made.
