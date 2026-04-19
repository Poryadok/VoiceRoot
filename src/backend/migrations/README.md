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
