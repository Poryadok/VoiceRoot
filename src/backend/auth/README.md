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
| **A — Flyway (default)** | `src/main/resources/db/migration/V*.sql` on Auth startup | `V1`–`V5` (see `db/migration/`) |
| **B — golang-migrate** | [src/backend/migrations/auth_db/](../migrations/auth_db/) | `000001` … `000006` |

**Equivalence:** keep Flyway and golang-migrate revisions aligned per [migrations README](../migrations/README.md). Do not mix both tools on the same empty DB without baselining Flyway; default is Path A.

## Env / properties (jdbc)

- `SPRING_DATASOURCE_URL`, `SPRING_DATASOURCE_USERNAME`, `SPRING_DATASOURCE_PASSWORD`
- `SPRING_DATA_REDIS_HOST`, `SPRING_DATA_REDIS_PORT`
- `USER_GRPC_ADDR` — адрес internal gRPC User Service; Auth не подключается к `user_db` напрямую
- `AUTH_JWT_PRIVATE_KEY_PEM` or `AUTH_JWT_PRIVATE_KEY_LOCATION`
- `AUTH_FLYWAY_ENABLED` (default `true`) — set `false` for Path B (schema applied only via golang-migrate).

## User Service — профили ([auth-and-contacts.md](../../../docs/features/auth-and-contacts.md))

Auth владеет только `auth_db`. Перед выдачей access JWT он вызывает internal User gRPC по
`USER_GRPC_ADDR`: `EnsurePrimaryProfile` возвращает канонический `profile_id`, а остальные
profile-related paths используют `ResolvePrimaryProfileIDs`, `SwitchProfile`,
`SetVerification` / `ClearVerification` и `MarkAccountRegular` (см.
[EXEC_PLAN.md](../../../docs/EXEC_PLAN.md),
[primary-profile-bootstrap.md](../../../docs/microservices/primary-profile-bootstrap.md)).
При ошибке или непригодном ответе User новая сессия не выдаётся.

Схема `profiles` и доступ к `user_db` принадлежат User Service:
[migrations/user_db](../migrations/user_db/). Локальный Compose передаёт Auth только
`USER_GRPC_ADDR=user:9090`, без credentials к User-owned БД.

## Tests

- `mvn -B test` — unit + `@ActiveProfiles("test")` REST/gRPC smoke (in-memory).
- JDBC/Testcontainers tests use Postgres for Auth-owned persistence and Redis. User interaction is
  exercised through a test gRPC server; Auth tests do not receive `user_db` credentials. Runs when
  Docker is available to the JVM; skipped in environments without Docker (e.g. plain
  `docker run … mvn` without mounting `/var/run/docker.sock`).

Canonical product spec: [docs/microservices/auth-service.md](../../../docs/microservices/auth-service.md).
