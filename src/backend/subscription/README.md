# Subscription Service

## Source-disabled entitlement producer seam

`internal/entitlementoutbox` and migration `000004_entitlement_outbox` provide
the first convergence building block: complete versioned protobuf snapshots,
transactional aggregate revision/outbox append, bounded DB-time leases, stable
JetStream message IDs, PubAck evidence and fenced completion/retry. No live
handler, provider adapter or startup worker calls this package. The legacy
publisher and existing consumer authority are unchanged.

This is not the provider lifecycle or consumer cutover. Normalized provider
ordering, scheduled reminders, protected snapshots/reconciliation, service-owned
consumer inbox/effects and privacy purge still follow
[`subscription-lifecycle-convergence-exec-plan.md`](../../../docs/testing/subscription-lifecycle-convergence-exec-plan.md).
An enabling change must complete those prerequisites rather than publish raw
snapshots from current arrival-ordered handlers.

## Source-disabled versioned provider transitions

`internal/providerlifecycle` and migration `000005_provider_lifecycle` add the
internal S2 reducer and PostgreSQL provider replay/order journal. The engine
requires complete verified facts with a positive provider version, target and
period metadata. It atomically records the saved provider outcome, permanent
aggregate revision and S1 outbox. Exact replays return the original outcome;
stale versions and retired subscription bindings cannot undo a newer purchase.
Conflicting provider event bytes or same-version facts commit an idempotent
quarantine record without changing entitlement state. The engine logs no raw
provider data; conflict records require restricted operational access.

Personal and Space Pro share active/grace/inactive transitions, seven-day grace
without restart for repeated failure, and cycle preservation through cancellation,
grace and expiry. Recovery, resume, renewal and repurchase clear the cycle.
Tests use deterministic fake-provider facts against the same engine. They do
not prove a live provider signature or authoritative reconciliation adapter.

No handler or startup path imports this package. Atomic reminder scheduling,
protected snapshots, account/P3 privacy fence integration, provider verification
and consumer convergence must land before activation. The new replay, binding
and quarantine rows join the documented P400D/P30D privacy retention inventory;
their purge worker and production alert delivery remain follow-ups.

Go service for subscription and Space Pro records, entitlement/limit lookups, local lifecycle operations, and billing webhook handling. It exposes gRPC `SubscriptionService`, persists to PostgreSQL `subscription_db`, publishes subscription-related events, and is wired through Gateway `/api/v1/subscription/**`.

The implementation is still a product stub per [PLAN](../../../docs/PLAN.md): checkout uses a test URL, CloudPayments is unimplemented, cancel/resume do not call the provider, and billing history is incomplete. See [Subscription Service](../../../docs/microservices/subscription-service.md) and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc), including Paddle webhook and Space Pro limit paths with test fixtures.
