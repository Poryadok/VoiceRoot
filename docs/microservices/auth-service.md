# Auth Service

## Обзор

Сервис аутентификации и управления сессиями. Единственный сервис на Java — обусловлено зрелостью Spring Security для сложных auth-сценариев.

**Язык**: Java 25 LTS
**Фреймворк**: Spring Boot 3.5, Spring Security 6
**БД**: PostgreSQL `auth_db`

## Ответственность

- Регистрация (email, телефон, гостевой аккаунт)
- Логин / логаут
- JWT access token (15 мин) + opaque refresh token (30 дней)
- Refresh token rotation (одноразовые)
- Refresh rows retain the session's active `profile_id`; refreshing a switched-profile session keeps that profile instead of silently issuing a primary-profile token. Legacy refresh rows without a profile retain primary-profile fallback behavior.
- Отзыв всех сессий через Auth-owned `session_epoch`; strict-потребители Gateway и Realtime проверяют floor fail-closed
- 2FA (TOTP — Google Authenticator и аналоги)
- JWT blacklist (Redis, для логаута и ротации)

## T14 Auth-to-User selected-profile authority (accepted target; signer/client implemented)

T14 authorization approval and code exchange use User's Auth-only
`GetSdkProfileEligibility` RPC to verify the exact selected `(account_id,
profile_id)`. Auth validates that both returned IDs exactly match the request,
`profile_revision` is positive, and neither `deleted` nor `frozen` is true. A
missing, mismatched, stale, deleted, frozen, malformed, timed-out or unavailable
answer denies with the existing coarse `invalid_sdk_identity` response; it must
not be translated into account/profile enumeration details.

At approval, Auth stores the selected profile ID and returned revision with the
approval. At exchange, Auth reads eligibility again and requires the same IDs,
a positive unchanged revision, and an eligible profile. A changed revision,
deletion or freeze invalidates the approval/code and requires a fresh
authorization and consent. The profile revision is distinct from the consent
revision and application-policy revision.

The call is authorized by an Auth-issued service principal on User's dedicated
TLS listener (`:9094`). Its signed RS256 credential is bound to the exact full
RPC name, a fresh `x-request-id`, and SHA-256 of deterministic protobuf request
bytes. It carries `principal_type=service`, `iss=auth`, `sub=service:auth`,
`aud=user`, `iat`, `nbf`, `exp`, and unique `jti`; it carries no account ID,
profile ID or session epoch. Credential lifetime is at most 30 seconds, with
five seconds of permitted future issue/not-before skew and no expiry grace.
Auth sends exactly one `authorization: Bearer ...` and one `x-request-id` and
does not send raw identity metadata. User performs shared Redis replay
admission; Redis failure or a repeated `jti` denies the call.

Auth principal signing keys are a separate keyset from `auth.jwt` client-token
keys. The signer/client is optional only while SDK authorization is disabled;
enabling it requires the complete validated keyset and TLS endpoint. Partial or
invalid keyset configuration is a startup error. The exact environment, JWKS
and rotation contract is in
[Deployment: Auth-to-User SDK profile principal](../DEPLOYMENT.md#auth-to-user-sdk-profile-principal).
The principal JWKS endpoint is published only when the complete keyset is
loaded. The Auth signer, dedicated JWKS endpoint and TLS User eligibility client
are implemented with focused Auth tests. This does not claim deployment,
operational key-rotation proof or provider acceptance.

### T16 GIS binding challenge and handoff (partial implementation)

T14 `start` records a binding intent and stable operation ID. T14 `approve`
revalidates source device/session, policy, and the selected regular target
profile, then creates the GIS challenge with private WorkloadProof v1
`POST /internal/v1/bindings/challenges`. The challenge binds the approved
identity/profile/revision/scope/consent/policy tuple, redirect hash, PKCE,
device key, and operation. It is created only after profile selection and
consent; no provider subject or token is sent to GIS at this point. GIS returns
the persisted challenge ID, nonce and expiry. Auth pins those values to the
approval receipt before returning the code response.

For an exact retry after a lost approval response, Auth returns the same
one-use code and challenge receipt only while the same source session/device
is current and the code is unexpired and unconsumed. The code is encrypted
with AES-256-GCM in the approval receipt and is never stored in plaintext.
`AUTH_GAME_BINDING_APPROVAL_CODE_KEY_B64` must provide the dedicated 32-byte
key; there is no default and it must not reuse Auth principal, device, or GIS
workload-proof keys. A changed target/profile conflicts, while an expired or
consumed receipt cannot authorize a retry.

GIS first reads the persisted challenge through Auth's existing
`SdkGameIntegrationPolicyClient`: private GET
`/internal/v1/bindings/challenges/{challenge_id}`, authenticated with the GIS
WorkloadProof v1 request/response HMAC and replay nonce. Its strict response
contains exactly `challenge_id`, `nonce`, `application_id`, `environment_id`,
`provider`, `redirect_uri_sha256`, `pkce_challenge`, `device_key_id`,
`device_key_thumbprint`, `operation_id`, `expires_at`, and `status`. Auth accepts
only the canonical requested UUID, a live `pending` challenge, and exact
app/environment/redirect/PKCE/device matches.

The browser-visible T14 approval code is consumed only at private GIS-authenticated
`POST /internal/v1/auth/game-bindings/handoffs/exchange`, with exact JSON
`challenge_id`, `operation_id`, `code`, `code_verifier`, and `device_proof`.
The device proof is the existing `voice-sdk-code-v1` proof over authorization
request ID, code hash and verifier hash. Auth locks and revalidates the current
SDK device/session, T14 approval, policy, target regular account and selected
profile revision, then atomically consumes the code and stores the linked-session
consent revision and exact handoff JWS receipt. Response is exactly
`{"handoff_jws":"<compact JWS>"}`, `application/json`, `no-store`; the JWS is
delivery-only and never returned through the browser exchange. Public T14
`/exchange` rejects authorizations carrying a GIS challenge. An identical
challenge/operation/code/verifier/device-proof retry returns the stored JWS
without extending its original expiry; changed tuple conflicts.

Auth's dedicated mTLS connector uses
`voice.auth.game-binding.mtls.port`, `.server-cert-file`, `.server-key-file`,
`.client-ca-file`, `.truststore-file`, `.truststore-password`, and
`.allowed-client-uri-san`, with environment bindings
`AUTH_GAME_BINDING_MTLS_PORT`, `_SERVER_CERT_FILE`, `_SERVER_KEY_FILE`,
`_CLIENT_CA_FILE`, `_TRUSTSTORE_FILE`, `_TRUSTSTORE_PASSWORD`, and
`_ALLOWED_CLIENT_URI_SAN`. Port unset/0 disables it; enabling requires all TLS
files and the exact GIS URI SAN. The PKCS12 JSSE truststore must contain exactly
the X.509 CA set in `client-ca-file`; the latter also configures Tomcat's
OpenSSL provider. Configure only the approved GIS and Messaging client CA set,
and keep the truststore password out of logs. Private route filters require a
verified peer certificate with the exact route-specific URI SAN; missing or
untrusted certificates and mismatched URI SANs fail closed. This private
listener is not Gateway-published and has no plaintext fallback.

Provider subjects are HMAC-SHA-256 digested in Auth with a dedicated 32-byte
standard-base64 key. Configure both
`voice.auth.game-binding.subject-digest.kid` /
`AUTH_GAME_BINDING_SUBJECT_DIGEST_KID` and
`voice.auth.game-binding.subject-digest.key-base64` /
`AUTH_GAME_BINDING_SUBJECT_DIGEST_KEY_B64`. Key ID is 1–32 URL-safe characters;
key bytes must be canonical standard Base64, exactly 32 bytes and nonzero. Do
not reuse Auth principal, client-token or device keys. The output is
`hmac-sha256-v1:{kid}:{lowercase hex}` over a version prefix and length-framed
provider, issuer, application ID, environment ID and verified provider subject.
Missing digest key keeps this exchange unavailable; raw subjects never enter GIS.

The existing T14 device proof is the handoff proof of possession. At exchange,
Auth verifies the `voice-sdk-code-v1` signature and atomically stores the
lowercase SHA-256 of the exact proof bytes against the handoff JTI, operation,
challenge and registered device. GIS forwards those same proof bytes in its
private online claim. Auth requires the persisted digest and tuple before
accepting a new claim, and still rechecks current consent, profile, policy,
source session and device state. Exact claim retries require identical proof
bytes; changed bytes conflict. No Auth JWS is exposed to the SDK to obtain a
second signature.

Auth exposes private `POST /internal/v1/auth/game-bindings/handoffs/claim`,
`POST /internal/v1/auth/game-bindings/handoffs/completion`, and
`POST /internal/v1/auth/game-bindings/handoffs/revoke` on a dedicated,
opt-in mTLS connector; these routes are not Gateway routes. The GIS client
certificate must chain to the configured client CA and contain the exact
configured URI SAN. The connector is disabled by default, rejects partial TLS
configuration, and has no plaintext fallback. Claim JSON is exactly
`operation_id`, `request_sha256`, `handoff_jws`, and `device_proof`; completion
JSON is exactly `claim_id`, `operation_id`, `outcome`, and optional
`binding_id`. Responses use the GIS typed receipt shape and `Cache-Control:
no-store`.

The claim ledger verifies the dedicated Auth handoff signature, locks the SDK
identity and consent grant, rechecks current app/environment, device,
ownership generation, target account epoch, selected profile and policy
revisions, and verifies device proof over the assertion JTI, operation ID and
request hash before persisting the claim. Exact operation/JTI/hash replay
returns the saved receipt; changed tuples conflict. Completion is
transactional and idempotent for the same terminal outcome and GIS binding ID.
Revoke names the stable challenge operation ID; it moves the grant to
`revoking` before checking accepted claims, denies new claims, and returns
`revoking` until every accepted claim has a durable success/failure receipt. A
retry with the same operation ID returns `revoked` only after the claim ledger
drains. This slice has focused controller, issuer and PostgreSQL claim/revoke
tests. GIS exchange operation/outbox and T14 challenge creation now have
focused handler and database lifecycle tests. The Auth-to-Messaging permit
aggregator described below is implemented; hosted cross-service replay and
revoke-drain acceptance remains a separate evidence gate.

### T16 Auth-to-Messaging execution permits

Auth exposes `POST /api/v1/auth/sdk/game-message/execution-permits` and
`POST /api/v1/auth/sdk/game-message/execution-permits/{permit_jti}/completion`
only to the registered Messaging workload over mandatory mTLS. The issue
request contains only the stable `operation_id` and SHA-256 of exact canonical
message payload bytes; `X-Voice-Device-Authority` is the original compact Auth
device assertion. Auth returns strict `{ "permit_jws": "..." }` JSON with
`no-store`. Completion returns the exact GIS receipt fields. These are
service-to-service routes, not player/Gateway routes.

For a new permit, Auth resolves exactly one persisted SDK authorization row in
`sdk_authorizations`, joined to its `sdk_linked_sessions` handoff receipt by
request ID and matching `game_binding_consent_revision`/`consent_revision`.
The linked-session expiry bounds only handoff delivery and replay; it is not the
duration of the send grant. The authorization transaction row's `expires_at`
is not a grant expiry and must not be used to extend or revoke the grant.

Before issuing `/api/v1/auth/sdk/device-authority`, Auth requires exactly one
current active grant for the request's application, environment and account;
it derives the actor from the active Auth SDK identity and the device from the
authenticated session. The production `SdkBindingAuthority` implementation
then reads
`GET /internal/v1/bindings/{binding_id}/authority` from GIS with v1
WorkloadProof and verifies the exact-byte signed `no-store` response. Auth
checks the exact application/environment/binding tuple, active GIS status,
positive binding revision, current consent/revision and local actor/device
consistency. Missing or ambiguous grants, inactive or mismatched GIS state,
invalid proof/MAC, malformed response and GIS unavailability all deny
assertion issuance. `character_context` is checked as part of the strict GIS
response schema but is not consumed or logged by Auth.

When Auth accepts the handoff claim and the player binding becomes active, it
creates a separate durable Auth-owned `sdk_game_message_grants` row keyed by
`(application_id, environment_id, target_account_id, target_profile_id)` and
bound to that binding ID. It stores the exact consent revision, canonical
scopes and policy/profile revisions, with lifecycle `active`, `revoking`, or
`revoked`. This grant has no implicit time-to-live: its lifetime source is the
persisted grant lifecycle. It is valid until an explicit binding/grant revoke
or a current account, profile, policy, or binding authority transition makes
it unusable. Reauthorization after revoke creates a new binding and grant
revision; it never revives the old grant. Auth serializes revoke and permit
admission on this grant row. Revoke changes it to `revoking` before waiting for
admitted operations to complete or expire, then marks it `revoked`; while
revoking, no new permit is admitted.

Auth derives `profile_id` only from the stored authorization's selected
`target_profile_id` (and its persisted profile-alias selection where
applicable), never from a request or device assertion. Send scope authority
comes from the durable grant's consented `scopes` and consent revision,
intersected with current application/environment policy and current Auth
account, device, session and binding state. The source SDK principal is the
standalone `sdk_identities` row; its account ID is not a regular `accounts`
row. Auth checks that SDK identity's active status and its current session and
device/key authority. The selected target account is independently checked
against `accounts.status` and `session_epoch`. An active `game_binding_status`
proves only that the player binding is active; it does not prove
`game.chat.send` consent. Auth requires the canonical `game.chat.send` grant
in persisted consent and current policy, and requires consent, scope, profile
and policy revisions to remain current. A current SDK authorization row is
used only to resolve the handoff receipt/consent tuple and must be consumed;
its short transaction expiry does not bound or extend the durable grant. Auth
also checks the target account epoch and current User profile eligibility and
revision. Missing, stale, revoked, expired, ambiguous or mismatched
consent/profile/binding/policy authority denies the request. Device
registration, revocation and proof of possession remain independent request
authority and do not change the durable grant key.

Auth verifies the device-status assertion with its dedicated principal keyset,
then calls GIS's exact
`POST /internal/v1/game-integrations/bindings/{binding_id}/execution-permits`
using WorkloadProof v2 and the same assertion bytes. GIS serializes issue with
binding revocation, returns the persisted permit for an exact retry, and denies
new permits after `revoking`. Auth validates the GIS response proof and tuple,
re-reads consent, account/device/profile/policy/binding authority under the same
Auth serialization lock, and signs only if every revision remains current.
The Messaging consumer checks GIS's exact current
`(application_id, environment_id, binding_id, chat_id)` mapping before it calls
this Auth permit route. This order prevents a missing or foreign chat link from
creating either an Auth or GIS execution permit. The exact Auth JWT
header/claim set, GIS request/response proof, 3750 ms maximum permit lifetime,
Messaging's 250 ms clock margin/transaction budget, and 4.25-second revoke
ceiling are frozen in the
[game-message API contract](../architecture/game-integration-api.md#messaging-ingress-and-t16-execution-permit).
An exact retry reuses GIS's permit and returns the same Auth permit; changed
operation/request digest or divergent completion conflicts. GIS or Auth
completion uncertainty remains pending.

### Subscription claims (A7 accepted target; not implemented)

Auth consumes the complete revisioned personal
`subscription.entitlement_changed` snapshot into a transactional `auth_db`
inbox/projection. Access JWT adds `subscription_revision` and
`subscription_entitled_until` beside `subscription_tier`. Paid benefit ends at
deadline equality even when the access token remains otherwise valid. A service
that reaches that boundary calls Subscription-owned
`ResolveEntitlementAtBoundary` with the claim revision as `minimum_revision`;
Auth is not a second decision owner and dependency failure cannot extend Premium.

Auth creates its durable consumer before a protected one-watermark snapshot,
applies `INACTIVE` tombstones, then drains queued events. The current optional
in-memory NATS tier store is not an authority/restore path. Full replay and RED
contract: [subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

When `auth.nats.url` is configured, the optional tier cache attaches only to a
centrally provisioned JetStream push consumer: stream `subscription_events`,
durable `auth_subscription_tier`, filter `subscription.>`, delivery subject
`_INBOX.voice.auth.subscription_tier`, no delivery group, explicit ACK, and
deliver-new. Auth fails startup if the consumer is absent or incompatible; it
does not create streams or consumers. An optional `auth.nats.creds-file` applies
to both Auth NATS connections for direct authenticated NATS deployments; the
staging pod-local leaf connection remains anonymous and does not mount app
credentials. Request replies use `_INBOX.voice.auth.requests.>`.
- OTP throttling в Auth-owned Redis: `auth:otp:send:<account_id>` резервируется
  одним `SET NX PX` до создания/отправки кода; `auth:otp:verify:<account_id>`
  ведёт атомарное admission до сравнения кода в Redis sorted-set sliding window
  одним Lua-вызовом. Defaults из
  архитектурного канона: один resend в минуту и три OTP verification attempts за десять
  минут. Redis error, corrupt Redis state или невозможный ответ закрывают OTP flow
  c `auth_unavailable`; ключи и Redis error detail клиенту не выдаются.
  In-memory throttle допустим только при явном `auth.persistence=memory` для
  test/local memory mode; JDBC deployment без Redis bean не стартует.
- Гостевые аккаунты (30-дневный TTL, ограниченные права)
- Конвертация гостевого аккаунта в полноценный
- Soft delete аккаунта (30-дневный grace period)
- JWKS endpoint для публичных ключей (используется Gateway и другими сервисами)
- **[auth-and-contacts](../features/auth-and-contacts.md):** перед выдачей access JWT Auth вызывает internal User gRPC `EnsurePrimaryProfile`; claim `profile_id` равен User-owned `profiles.id`. Auth не получает credentials к `user_db`; см. [primary-profile-bootstrap.md](primary-profile-bootstrap.md), [EXEC_PLAN.md](../EXEC_PLAN.md).
- OTP генерация и валидация (email)

### PR и ревью (bootstrap JWT ↔ User)

- Перед merge — зелёный job **`backend-auth`** в CI (`mvn -B test`). Интеграция Auth JDBC + Redis и контракт Auth ↔ User gRPC покрывают регистрацию / login / refresh / validate, включая совпадение `profile_id` с ответом `EnsurePrimaryProfile` и fail-closed поведение.
- Maven внутри контейнера **без** доступа к Docker socket хоста может **пропускать** Testcontainers suites. `backend-auth` после единственного `mvn -B test` fail-closed проверяет Surefire report каждого класса с `@Testcontainers(disabledWithoutDocker = true)`: report должен существовать, содержать `tests > 0`, `skipped=0`, `failures=0` и `errors=0`; ориентир — CI или хостовый `mvn test` с Docker ([TESTING.md](../TESTING.md), job Auth в [.github/workflows/ci.yml](../../.github/workflows/ci.yml)).
- Меняете claims JWT или схему `profiles` — синхронизируйте потребителей (Gateway, Go) с [`DATA_MODEL.md`](../DATA_MODEL.md) и при необходимости прогоните buf / контрактные проверки.

## API (gRPC)

Канон: [`protos/voice/auth/v1/auth.proto`](../../protos/voice/auth/v1/auth.proto). Кратко:

```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (RegisterResponse);   // session: AuthSession
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
  rpc Enable2FA(Enable2FARequest) returns (Enable2FAResponse);
  rpc Verify2FA(Verify2FARequest) returns (Verify2FAResponse);
  rpc VerifyOTP(VerifyOTPRequest) returns (VerifyOTPResponse);
  rpc ConvertGuest(ConvertGuestRequest) returns (ConvertGuestResponse);
  rpc DeleteAccount(DeleteAccountRequest) returns (DeleteAccountResponse);
  rpc RestoreAccount(RestoreAccountRequest) returns (RestoreAccountResponse);
  rpc ValidateToken(ValidateTokenRequest) returns (ValidateTokenResponse); // internal
  rpc GetJWKS(GetJWKSRequest) returns (GetJWKSResponse); // public
  // See docs/features/multi-profile.md; returns a session with the selected profile claim.
  rpc SwitchActiveProfile(SwitchActiveProfileRequest) returns (SwitchActiveProfileResponse);
  // Internal — platform moderation changes account status (reports.md).
  rpc SetAccountStatus(SetAccountStatusRequest) returns (SetAccountStatusResponse);
  // Internal — Social SyncPhoneContacts maps phone hashes to primary profile IDs.
  rpc ResolvePhoneHashes(ResolvePhoneHashesRequest) returns (ResolvePhoneHashesResponse);
  rpc PutE2EKeyBackup(PutE2EKeyBackupRequest) returns (PutE2EKeyBackupResponse); // encryption.md
  rpc GetE2EKeyBackup(GetE2EKeyBackupRequest) returns (GetE2EKeyBackupResponse);
}
```

`SwitchActiveProfile` takes `access_token`, `profile_id`, and `device_info_json`;
the response contains the replacement `AuthSession`. The active profile claim is
selected by Auth using the User-owned profile contract described in
[multi-profile.md](../features/multi-profile.md).

`SetAccountStatus` is the internal platform-moderation RPC. Its request carries
`account_id`, `status` (`active` or `suspended`), and `reason`; it returns an
empty response. The moderation contract is described in
[reports.md](../features/reports.md).

`ResolvePhoneHashes` is an internal Social RPC used by `SyncPhoneContacts`. It
accepts `phone_hashes` and returns `matches` containing each resolved
`phone_hash` and primary `profile_id`. This internal mapping RPC does not change
the post-alpha/G3 phone-book discovery scope in
[auth-and-contacts.md](../features/auth-and-contacts.md).

### Ownership-transfer step-up proof

Auth implements `IssueOwnershipTransferProof` and `ConsumeOwnershipTransferProof`.
The issue surface requires a verified Gateway delegated-user principal; consume
requires a verified Space service principal. Gateway routing and Space transfer
integration remain separate rollout requirements. Unverified/legacy metadata never
creates proof authority.

- Auth checks password on every request. If the account has 2FA enabled, it also
  requires a valid TOTP or one unused backup code. It issues an opaque
  high-entropy proof bound to `account_id`, active `profile_id`, `space_id`,
  `new_owner_profile_id`, UUID `operation_id`, current `session_epoch`, and
  verified-factor set. Plaintext is returned once; Auth persists only its hash.
- The proof expires after five minutes and is revoked when session epoch, password,
  2FA or other account security state changes. Validation/consume never treats a
  missing, expired, revoked, mismatched or previously used proof as usable.
- Only the authenticated Space service principal may call consume. Consume is
  atomic and one-use; its durable receipt is keyed by `operation_id` and includes
  the exact bound identities. A retried identical consume returns that receipt,
  while a changed binding or another operation fails closed.
- Auth owns factors, proof hash, revocation state and receipt. Space owns the
  transfer request/idempotency record and no service other than Auth stores the
  proof plaintext. Gateway derives actor only from verified claims, relays the opaque
  proof to Space, redacts it from logs/traces/metrics, and neither creates, validates
  nor persists it.
- Public failure disclosure is deliberately coarse: no session is
  `UNAUTHENTICATED`; a factor failure or unusable/mismatched proof is
  `PERMISSION_DENIED`; malformed bindings are `INVALID_ARGUMENT`. Trusted caller
  failure, store failure or unavailable Auth never permits a transfer.

See [spaces.md](../features/spaces.md#контракт-подтверждения-передачи-владения)
for the end-to-end request, Space idempotency and error contract.


#### Durable proof implementation

`auth.persistence=jdbc` provides the proof service; memory mode has no proof store
and the adapter fails closed. Principal listener/configuration is documented in
[Auth README](../../src/backend/auth/README.md). The public issue route is
`POST /api/v1/auth/ownership-transfer-proof` through Gateway; the internal consume
RPC is never a public route.

Flyway `V12__ownership_transfer_proofs.sql` (golang-migrate mirror
`000013_ownership_transfer_proofs.up.sql`) adds `ownership_transfer_proofs` with
unique `operation_id`, SHA-256 proof hash, exact account/profile/Space/target/epoch
bindings, verified factors, five-minute expiry and durable receipt ID/time. A proof
contains 32 random bytes encoded as unpadded base64url. Re-issuing an existing
operation cannot replace its proof or return its plaintext again. An identical
consume retry returns the already committed receipt, including after later expiry
or revocation; it never creates a new grant. Changed proof/bindings fail closed.
Times are normalized to PostgreSQL microsecond precision before persistence and
response so retried receipts remain identical.

`accounts.security_revision` is monotonic. Database triggers advance it on changes
to password, TOTP secret/enabled state, session epoch, account status/deletion/type,
email/phone and pending-verification state, and on backup-code replacement/use.
Reverting credentials does not revive an older proof. Issue and consume hold the
account row lock for their complete transaction. Existing JDBC backup-code consume
and replacement use the same account-before-backup lock order; replacement and
factor consumption are atomic. An issue that consumes a backup factor reloads its
new revision before saving the proof, and a later write failure rolls both back.
Only the existing receipt replay bypasses new-grant expiry/security checks.

Focused verification: `OwnershipTransferProofServiceTest`,
`OwnershipTransferProofJdbcIntegrationTest`, and `AuthOwnershipProofAdapterTest`
cover factors, all bindings, expiry, revocation, rollback, concurrent consumers and
security writers, coarse status disclosure and hash-only persistence. Proofs and
factor material are never included in adapter errors or logs.

#### Consumed-receipt recovery

`GetOwnershipTransferReceipt` is an internal read-only recovery RPC for Space's
durable ownership-transfer journal. It recovers a previously committed receipt
after a consume response is lost, without storing or resending the opaque proof.
It creates no grant and never consumes an issued proof. There is no public route.

The request contains seven required fields: UUID `account_id`, `profile_id`
(the original owner/actor), `space_id`, `new_owner_profile_id`, `operation_id`,
positive original `session_epoch`, and `proof_digest` (exactly 64 lowercase hex
characters encoding SHA-256 of the original opaque proof's UTF-8 bytes, with no
prefix or whitespace). Every field must match the stored consumed proof.
The response contains only `receipt_id`, the five bound UUID fields,
`session_epoch`, `consumed_at`, and `verified_factors`, identical to the original
consume receipt. Neither proof plaintext nor its digest is returned.

Only a fresh verified Space service principal may call this RPC on the private
Auth TLS listener. It uses the same Phase 0 audience, exact RPC, request hash,
request ID, credential expiry and replay checks as consume. Gateway delegated
users and other service callers cannot recover receipts. The legacy listener
denies this method, including requests with signed service credentials.

Missing or invalid credentials yield `UNAUTHENTICATED`; a verified wrong caller
yields `PERMISSION_DENIED`. Malformed UUIDs, nonpositive epochs or malformed
digests yield `INVALID_ARGUMENT`. Missing, unconsumed and mismatched records all
yield the same coarse `PERMISSION_DENIED`; storage/configuration failure yields
`UNAVAILABLE`. Errors do not disclose stored bindings, digest, factors or account
state.

Recovery matches the original epoch; it does not reauthorize against the current
account, session floor, password, 2FA, security revision or proof expiry. A retained
consumed receipt remains recoverable after account deletion, including physical
removal of its account row. The store uses a separate read-only `READ_COMMITTED`
transaction, suspending any caller transaction; an uncommitted consume cannot
become recovery authority, even in the same thread. Lookup takes no account or
proof row lock and changes no rows.

An absent receipt is not evidence that an in-flight consume will never commit.
Space must persist its immutable original bindings and proof digest before
consume, and persist a confirmed receipt before advancing its transfer protocol.
Once Space decides to abort, a late receipt must not revive that operation.

Consumed receipts survive ordinary five-minute pending-proof cleanup. This slice
adds no cleanup job or retention extension. The existing 30-day account-erasure
policy and [data erasure rules](../DATA_MODEL.md) remain authoritative. Before
activating receipt erasure or pseudonymization, Auth and Space must coordinate
pending-journal settlement and durable terminal operation fences; historical
lookup is not an exemption from erasure policy.

`OwnershipTransferReceiptLookupTest` and
`OwnershipTransferReceiptLookupJdbcIntegrationTest` cover exact binding/digest
matching, historical and hard-deleted-account recovery, no state mutation, and
uncommitted/rolled-back consume isolation on separate and ambient transactions.

### Registration input validation

`POST /api/v1/auth/register` rejects a supplied malformed email with HTTP `400`
and `{ "error": "validation_failed" }` before creating an identity or sending
an email verification code. Email may be omitted for guest and phone registration.

### OAuth2 authorization code flow

Auth exposes an authorization-code flow for the Developer Portal and the
separately configured admin client. The REST controller is rooted at
`/api/v1/auth`:

| Method and path | Contract |
|-----------------|----------|
| `GET /oauth2/authorize` | Returns the sign-in form. Requires `response_type=code`, `client_id`, `redirect_uri`, `code_challenge`, and `code_challenge_method=S256`; `state` is optional. |
| `POST /oauth2/authorize` | Accepts form fields for the same authorization request and `email` or `phone`, `password`, and optional `totp_code`; redirects with the authorization `code` and optional `state`. |
| `POST /oauth2/token` | Accepts `grant_type=authorization_code`, `code`, `redirect_uri`, `client_id`, `code_verifier`, and `client_secret`; returns `access_token`, `token_type`, and `expires_in`. |
| `GET /.well-known/openid-configuration` | Returns `issuer`, `authorization_endpoint`, `token_endpoint`, and `jwks_uri`. |

Only `response_type=code` and `grant_type=authorization_code` are accepted.
The configured client must be enabled, the redirect URI must exactly match one
of that client's configured URIs, and the authorization request and exchange
use the S256 PKCE challenge. The authorization code is bound to the client,
redirect URI, challenge, and logged-in account/profile, and expires according
to that client's `authorization-code-ttl` (default `PT60S`). The token response
contains an access token; this endpoint does not return a refresh token. A
configured client secret is checked during exchange; an empty client-secret
setting skips that check.

Client settings live under `auth.oauth.developer-portal` and `auth.oauth.admin`:
each has `enabled`, `client-id`, `client-secret`, `redirect-uris`, and
`authorization-code-ttl`. `auth.oauth.public-api-base-url` supplies the base
used in the discovery document. The application configuration currently
defaults the Developer Portal client to enabled and the admin client to
disabled; both require an enabled client ID and configured redirect URI to
complete a flow. OAuth errors use an `{ "error": "..." }` response body.

### ConvertGuest (guest → regular)

REST: `POST /api/v1/auth/convert-guest` (Gateway transcoding). Спека UX: [auth-and-contacts.md](../features/auth-and-contacts.md) § «Регистрация гостевого аккаунта».

| Поле | Семантика |
|------|-----------|
| `email` / `phone` | Идентификатор постоянного аккаунта (хотя бы один) |
| `password` | **Новый пароль**, который пользователь задаёт в форме convert-guest для `regular`-аккаунта |

- Авторизация: **JWT гостя** (Bearer); transport-пароль, сгенерированный при guest bootstrap, **не проверяется** в `ConvertGuest`.
- После submit создаётся pending email identity; тот же `account_id` / primary
  `profile_id` и session сохраняются, но права остаются guest-level.
- Только успешный email verification атомарно меняет `accounts.type` → `regular` и
  публикует NATS `user.guest_converted`; resend идемпотентен и не создаёт новый аккаунт.
- Негативные кейсы (duplicate email, password &lt; 8, non-guest token): `ConvertGuestIntegrationTest` (Auth Maven).

### Session-bound email-verification recovery

`GetEmailVerificationStatus` and REST `GET /api/v1/auth/verification-status`
derive the caller only from the restricted session and return typed
`GUEST`, `EMAIL_PENDING` (`NONE`/`ACTIVE`), `PROMOTION_PENDING`, or `REGULAR`.
Every issued `AuthSession` (REST and gRPC) carries optional
`email_verification_required`: present `true` for pending email registration,
guest conversion, and delayed promotion; present `false` only for an anonymous
guest with no email or phone; absent for legacy/unknown state. Consumers must
keep absence distinct from `false` and use the authenticated status endpoint
to resolve it. `account_type=guest` alone does not distinguish these states.
Email verification OTP send/verify have no public email identifier. Registration
and convert issue the restricted session, then send once through that principal.
OTP is six digits for ten minutes; resend invalidates an old code without
resetting its attempt budget. A delayed durable promotion returns `202` and is
recovered through status; a replacement regular session is issued only after
promotion completes.

### E2E key backup ([encryption.md](../features/encryption.md), REST via Gateway)

Клиент хранит парольно-зашифрованный бэкап ключей Signal на сервере; сервер видит только opaque blob ([encryption.md](../features/encryption.md)).

| gRPC | REST (Gateway transcoding) | Назначение |
|------|----------------------------|------------|
| `PutE2EKeyBackup` | `PUT /api/v1/auth/e2e-key-backup` | Сохранить/обновить blob (`encrypted_blob`, опционально `password_hint`); `204 No Content` |
| `GetE2EKeyBackup` | `GET /api/v1/auth/e2e-key-backup` | Получить blob для восстановления на новом устройстве; `404` до первого PUT |

- **Владение данными:** пароль и ключ расшифровки — только на клиенте; Auth хранит `encrypted_blob` как есть.
- **Лимиты Gateway:** `E2EKeyBackupPut` 5/min, `E2EKeyBackupGet` 30/min (`ratelimit.go`).
- **Клиент:** `VoiceE2eClient` + UI в `e2e_chat_settings.dart`; см. также [messaging-service.md](messaging-service.md) (key backup не в Messaging).

## Модель данных

```
accounts
├── id (UUID)
├── email (nullable, unique)
├── phone (nullable, unique)
├── password_hash (bcrypt)
├── type (regular | guest)
├── status (active | suspended | deleted)
├── session_epoch (положительный монотонный durable epoch, default 1)
├── email_verified_at (nullable; null = restricted pending identity)
├── totp_secret (encrypted, nullable)
├── totp_enabled (bool)
├── deleted_at (nullable, soft delete)
├── last_online_at (nullable; Auth account activity, Flyway V5)
├── created_at
└── updated_at

refresh_tokens
├── id (UUID)
├── account_id (UUID, logical ref → accounts.id)
├── token_hash (SHA-256)
├── device_info (jsonb)
├── expires_at
├── created_at
└── revoked_at (nullable)

otp_codes
├── id (UUID)
├── account_id (UUID, logical ref → accounts.id)
├── code (encrypted)
├── type (email_verify | password_reset)
├── expires_at
├── used_at (nullable)
└── created_at

backup_codes ([privacy.md](../features/privacy.md), Flyway V2)
├── id (UUID)
├── account_id (UUID, logical ref → accounts.id)
├── code_hash (CHAR(64))
├── used_at (nullable)
└── created_at

e2e_key_backups ([encryption.md](../features/encryption.md))
├── account_id (UUID, PK, logical ref → accounts.id)
├── encrypted_blob (TEXT, client-encrypted opaque payload)
├── password_hint (nullable)
└── updated_at
```

### V1 (core DM scope) — детальный профиль для DDL

```
accounts
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── email VARCHAR(320) NULL
├── phone VARCHAR(32) NULL
├── password_hash TEXT NOT NULL
├── type VARCHAR(16) NOT NULL CHECK (type IN ('regular','guest'))
├── status VARCHAR(16) NOT NULL CHECK (status IN ('active','suspended','deleted'))
├── session_epoch BIGINT NOT NULL DEFAULT 1
├── email_verified_at TIMESTAMPTZ NULL
├── totp_secret BYTEA NULL
├── totp_enabled BOOLEAN NOT NULL DEFAULT false
├── deleted_at TIMESTAMPTZ NULL
├── last_online_at TIMESTAMPTZ NULL -- Flyway V5__accounts_last_online_at.sql
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

refresh_tokens
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → accounts.id
├── token_hash CHAR(64) NOT NULL
├── device_info JSONB NOT NULL DEFAULT '{}'::jsonb
├── expires_at TIMESTAMPTZ NOT NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── revoked_at TIMESTAMPTZ NULL

otp_codes
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → accounts.id
├── code BYTEA NOT NULL
├── type VARCHAR(32) NOT NULL CHECK (type IN ('email_verify','password_reset'))
├── expires_at TIMESTAMPTZ NOT NULL
├── used_at TIMESTAMPTZ NULL
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

backup_codes ([privacy.md](../features/privacy.md), Flyway V2__backup_codes.sql)
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → accounts.id
├── code_hash CHAR(64) NOT NULL
├── used_at TIMESTAMPTZ NULL
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

└── e2e_key_backups ([encryption.md](../features/encryption.md), Flyway V4 / golang-migrate 000005)
├── account_id UUID PRIMARY KEY -- logical ref → accounts.id
├── encrypted_blob TEXT NOT NULL
├── password_hint TEXT NULL
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Индексы v1:
- `UNIQUE INDEX accounts_email_uq ON accounts(email) WHERE email IS NOT NULL`
- `UNIQUE INDEX accounts_phone_uq ON accounts(phone) WHERE phone IS NOT NULL`
- `INDEX refresh_tokens_account_active_idx (account_id, expires_at DESC) WHERE revoked_at IS NULL`
- `INDEX refresh_tokens_token_hash_idx (token_hash)`
- `INDEX otp_codes_account_type_idx (account_id, type, expires_at DESC)`
- `INDEX backup_codes_account_active_idx (account_id) WHERE used_at IS NULL` (Flyway V2)
- `INDEX accounts_guest_last_online_idx (last_online_at) WHERE type = 'guest' AND status = 'active'` (Flyway V5)

`email_verified_at` and the verification-gated promotion are shipped for email
registration and `convert-guest` (PR #180): Auth creates a restricted pending
session, accepts bearer-scoped `POST /api/v1/auth/otp/send` and `otp/verify`, and
promotes the account to `regular` only after successful verification, with durable
recovery for the User sync and conversion event. Remaining client UX and live-provider
acceptance are tracked in [todo/client.md](../todo/client.md) and
[todo/backend.md](../todo/backend.md).

Правило статуса удаления:
- source of truth для логического удаления — `deleted_at`.
- `status='deleted'` должен выставляться синхронно с `deleted_at IS NOT NULL` (инвариант уровня приложения/триггера).
- `deleted_at` открывает 30-дневное recovery window; после него отдельный идемпотентный
  erasure job удаляет/pseudonymizes PII и credentials, после чего restore невозможен.
- Message history сохраняет только непривязанный к публичному профилю author tombstone;
  legal/anti-abuse retention хранится отдельно по production policy.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`user.events`** (совместно с User для событий профиля; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                      |
|-------------------------|-----------------------------|
| `user.registered`       | account_id, type, method    |
| `user.logged_in`        | account_id, device_info, ip |
| `user.logged_out`       | account_id, device_info     |
| `user.2fa_enabled`      | account_id                  |
| `user.guest_converted`  | account_id                  |
| `user.account_deleted`  | account_id; stable envelope `event_id` |
| `user.account_restored` | account_id                  |

## Зависимости

### Linked provider verification

`linked_identities` хранит единственную account/provider identity вместе с owning
`profile_id` и монотонной `source_revision`. Активную identity нельзя молча перенести на
другой профиль: V1 требует unlink → link. `verification_source_sync_targets` — durable
outbox-like state по account/profile/provider; target считается synced только после успешного
`UserService.ApplyVerificationSourceState`. Callback, unlink и scheduled refresh повторяют
pending targets. Provider outage сохраняет последнее verified state; подтверждённая потеря
критерия создаёт новую revoked revision. Owner-only list возвращает `profile_id`, чтобы клиент
показывал sources выбранного профиля.

- **User Service gRPC** (`USER_GRPC_ADDR`) — provisioning/resolve/switch профилей,
  синхронизация verification и завершения guest conversion. User единолично владеет `user_db`;
  недоступность, `DEADLINE_EXCEEDED` или непригодный ответ User блокирует выдачу новой сессии.
  Каждый blocking RPC (`EnsurePrimaryProfile`, `ResolvePrimaryProfileIDs`, `SwitchProfile`,
  `ApplyVerificationSourceState`, legacy `SetVerification` / `ClearVerification`,
  `MarkAccountRegular`) получает новый per-call deadline:
  `auth.user-grpc.deadline` / `AUTH_USER_GRPC_DEADLINE` — положительная ISO-8601 `Duration`.
  Если переменная **отсутствует**, используется `PT15S`; явные пустое, malformed, zero или negative
  значения являются ошибкой конфигурации и останавливают startup. Deadline создаётся при создании
  каждого `ClientCall`, а не один раз при создании Spring singleton stub.
- **Redis** — JWT blacklist (запись при logout и отзыве одного access token), OTP throttling и T056-P1 minimum-epoch floor без TTL. Auth записывает floor операцией `max` без TTL, а Gateway и Realtime в strict-режиме читают его fail-closed. Сквозные HTTP rate limits (в т.ч. лимит попыток входа с одного IP) — на **API Gateway**; те же лимиты вторым слоем в Auth не дублируем. Подробнее: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).
- **Resend** — отправка email (верификация, password reset)
- **NATS** — публикация событий

## Безопасность

- Пароли: bcrypt (cost 12)
- TOTP секреты: AES-256-GCM шифрование at rest; `AUTH_TOTP_ENCRYPTION_KEY` обязателен при `auth.persistence=jdbc` без `auth.totp.test-bypass` (иначе Auth не стартует; `DEFAULT_DEV_KEY` только memory/dev bypass)
- Refresh token: только хэш в БД, оригинал — только клиенту
- Нет SMS 2FA (v1) — только TOTP
- IP logging для аудита

### T056-P1: session epoch

`accounts.session_epoch` — положительный монотонный durable source of truth
Auth. Auth атомарно увеличивает его для отзыва всех сессий и никогда не
уменьшает. Каждый access JWT, выпущенный Auth, содержит положительный integer
claim `session_epoch`; `jti` остаётся per-session механизмом logout/отзыва.

Auth — единственный writer ключа `auth:session:min_epoch:<account_id>` со
значением положительного `int64` без TTL; запись выполняется только операцией
`max`. Перед каждым прямым выпуском JWT/session — registration (regular и guest),
login, refresh, OAuth exchange, 2FA reissue, profile switch, guest conversion
и restore — Auth подготавливает durable epoch относительно floor. Redis-ahead
значение reconcile-ится только вверх; ошибка floor, невалидное значение или
неуспешный reconcile дают fail-closed. При ошибке подготовки epoch
JDBC-транзакция create+prepare откатывается до User RPC; успешная транзакция
завершается до RPC. Memory profile этот rollback не моделирует.

При `auth.persistence=jdbc` startup Auth до запуска любого `SmartLifecycle`
постранично seed-ит и сверяет Redis floor с durable `accounts.session_epoch`.
`auth.session-epoch.seed.page-size` задаёт положительный размер страницы и по
умолчанию равен `256`. Hook зависит от инициализации БД, но не от конкретного
Flyway bean: он работает и с внешними миграциями, а недоступный Redis или
отсутствующая schema завершают startup ошибкой.

В Compose включены strict-проверки Gateway и Realtime: они отклоняют
missing/corrupt claim/floor и ошибку чтения Redis; Gateway проверяет non-empty
`jti` blacklist до floor, а Realtime — на upgrade, inbound operations и
outbound fan-out.

Compose strict proof не завершает rollout для всех окружений: требуется отдельная
operational acceptance. Immediate account-targeted close через Redis Pub/Sub пока
не реализован; authority остаются strict JWT/floor проверки.

## Space deletion proof and recovery (P3 Auth slice implemented)

This is a distinct `space_delete` family and never reuses ownership-transfer
tokens, rows or receipts. Public `IssueSpaceDeletionProof` binds account, active
profile, positive session epoch, canonical Space/operation IDs and the exact
UTF-8 confirmation name. Password is always verified; enabled 2FA requires
exactly one TOTP or unused backup code. The opaque proof is returned once; Auth
stores only its SHA-256, exact-name SHA-256, remaining bindings and security
revision. TTL is exactly five minutes and equality is expired; password/session/
2FA security changes revoke it.

Only signed Space workload identity may consume, look up or acknowledge a
deletion receipt. Consume is atomic and immutable; exact replay returns the same
receipt, while changed purpose or binding fails closed. Lookup requires account,
profile, epoch, Space, operation plus both name and proof digests, creates no
grant and reveals only an already committed receipt. Missing, unconsumed,
expired-before-consume, revoked and mismatched states all return coarse
`PERMISSION_DENIED`. Space stores exact receipt bytes/hash before acknowledgement;
missing lookup cannot abort an in-flight consume.

Unconsumed proof rows retain through `expires_at`. A consumed receipt awaiting
Space acknowledgement has no time-based deletion. An acknowledged receipt
retains until `max(consumed_at + 30 days, acknowledged_at + 24 hours)`. If
account erasure precedes acknowledgement, raw bindings become an Auth-keyed
HMAC-SHA-256 lookup index over `voice-auth-space-delete-receipt-v1`, NUL and
deterministic binding bytes while the immutable receipt remains recoverable.
Only Auth workload identity has that distinct key; staff and break-glass have no
access. Rotation is `P90D`, maximum restorable-backup age is `P30D`, missing key
fails closed, and destruction waits for every dependent receipt and backup.
This pseudonymous evidence is accepted as compatible with account erasure.

The Auth implementation is enabled with `auth.persistence=jdbc`. Flyway
`V13__space_deletion_proofs.sql` and the equivalent golang-migrate
`000014_space_deletion_proofs.up.sql` create the separate hash-only proof and
receipt store. The ordinary listener exposes only issue; consume, committed-only
lookup and acknowledgement are available only on the protected listener to a
verified `service:space` principal. Issue resolves its actor only from the
`Authorization` metadata of the current request; missing metadata fails with
`UNAUTHENTICATED` and never falls back to a remembered credential from another
request. Deployments must provide the dedicated `ReceiptErasureKeyring` before
account erasure can pseudonymize or recover receipts; an absent or missing
retained key fails closed.
