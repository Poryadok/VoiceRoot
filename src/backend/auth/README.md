# Auth Service

Java/Spring Boot service: register, login, refresh (rotation), logout, JWT validation, JWKS.

## Ports

| Surface | Port | Config |
|--------|------|--------|
| REST / Actuator | 8080 (default) | `server.port` |
| gRPC | 9090 (default) | `auth.grpc.port` (`-1` disables embedded server) |

Docker image: `EXPOSE 8080 9090`.

## Persistence (`auth.persistence`)

| Value | Repositories | JWT signing | Blacklist |
|-------|----------------|-------------|-----------|
| `jdbc` (default) | PostgreSQL via `JdbcAccountRepository`, `JdbcRefreshTokenRepository` | PKCS#8 RSA PEM: `auth.jwt.private-key-pem` or `auth.jwt.private-key-location` (e.g. `file:/run/secrets/jwt.pem`) | Redis (`spring.data.redis.*`) |
| `memory` | In-memory (dev/tests) | Ephemeral RSA (`JwtService.forTests`) | In-memory |

Spring profile `test` sets `auth.persistence=memory` (see `src/test/resources/application-test.properties`).

## Database

Schema for `auth_db` is defined in two places; apply it with **one** tool per database ([migrations README](../migrations/README.md) — section `auth_db` (Auth): Flyway vs golang-migrate).

| Path | Mechanism | Order |
|------|------------|-------|
| **A — Flyway (default)** | `src/main/resources/db/migration/V1__auth_schema.sql` on Auth startup | single migration `V1` |
| **B — golang-migrate** | [src/backend/migrations/auth_db/](../migrations/auth_db/) | `000001_init` **then** `000002_refresh_tokens_access_jti` |

**Equivalence:** Flyway `V1` ≡ golang-migrate `000001` + `000002` in sequence. Do not mix both tools on the same empty DB without baselining Flyway; default is Path A.

## Env / properties (jdbc)

- `SPRING_DATASOURCE_URL`, `SPRING_DATASOURCE_USERNAME`, `SPRING_DATASOURCE_PASSWORD`
- `SPRING_DATA_REDIS_HOST`, `SPRING_DATA_REDIS_PORT`
- `AUTH_JWT_PRIVATE_KEY_PEM` or `AUTH_JWT_PRIVATE_KEY_LOCATION`
- `AUTH_FLYWAY_ENABLED` (default `true`) — set `false` for Path B (schema applied only via golang-migrate).

## User DB (`user_db`) — первичный профиль (Фаза 1)

При `auth.persistence=jdbc` нужен второй JDBC URL к **`user_db`**, чтобы перед выдачей access JWT создавалась строка в `profiles` (см. [EXEC_PLAN.md](../../../docs/EXEC_PLAN.md), [primary-profile-bootstrap.md](../../../docs/microservices/primary-profile-bootstrap.md)):

- `auth.user-db.jdbc-url` (или env `AUTH_USER_DB_JDBC_URL`)
- `auth.user-db.username` / `auth.user-db.password` (или `AUTH_USER_DB_USERNAME` / `AUTH_USER_DB_PASSWORD`; по умолчанию совпадают с `spring.datasource.*`)

Схема `profiles` — [migrations/user_db](../migrations/user_db/). Локальный Compose: `docker/postgres/02-user-schema.sh` + `user_db_init.sql.snippet`.

## Tests

- `mvn -B test` — unit + `@ActiveProfiles("test")` REST/gRPC smoke (in-memory).
- `AuthJdbcRedisIntegrationTest` — Postgres (`auth_db`) + Redis + отдельный Postgres (`user_db` со схемой профилей) через Testcontainers (`@ActiveProfiles("integration")`). Runs when Docker is available to the JVM; skipped in environments without Docker (e.g. plain `docker run … mvn` without mounting `/var/run/docker.sock`).

Canonical product spec: [docs/microservices/auth-service.md](../../../docs/microservices/auth-service.md).
