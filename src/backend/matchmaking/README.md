# Matchmaking Service

Phase 7: game catalog + per-profile player game settings (region, role, rank).

## Endpoints

| Surface | Path / RPC |
|---------|------------|
| Health | `GET /health` |
| gRPC | Catalog: `ListGames`, `GetGame`, `CreateGame`, `UpdateGame`, `SearchGames` |
| gRPC | Player profile: `GetMyPlayerProfile`, `GetPlayerProfile`, `UpsertPlayerGameEntry`, `DeletePlayerGameEntry` |
| Gateway | `GET/POST/PATCH /api/v1/matchmaking/games*` |
| Gateway | `GET /api/v1/matchmaking/profile/me`, `GET .../profile/{id}`, `PUT/DELETE .../profile/games/{game_id}` |

## Environment

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL `matchmaking_db` (required for gRPC) |
| `MATCHMAKING_GRPC_LISTEN` | gRPC bind address (default `:9090`) |
| `LISTEN_ADDR` | HTTP health bind (default `:8080`) |

## Database

Migrations: `src/backend/migrations/matchmaking_db/`. Fresh compose applies `docker/postgres/matchmaking_db_init.sql.snippet` (seed: Dota 2, CS2, Valorant, PUBG).

Queue and matching RPCs are not implemented yet. Apply migration `000002_profile_game_entries` on existing DB volumes (or recreate Postgres volume).
