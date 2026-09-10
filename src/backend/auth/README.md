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
- `AUTH_USER_GRPC_DEADLINE` — positive ISO-8601 `Duration` для каждого blocking Auth→User RPC;
  если переменная отсутствует, действует `PT15S`. Явные blank/malformed/zero/negative значения
  останавливают startup.
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

## Ownership-proof principal listener

The legacy gRPC listener on `auth.grpc.port` (9090) always rejects
`IssueOwnershipTransferProof` and `ConsumeOwnershipTransferProof`. Existing Auth
RPCs keep their current listener. The dedicated principal listener exposes only
these two proof methods, and only when `S2S_JWKS_URLS_JSON` is configured:

| Setting | Contract |
|---|---|
| `AUTH_PRINCIPAL_GRPC_PORT` | Dedicated listener, default 9091; must differ from the legacy port. Port 0 is allowed only in explicit local/test profiles. |
| `AUTH_GRPC_TLS_CERT_FILE`, `AUTH_GRPC_TLS_KEY_FILE` | Paired PEM certificate chain and private-key files for the dedicated listener. Mount from an Auth-owned secret, for example `/run/secrets/auth-grpc/tls.crt` and `tls.key`. |
| `S2S_JWKS_URLS_JSON` | Nonempty issuer-to-HTTPS-URL object containing `gateway` and `space`. |
| `S2S_JWKS_REFRESH_AFTER`, `S2S_JWKS_HARD_EXPIRY`, `S2S_UNKNOWN_KID_COOLDOWN` | Defaults `30s`, `2m`, `5s`; positive duration values, hard expiry at least refresh interval. Explicit blank/invalid values fail startup. |

Missing JWKS configuration leaves the private listener disabled and proof methods
denied on the legacy listener. Partial/invalid configuration is a startup error.
Principal TLS is mandatory unless every active Spring profile is explicitly
`local` or `test`; mixed production/test profiles cannot enable plaintext.
There is no plaintext retry following a TLS error. Private startup also fails if
either proof RPC is absent, so verifier installation alone cannot activate an
incomplete proof service.

Gateway calls issue with its derived `delegated_user` principal; Space calls
consume with `service:space`. Both use one Bearer credential and one
`x-request-id`, an exact full RPC name and deterministic protobuf SHA-256
request binding. Raw identity metadata is rejected. Validation checks RS256,
issuer, audience `auth`, current key ID, all time claims (30-second maximum
lifetime, five-second iat/nbf skew, no expiry grace), caller permissions and
replay before the handler.

Auth reads the existing Redis minimum session-epoch floor for delegated users
through `SessionEpochFloorStore`; missing, corrupt, stale or unavailable floors
deny admission. Shared replay uses the existing `spring.data.redis.*` connection
and one atomic SET-NX-with-expiry operation on
`auth:principal:replay:<issuer>:<SHA-256-of-jti>`. Both Redis operations are bounded
by two seconds. In-memory replay is available only with `auth.persistence=memory`
and explicit local/test profiles.

JWKS fetches are lazy (no Gateway/Auth startup dependency cycle), HTTPS only,
without redirects, bounded by two seconds and 64 KiB. Java's default certificate
and hostname verification applies. A private CA must be installed in the JVM
trust store (standard `javax.net.ssl.trustStore` configuration). Gateway and Space
each publish a complete current+next RS256 signing set before switching active
keys; retain old keys for at least 30 seconds after their last issuance. Incomplete
refresh never replaces or extends the last-good set, and hard expiry fails closed.
Auth consumes these service keys; its existing client-JWT signing configuration
does not sign principal credentials. Legacy `S2S_SIGNING_KEY_PEM` and
`S2S_SIGNING_KID` aliases are rejected.

Deployment must route only these proof calls to port 9091, install the Auth
certificate chain and CA trust for Gateway/Space clients, and verify the
certificate's Auth service DNS name. Existing callers remain on 9090. Do not
publish port 9091 externally. Test fixture keys under `src/test/resources` are
public test data and must never be deployed. Verification denials use the
existing gRPC request logging with coarse status descriptions; credentials and
proof bodies are not logged.
