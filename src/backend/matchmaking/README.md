# Matchmaking Service

Phase 7: game catalog, player profile entries, and solo search queue.

## Endpoints

| Surface | Path / RPC |
|---------|------------|
| Health | `GET /health` (includes `redis` status when configured) |
| gRPC | `MatchmakingService` |
| Gateway catalog | `GET/POST/PATCH /api/v1/matchmaking/games*` |
| Gateway profile | `GET/PUT/DELETE /api/v1/matchmaking/profile/**` |
| Gateway queue | `POST /api/v1/matchmaking/search`, `GET/DELETE /api/v1/matchmaking/search/{session_id}` |

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL `matchmaking_db` (required for gRPC) |
| `MATCHMAKING_GRPC_LISTEN` | gRPC bind address (default `:9090`) |
| `MATCHMAKING_REDIS_ADDR` | Redis for FIFO queues and active-search locks (required for `StartSearch`) |
| `MATCHMAKING_REDIS_PASSWORD` | Optional Redis password |
| `NATS_URL` | Optional JetStream publisher for `mm.search_started` / `mm.search_cancelled` |
| `LISTEN_ADDR` | HTTP health bind (default `:8080`) |

## Degradation (Tier 2)

- Catalog and profile RPCs work with Postgres only.
- `StartSearch` returns **Unavailable** when Redis is down or unreachable (queue infra required).
- NATS publish failures are logged; search still persists (fail-open on events).

## Database

Migrations: `src/backend/migrations/matchmaking_db/`. Fresh compose applies `docker/postgres/matchmaking_db_init.sql.snippet` and `matchmaking_db_search_sessions.sql.snippet`.

Queue, matching (match squad), ratings, and party-from-voice are not implemented in this phase.
