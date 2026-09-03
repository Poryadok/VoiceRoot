# Moderation Service

Go service for reports, sanctions, appeals, shadow-ban checks, auto-moderation, and moderation audit export. It exposes gRPC `ModerationService`, persists to PostgreSQL `moderation_db`, publishes `moderation.events`, and is wired through Gateway user/admin routes.

The product loop remains partial: report pagination and abuse controls are incomplete, auto-moderation is narrower than the specification, and federation moderation is deferred with federation itself. See [Moderation Service](../../../docs/microservices/moderation-service.md), [PLAN](../../../docs/PLAN.md), and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc) and [`internal/store`](internal/store).
