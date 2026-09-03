# User Service

Go service for profiles, multi-profile lifecycle, settings, privacy, presence, onboarding, verification, profile search, and avatar upload presigning. It exposes gRPC `UserService`, uses PostgreSQL `user_db` plus Redis for presence, and is wired through Gateway `/api/v1/users/**`.

The broader feature remains partial: durable PostgreSQL `last_seen_at`, `show_last_seen` enforcement, `SetPrimaryProfile`, and parts of Premium avatar/status and verification behavior are still open. See [User Service](../../../docs/microservices/user-service.md), [PLAN](../../../docs/PLAN.md), and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc) and [`internal/store`](internal/store).
