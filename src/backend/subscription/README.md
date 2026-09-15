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

Go service for subscription and Space Pro records, entitlement/limit lookups, local lifecycle operations, and billing webhook handling. It exposes gRPC `SubscriptionService`, persists to PostgreSQL `subscription_db`, publishes subscription-related events, and is wired through Gateway `/api/v1/subscription/**`.

The implementation is still a product stub per [PLAN](../../../docs/PLAN.md): checkout uses a test URL, CloudPayments is unimplemented, cancel/resume do not call the provider, and billing history is incomplete. See [Subscription Service](../../../docs/microservices/subscription-service.md) and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc), including Paddle webhook and Space Pro limit paths with test fixtures.
