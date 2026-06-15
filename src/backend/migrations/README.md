# SQL migrations (DATA_SCOPE v1)

Per-database folders for Phase 0–1 ([docs/DATA_SCOPE_V1.md](../../../docs/DATA_SCOPE_V1.md)). Filenames follow [golang-migrate](https://github.com/golang-migrate/migrate) (`NNNNNN_name.up.sql` / `.down.sql`).

| Directory | Database | Owner (typical) |
|-----------|----------|-----------------|
| `auth_db/` | `auth_db` | Auth Service — DDL also in Flyway ([`src/backend/auth/src/main/resources/db/migration/`](../auth/src/main/resources/db/migration/)); **one tool per DB** (see below) |
| `user_db/` | `user_db` | User Service |
| `social_db/` | `social_db` | Social Service |
| `chat_db/` | `chat_db` | Chat Service |
| `messaging_db/` | `messaging_db` | Messaging Service |
| `file_db/` | `file_db` | File Service |
| `bot_db/` | `bot_db` | Bot Service |

Apply against the matching database only; do not run one folder against another DB ([docs/OPERATIONS.md](../../../docs/OPERATIONS.md)).

## `auth_db` (Auth): Flyway vs golang-migrate

`auth_db` is owned by the Auth service. The same schema can be applied in **two** ways; use **exactly one** per database (mutual exclusion — do not run golang-migrate on `auth_db` and then start Auth with default Flyway on the same DB, or Flyway will try `V1` against existing tables and fail).

| Path | Who applies | Order | Auth startup |
|------|-------------|-------|----------------|
| **A — Flyway (default)** | Auth on boot | Single script: `V1__auth_schema.sql` | `AUTH_FLYWAY_ENABLED` omitted or `true` |
| **B — golang-migrate** | Ops / CLI / Docker `migrate` | `000001_init` **then** `000002_refresh_tokens_access_jti` (do not skip `000002` if JDBC refresh uses `access_jti`) | `AUTH_FLYWAY_ENABLED=false` |

**Equivalence (current schema):** Flyway `V1` ≡ golang-migrate `000001` + `000002` applied in sequence. Future DDL changes should keep Flyway `Vn` and `auth_db/NNNNNN_*.up.sql` in lockstep.

**Examples below** run migrate only for **Go-owned** databases (`user_db`, `social_db`, `chat_db`, `messaging_db`). For `auth_db`, use Path A (start Auth) or Path B (migrate then Auth with Flyway disabled) — see [Auth README](../auth/README.md).

Example with [migrate](https://github.com/golang-migrate/migrate) CLI for a Go-owned DB (adjust password/host):

```text
migrate -path src/backend/migrations/user_db -database "postgres://voice:voice@localhost:5432/user_db?sslmode=disable" up
```

Repeat for `social_db`, `chat_db`, `messaging_db`, `file_db`, `bot_db`.

**Phase 15 E2E (compose Path A):** `e2e_key_backups` via Auth Flyway `V4__e2e_key_backups.sql` on boot. `chat_db` / `messaging_db` DDL (`e2e_enabled`, `is_e2e`, `e2e_prekey_bundles`) via idempotent `docker/postgres/incremental_*.sql.snippet`, or `make compose-migrate-phase15` for golang-migrate on Go-owned DBs.

Path B for `auth_db` only (then set `AUTH_FLYWAY_ENABLED=false` for Auth):

```text
migrate -path src/backend/migrations/auth_db -database "postgres://voice:voice@localhost:5432/auth_db?sslmode=disable" up
```

## Without local CLI (Docker)

If the `migrate` binary is not installed, use the official image [`migrate/migrate`](https://hub.docker.com/r/migrate/migrate) on the same Docker network as Compose Postgres (project network is usually `voice_default` when started from the repo root; service hostname is `postgres`).

From repo root, PowerShell (paths use `/` for the volume so Docker accepts them on Windows). **Go-owned DBs** (safe alongside Auth using default Flyway on `auth_db`):

```powershell
cd d:\Git\Voice
$dbs = @("user_db", "social_db", "chat_db", "messaging_db", "file_db")
foreach ($d in $dbs) {
  docker run --rm --network voice_default `
    -v "d:/Git/Voice/src/backend/migrations/${d}:/migrations" migrate/migrate `
    -path /migrations `
    -database "postgres://voice:voice@postgres:5432/${d}?sslmode=disable" up
}
```

**Path B — `auth_db` only** (omit if Auth applies schema via Flyway; requires `AUTH_FLYWAY_ENABLED=false` when starting Auth):

```powershell
docker run --rm --network voice_default `
  -v "d:/Git/Voice/src/backend/migrations/auth_db:/migrations" migrate/migrate `
  -path /migrations `
  -database "postgres://voice:voice@postgres:5432/auth_db?sslmode=disable" up
```

- Adjust `d:/Git/Voice` to your clone path. Credentials must match [`.env`](../../../.env.example) / [docker-compose.yml](../../../docker-compose.yml) (`POSTGRES_USER` / `POSTGRES_PASSWORD`).
- One-off `down 1` (same pattern): replace the trailing `up` with `down 1`.
- If your Compose project name differs, check the network with `docker network ls` and pass `--network <name>` accordingly.
