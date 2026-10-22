# Private Chat → production Auth fixture (ABI v1)

This test-only launcher implements Auth-floor c21/c22 and BE-131 c41, accepted
ABI SHA256 `788c2b17c346faf7cc73cac6d164fdaa0d241ef816860befe428c8978ab616cc`.
It runs actual `AuthGrpcService`, production principal configuration/HTTPS JWKS,
Redis replay, JDBC account authority, retained Redis floors and mTLS transport.
It does not implement a replacement provider or change production authority.

Use an owned private Linux network, fresh disposable PostgreSQL and Redis, and
no host-published ports. The launcher refuses a nonempty accounts table or Redis
database. The orchestrator owns container/database/network removal; Java closes
only its server and clients and leaves seeded facts for the owned cleanup.

Compile Auth test classes and resolve the ordinary Maven test dependency
classpath. With `TEST_CLASSPATH` containing `target/test-classes`,
`target/classes` and all test dependencies, invoke:

```sh
java -cp "$TEST_CLASSPATH" \
  voice.backend.auth.sessionepoch.AuthChatFloorInteropFixture \
  "$FIXTURE_OUTPUT" 60
```

The lifetime is 1..600 seconds after readiness. Run the JVM separately from the
Go caller. OpenSSL is required only by the Java acceptance-test material helper;
the launcher consumes caller-owned credential files and does not generate keys.

All fixture inputs use prefix `VOICE_AUTH_CHAT_FLOOR_INTEROP_`:

| Suffix | Meaning |
|---|---|
| `RUN_ID` | Canonical nonzero UUID for this isolated run |
| `INPUT_ROOT` | Existing declared fixture mount containing output and every referenced file |
| `AUTH_SOURCE_SHA`, `LAUNCHER_SOURCE_SHA` | Full lowercase Git source pins asserted by the immutable artifact orchestrator |
| `HOST`, `TLS_SERVER_NAME` | Internal advertised Java hostname and TLS verification name |
| `CA_FILE`, `CLIENT_CERT_FILE`, `CLIENT_KEY_FILE` | Go trust CA and distinct Chat client credentials |
| `JDBC_URL`, `JDBC_USERNAME`, `JDBC_PASSWORD_FILE` | Owned PostgreSQL connection; password read from file |
| `REDIS_HOST`, optional `REDIS_PORT` | Owned fresh Redis, default6379 |
| `JWKS_TRUSTSTORE_FILE`, `JWKS_TRUSTSTORE_PASSWORD_FILE` | JSSE store/password files for actual Chat HTTPS JWKS trust |
| optional `ACCOUNT_ID` | Canonical nonzero fixture account UUID, otherwise derived from run |
| optional `DURABLE_EPOCH`, `RETAINED_EPOCH` | Positive seed facts, default7/9 |

Use existing production `AUTH_GRPC_TLS_CERT_FILE`, `AUTH_GRPC_TLS_KEY_FILE`,
`AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE` and `S2S_JWKS_URLS_JSON`. The latter
retains required gateway/space entries and adds the distinct Chat HTTPS endpoint;
other issuers must not receive Chat signing keys. The launcher forces
`auth.persistence=jdbc`, the production profile and UTC. Actual production
configuration validates URLs/cache/alias rules. It configures JSSE trust in its
own JVM before constructing the production HttpClient. Supply private keys and
passwords as files, never arguments/logs. Symlink/file escapes from INPUT_ROOT
are rejected. Output must exist inside that root and must have no prior
`ready.json` or `closed.json`. No identity or credential is borrowed from Bot.

`ready.json` is atomically published only after existing Flyway migrations,
committed run-owned active/deleted seeds, Redis floor initialization, production
config validation and an mTLS server bound to internal port0. Its fields are:
`schema_version=1`, `run_id`, `auth_source_sha`, `launcher_source_sha`, `host`,
`port`, `tls_server_name`, `ca_file`, `client_cert_file`, `client_key_file`,
`rpc`, `account_id`, `unknown_account_id`, `deleted_account_id`,
`durable_epoch`, `retained_epoch`, `expected_floor`, `started_at`, `expires_at`.
The expected floor is max(durable,retained). Source fields are caller assertions;
the proof packet must bind them to actual executed source/archive, not merely
repeat readiness. Credentials remain files inside the owned mount. Readiness
does not prove successful JWKS resolution: the actual signed Go call must.

At lifetime expiry or owned SIGTERM Java terminates gRPC, closes principal
replay/HTTP and Redis clients, removes its readiness and writes `closed.json`
with run identity and termination/removal observations. It never deletes the
owned mount or external processes. Setup failures publish no readiness. The
ordinary production Redis floor workers use virtual threads with an idle
timeout; no production visibility/cleanup API is added for this test fixture.

The Go consumer owns fresh pure Chat JWT/request/JTI generation and its HTTPS
JWKS process. It reads the fixture facts; it does not access Auth DB/Redis.
Alternate authority states require a new owned fixture run. The outer fixture
owner may stop its own dependencies for outage cases. No management endpoint
exists. Java scoped acceptance is distinct from actual Go checker, delegated
Create, current type/profile provenance and public Space/current ACL proof.

Run focused Java lifecycle proof with fresh private PG/Redis and environment
`AUTH_CHAT_FIXTURE_TEST_JDBC_URL` plus `AUTH_CHAT_FIXTURE_TEST_REDIS_HOST`:

```sh
mvn -B -Dtest=AuthChatFloorInteropFixtureTest,AuthChatFloorInteropFixtureIntegrationTest test
```

The integration class is opt-in to avoid allocating or publishing infrastructure
from an ordinary unit invocation. Required fixture evidence explicitly enables
it and records zero skips, actual HTTPS fetch, Redis replay, mTLS, readiness and
self-termination. These Java calls do not substitute for the Go consumer proof.
