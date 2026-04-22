# SQL migrations (DATA_SCOPE v1)

Per-database folders for Phase 0–1 ([docs/DATA_SCOPE_V1.md](../../../docs/DATA_SCOPE_V1.md)). Filenames follow [golang-migrate](https://github.com/golang-migrate/migrate) (`NNNNNN_name.up.sql` / `.down.sql`).

| Directory | Database | Owner (typical) |
|-----------|----------|-----------------|
| `auth_db/` | `auth_db` | Auth Service (Flyway in Java module when added) |
| `user_db/` | `user_db` | User Service |
| `social_db/` | `social_db` | Social Service |
| `chat_db/` | `chat_db` | Chat Service |
| `messaging_db/` | `messaging_db` | Messaging Service |

Apply against the matching database only; do not run one folder against another DB ([docs/OPERATIONS.md](../../../docs/OPERATIONS.md)).

Example with [migrate](https://github.com/golang-migrate/migrate) CLI (adjust password/host):

```text
migrate -path src/backend/migrations/auth_db -database "postgres://voice:voice@localhost:5432/auth_db?sslmode=disable" up
```

Repeat for `user_db`, `social_db`, `chat_db`, `messaging_db`.

## Without local CLI (Docker)

If the `migrate` binary is not installed, use the official image [`migrate/migrate`](https://hub.docker.com/r/migrate/migrate) on the same Docker network as Compose Postgres (project network is usually `voice_default` when started from the repo root; service hostname is `postgres`).

From repo root, PowerShell (paths use `/` for the volume so Docker accepts them on Windows):

```powershell
cd d:\Git\Voice
$dbs = @("auth_db", "user_db", "social_db", "chat_db", "messaging_db")
foreach ($d in $dbs) {
  docker run --rm --network voice_default `
    -v "d:/Git/Voice/src/backend/migrations/${d}:/migrations" migrate/migrate `
    -path /migrations `
    -database "postgres://voice:voice@postgres:5432/${d}?sslmode=disable" up
}
```

- Adjust `d:/Git/Voice` to your clone path. Credentials must match [`.env`](../../../.env.example) / [docker-compose.yml](../../../docker-compose.yml) (`POSTGRES_USER` / `POSTGRES_PASSWORD`).
- One-off `down 1` (same pattern): replace the trailing `up` with `down 1`.
- If your Compose project name differs, check the network with `docker network ls` and pass `--network <name>` accordingly.
