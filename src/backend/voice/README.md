# Voice Service

Go microservice: DM/group voice calls, LiveKit room lifecycle, gRPC `VoiceService`. Gateway `/api/v1/voice/**`. Group rooms are capped at 32 participants in code. See `docs/microservices/voice-service.md`.

## Lifecycle storage

PostgreSQL `voice_db` is the durable source of truth for room lifecycle data.
Redis is a rebuildable projection for active-session state; it does not replace
the PostgreSQL backup. `VOICE_DATABASE_URL` is the only lifecycle DSN and must target `voice_db`.
`POSTGRES_CONNECT_TIMEOUT` bounds startup connection and ping work (the repository
default is used when it is absent).

A missing `VOICE_DATABASE_URL` keeps the R22.2 lifecycle path source-disabled:
the process starts without a lifecycle store, and no coordinator, lifecycle
handler, Redis bridge, publisher, or external-effects worker is registered. A
configured DSN must parse, connect, and ping successfully or startup fails.

## Health endpoints

- `/health` is process liveness and does not depend on PostgreSQL.
- `/ready` checks the `voice_db` connection and required schema with a bounded
  context, returning HTTP 503 when either readiness check fails.

Run `src/backend/migrations/voice_db/000001_room_lifecycle.up.sql` before rolling
out Voice with a configured DSN.
