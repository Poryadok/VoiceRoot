# Matchmaking Service

matchmaking (docs/features/matchmaking.md): game catalog, player profile, solo search queue, match squad provisioning, ratings, and history.

## Endpoints

| Surface | Path / RPC |
|---------|------------|
| Health | `GET /health` (includes `redis` status when configured) |
| gRPC | `MatchmakingService` |
| Gateway catalog | `GET/POST/PATCH /api/v1/matchmaking/games*` |
| Gateway profile | `GET/PUT/DELETE /api/v1/matchmaking/profile/**` |
| Gateway queue | `POST /api/v1/matchmaking/search`, `GET/DELETE /api/v1/matchmaking/search/{session_id}` |
| Gateway match | `POST /api/v1/matchmaking/matches/{id}/respond`, `complete`, `rate` |
| Gateway history | `GET /api/v1/matchmaking/history` |

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL `matchmaking_db` (required for gRPC) |
| `MATCHMAKING_GRPC_LISTEN` | gRPC bind address (default `:9090`) |
| `MATCHMAKING_REDIS_ADDR` | Redis for FIFO queues and active-search locks (required for `StartSearch`) |
| `CHAT_GRPC_ADDR` | Optional Chat service for match squad text chat |
| `VOICE_GRPC_ADDR` | Optional Voice service for match squad group voice |
| `NATS_URL` | JetStream `mm.*` events (match_found, search_timeout, …) |
| `LISTEN_ADDR` | HTTP health bind (default `:8080`) |

## Status

- Solo queue (`party_size=1`), matcher worker, timeout sweeper, ratings, and history are implemented.
- Match squad provisions ephemeral group chat + group voice when Chat/Voice gRPC are configured.
- Game catalog: seeded games + browse UI; user `SubmitGameRequest` → `pending_moderation`; staff List/Approve/Reject via Admin.

## Database

Migrations: `src/backend/migrations/matchmaking_db/`.

## Protected search principal (source disabled)

`internal/searchprincipal` implements the request-bound subject verifier required
by the [A3 party contract](../../../docs/todo/backend.md). It accepts one signed
Gateway delegated-user credential, validates its exact StartSearch protobuf hash,
RPC, audience, request ID, lifetime and current positive session epoch, and
requires a replay guard. Raw identity metadata and duplicate credentials deny
before handler execution. Only this verifier can create the private search
subject context; changing the request invalidates the subject accessor.

The package is **not wired into `main.go` or Gateway**. It does not secure the
legacy metadata-based user surface by itself. Runtime cutover still requires
Gateway issuance, TLS and rotation configuration, durable replay/epoch checks,
and the authoritative Voice snapshot and membership-event source described in
the A3 contract. The existing Redis-only voice roster is not a snapshot fallback.
