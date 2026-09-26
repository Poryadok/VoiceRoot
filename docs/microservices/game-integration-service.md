# Game Integration Service

## Authority and storage

This separate Go service owns developer applications, environments,
installations, external resource mappings, game bindings, communication
sessions, desired managed grants, and orchestration operations. Its PostgreSQL
database is `game_integration_db`; it has no SQL access to Auth, User, Chat,
Space, Voice, Bot, or Federation databases. Foreign IDs are logical UUID
references. Auth is the only account issuer; game backend facts do not become
Voice user credentials.

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
   sandbox credentials, bindings, subjects or data.
3. An approved application may create `sandbox` and `production` environments,
   each with its own ID, provider allowlist, redirect/origin allowlist,
   installation and credential namespace. Production environment activation
   requires the production application admission transition.
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
`Cache-Control: no-store`; no owner account ID is accepted in the body. The
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

This contract is subordinate to the game integration feature and API canon;
later sections will add sessions, managed grants, bot and node-facing operations
with their own tests and migration revisions.
