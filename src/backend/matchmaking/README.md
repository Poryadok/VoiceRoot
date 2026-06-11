# Matchmaking Service

Phase 7 catalog: games with roles/ranks in `config` JSONB.

## Endpoints

| Surface | Path / RPC |
|---------|------------|
| Health | `GET /health` |
| gRPC | `MatchmakingService` catalog RPCs (`ListGames`, `GetGame`, `CreateGame`, `UpdateGame`, `SearchGames`) |
| Gateway | `GET/POST/PATCH /api/v1/matchmaking/games*` |

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL `matchmaking_db` (required for gRPC) |
| `MATCHMAKING_GRPC_LISTEN` | gRPC bind address (default `:9090`) |
| `LISTEN_ADDR` | HTTP health bind (default `:8080`) |

## Database

Migrations: `src/backend/migrations/matchmaking_db/`. Fresh compose applies `docker/postgres/matchmaking_db_init.sql.snippet` (seed: Dota 2, CS2, Valorant, PUBG).

Queue, matching, and player profile RPCs are not implemented in this phase.
