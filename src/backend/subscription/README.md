# Subscription Service

Go service for subscription and Space Pro records, entitlement/limit lookups, local lifecycle operations, and billing webhook handling. It exposes gRPC `SubscriptionService`, persists to PostgreSQL `subscription_db`, publishes subscription-related events, and is wired through Gateway `/api/v1/subscription/**`.

The implementation is still a product stub per [PLAN](../../../docs/PLAN.md): checkout uses a test URL, CloudPayments is unimplemented, cancel/resume do not call the provider, and billing history is incomplete. See [Subscription Service](../../../docs/microservices/subscription-service.md) and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc), including Paddle webhook and Space Pro limit paths with test fixtures.
