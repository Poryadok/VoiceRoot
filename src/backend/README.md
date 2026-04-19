# Backend

Go services, API Gateway, Realtime, and shared Go libraries will live under this tree (layout to be refined per service, e.g. `gateway/`, `messaging/`, `pkg/`).

SQL migrations for Phase 0–1 databases ([docs/DATA_SCOPE_V1.md](../../docs/DATA_SCOPE_V1.md)) are in [`migrations/`](migrations/). Ownership and tools: [docs/OPERATIONS.md](../../docs/OPERATIONS.md) (Flyway for Auth Java, golang-migrate for Go).
