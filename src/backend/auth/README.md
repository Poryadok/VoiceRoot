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

- **Flyway** (on by default): `src/main/resources/db/migration/V1__auth_schema.sql` (includes `refresh_tokens.access_jti`).
- **golang-migrate** tree: [src/backend/migrations/auth_db/](../../migrations/auth_db/) — if you already applied `000001` only, run [000002_refresh_tokens_access_jti.up.sql](../../migrations/auth_db/000002_refresh_tokens_access_jti.up.sql) before relying on JDBC refresh metadata; avoid applying both Flyway V1 and migrate `000001` on the same empty database without a single chosen path (see [migrations README](../../migrations/README.md)).

## Env / properties (jdbc)

- `SPRING_DATASOURCE_URL`, `SPRING_DATASOURCE_USERNAME`, `SPRING_DATASOURCE_PASSWORD`
- `SPRING_DATA_REDIS_HOST`, `SPRING_DATA_REDIS_PORT`
- `AUTH_JWT_PRIVATE_KEY_PEM` or `AUTH_JWT_PRIVATE_KEY_LOCATION`
- `AUTH_FLYWAY_ENABLED` (default `true`) — set `false` if schema is owned solely by external migrate.

## Tests

- `mvn -B test` — unit + `@ActiveProfiles("test")` REST/gRPC smoke (in-memory).
- `AuthJdbcRedisIntegrationTest` — Postgres + Redis via Testcontainers (`@ActiveProfiles("integration")`). Runs when Docker is available to the JVM; skipped in environments without Docker (e.g. plain `docker run … mvn` without mounting `/var/run/docker.sock`).

Canonical product spec: [docs/microservices/auth-service.md](../../../docs/microservices/auth-service.md).
