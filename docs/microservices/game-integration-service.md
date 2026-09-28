# Game Integration Service

## Authority and storage

This separate Go service owns developer applications, environments,
installations, external resource mappings, game bindings, communication
sessions, desired managed grants, and orchestration operations. Its PostgreSQL
database is `game_integration_db`; it has no SQL access to Auth, User, Chat,
Space, Voice, Bot, or Federation databases. Foreign IDs are logical UUID
references. Auth is the only account issuer; game backend facts do not become
Voice user credentials.

GIS uses the dedicated `gameintegration_runtime` PostgreSQL login in runtime.
The privileged Compose database initializer provisions or rotates that login
from `GAME_INTEGRATION_DB_PASSWORD` before applying GIS migrations. The GIS
container receives only the dedicated runtime URL; the initializer uses the
separate privileged `POSTGRES_USER` credential for role provisioning and
migrations. The same role provisioning SQL is used by the dev/CI
`compose-migrate-all.sh` helper so migrations cannot run ahead of role
creation. The GIS role has no superuser, database/role creation, inheritance,
RLS bypass, or replication privileges. Migration `000003_t10_runtime_principal`
grants GIS table and sequence DML in `game_integration_db` only. It does not
change cluster-wide `PUBLIC` connect privileges. Compose supplies a local
development default; deployments must provide the runtime secret through their
secret manager.

The current implementation sequence is registry → identity/binding → session
orchestration → managed community projection. A registry implementation alone
does not enable a public game communication capability.

## G06/Q11: application bootstrap decision

1. A regular Voice account creates an application in `draft` state with a
   stable `application_id`, name, optional catalog `game_id`, and its own
   `owner_account_id`. The caller's authenticated `user_id` supplies ownership;
   neither request JSON nor a game service credential may select another owner.
2. A separate regular Voice operator account, listed in the deployment's
   `GAME_INTEGRATION_OPERATOR_ACCOUNT_IDS`, approves the application for
   `sandbox` via `POST /api/v1/game-integrations/applications/{id}/admissions/sandbox`.
   Its bearer token undergoes the same signature, audience, expiry,
   session-epoch and blacklist checks as a developer token. An empty allowlist
   disables approval. The transition is audited and transactional; the
   applicant cannot approve their own application even if on the allowlist.
   Production admission is a separate reviewed transition and cannot inherit
   sandbox credentials, bindings, subjects or data. The staged development
   workflow is defined in the API contract below; it creates only a pending
   production environment and does not enable production.
3. Sandbox admission creates one `sandbox` environment with its own ID, provider
   allowlist, redirect/origin allowlist, installation and credential namespace.
   Staged production admission creates an independent `production` environment
   with a separate ID and owner-configured policy, initially `pending`. No
   sandbox credential, installation, binding, subject or data is copied. The
   current credential API remains sandbox-only and cannot issue a production
   credential. Production activation, live provider/user proof and out-of-band
   secret provisioning remain OPEN.
4. Game service credentials are 256-bit HMAC-derived opaque secrets from a
   random credential ID and deployment-held 256-bit key; the database stores
   only keyed digests. The issue response returns the secret and an identical
   idempotent retry can recover it for ten minutes after creation. After that
   the API refuses redisplay and the owner must rotate. This resolves the
   original one-time-display versus lost-response retry conflict. Credentials
   are scoped to one application/environment and explicit service scopes.
   Issuing a replacement bounds every previous active credential to at most
   ten more minutes; explicit revoke ends admission immediately. A credential cannot authenticate
   as a player, register a player device key, choose an Auth provider, or
   approve its own application.
5. The operator enrollment API uses an independent operator principal and is
   never exposed by the developer service token. The bootstrap workflow must
   be executable through authenticated APIs/operator automation from a clean
   database; the developer portal is a later client of these APIs.
6. Every mutation accepts an actor-scoped idempotency key and immutable request
   hash. The first accepted result is durable. Repeating an identical request
   returns its original resource/operation; reusing its key with different
   content returns a conflict. A crash after commit cannot issue a second app
   or credential.

`POST /api/v1/game-integrations/applications/{app_id}/environments/{env_id}/credentials`
accepts an owner bearer, `Idempotency-Key` and an exact `scopes` array from
`game.events.write`, `game.sessions.write`, `game.roster.write` and
`game.commands.read`. It returns `vgi1_{credential_id}_{secret}` with
`Cache-Control: no-store`; the secret is unpadded base64url and may contain `_`,
so the credential ID is the first field after `vgi1_` and the remaining bytes
are parsed as the secret. No owner account ID is accepted in the body. The
endpoint is unavailable until `GAME_INTEGRATION_CREDENTIAL_KEY_B64` contains
one base64-encoded 32-byte deployment secret. It must be supplied via the
deployment secret manager; it is not a development default. The key must be
retained while issued credentials remain valid; replacing it invalidates
their HMAC verification and requires reissue. Service credential verification
checks the digest, required scope, app/env state, expiry and revocation from
the registry database on each call. `DELETE /api/v1/game-integrations/applications/{app_id}/environments/{env_id}/credentials/{credential_id}`
revokes an owned credential immediately and is idempotent. Production
admission and a live game-service operation consuming the verifier remain
dependent steps.

The future command contract is frozen separately in
[`game-integration-api.md`](../architecture/game-integration-api.md) and
[`game-bot-interactions.md`](../features/game-bot-interactions.md): callback
delivery is installation-scoped, and proposed GIS invoke/admission/result/status
routes use the documented v1 envelope. These routes are not implemented by this
registry slice. The current credential allowlist above remains authoritative for
deployed code; `game.commands.execute` is a future explicit scope and must not be
accepted until its API, verifier, migration, and negative tests are implemented.
The controlled T07a receiver is test infrastructure in an independent module
and database, never a production GIS registry or `game_integration_db` table.

The T51 event ingress and GIS→Bot→Messaging transport are specified in the
[canonical Game Event v1 contract](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
It uses the existing `game.events.write` credential scope. GIS will own the
event inbox, immutable dedupe identity, publication outbox, and operation
status. This docs-only freeze does not implement the event route or satisfy the
open installation-to-Bot, binding-authority, and resource-mapping prerequisites.

The approved sandbox owner configures Auth admission with
`PUT /api/v1/game-integrations/applications/{app_id}/environments/{env_id}/policy`.
The body carries `expected_revision`, exact redirect URIs, browser origins,
provider `google`, and a subset of the six `game.*` player scopes frozen with
Auth. The mutation is owner-scoped, audited and CAS-protected. A sandbox may
use HTTPS redirects, HTTP loopback callbacks and the exact
`voicegame://auth/...` callback; an arbitrary HTTP host, URI userinfo or
fragment is refused. Auth receives only active, nonempty policy with its
monotonic revision. A suspended app or empty policy returns unavailable, so
no browser or SDK code may substitute a local allowlist.

Auth resolves a policy at
`GET /internal/v1/authorizations/environments/{environment_id}`. It uses a
dedicated shared 32-byte workload key, supplied as base64 in
`GAME_INTEGRATION_AUTH_WORKLOAD_KEY_B64` to Game Integration and Auth through
their deployment secret managers. Missing keys disable this route. Auth sends
one each of `X-Voice-Workload: auth`, `X-Voice-Timestamp` (canonical Unix
seconds), `X-Voice-Nonce` (lowercase UUID) and `X-Voice-Signature` (unpadded
base64url HMAC-SHA256). The signed UTF-8 message is
`v1\nGET\n{escaped_path}\n{timestamp}\n{nonce}\n{lowercase_sha256_of_empty_body}`.
The server rejects query/body, a timestamp outside ±30 seconds, duplicate
headers, invalid signature and reused nonce; Redis `SETNX` stores each nonce
for 60 seconds. Redis failure is `503`, never an authentication fallback.
The response is `no-store` and includes app/env IDs, current policy revision,
display name, exact redirects/origins, providers and player scopes. Auth
re-resolves and compares revision at request, approval and code exchange.
For a successful `200` response Game Integration also echoes the request's
timestamp and nonce in `X-Voice-Response-Timestamp` and
`X-Voice-Response-Nonce`, and sends `X-Voice-Response-Signature` as unpadded
base64url HMAC-SHA256 under the same workload key. The signed UTF-8 message is
`v1\n200\n{escaped_path}\n{timestamp}\n{nonce}\n{lowercase_sha256_of_exact_response_body}`.
Auth checks the response status, echoed request fields and MAC over the exact
received bytes before parsing or using the policy. Missing or invalid response
proof fails closed. This authenticates the response over the Compose internal
HTTP hop; externally configured endpoints still require HTTPS.

The operator principal wire and key rotation overlap are fixed in the contract
PR before the corresponding public endpoint is enabled. Until that endpoint
exists, no production credential is issued. This gate is an implementation
dependency, not an omitted sprint outcome.

## Initial registry data model

- `applications`: UUID primary key, owner account UUID, name, optional catalog
  game UUID, lifecycle status, monotonic revision, creation/update timestamps.
- `environments`: UUID primary key, application UUID, kind, lifecycle status,
  provider/redirect/origin policy, revision and timestamps. Unique application
  plus kind for the first version; additional named environments require an
  explicit API version.
- `service_credentials`: UUID primary key, environment UUID, digest, scopes,
  creation/expiry/revocation timestamps and rotation generation. Never return
  a stored digest in public reads.
- `registry_operations`: actor namespace, route, idempotency key, normalized
  request hash, stable result ID/status and timestamps. Unique actor/route/key.
- `registry_audit`: immutable actor, app/env, operation, previous/new state,
  result and timestamp; sensitive proof and raw secret material are omitted.

All IDs use the repository UUID rules. Schema changes are service-owned
migrations under `src/backend/migrations/game_integration_db/` and deploy as an
expand/contract sequence. The database and migration job are provisioned with
the service before enabling any route.

## Registry acceptance

- Same owner and idempotency key with identical canonical request yields one
  application ID even across process restart; different body is a conflict.
- Another owner cannot reuse the first owner's key to read or modify that app.
- An applicant cannot approve an app; operator approval is audited and
  idempotent; a suspended app cannot issue new credentials.
- Sandbox credential fails on production, wrong app/environment, player
  message/credential/device paths and after revoke; no raw secret persists.
- Clean bootstrap creates owner/app/sandbox and an independently approved
  environment without direct SQL or developer portal access.

## T12: registry security, callback admission and diagnostics

`POST /api/v1/game-integrations/applications/{app_id}/environments/{env_id}/installations`
is an owner route. It takes `callback_url` and a canonical `bot_id` in the
body, an `Idempotency-Key`, and the application/environment IDs from the path
and trusted bearer. The owner ID is never accepted from the body: GIS reads it
from `applications.owner_account_id`. Before persistence, GIS obtains an
authenticated Bot authority proof for that exact Bot and owner. Registration
requires an active environment and a canonical HTTPS callback on port 443. The
URL cannot contain userinfo, a query, or a fragment; each path segment must be
literal ASCII unreserved text. Registration resolves DNS and rejects the URL if
any answer is non-public or special-use. The shared callback transport resolves
again when dialing, validates every answer, pins the socket to an approved IP,
and keeps TLS verification bound to the original hostname. Redirects are
terminal responses. T12 stores the callback with its app, environment, and
installation and the proven Bot ID; it does not dispatch commands. T15 must use
this transport for every command callback.

The GIS→Bot authority proof uses a dedicated shared 32-byte
`GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64` and the signed request/response
protocol frozen in [game-integration-api.md](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
This key is distinct from the GIS↔Auth workload key. It authenticates the
`gameintegration` workload to the narrow Bot audience, binds the exact request
path/body, rejects replayed 61-second nonces through Bot Redis, and signs the
exact successful response. GIS sets `BOT_INTERNAL_URL` and the key together;
Bot receives the same key plus `BOT_REDIS_ADDR` and optional
`BOT_REDIS_PASSWORD`. Generate a dedicated local-only value with
`openssl rand -base64 32`; do not reuse the GIS↔Auth key or commit the value.
Missing/malformed configuration, failed verification, unavailable Bot/Redis,
owner mismatch, or non-live Bot denies installation creation without an
installation row or successful idempotency result; a sanitized denial audit
may be recorded. The authenticated account is checked against the GIS
application registry; an asserted owner in the body is rejected. The clean
bootstrap acceptance uses a fake Bot authority verifier and does not prove a
live Bot deployment or provider admission.

Installation registration is limited to 120 attempts per application per UTC
minute across all of that application's environments. The 121st request returns
`429 RATE_LIMITED` with integer `Retry-After` seconds to the next minute
boundary. Repeated quota denials update one sanitized audit event per app and
minute. Reads, current application/environment/policy/credential routes,
operator actions, and future T15 delivery are outside this selected bucket.
An authenticated owner's quota is admitted durably before the Bot authority
proof call, so a denied or unavailable Bot proof consumes an attempt and cannot
be used to amplify internal proof traffic. The 121st attempt is rejected before
the Bot call. A pending idempotency claim expires after 30 seconds based on its
UTC `updated_at`; GIS atomically refreshes the lease under a row lock before
retrying the read-only proof, without consuming another quota slot. A fresh
pending duplicate returns unavailable without calling Bot. A foreign owner is
denied before quota admission. Invalid bearer requests create no registry,
quota, or audit writes.

`PUT /api/v1/game-integrations/applications/{app_id}/suspension` is restricted
to configured regular operator accounts and takes `{"suspended": boolean}`
plus an `Idempotency-Key`. A blocked lifecycle transition returns `409
APPLICATION_STATE_CONFLICT`; suspended application credential and Auth policy
consumers fail closed with `503 APP_SUSPENDED`. State change and audit commit in
one transaction. Restore returns the prior application state and never
reactivates revoked credentials or independently suspended/retired
environments. An identical retry returns its originally persisted result
snapshot and revision.

`GET /api/v1/game-integrations/applications/{app_id}/diagnostics` is owner-only
and returns application/environment/installation IDs and states, quota use and
reset time, developer-entered provider assertions, and at most 20 newest audit
events. It omits callback URLs, credentials and digests, OAuth subjects,
assertions, provider proofs, payloads, and secrets. Provenance values remain
separate: `developer_asserted`, `operator_approved`, and `provider_verified` /
`provider_admitted`. T12 records developer assertions and operator approval;
provider status remains `not_verified` without independent provider evidence.
Audit result values are `success` or `denied`; source identifies the trusted
actor/provenance. The T12 migration backfills only old operator
`approve_sandbox` events as `operator_approved` and leaves other legacy events
at `system` / `success`.

The T11 HTTP bootstrap acceptance is
`rtk make game-integration-bootstrap-acceptance`. It starts a disposable empty
registry, uses synthetic Voice access tokens verified through a fake JWKS, and
exercises the applicant/operator API boundary, sandbox policy ownership,
credential issue/retry, generation rotation, ten-minute maximum overlap,
owner-only revoke, immediate admission denial, and audit idempotency. A repeated
issue request returns the same secret and normalized expiry during its ten-minute
reveal window and returns `CREDENTIAL_REVEAL_EXPIRED` after it; rotation creates
a new generation while bounding the old credential's remaining lifetime to ten
minutes. Revoke returns success on retry and writes only one audit row. This
verifies the GIS path only; it is not a real Google OIDC run and does not close
the broader Q11 GIS-plus-node clean-start gate.

`game-integration-bootstrap-acceptance` runs
`TestGameIntegrationCleanBootstrapUsesOwnerAndSeparateOperatorAPIs`, which uses
only API-created registry rows. The ordinary full GIS suite separately runs
`TestGameIntegrationProductionEnvironmentCredentialDeniedWithSeededFixture`;
that security test uses a SQL fixture for a production environment the current
API cannot create, and is intentionally excluded from Q11 clean-start. It is not
evidence of production admission.

This contract is subordinate to the game integration feature and API canon;
later sections will add sessions, managed grants, bot and node-facing operations
with their own tests and migration revisions.
