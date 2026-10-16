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
   idempotent retry can recover it strictly before ten minutes after creation;
   equality and later are expired. After that
   the API refuses redisplay and the owner must rotate. This resolves the
   original one-time-display versus lost-response retry conflict. Credentials
   are scoped to one application/environment and explicit service scopes. Each
   credential expires 90 days after creation; equality with `expires_at` is
   expired. Replacement caps each previous credential at the earlier of its
   original expiry and ten minutes after replacement. The retry reveal window
   does not extend the credential's absolute TTL. The 90-day expiry instant is
   expired. Explicit revoke ends admission on the next verification. Every
   application or environment status transition atomically marks all child
   credentials revoked and zeros their keyed digest. Migration to authority-v2
   also retires every existing credential; a rollback never restores those
   secrets. Restoring a prior status cannot revive credentials; the owner must
   issue a new credential.
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
accepts an owner bearer, `Idempotency-Key` and an exact, non-empty `scopes`
array selected by the owner from `game.events.write`, `game.sessions.write`,
`game.roster.write`, `game.commands.read`, and `game.sessions.manage`. Only the
requested scopes are granted; issuing one scope does not imply any other
capability. `game.sessions.manage` authorizes only the app/environment's
documented managed-session API. It does not grant arbitrary Voice chat, roster,
membership, or administrative access. It returns `vgi1_{credential_id}_{secret}` with
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

### GIS player binding authority and execution permits (T16 producer slice)

GIS owns player binding facts in `game_integration_db.player_bindings`:
binding/app/environment IDs, account/actor/profile/device references, lifecycle
state and a monotonic GIS `binding_revision`. These identity references are
logical UUIDs; Auth/User databases are never queried or written. The new
execution-time read is private
`GET /internal/v1/bindings/{binding_id}/authority`. It uses the existing Auth
workload v1 HMAC and Redis replay guard and returns only GIS-owned
`application_id`, `environment_id`, `binding_id`, `status`,
`binding_revision`, and `character_context`; the response is `no-store` and
signed over its exact bytes. It never serializes account, actor or profile IDs.
`active`, `revoking` and `revoked` are distinct states; reads never cache them.

GIS implements the reserved
`POST /api/v1/game-integrations/bindings/exchange` route (API section 4) as the
producer sequence. It requires a regular bearer and strict JSON containing
`challenge_id`, `code`, `code_verifier` and `device_proof`; GIS takes operation,
app and environment from its persisted challenge. It hashes the canonical
decoded request and commits the operation row before any Auth network call.
Auth remains the source of provider proof, consent, delegated scopes and
selected-profile eligibility.
Auth issues a one-use, at-most-30-second `voice.game-binding-handoff+jwt` only
after validating game/provider proof, current profile, device/ownership
generation, app/environment policy and consent. Claims bind GIS challenge ID
and nonce, app/environment, canonical redirect URI, PKCE S256 challenge,
device-key identity, provider and a namespaced versioned keyed subject digest,
operation ID, source account/actor/device generation, target account/profile
and profile revision, sorted scopes, and consent/policy revisions. Raw provider
tokens and subjects never enter or persist in GIS. GIS allocates the binding ID.

Auth creates a persisted GIS challenge only after T14 `approve` has revalidated
the signed-in source device/session, current app policy and the explicitly
selected regular target profile. It calls private
`POST /internal/v1/bindings/challenges` using WorkloadProof v1. The strict
request binds app/environment/provider, operation ID, source account/actor/
device and generation, selected target account/profile and profile revision,
consent/policy revisions, sorted scopes, redirect URI hash, PKCE S256
challenge, device key/thumbprint and expiry. It contains no provider subject or
token. GIS returns exactly `challenge_id`, `nonce`, and `expires_at`, signs the
exact response bytes, and stores an exact request hash with the operation.
An exact retry returns the persisted ID/nonce/expiry without extending TTL;
same operation with changed input conflicts. Challenges expire within five
minutes. This is an Auth-only service route, not a public or Gateway route.

Auth reads a persisted GIS challenge at private
`GET /internal/v1/bindings/challenges/{challenge_id}` using WorkloadProof v1;
the exact response fields are `challenge_id`, `nonce`, `application_id`,
`environment_id`, `provider`, `redirect_uri_sha256`, `pkce_challenge`,
`device_key_id`, `device_key_thumbprint`, `operation_id`, `expires_at`, and
`status`. GIS returns only a live `pending` row, signs the exact response bytes,
and rejects missing, invalid or replayed Auth workload proof. Auth compares
the read tuple to the T14 approval pinned to the one-use code before it signs
the post-consent handoff. The selected profile, consent and policy revisions
are therefore fixed before the challenge is created; the namespaced provider
subject digest is added only to the signed handoff after consent.

GIS calls Auth's private mTLS
`POST /internal/v1/auth/game-bindings/handoffs/exchange` with exact JSON
`challenge_id`, `operation_id`, `code`, `code_verifier`, and `device_proof`.
Auth responds with exactly `{"handoff_jws":"<compact JWS>"}` and `no-store`.
GIS keeps the JWS internal and passes the same original device proof bytes to
Auth's online claim; Auth pinned their SHA-256 to the issued JTI at atomic T14
code consume. The GIS client requires HTTPS, private CA trust and a client
certificate/key, uses a two-second timeout, forbids redirects, bounds the
response and rejects unknown/trailing JSON. `GIS_AUTH_GAME_BINDING_BASE_URL`,
`GIS_AUTH_GAME_BINDING_CA_FILE`, `GIS_AUTH_GAME_BINDING_CLIENT_CERT_FILE` and
`GIS_AUTH_GAME_BINDING_CLIENT_KEY_FILE` must be set together; absent transport
credentials disable this path.

The signed handoff is delivery-only and cannot authorize offline creation. GIS
online-claims the assertion JTI with Auth, forwarding the exact original device
proof bytes. Auth revalidates the grant, consent/policy revisions, profile,
device and revoke state, checks the proof digest pinned at code consume, then
persists an in-flight operation. Auth revocation blocks new claims and waits
for accepted claims to complete; uncertain completion remains
pending/unavailable. GIS verifies the Auth receipt and checks the handoff's
challenge, nonce, app/environment, redirect/PKCE and device-key tuple against
the persisted GIS challenge. It then locks the challenge and atomically writes
a `pending` binding, consumes the challenge and adds the durable completion
outbox row. A deterministic app/environment or already-linked-subject denial
consumes the challenge and queues a failed Auth completion; transient database
errors leave the accepted claim recoverable. Unique challenge/JTI/operation
constraints prevent duplicate creation. Exact retries require the same
canonical body and device proof and
reuse the saved handoff/claim; changed input conflicts. A background worker
retries Auth completion until receipt; only after that receipt does GIS promote
the binding to `active`. Uncertain completion returns retryable unavailable
and cannot activate the binding. Permit revoke/drain is bounded to 4.25s from
the GIS binding's durable `active → revoking` commit. The test records the
endpoint request-to-transition duration and Auth grant's separate `revoking`
timestamp. For each permit issued before the GIS transition, the terminal
state at `t1` is `committed`, `aborted`, or `expired` after the 3750ms maximum
lease plus 500ms drain margin. The enclosing endpoint may return a retryable
pending result if handoff claims have not drained. Uncertain completion cannot
return success. Restore/relink needs fresh proof/consent
and a new GIS binding UUID. The proposed developer restore SLO is ≤5.0s,
measured monotonically from activation of the new binding plus current Auth
grant/consent (`t0`) to the first accepted operation using a freshly
Auth-signed assertion for that binding (`t1`); it is a new provisional target,
not an existing contract. The hosted consumer harness observes `t0` after
both disposable stores commit its synthetic relink and conservatively records
`t1` when the successful Messaging response arrives, which upper-bounds the
earlier durable operation commit. It changes the binding ID and authority
revision without claiming provider or public relink lifecycle acceptance.
Old-binding proof remains denied for new writes.
The challenge producer is wired to T14 approval; Gateway does not yet publish
these APIs. The Auth-to-Messaging execution-permit bridge and Messaging
consumer are implemented, while cross-service Compose acceptance and broader
selected-profile privacy fanout remain open dependencies.

Owner revocation is `DELETE /api/v1/game-integrations/bindings/{binding_id}`
with a regular bearer, `Idempotency-Key` UUID and strict
`{"expected_revision":n}` body. GIS checks the Auth-selected account, moves
the binding to `revoking` to deny new permits, and drains issued permits before
it calls Auth's private mTLS `/handoffs/revoke` using the binding's stable
challenge operation ID. Auth blocks new handoff claims and acknowledges
`revoked` only after accepted claims complete. Either drain can remain pending;
GIS returns retryable unavailable and the caller retries with the same
idempotency key. Gateway publication remains pending.

T16's binding proof does not authorize a target chat. For T15 message writes,
GIS checks the signed exact `(application_id, environment_id, binding_id,
chat_id)` tuple against GIS-owned app-linked chat and active participant state;
it does not impose a global one-chat-per-binding rule. T30/T31 owns mapping
provisioning. The separate T51 event recipient has no chat/session selector,
so it must resolve exactly one eligible app-linked chat and fail closed on zero
or multiple candidates before creating an outbox row. No implicit primary chat
or cross-service database read is allowed. A GIS-to-Chat call must use a
recipient-specific signed service principal bound to the exact RPC/request
hash and TLS identity; raw `x-voice-profile-id` or nonempty internal-caller
headers are not authorization. T31 owns that Chat trust path and any managed
game RPC needed to enforce it.

Auth requests a bounded message permit at
`POST /internal/v1/game-integrations/bindings/{binding_id}/execution-permits`
with exact JSON `{"operation_id":"<canonical UUID>"}` and the raw compact
Auth device assertion in `X-Voice-Device-Authority`. Auth must validate that
assertion with its dedicated principal keyset before forwarding it. GIS parses
the exact assertion claims, verifies their identity against its stored binding,
and authenticates Auth's private call with workload proof v2. It requires one
each of `X-Voice-Workload: auth`, `X-Voice-Workload-Version: 2`, timestamp,
nonce, signature, content type and device assertion headers. The UTF-8 HMAC
input is exactly
`v2\n{METHOD}\n{escaped_path}\n{timestamp}\n{nonce}\n{lowercase_sha256_of_exact_body}\n{lowercase_sha256_of_exact_assertion_header_bytes}`.
The timestamp uses the existing ±30 second bound, the Redis nonce lifetime is
60 seconds, and missing key, replay, substitution or malformed bytes fail
closed. Successful issuance returns `200` for both first issue and exact
retries. The permit response contains only `permit_id`, `binding_id`,
`application_id`, `environment_id`, `binding_revision`, `assertion_jti`,
`operation_id` and RFC3339 UTC `expires_at`; Auth composes its own identity and
authority fields. Response HMAC uses the existing v1 status/path/timestamp/
nonce/exact-body format, with timestamp and nonce echoed and `no-store` set.

GIS serializes new permit issue with the binding row lock and persists each
operation/permit, assertion JTI and exact assertion hash. An exact operation
and assertion retry returns the saved permit without extending its expiry; a
different assertion under the same operation ID conflicts. New permits are
denied once a binding enters `revoking`. Expiry is no later than
`min(database_now + 3750ms, assertion.exp)`. Auth sends an idempotent completion
receipt at
`POST /internal/v1/game-integrations/execution-permits/{permit_id}/completion`
with exact `{"operation_id":"<UUID>","outcome":"committed|aborted"}`;
the response body is exactly the receipt fields `permit_id`, `operation_id`,
`outcome`, and `status:"completed"`, signed with the v1 workload response
proof. An identical outcome retry returns the receipt; divergent operation or
outcome conflicts. A completion arriving after expiry plus 250ms is rejected.

GIS revocation is a service seam, not a public player route. It CAS-transitions
`active` to `revoking` and increments the GIS revision, which immediately
prevents fresh permits. A successful revoke response is returned only after
every issued permit has a committed/aborted receipt or has expired plus the
500ms drain margin, then state becomes `revoked`. The hosted drain measurement
uses `t0` at the GIS binding's durable `active → revoking` commit and `t1` at
the final durable terminal state for every earlier permit: `committed`,
`aborted`, or `expired` after the 500ms drain margin. `t1 - t0` is bounded to
4.25 seconds. Endpoint request-to-`t0` and Auth grant's separate `revoking`
timestamp are recorded separately. On timeout it
leaves the binding `revoking` and the same operation ID resumes the drain.
Unknown completion therefore holds revocation pending until the fixed lease
expires. There is no shared permit cache. Auth
owns the device assertion/JWKS and grant, links the current binding reference,
and aggregates the permit issue/completion routes; GIS owns the durable
binding/permit ledger and verifies Auth's WorkloadProof v2 calls. The T14
challenge/exchange writer and GIS Auth mTLS client are implemented as bounded
development paths. The hosted T16 consumer acceptance remains pending; public
Gateway publication, T30/T31 production mapping provisioning, and production
admission remain separate open gates.

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

This contract is subordinate to the game integration feature and API canon.
The T32 section below freezes session/roster behavior; runtime implementation,
managed grants, bot and node-facing operations remain separate work with their
own tests and migration revisions.

## T30: resource mappings and immutable receipts

GIS owns T30 mapping and receipt rows in `game_integration_db`; it never reads
or writes Chat/Voice databases. The migration adds `game_resource_mappings`,
`game_resource_operations`, and `game_resource_binding_chats`. A mapping's
namespace is exactly `(application_id, environment_id, external_key)`. Identical key text in another
application or environment is independent. A scoped key has one resource kind
(`chat`, `voice`, or `space`) and one Voice resource ID; replacing its kind,
resource ID, or creation proof conflicts. Retired/tombstoned keys are retained
as non-reusable fences.

Each GIS mutation is keyed by `(application_id, environment_id, operation_id)`
and persists the canonical request hash and immutable terminal receipt. An
exact retry returns the stored receipt, including after a process restart. A
reused operation ID with a different request hash, or a scoped key already
bound to a different resource/proof, conflicts without changing either row.
The request hash covers the complete T30 mapping mutation: resource kind,
external key, resource intent, and associated Chat creation receipt. Binding
and Chat authorization tuples belong to the separate T31 roster revision and
are not part of a T30 mapping operation.

Chat creation provenance is a same-scope tuple of the exact Chat
`ProvisionManagedChat` `operation_id`, deterministic full-protobuf
`request_hash`, and returned `chat_id`. A `chat` mapping's resource ID must
equal that receipt's `chat_id`. A `voice` mapping must persist an associated
Chat ID and the same-scope Chat creation receipt; GIS exposes the mapping only
while the referenced Chat mapping and receipt still agree. A `space` mapping
uses the same app/environment/external-key namespace and retains its own
resource kind and ID. If the Chat response is uncertain, T31's GIS provisioning
caller retries the identical Chat operation ID and request bytes before
committing the mapping; Chat returns its original durable receipt, so the
retry cannot create a second Chat resource.

GIS also owns the exact binding-to-chat relation used by game-authored message
authorization. T31 party/session roster acceptance owns populating and removing
`game_resource_binding_chats` from complete, revisioned authoritative roster
updates. Stale, incomplete, failed, or partial roster input never clears these
rows. T32's current lease and binding revoke rules govern expiry and revocation;
an absent, expired, or revoked relation denies access. Until T31 wiring exists,
the table is empty and the lookup denies. The lookup key is
`(application_id, environment_id, binding_id, chat_id)` and succeeds only for
an active GIS binding and active same-scope Chat mapping. No partial binding,
membership-only, cross-environment, or caller-selected fallback lookup is
allowed. Missing, retired, tombstoned, expired, revoked, or mismatched state
denies new writes.

T30 stores and tests the mapping and lookup seams only. Mapping creation is not
exposed to a runtime caller until T31 passes the returned Chat ID and exact
`ProvisionManagedChat` request hash from the Chat RPC response into GIS. The
GIS unit fixture derives that deterministic hash from the generated request
proto with `principal.RequestHash`; it checks tuple validation, not that an RPC
was executed. T31 integrated acceptance must prove the actual Chat handler
receipt is passed through and an uncertain RPC is retried with the identical
operation and request before GIS commits the mapping.

Messaging uses the private
`POST /internal/v1/game-integrations/resource-mappings/authorize-chat` route.
The private listener is disabled unless its address, server certificate/key,
Messaging client CA, and current 32-byte WorkloadProof key are configured
together; partial configuration fails startup. It is separate from the public
HTTP listener and does not register on the public mux.
The strict JSON body contains exactly `application_id`, `environment_id`,
`binding_id`, and `chat_id`, each a canonical UUID. The route is served on the
GIS private listener over HTTPS with a pinned server CA/name and a Messaging
client certificate verified against the GIS Messaging CA with URI SAN
`spiffe://voice/service/messaging`. The HTTP proof reuses the exact
WorkloadProof v1 contract implemented by `httpapi.WorkloadVerifier.VerifyBody`
and `responseSignature`; it changes only the expected service principal from
`auth` to `messaging`, with a separate dedicated 32-byte key. Request headers
are exactly one each of `X-Voice-Workload: messaging`, `X-Voice-Timestamp`
(canonical Unix seconds), `X-Voice-Nonce` (lowercase UUID),
`X-Voice-Signature` (unpadded base64url HMAC-SHA256), and
`Content-Type: application/json`. The request HMAC input is exactly
`v1\n{METHOD}\n{escaped_path}\n{timestamp}\n{nonce}\n{lowercase_sha256_of_exact_body}`;
timestamp skew is ±30 seconds and Redis `SETNX` retains each nonce for 60
seconds. WorkloadProof v1 has no key-ID header. GIS verifies the request against
the current Messaging key and, during rotation, one configured previous key.
The previous key has an explicit acceptance deadline no more than five minutes
after rotation begins; a successful response is signed with the same key that
verified the request. The client holds both keys during this overlap so it can
verify that response. Keys must be distinct 32-byte secrets. Malformed or
partial key configuration fails startup; emergency revoke unsets the previous
key and restarts GIS, after which the old key is rejected immediately. No
change is made to Auth's verifier or the v1 HMAC bytes.
GIS signs exact response bytes with the existing status/path/timestamp/nonce
response proof. The response contains only `allowed` and, when allowed,
`mapping_revision`; a denial reveals no external key or resource details.
Missing TLS identity, key, proof, replay protection, or mapping state fails
closed. Messaging supplies the app/environment/binding/chat values it just
verified from the signed game message and Auth authority; GIS independently
checks the exact tuple against its own rows.

This producer and lookup contract does not implement T31 session orchestration,
roster synchronization, or Chat/Voice provisioning.

## T31: durable session orchestration

At base `c31a8e0b771f99d00f00a760a2c6e2f7ec67cbcd`, this was a target contract.
The T31 slice adds the GIS public session/operation/close routes, durable session
store, worker, active-event outbox, HTTPS claim/ACK API and PostgreSQL migration. Chat managed
provision/sync APIs already exist; Voice close and Role grant APIs are also
implemented as owner services.

The GIS T31 runtime enables owner calls only with complete mTLS tuples:
`GIS_CHAT_GRPC_ADDR`, `GIS_CHAT_TLS_CA_FILE`,
`GIS_CHAT_CLIENT_CERT_FILE`, `GIS_CHAT_CLIENT_KEY_FILE`;
`GIS_VOICE_GRPC_ADDR`, `GIS_VOICE_TLS_CA_FILE`,
`GIS_VOICE_CLIENT_CERT_FILE`, `GIS_VOICE_CLIENT_KEY_FILE`; and
`GIS_ROLE_GRPC_ADDR`, `GIS_ROLE_TLS_CA_FILE`,
`GIS_ROLE_CLIENT_CERT_FILE`, `GIS_ROLE_CLIENT_KEY_FILE`. GIS uses a distinct
client certificate for each owner trust boundary. It also requires
`GAME_INTEGRATION_CREDENTIAL_KEY_B64` for the public session credential check
and a configured public HTTPS listener for the session event claim/receipt API;
a partial owner tuple, missing signer, credential key, or HTTPS listener fails
startup. T31 does not require `NATS_URL` and does not publish active-session
events to JetStream. The signer uses
`GAME_INTEGRATION_PRINCIPAL_PRIVATE_KEY_FILE` and
`GAME_INTEGRATION_PRINCIPAL_KID` for the current key, plus
`GAME_INTEGRATION_PRINCIPAL_NEXT_PRIVATE_KEY_FILE` and
`GAME_INTEGRATION_PRINCIPAL_NEXT_KID` for its overlap key. Its workload-TLS
HTTPS endpoint at `/internal/v1/principal/jwks.json` publishes both public keys
and never private material. GIS signs owner requests only with the current KID
and issuer `gameintegration`; Voice uses a distinct issuer and keys.

The local Phase0 fixture configures GIS-to-Chat at `chat:9091`, GIS-to-Voice at
`voice:9091`, and GIS-to-Role at `role:9091`. It supplies separate GIS client
certificates for Chat, Voice and Role, and a separate Voice client certificate
for Role checks. T31's `/api/v1/sessions` route, stage worker,
`/api/v1/session-events/claim`, and `/api/v1/session-events/{event_id}/ack`
HTTPS routes are enabled together only when the full local fixture tuple is
present.

The normative public request, response, authentication scope, canonical hash,
status query, and retention contract is frozen in
[`game-integration-api.md`](../architecture/game-integration-api.md#t31-durable-session-orchestration).
GIS derives app/environment from the verified game-server principal and
requires `game.sessions.manage`. `POST /api/v1/sessions` stores the operation
before returning `202`; the two-second acceptance target applies only while
GIS PostgreSQL is healthy. `GET /api/v1/operations/{operation_id}` is a
read-only view scoped to that principal's app/environment.

GIS persists stages `accepted`, `chat_ready`, `roster_ready`, `voice_ready`,
`grants_ready`, and `active`, along with the exact downstream operation IDs,
request hashes, resource IDs, and durable owner receipt IDs. A worker claims a
stage with a renewable lease; an expired lease is reclaimed within six seconds
after restart when GIS and dependencies are healthy. Retries use exponential
backoff from one second to a 30-second cap. Timeouts, connection loss, and
owner `Unavailable`/`DeadlineExceeded` are unknown outcomes: keep the stage,
retry the exact same operation ID and request, and require the owner receipt
before advancing. Only an allow-listed durable owner rejection is terminal:
the private RPC returns `google.rpc.ErrorInfo` reason
`GIS_PERMANENT_OWNER_REJECTION` bound to the exact owner domain, RPC,
category, operation ID, and canonical request hash. GIS validates every
field against the request it sent; generic gRPC codes, malformed requests,
principal/setup failures, and persistence errors remain retryable or
unknown. Chat operation/resource conflicts are terminal; a Chat roster
`NOT_FOUND` is terminal only after the durable Chat-create receipt has been
recorded. Voice provisioning conflicts and Role operation conflicts or
terminal-revoked grants are terminal. A typed rejection marks the create
operation failed and creates a deterministic failure operation; when Voice
resources exist, that operation persists Voice-close and Role-revoke receipts
before the session reaches `failed`. An error never marks the session active.
If close wins while a create-stage owner call may have
committed, the close operation replays that same owner key and persists its
receipt before terminal cleanup. If no Voice room exists after reconciliation,
it closes without Voice/Role calls; otherwise it runs the terminal cleanup
stages and retains the durable receipts. No
dependency-outage completion SLA is implied.

During close reconciliation, an exact typed Chat-roster `resource_missing`
after the saved Chat-create receipt and Role-apply `terminal_revoked` are the
only rejections that prove no effect for the rejected stage. GIS durably records
that classification and advances without inventing a receipt; normal close
cleanup still runs when a Voice room exists. A typed operation or Chat resource
conflict may hide a prior effect, so GIS records an explicit
`create_reconcile_unresolved:<stage>:<category>` close stage, keeps the session
`closing`, sets no automatic retry, and returns that same custody on replay.
Unknown and generic errors retain exact-request retry behavior.

Party Chat provisioning is keyed by the party external key and produces one
stable mapping/receipt shared by its matches. A match or fleet session keeps
its own external key, Voice resource, roster revision, and operation receipts;
when parented, it references but does not own the party Chat. An unparented
match/fleet owns a dedicated Chat and Voice room under its own key. A complete
owned Chat roster is synchronized before Voice provisioning because Voice
admission requires current Chat membership. A parented child reuses the
party's current Chat and roster receipt; its complete profile UUID roster must
be a subset of the party roster, and GIS never syncs a child roster into the
party Chat. Fleet uses the same optional parent rule as match; no inferred
parent or cross-scope lookup is allowed. The request supplies complete profile
UUID `members` and monotonic `roster_revision`, with T32 stale/equal-revision
handling. `display_name` is required for party/unparented resources and
forbidden for parented children; it never mutates parent Chat metadata.
Owner IDs are deterministic within app/environment/resource/stage (and roster
revision for roster sync), and GIS persists them before the call. Remote
receipts are validated and stored before stage advancement. The `active`
session state and one outbox row commit atomically. GIS constructs its complete
v1 UTF-8 JSON event body once inside the active transition transaction, stores
its exact bytes and SHA-256 atomically with active state and the outbox row, and
serves those same bytes on the app/environment HTTPS pull API. The exact
seven fields are `event_id`, `application_id`, `environment_id`, `session_id`,
`operation_id`, `kind`, and `active_at`; UUIDs are canonical lowercase
hyphenated strings, `kind` is `party|match|fleet`, and the timestamp is UTC RFC
3339. `payload jsonb` is inspection only. A migration adds `payload_bytes
bytea`, `payload_sha256 bytea` with a 32-byte check, app/environment-scoped
`claim_lease_id` and `claim_lease_until` (replacing the relay-global lease
columns, not reusing them), plus durable
`consumer_ack_lease_id` and `consumer_ack_payload_sha256` fields.
It backfills pending rows with the frozen v1 encoder before serving claims and
requeues rows whose old `delivered_at` meant only JetStream PubAck.

`POST /api/v1/session-events/claim` requires HTTPS and the current app/env game
server credential with `game.sessions.manage`. GIS derives the scope from that
credential and filters app/environment before locking eligible outbox rows
with `FOR UPDATE SKIP LOCKED`. It leases one undelivered, unleased/expired row
for 30 seconds and returns status 200, the exact stored body bytes, and headers
`X-Voice-Event-Id`, `X-Voice-Payload-SHA256`, `X-Voice-Claim-Lease-Id`, and
`X-Voice-Claim-Lease-Expires-At`. Empty returns 204 with `Retry-After: 1`.
`POST /api/v1/session-events/{event_id}/ack` requires the same credential and
JSON `{\"lease_id\":\"<UUID>\",\"payload_sha256\":\"<lowercase hex>\"}`.
The first ACK must match that row's current unexpired lease and stored digest;
GIS atomically records delivery plus the accepted lease/digest, then clears the
lease. An exact ACK retry is 200 idempotently after commit; conflicting data is
409. Foreign/unknown event is 404; missing/expired/revoked credential is 401,
insufficient scope is 403, and expired/superseded lease or digest mismatch is
409. A lost claim response reserves the event until lease expiry; an empty
claim returns 204 until the lease expires, then the same bytes can be claimed
with a new lease. Receiver DB failure means no ACK. If ACK's response is lost
after GIS committed, retry the same ACK; if lease expiry races the first ACK,
re-claim, no-op the committed receiver inbox duplicate, and ACK using the new
lease. The migration stores the accepted ACK as `consumer_ack_lease_id` and
`consumer_ack_payload_sha256`; GIS sets `delivered_at` only after that
authenticated ACK. GIS database failure before claim/ACK commit returns 503;
the client retries without treating the event as delivered.

The named acceptance receiver is the test-only controlled game backend in
`src/backend/controlledgame/`, with an independent PostgreSQL schema. It polls
the real GIS HTTPS claim API, stores inbox identity `(application_id,
environment_id,event_id)` and exact-body digest in the same transaction as its
verifiable activation effect, then ACKs. Same identity+digest is a no-op;
same identity with another digest is an integrity conflict. The GIS
JetStream-specific `internal/sessionevents/jetstream.go` publisher/relay is not
part of this delivery lane; no game server receives master NATS access. The
controlled receiver remains test infrastructure, not a public SDK product.

GIS uses forward recovery after any possibly committed owner effect. Chat and
Voice provide no deletion API, so GIS never deletes resources. Explicit
close and failure compete on one transactionally stored
`terminalization_operation_id` compare-and-set. The winner selects the only
Voice close/Role revoke owner operation IDs. A losing operation coalesces onto
the winner and exposes its ID/status/receipt IDs; it never sends a second close
operation to an already closed room. If close wins, the create operation stays
failed while session status is closed; if failure wins, an explicit close
replay observes failed session status. Restart resumes the stored winning
terminalization stage. If neither
exists, GIS records terminal failure directly. Any created Chat remains
retained and inactive. Closing is a separate idempotent operation that closes
only the addressed session's Voice room and revokes only its grants. It cannot
close/delete a referenced party Chat or remove party membership. The close
request hash binds operation kind, target session, API version, verified
scope, and body. The route is
`POST /api/v1/sessions/{session_id}/close` with JSON `operation_id`; it returns
202 with `status: pending`, `session_status: closing`, and
`stage: voice_close_pending`. A party can close only with no active children.
GIS changes to `role_revoke_pending` after Voice close receipt and reports
`closed` only after Role revoke receipt. Voice fences new admission at close
commit and active media within five seconds. Session history and terminal receipts follow the T32 30-day
boundary; external-key tombstones are non-content and indefinite.

## T32: session lifecycle and roster authority

GIS owns stable app/environment external-resource mappings, session operation
receipts, accepted roster revisions, and lease deadlines. A party mapping is
stable across its matches; each match has its own key and may reference the
party. Closing/failing a match does not retire the party key or delete shared
Chat resources. A purged key retains a non-content tombstone and cannot
silently recreate a resource. GIS invokes Chat/Voice owner APIs and never
writes their databases directly.

Roster authority belongs to the authenticated app/environment game backend or
managed broker. GIS accepts complete snapshots only, verifies page/checksum
completeness, and applies revision comparison atomically in its database:
lower revision is stale no-op; same revision and same body replays the saved
receipt without lease renewal; same revision and changed body conflicts; only
a complete higher revision changes desired state and renews the lease. The
lease deadline is GIS database commit time plus exactly 60 seconds. Partial
snapshots, failed fetches, and transport-level empty responses cannot revoke
members; revocation requires a complete higher-revision empty roster or
explicit tombstone. Retries do not renew the lease.

At the exact lease deadline, new admission, reconnect, and governed reads/writes
fail closed. Voice owns media admission/ejection and fences active media within
the existing five-second revocation bound. Host transfer requires an
authenticated complete next-revision CAS whose successor is already in the
roster. GIS atomically changes session-control authority only; it never changes
Voice roles or Voice Owner. Client host claims are rejected. If the departing
host has no successor, its control is revoked immediately and the session
closes at lease expiry.

Chat enforces `since_join` using immutable message `created_at` and membership
interval `[joined_at, revoked_at)`, with rejoin opening a new interval and no
gap access. The same entitlement applies to history, search, quotes/thread
context, attachment metadata, and fetch-time download authorization. Match
history access expires exclusively at close plus 30 days. Terminal operation
receipts remain retryable for 30 days; after purge, the non-content external-key
tombstone persists without expiry and prevents silent resource recreation.
Keep-group includes only consenting participants and never copies the match
transcript. Chat owns
content, membership, and retention; GIS orchestrates via owner APIs.
Acceptance boundaries are SE03/SE04/SE07. Runtime code and exact-SHA evidence
remain open.

### T31 Gateway boundary

Gateway maps the existing `/api/v1/sessions`, `/api/v1/operations/{operation_id}`,
and `/api/v1/session-events/...` first-segment aliases to its configured
`game-integrations` upstream. Those T31 routes bypass only Voice-JWT validation
and forward the original `Authorization` header unchanged. GIS verifies the
app/environment `vgi1` credential and `game.sessions.manage` scope itself.
Public clients connect to the configured HTTPS Gateway hostname; TLS terminates
at the external Gateway proxy/Ingress. GIS remains on its private HTTP upstream
and must not be exposed over public HTTP.
