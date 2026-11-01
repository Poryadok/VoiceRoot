# User Service

## Space account-capture startup configuration

`USER_SPACE_CAPTURE_PRINCIPAL_*` transport settings are recognized by presence,
including an explicitly empty value. Any configured capture listener requires
an explicit nonempty `GRPC_LISTEN`, TLS certificate/key, client CA, replay Redis
address and JWKS client certificate/key. The optional fields are replay Redis
password and JWKS CA override; Space's HTTPS JWKS is selected from
`S2S_JWKS_URLS_JSON`. This reuses only the exact `user` audience and separate
`space-account-capture` capability. Shared verifier settings alone do not enable
capture; no default port or ordinary User listener supplies capture authority.

Partial or malformed transport settings fail startup before health or gRPC
admission. Entirely absent capture settings preserve the existing startup path.
Capture-only custody, identity or JWKS settings without the dedicated listener
also fail startup instead of silently selecting the legacy path.
When enabled, User also requires:

- A verified TLS `DATABASE_URL`, with no libpq `PG*` environment overrides;
  `USER_IDENTITY_KEY_MANIFEST` and `USER_IDENTITY_KEYS_DIR` for the complete
  retained identity keyset.
- `USER_CUSTODY_ADDR`, `USER_CUSTODY_TLS_CA_FILE`,
  `USER_CUSTODY_TLS_SERVER_NAME`, `USER_CUSTODY_TLS_CERT_FILE`,
  `USER_CUSTODY_TLS_KEY_FILE`, `USER_CUSTODY_SIGNING_KEYS_DIR`,
  `USER_CUSTODY_ACTIVE_KID` and `USER_CUSTODY_HEARTBEAT_TIMEOUT`.
- `USER_CUSTODY_EPOCH`, `USER_CUSTODY_SOURCE_ID`,
  `USER_CUSTODY_HISTORY_GENERATION`, `USER_CUSTODY_USER_SOURCE_GENERATION` and
  `USER_CUSTODY_KEYSET_DIGEST`, pinned from independent custody evidence.
- `USER_PRINCIPAL_JWKS_LISTEN`, `USER_PRINCIPAL_JWKS_TLS_CERT_FILE`,
  `USER_PRINCIPAL_JWKS_TLS_KEY_FILE`, and the `USER_IDENTITY_AUTH_*` protected
  Auth-parent transport settings.

User publishes its public JWKS before waiting up to five minutes for custody, then requires a
qualified v2 stream and verifies the actual database's system identifier,
database name/OID, identity generations, complete keyset and active guards in
one source transaction. It installs one guarded profile store for its service
and consumers and registers capture only on the dedicated Space principal
listener. Each capture takes its own held custody stream and repeats the
physical check inside its actual User transaction. Startup never initializes a
source or activates guards.

The custody service starts closed. An optional operator endpoint requires all of
`CUSTODY_OPERATOR_LISTEN`, `CUSTODY_OPERATOR_TLS_CERT_FILE`,
`CUSTODY_OPERATOR_TLS_KEY_FILE`, `CUSTODY_OPERATOR_CLIENT_CA_FILE`,
`CUSTODY_OPERATOR_CLIENT_CERT_SHA256`,
`CUSTODY_OPERATOR_SIGNER_PUBLIC_KEY_FILE`, `CUSTODY_SOURCE_ADMIN_DATABASE_URL`,
`CUSTODY_SOURCE_DATABASE_URL`, `CUSTODY_USER_KEY_MANIFEST` and
`CUSTODY_USER_KEYS_DIR`. It uses mutual TLS and pins the client leaf certificate
to the configured lowercase SHA-256 digest; certificate rotation requires a
coordinated configuration update. `GET /challenge` exposes the
current leader's one-use challenge and `POST /open` accepts a short-lived
Ed25519-signed decision that binds separately retained source/restore, writer
deployment and Auth evidence digests. Authenticated `POST /close` closes admission, drains
outstanding permits, stops that leader, fences its source and writer sessions,
and exits the controller; a later opening needs recovery and a new decision.
Once started, fencing continues even if the operator disconnects. Orderly
operator-enabled server shutdown also attempts the same drain and physical
fence before releasing its database connections, and reports failure if the
fence cannot be completed. Forced termination cannot perform this cleanup.
The offline
`sourcecustody sign-open` command reads a
reviewed JSON decision on stdin and a PKCS#8 Ed25519 private key from
`CUSTODY_OPERATOR_SIGNER_PRIVATE_KEY_FILE`; it writes the signed decision to
stdout. Keep that signing key under the independent custodian's control.
`GuardCutover` must be explicitly signed to activate guards on a fenced,
previously inactive source; a previously active guard must not be reactivated.

The custodian must independently establish and preserve the asserted complete
writer deployment, Auth continuity and exclusive administrative, network,
restore and promotion custody. A signature, a database identifier or this
endpoint does not prove those facts. Before a new operator-enabled controller
takes leadership, it checks the recorded phase and, if opening was interrupted
or the previous incarnation was active, fences the recorded source and writer.
Recovery disables connections and drains source and writer sessions before
recording `RECOVERED_CLOSED`; a live leader prevents the recovery attempt.
The `recover-activation` command also performs this operation with explicit
physical-source and writer-role inputs. A new leader must receive a fresh
challenge and new signed decision. `RECOVERY_REQUIRED` alone does not authorize
reopening.
The optional, separately operated `watch-activation` command uses the same
explicit physical-source and writer-role inputs as `recover-activation`. After
checking an active leader and its source binding, it emits `WATCHING`, monitors
the leader lock, and holds that lock while it fences the source and writer after
leader loss. If a different leader takes the lock first, the monitor closes
admission and waits a bounded time for the lock without terminating that leader;
it reports failure if it cannot complete the fence. It exits after recovery or
on an error. If no leader is present at startup, or leadership is lost during
startup verification, it attempts recovery without emitting `WATCHING` when it
can acquire the leader lock and validate the recorded source.
On a normal stop after `WATCHING`, it attempts to terminate the same observed
leader session and fence the source before exiting. It does not terminate a
replacement leader, and reports failure if it cannot obtain the lock and fence
within the bounded shutdown period. Operators must allow time for this work;
forced termination cannot perform it.

The monitor does not gate opening and cannot fence if it is itself absent or
cannot reach either database. `WATCHING` is not proof of future supervision,
exclusive custody or source continuity. Independent supervision, a safe
admission gate, automatic fencing under monitor loss, and a complete deployed
Ban proof remain open; do not enable this path without those controls. See
[Backend gaps](../todo/backend.md#space).

For an existing version-22 identity source, `quarantine-existing` first fences
the supplied physical database. `check-existing-identity` then requires the
independent control/source system IDs, database name/OID, source ID and epoch,
`CUSTODY_HISTORY_GENERATION`, `CUSTODY_USER_SOURCE_GENERATION`,
`CUSTODY_KEYSET_DIGEST`, `CUSTODY_USER_KEY_MANIFEST` and
`CUSTODY_USER_KEYS_DIR`. It checks the retained identity state and publishes
only a `CLOSED` control record. It does not create lineage, prove currentness,
or authorize capture: a separately reviewed signed opening is still required.

## Notification primary lookup (BE-118 prerequisite)

The separate `notification-primary` capability exposes only the existing
read-only `ResolvePrimaryProfileIDs` RPC on an opt-in mTLS listener (`:9098`).
It accepts one canonical nonzero account UUID and requires verified issuer
`notification`, subject `service:notification`, audience `user`, exact RPC,
request ID and deterministic full-request hash with shared Redis replay denial.
Notification cannot use the ordinary listener's internal-header path; that
compatibility path retains the documented exact Auth caller for primary lookup.
The existing Notification presence capability gains no lookup method.

Configuration uses `USER_NOTIFICATION_PRIMARY_PRINCIPAL_*`: TLS certificate/key,
client CA, replay Redis, optional listen override and JWKS CA plus mutual JWKS
client certificate/key; `S2S_JWKS_URLS_JSON` supplies Notification's HTTPS JWKS.
Absent configuration leaves the listener disabled; partial configuration fails
startup. No deployment credentials or listener activation is included here.

The lookup reads User's existing primary/non-deleted row; frozen primary remains
canonical. Missing/no-primary/deleted accounts are omitted without creating a
profile. Reminder delivery must treat an omitted/unavailable lookup as an
unresolved recipient and retry only inside its window. This prerequisite does
not implement full grace delivery; see the [primary read ExecPlan](../testing/notification-primary-profile-read-exec-plan.md).

## Space admission profile eligibility (SPACE-ADMISSION-IDENTITY-v1)

`GetSpaceAdmissionProfileEligibility` is a separate Auth-only current-pair
read. Its dedicated mutual-TLS listener defaults to `:9097` and uses
`USER_ADMISSION_PRINCIPAL_*` settings, including replay Redis, TLS certificate/key,
client CA and JWKS client identity, plus Auth's HTTPS JWKS entry. Partial config
fails startup. The ordinary, Social and SDK listeners do not register this RPC;
the SDK caller allowlist remains unchanged.

The signed request binds exact account/profile IDs and a 32-byte observation
digest of Auth's full request. User echoes all three and reads its owned current
row's positive `sdk_eligibility_revision`, deleted and frozen flags. This reuses
the existing read-only store query, never the SDK wire method or SDK consent.
Guests and regular profiles work equally; no provisioning, switching, membership
grant, history/lifetime or terminal-erasure authority is added. Unknown/foreign
pairs deny; invalid revisions and storage failure are unavailable. Auth rejects
deleted/frozen profiles and validates the exact response binding before its
final account reread. Existing eligibility revision triggers remain authoritative
for relevant owner/deleted/frozen changes. Deployment/assembled admission and
acceptance require their separate version-bound proof.

## Owning authority source catalog

The authority source serves exact clean `user_db` versions **21** and **22**.
For version 21, migrations 20 (`profile_last_seen`) and 21 (pending organization-challenge cleanup and its
unique index) preserve the source's four relations, columns, constraints,
indexes, eleven triggers and five function bodies. The real pinned migrator
qualification compares their DDL before/after 19→21 and verifies the unchanged
authority revision and profile tuple. The authority projection and function hashes
remain unchanged; dirty, older and unknown versions still fail closed.

Migration19's permanent authority floors and inactive fences remain forward-only.
The maintenance oracle returns through the unrelated additive migrations to19,
then proves that its DOWN is refused and retained fences survive. The User
authority reader also accepts exact22 after checking the named relations,
triggers and function bodies in the generated catalog under its read transaction.
This check does not qualify the complete identity schema, constraints or indexes.
Privileged DDL outside the migration protocol is outside the lock guarantee.
A version hint does not grant authority: the locked transaction rechecks the
exact clean version. The
exact21 reader rejects identity-migration artifacts rather than treating a
relabelled identity schema as 21. This reader qualification does not activate
identity association or establish source custody, writer readiness, or
cross-service/runtime acceptance.

## Обзор

Управление профилями пользователей, настройками, приватностью и онлайн-статусом.

**Язык**: Go
**БД**: PostgreSQL `user_db`, Redis (presence cache, shared profile-mutation quotas)

## Ответственность

The additive [ListThreads profile-eligibility source contract](../testing/listthreads-versioned-readmodel-exec-plan.md#effective-membership-contract-v1)
requires per-profile transactional eligibility revisions/revocation counters,
durable negative/tombstone lookup and authoritative state/head validation for
signed caller `chat`. All deleted/disabled/frozen causes participate; presence
or switching profiles alone does not revoke another profile. Deleted/archived
profiles are ineligible viewers while domain data and other viewers' history
remain intact. This unimplemented target requires a separate protected API; it
does not widen Auth-only `GetSdkProfileEligibility` or reuse public `GetProfile`.
Account/session state remains Auth-owned.

- Профили пользователей (аватар, имя, био, баннер, статус)
- Мульти-профили (2 бесплатно / 5 Premium, независимые контакты и настройки)
- Проверка принадлежности и доступности профиля для переключения сессии в Auth
- Username система (`@username#1234`, Premium `@username`, верифицированные ✅, компании ®)
- Настройки приватности (3 пресета + гранулярный контроль по полям)
- Presence (online / idle / DND / invisible)
- Кастомный статус (Premium)
- Game detection статус (из десктоп-клиента)
- Last seen timestamp
- Onboarding state (шаги туториала)
- Управление настройками (язык, тема, уведомления)
- **Поиск профилей для добавления в друзья / открытия DM** ([friends.md](../features/friends.md)) — запрос к `user_db` (не Search Service и не `search_db`; см. [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md), таблица фич)

## Поиск профилей vs Search Service

| Канал | RPC | Когда |
|-------|-----|--------|
| **User Service** | `SearchProfiles` | [friends.md](../features/friends.md): подбор по нику/отображаемому имени из канонических данных профиля; учёт приватности и блокировок (совместно с Social при необходимости). |
| **Search Service** | `SearchUsers` / `SearchGlobal` | [search.md](../features/search.md): полнотекст и глобальный поиск по проекциям в `search_db` / внешнем движке — [search-service.md](search-service.md). |

Клиент для чеклиста «Поиск пользователей» в [PLAN.md](../PLAN.md) опирается на **`UserService.SearchProfiles`** (HTTP-префикс того же сервиса: `/api/v1/users/**` — [api-gateway.md](api-gateway.md), в т.ч. **presigned аватар:** `POST /api/v1/users/me/avatar/presigned-upload` → `CreateAvatarPresignedUpload`).

## API (gRPC)

```protobuf
service UserService {
  // Профили
  rpc EnsurePrimaryProfile(EnsurePrimaryProfileRequest) returns (EnsurePrimaryProfileResponse); // S2S Auth bootstrap; см. primary-profile-bootstrap.md
  rpc ResolveAccountIDForProfile(ResolveAccountIDForProfileRequest) returns (ResolveAccountIDForProfileResponse); // internal Messaging/Chat: profile_id -> account_id for DM lifecycle
  rpc GetDMPeerDisplayNames(GetDMPeerDisplayNamesRequest) returns (GetDMPeerDisplayNamesResponse); // internal Chat: title-only existing DM peer metadata
  rpc ResolvePrimaryProfileIDs(ResolvePrimaryProfileIDsRequest) returns (ResolvePrimaryProfileIDsResponse); // S2S Auth: batch account_id -> existing primary profile_id
  rpc MarkAccountRegular(MarkAccountRegularRequest) returns (MarkAccountRegularResponse); // S2S Auth: guest -> regular profile marker
  rpc GetSdkProfileEligibility(GetSdkProfileEligibilityRequest) returns (GetSdkProfileEligibilityResponse); // Auth-only signed listener: exact read-only SDK target profile state
  rpc RecordSdkAuthorTombstone(RecordSdkAuthorTombstoneRequest) returns (RecordSdkAuthorTombstoneResponse); // Auth-only signed listener: immutable SDK historical-author receipt
  rpc GetProfile(GetProfileRequest) returns (Profile);
  rpc GetProfiles(GetProfilesRequest) returns (ProfileList); // batch
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile);
  rpc CreateProfile(CreateProfileRequest) returns (Profile); // мульти-профиль
  rpc DeleteProfile(DeleteProfileRequest) returns (Empty);
  rpc SwitchProfile(SwitchProfileRequest) returns (SwitchProfileResponse); // проверка профиля; сессию заменяет Auth
  rpc ListMyProfiles(Empty) returns (ProfileList);
  rpc SearchProfiles(SearchProfilesRequest) returns (SearchProfilesResponse);

  // Приватность
  rpc GetPrivacySettings(GetPrivacyRequest) returns (PrivacySettings);
  rpc UpdatePrivacySettings(UpdatePrivacyRequest) returns (PrivacySettings);

  // Presence
  rpc UpdatePresence(UpdatePresenceRequest) returns (Empty);
  rpc GetPresence(GetPresenceRequest) returns (PresenceStatus);
  rpc GetBulkPresence(GetBulkPresenceRequest) returns (GetBulkPresenceResponse); // map profile_id -> PresenceStatus
  rpc GetNotificationRoutingPresence(GetNotificationRoutingPresenceRequest) returns (GetNotificationRoutingPresenceResponse); // Notification-only active-session boolean

  // Настройки
  rpc GetSettings(GetSettingsRequest) returns (UserSettings);
  rpc UpdateSettings(UpdateSettingsRequest) returns (UserSettings);

  // Onboarding
  rpc GetOnboardingState(Empty) returns (OnboardingState);
  rpc CompleteOnboardingStep(CompleteStepRequest) returns (OnboardingState);

  // Verification
  rpc GetVerificationStatus(GetVerificationRequest) returns (VerificationStatus);
  rpc ApplyVerificationSourceState(ApplyVerificationSourceStateRequest) returns (ApplyVerificationSourceStateResponse); // internal Auth; monotonic per-source CAS
}
```

Канон RPC и сообщений — репозиторий `protos/voice/user/v1/user.proto`.

### Owner-only verification challenge readback

`GetVerificationStatusResponse` preserves effective `verification_status` at
field 1 and adds optional `pending_organization_challenge` at field 2:
`domain`, exact `txt_record` (`voice-verify=` plus the persisted token), and
`expires_at` (`google.protobuf.Timestamp`, PostgreSQL `created_at + 24 hours`).
The caller must own the requested nondeleted profile. Missing credentials,
invalid profile ID, and foreign/unknown/deleted profiles retain the existing
Unauthenticated, InvalidArgument, and NotFound errors respectively. No current
unexpired pending request means successful absence of the optional message;
storage errors remain errors. Public Profile and VerificationStatus do not
expose challenge data.

The read checks ownership and pending token in one PostgreSQL statement snapshot
and uses the existing strict 24-hour lifetime predicate. It does not generate,
supersede, expire, consume, or verify requests, query DNS, or change badges,
sources, revisions, or events. Navigation/reload uses this GET, never Start.
This is an observation: concurrent Start or expiry can invalidate it before
Check, whose exact-token completion rules remain authoritative. Clients must
discard delayed responses after account/session/profile changes. After Start,
clients can reread GET for the current canonical challenge and expiry.

Organization account kind and its authoritative enforcement remain unresolved:
the feature requires an Organization account, while Auth currently models
`regular|guest`. This readback does not define that kind, infer eligibility from
badges/domain, or enable organization verification controls.

### Переключение профиля и сессия Auth

Несмотря на историческое имя, `UserService.SwitchProfile` проверяет целевой
профиль аккаунта и возвращает `SwitchProfileResponse.profile`. Чужой,
удалённый или замороженный профиль не допускается. RPC не выдаёт access/refresh
tokens, не отзывает прежний JWT и сам не меняет активную identity сессии.
В текущем runtime проверка также применяет действующий free-tier freeze;
её нельзя считать общей read-only операцией.

Заменой сессии владеет Auth: `SwitchActiveProfile` использует User-проверку,
затем в своей security transaction повторно проверяет прежний bearer,
отзывает его JTI и выдаёт новый `AuthSession` с выбранным `profile_id`.
Публичный `POST /api/v1/auth/switch-profile` проходит через Auth REST;
Gateway не подменяет его User RPC или Auth gRPC transcoding, сохраняя
нормализацию `device_info` на REST-пути. Детали session-контракта —
[Auth Service](auth-service.md#api-grpc).

`user.profile_switched` — необязательное событие User-проверки при разных
исходном и целевом профилях. Оно может быть опубликовано до завершения
Auth-транзакции, поэтому не подтверждает выдачу новой сессии или отзыв старой.
Клиент меняет session identity по успешному ответу Auth с новым `AuthSession`.
Имена существующих RPC и событий сохраняются; их разные границы не означают
два независимых способа переключить Auth-сессию.

### Internal Auth profile seam

`EnsurePrimaryProfile`, `ResolvePrimaryProfileIDs` и `MarkAccountRegular` доступны только internal callers; они не являются Gateway REST API. `ResolvePrimaryProfileIDs` — read-only batch lookup: возвращает только существующие non-deleted primary profiles, пропускает unknown/no-primary/deleted записи и не создаёт профиль. Frozen primary остаётся каноническим и возвращается. После successful guest→regular conversion в Auth `MarkAccountRegular` снимает `is_guest_account` у всех профилей account, включая soft-deleted; повторный вызов или неизвестный account успешен без изменений.

SDK conversion uses two additional Auth-only methods on a dedicated signed-principal listener at `:9094`; they are not registered on the ordinary User gRPC listener or exposed as Gateway routes. The service accepts only issuer `auth`, audience `user`, subject `service:auth`, and the exact RPC in the signed principal. `request_hash` binds the full deterministic protobuf request as `sha256:<lowercase SHA-256 hex>`; each call also has a unique `request_id`. Service principals omit account/profile IDs and use session epoch zero. Redis replay protection is shared across User instances, and an incomplete listener configuration fails closed.

`GetSdkProfileEligibility` performs a read-only lookup of the exact `(account_id, profile_id)` pair. It returns that pair, the positive `profile_revision`, and the `deleted` and `frozen` flags; it never creates a profile. `RecordSdkAuthorTombstone` records an immutable historical SDK author alias without changing message authors or granting history. Its version-1 `request_hash` is lowercase SHA-256 hex over compact UTF-8 canonical JSON excluding `request_hash`, with keys sorted lexicographically: `expected_profile_revision`, `freeze_receipt_id`, `frozen_authority_epoch`, `frozen_binding_id`, `operation_id`, `source_account_id`, `source_actor_id`, `target_account_id`, `target_profile_id`, `version`. UUID values are canonical lowercase non-nil RFC 4122 strings; integer values use base-10 notation. A retry with the same operation and body returns the original receipt; a changed body or reused source actor conflicts. PostgreSQL rejects update, delete, and truncate of committed receipts.

`ResolveAccountIDForProfile` — отдельный read-only internal lookup только для exact caller `messaging` или `chat`: возвращает только `account_id` владельца указанного `profile_id`, включая soft-deleted profile. Messaging использует его для lifecycle account check до DM write; Chat — для fresh `ListChats` snapshot, чтобы убрать DM с deleted peer. Он не возвращает `Profile`, не применяет public visibility/block filters и не имеет Gateway REST route. Missing, wrong, padded или multiple internal caller metadata отвергается; invalid UUID — `INVALID_ARGUMENT`, unknown profile — `NOT_FOUND`, ошибка store — `INTERNAL`.

`GetDMPeerDisplayNames` — отдельный title-only internal lookup только для единственного exact caller `chat`. Chat передаёт лишь профили собеседников, уже полученные из `DMPeerProfileIDs` для авторизованных строк `ListChats`; User возвращает только `profile_id` и `display_name` активных профилей. Метод не возвращает account IDs, avatar, bio или verification data, ограничивает запрос 100 ID и не имеет Gateway REST route.

`ApplyVerificationSourceState` — internal-only Auth seam. Он принимает `twitch` или
`youtube`, положительную монотонную revision и применяет только более новое состояние
source. Public `GetVerificationStatus` возвращает итоговый status/badge и не раскрывает
provider source rows.

**`PrivacySettings` V1 contract (implemented in proto, DDL and User store):**

```protobuf
message PrivacySettings {
  string profile_id = 1;
  PrivacyPreset preset = 2;
  PrivacyAudience show_online = 3;
  // Additive field 21 in the canonical proto; independent from show_online.
  PrivacyAudience show_last_seen = 21;
  optional bool show_read_receipts = 20; // DM ✓✓ opt-out; default true — [privacy.md](../features/privacy.md) § Read receipts
  // … allow_dm, allow_friend_requests, allow_forward, allow_guest_dm, …
}
```

Read-path enforcement for `show_last_seen` — § «Heartbeat» below; feature UX — [presence.md](../features/presence.md), [privacy.md](../features/privacy.md).

**Rollout `show_last_seen`:** migration `000013` is an expand migration. Its
column remains nullable while old User Service binaries can still INSERT/UPSERT
`privacy_settings` without it. A new reader treats only NULL as the documented
preset-specific default (`personal` friends, `gaming` everyone, `work` space
members); an explicit JSON audience is authoritative. Contracting nullability
requires a later release after all writers send the field.

## Модель данных

### Social privacy principal boundary

The dedicated TLS listener (9091) registers only `GetPrivacySettings`,
`GetProfile`, `GetProfiles` and `ListProfileIDsForAccount`; each requires a verified
request-bound `service:social` credential and shared Redis replay admission
before store access. The domain entrypoint verifies the Social identity and
exact RPC/request binding again. On ordinary 9090, a raw Social caller marker
is rejected and Social has no ordinary path for these lookups. The protected
`GetProfile` retains its public/viewerless projection; the protected account
lookup returns only profile IDs for Social's account-level block cascade.
The protected `GetProfiles` batch is limited to 500 profile IDs per request
and returns the existing profile projection for Social's ownership resolution;
the ordinary public listener and profile privacy projection are unchanged.
Configuration and rotation: [DEPLOYMENT.md](../DEPLOYMENT.md#social-privacy-principals).

### File retention-owner principal boundary

File reaches `ResolveAccountIDForProfile` only through the separate TLS listener
on `USER_FILE_PRINCIPAL_GRPC_LISTEN` (default `:9092`). It admits only issuer
`file`, audience `file`, and the exact request-bound resolver RPC; no ordinary
9090 caller and no Social principal receives this authority. Activation needs
the complete `USER_FILE_PRINCIPAL_TLS_CERT_FILE`,
`USER_FILE_PRINCIPAL_TLS_KEY_FILE`, and
`USER_FILE_PRINCIPAL_REPLAY_REDIS_ADDR` set plus the `file` HTTPS entry in
`S2S_JWKS_URLS_JSON`; any explicit partial File configuration fails startup.

### Notification routing-presence principal boundary

Notification reaches `GetNotificationRoutingPresence` only on the separate
TLS listener at `:9095`. The listener accepts the exact request-bound
`service:notification` principal with audience `user`; the RPC request hash
binds the full deterministic protobuf request and a unique request ID. It
returns only `has_active_session`, never status text, last-seen, game or call
metadata. The ordinary User gRPC service and public/viewer-filtered presence
methods do not register this internal RPC. With no listener configuration it
remains absent; a partial configuration fails startup. A presence/store error
is returned to Notification rather than converted into an offline result.

The boolean is true for live `online`, `idle` and `dnd` sessions. Explicit
`offline` and `invisible` are false for routing purposes. A live snapshot with
an unknown status is unavailable, not offline, so Notification does not send a
push from an unrecognized state.

### Messaging scheduled-delivery presence principal boundary

Messaging reaches `GetScheduledMessageDispatchPresence` only through the
separate signed-principal mTLS listener configured by
`USER_MESSAGING_PRINCIPAL_TLS_CERT_FILE`,
`USER_MESSAGING_PRINCIPAL_TLS_KEY_FILE`, and
`USER_MESSAGING_PRINCIPAL_CLIENT_CA_FILE` (the trusted client certificate
authority bundle), and `USER_MESSAGING_PRINCIPAL_REPLAY_REDIS_ADDR`; the optional listener address is
`USER_MESSAGING_PRINCIPAL_GRPC_LISTEN` (default `:9096`). The listener admits
only issuer `messaging`, audience `user`, and this exact method. Its signed
request binds the claimed schedule ID and positive persisted dispatch
generation, sender account/profile, authoritative Chat type, and closed `at`
or `when_online` mode. A recipient profile is present only for a DM and is
derived by Messaging from current authoritative membership. The request cannot
be used as a general presence or profile lookup. The ordinary User listener
does not implement this method.

The User `messaging` principal runtime also fetches Messaging's principal JWKS
over mTLS. Its outbound identity is the separate pair
`USER_PRINCIPAL_JWKS_CLIENT_CERT_FILE` and
`USER_PRINCIPAL_JWKS_CLIENT_KEY_FILE`; these are not the inbound User listener
certificate/key above. The Messaging JWKS listener's client-CA configuration
must trust this User client identity. User continues to verify the JWKS server
using the configured `S2S_JWKS_CA_FILE` and the HTTPS endpoint hostname.

User verifies that the sender profile still belongs to the bound account and
is active, then returns its persisted `sender_is_guest` fact. For non-DM `at`
dispatch, `user_policy_allows_dispatch` means only this sender check passed;
Messaging remains responsible for Chat, Space, Role, moderation, and all
other delivery guards. For a DM, both modes additionally apply current DM
audience, guest-DM, and block policy. `at` does not consult presence or
`show_online`; `when_online` additionally checks current `show_online`
visibility and the peer's live status, with `recipient_online` true only for
exact `online`. `idle`, `dnd`, offline, invisible, or hidden peers are not
reported online. A missing/deleted peer or denied DM policy returns a false
User policy result. An unknown live state or unavailable authority, privacy,
block, or presence dependency returns an error so Messaging retains and
retries the durable schedule rather than interpreting uncertainty as
offline or eligible. Neither mode replaces Messaging's ordinary send-time
guards.

```
profiles
├── id (UUID)
├── account_id (UUID, logical ref → auth_db.accounts.id; без меж-БД REFERENCES)
├── username (string)
├── discriminator (string, 4 digits)
├── display_name
├── avatar_url
├── banner_url
├── bio (text, 500 chars)
├── custom_status (text, nullable — Premium; durable profile metadata, not Redis presence)
├── locale (en | ru)
├── theme (light | dark | high_contrast)
├── is_primary (bool)
├── verification_type (none | personal | organization)
├── verification_badge (nullable)
├── created_at
└── updated_at

privacy_settings
├── profile_id (UUID, logical ref → profiles.id)
├── preset (personal | gaming | work)
├── show_online (everyone | friends | nobody)
├── show_last_seen (everyone | friends | nobody) — «был(а) N назад»; independent from live online status
├── show_game_status (everyone | friends | nobody)
├── show_mm_rating (everyone | friends | nobody)
├── show_phone (friends | nobody)
├── show_stories (everyone | friends | nobody)
├── allow_dm (everyone | friends | friends_of_friends | nobody)
├── allow_friend_requests (everyone | friends_of_friends | nobody)
├── allow_guest_dm (bool)
└── updated_at

presence (Redis Hash — ephemeral)
├── profile_id → { status, game, custom_status, call_info, ts_unix }
└── TTL ~5 min; refreshed on heartbeat / WS activity

last_seen (Redis string — interim durable)
├── key: voice:user:last_seen:{profile_id}
├── value: unix timestamp (UTC)
└── TTL 30 days; compatibility cache refreshed on every heartbeat

profile_last_seen (PostgreSQL — durable)
├── profile_id (UUID PK, logical ref → profiles.id)
├── last_seen_at (TIMESTAMPTZ, UTC)
└── updated on session end, heartbeat boundary, explicit offline transition
```

**Interim vs target:**

| Store | Назначение | Статус |
|-------|------------|--------|
| **Redis session hash** | Live `status`, game, custom status, call | **Implemented** — TTL 5 min |
| **Redis `last_seen` string** | Interim «был(а) N назад» when session expired | **Implemented** — TTL 30 d; refreshed every heartbeat |
| **PostgreSQL `last_seen_at`** | Durable last seen for header / profile | **Implemented in source** — `user_db/000020`, monotonic heartbeat observations |

Redis-only interim **недостаточен** для long-tail «был 2 недели назад» (30 d cap) и privacy enforcement at scale — target is PG + read-time filter ([presence.md](../features/presence.md) § Redis cache and durable storage).

При реализации PG: запись `last_seen_at` при transition to offline / idle boundary / graceful disconnect; `GetBulkPresence` / `GetPresence` **merge** Redis live status + PG `last_seen_at` when offline; apply **`show_last_seen`** filter per viewer.

### Heartbeat (`UpdatePresence`)

| Step | Action |
|------|--------|
| 1 | Upsert session hash; EXPIRE 5 min |
| 2 | SET `last_seen` key = now unix; EXPIRE 30 d |
| 3 | If the status enum or game title changed → publish `user.presence_changed` with `old_status`, `new_status`; for the first live observation `old_status` is empty and `new_status` is the canonical current status. A game-only change uses the same current status in both fields, including when the title is cleared. Other same-enum heartbeats still complete steps 1–2 but publish nothing. |
| 4 | Realtime fan-out `presence_update` to friends/subscribers **after** privacy filter (spec) |

**`show_last_seen` enforcement:** when `show_last_seen = nobody` (or viewer is outside its allowed audience), `GetPresence` / `GetBulkPresence` **omit** `last_seen` timestamp (live online independently respects `show_online`). Self may read it; viewerless calls, missing privacy rows and audience dependency errors fail closed. Invisible is shown as offline to others and never leaks a hidden timestamp. Header «был(а)…» in DM — [presence.md](../features/presence.md). Durable PostgreSQL `last_seen_at` uses the same filtering path; delivery evidence is tracked in BE-050.

### Current code vs full spec

**Deployed migrations** используют `profiles`, `onboarding_state`,
`privacy_settings` и `profile_verification_sources`. V1 privacy settings are
implemented in proto/code; extended Premium fields remain a separate gap.

```
profiles
├── id UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── account_id UUID NOT NULL -- logical ref → auth_db.accounts.id
├── username VARCHAR(32) NOT NULL
├── discriminator CHAR(4) NOT NULL CHECK (discriminator ~ '^[0-9]{4}$')
├── display_name VARCHAR(64) NOT NULL
├── avatar_url TEXT NULL
├── bio TEXT NULL CHECK (char_length(bio) <= 500)
├── locale VARCHAR(8) NOT NULL DEFAULT 'ru' CHECK (locale IN ('ru','en'))
├── theme VARCHAR(32) NOT NULL DEFAULT 'dark' CHECK (theme IN ('light','dark','high_contrast'))
├── is_primary BOOLEAN NOT NULL DEFAULT true
├── verification_type VARCHAR(32) NOT NULL DEFAULT 'none' CHECK (verification_type IN ('none','personal','organization'))
├── verification_badge VARCHAR(32) NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

onboarding_state
├── profile_id UUID PRIMARY KEY -- logical ref → profiles.id
├── completed_steps JSONB NOT NULL DEFAULT '[]'::jsonb
├── completed BOOLEAN NOT NULL DEFAULT false
├── completed_at TIMESTAMPTZ NULL
├── created_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()

profile_verification_sources
├── profile_id UUID REFERENCES profiles(id)
├── source VARCHAR(32) -- twitch | youtube | organization_dns | legacy_personal
├── verification_type VARCHAR(32) -- personal | organization
├── badge VARCHAR(32) NOT NULL
├── verified BOOLEAN NOT NULL
├── revision BIGINT NOT NULL -- monotonic CAS per source
└── PRIMARY KEY (profile_id, source)
```

Resolver атомарно обновляет совместимые поля `profiles.verification_type` /
`verification_badge` с приоритетом `organization > personal > none`; Twitch и YouTube
образуют OR. Migration переносит существующий итоговый badge в source row и выравнивает
sequence, поэтому первый новый provider update не проигрывает backfill revision.

Индексы:
- `UNIQUE (username, discriminator)`
- `UNIQUE (account_id) WHERE is_primary = true` (один primary-профиль на account)
- `INDEX profiles_account_id_idx (account_id)`
- `INDEX profiles_created_at_idx (created_at DESC)`

**`last_seen_at`:** User-owned `profile_last_seen` (`user_db/000020`) stores only profile ID and UTC timestamp. Every heartbeat persists before Redis acknowledgement; `GREATEST` prevents concurrent/older observations from regressing time. Single/batch reads merge the newer PG/cache timestamp before existing viewer filtering. Durable-store errors fail the read/write; live routing still depends on Redis sessions. Expand first, then deploy the writer; rollback the binary before dropping the table. No historical timestamps are synthesized. Runtime/CI and independent review remain separate delivery evidence.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`user.events`** (совместно с Auth для событий учётной записи; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                                     |
|-------------------------|--------------------------------------------|
| `user.profile_created`  | profile_id, account_id                     |
| `user.profile_updated`  | profile_id, changed_fields                 |
| `user.profile_switched` | account_id, old_profile_id, new_profile_id |
| `user.presence_changed` | profile_id, **old_status**, **new_status** |
| `user.game_detected`    | profile_id, game_name                      |
| `user.settings_changed` | profile_id, changed_keys                   |
| `user.verified`         | profile_id, verification_type              |

**`user.presence_changed`:** the event is published for a status enum transition or game-title change in `UpdatePresence` (`user/internal/userevents/jetstream.go`): its first observation uses an empty `old_status`; a game-only update uses equal `old_status` / `new_status` so consumers can refresh the viewer-filtered snapshot. The legacy `status` value remains the current status for existing consumers, and the JetStream payload carries additive `old_status` / `new_status` fields.

## Зависимости

### Configured Space cold start

User constructs its configured `SPACE_GRPC_ADDR` client before handlers, but
waits for its bounded transport Ready state after starting fully initialized
User gRPC listeners. Every ordinary, dedicated and source listener denies RPC
admission until all startup prerequisites and listener starts succeed; HTTP
health cannot succeed before that release. Failure or cancellation revokes
admission and closes started resources. Transport readiness does not establish
identity, privacy, co-membership or other business authority: existing per-RPC
guards still apply after release and deny unavailable authority.

The staging `voice-user-peer` ClusterIP publishes unready User endpoints only on
ordinary port 9090. Only Space's `USER_GRPC_ADDR` uses it; User still waits on the
ordinary ready-gated Space Service. Ordinary Services and shared ConfigMap
addresses remain unchanged. This discovery edge is not authentication. Explicit
protected capture configuration still requires its dedicated mTLS endpoint and
complete issuer/runtime configuration, never ordinary-port fallback.

- **Auth Service** — account_id валидация
- **Subscription Service** — проверка лимитов (мульти-профили, кастомный статус)
- **Space Service** — `AreCoMembers` для проверки privacy-аудитории `space_members` (User подключает S2S-клиент через `SPACE_GRPC_ADDR`)
- **Redis** — presence кэш (TTL 5 мин, heartbeat) и обязательный общий допуск
  `CreateProfile`/`DeleteProfile` через существующий `USER_REDIS_ADDR`: независимо
  по 10 попыток/скользящий час/аккаунт. Нет локального fallback; отсутствие
  конфигурации или сбой Redis закрывает только эти операции с `UNAVAILABLE`.
  Квота и порядок отказа описаны в [Multi-Profile](../features/multi-profile.md#защита-от-злоупотреблений).
- **File Service** — загрузка аватара/баннера; **not yet deployed** as standalone service — минимальный R2/presigned для статичного аватара может жить в User ([user-profile.md](../features/user-profile.md), [PLAN.md](../PLAN.md))

## Subscription downgrade projection (A7 accepted target; not implemented)

User owns a durable personal-entitlement projection and consumer inbox keyed by
Subscription's aggregate revision. `GRACE_PERIOD` keeps all profiles usable.
During scheduled cancellation or grace, the primary ID reserves one slot and,
when an eligible secondary exists, the picker requires the exact pair of primary
plus one distinct owned, non-deleted profile that is not disabled or frozen for
another reason, without freezing yet. The selection is bound to
Subscription's stable `downgrade_cycle_id`, which survives only the direct
successor `INACTIVE` transition and an intervening failed-payment grace, but is
cleared by recovery/resume/renewal or a new `STARTED` activation; exact replay is
idempotent.
Retaining the primary ID never clears an independent disabled/frozen reason.

The first `INACTIVE` revision with more than two non-deleted profiles applies the
stored pair atomically; if absent/invalid, it uses primary plus the
earliest-created eligible secondary. With no eligible secondary it selects only
the primary slot and never revives unrelated disabled state. Every other
non-deleted profile receives a separate subscription-freeze overlay, including
one already disabled for another reason, so later removal of that reason cannot
exceed the free limit.
A selection from a cleared/different cycle after recovery is rejected without
mutation. If the current profile is not kept, Flutter refreshes
profile/token context to a kept profile before returning to the app.

Subscription-owned freeze provenance is stored separately from soft deletion,
moderation and other disabled states. Every later `ACTIVE` result, including a
new purchase/`STARTED`, unfreezes every and only subscription-frozen profile and
clears the old cycle/pending selection; it never
revives deleted or otherwise disabled data. Full replay/order and RED evidence is
in
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

## Search profile projection authority

Search obtains profile bootstrap snapshots and journal replay only through the
TLS listener on `:9093`. It requires Search's request-bound principal, replay
admission, the Search JWKS issuer entry and a dedicated
`USER_SEARCH_PROJECTION_CURSOR_HMAC_KEY`; missing or partial configuration
fails startup. The ordinary User listener never exposes these RPCs.

### Account deletion projection rollout

Auth remains the sole account-state authority. User records a durable
`ACCOUNT_INACTIVE` overlay from `user.account_deleted` and emits only
revisioned Search Delete records for every owned profile, including an already
soft-deleted profile. Account restore does not clear this overlay.

This PR ships the additive migration and a default-off consumer only:
`USER_ACCOUNT_DELETE_CONSUMER_ENABLED=false`. Setting it to `true` requires
`USER_ACCOUNT_DELETE_NATS_CREDS_FILE`; User creates a separate consumer
connection from that mounted credential and never grants it stream-management
rights.

PR B activation prerequisites are mandatory: broker credentials/ACLs grant
Auth publish access to the exact `user.account_deleted` subject and User only
subscribe/consumer access; the staging and production User migration job has
applied `000017_account_lifecycle_search_tombstone`; then deploy a canary before
enabling the consumer. No Compose manifest enables this consumer in PR A.

### Messaging profile deletion-state authority (BE-076)

GetProfileDeletionStates is a read-only Messaging-only method on the existing
USER_MESSAGING_PRINCIPAL_ mTLS listener (:9096). It requires the exact
request-bound service:messaging principal (iss=messaging, aud=user, exact
RPC/hash/request ID), shared Redis replay admission and the existing trusted
client certificate/JWKS identity configuration. Ordinary User, Social, Auth,
Notification and Gateway do not expose this authority.

A request contains 1..500 distinct canonical lowercase nonnil profile UUIDs.
An empty, oversized, duplicate or malformed batch returns INVALID_ARGUMENT
before store access. The response contains exactly one states entry per
requested ID in request order. PROFILE_DELETION_STATE_NOT_DELETED means the
User-owned row exists with deleted_at IS NULL; SOFT_DELETED means its
deleted_at IS NOT NULL; an absent row is explicitly UNKNOWN. Unspecified
is never emitted. Privacy, presence, frozen profiles and Auth account eligibility
(including ACCOUNT_INACTIVE) are not deletion facts. A storage failure returns
UNAVAILABLE, never a fabricated unknown/deleted result. Messaging compaction
removes views only for explicit SOFT_DELETED; unknown profiles retain views.
No cross-service SQL, public route, raw caller marker or fallback lookup exists.

### Catalog actor ownership/deletion snapshot

Chat and File use the separate internal `voice.catalogactor.v1.CatalogActorAuthorityService/GetCatalogActorStates`
service on the dedicated `USER_CATALOG_PRINCIPAL_` mTLS listener (default `:9098`).
It is absent from ordinary User registration and has no Gateway route. The existing
Messaging-only `GetProfileDeletionStates` API and listener remain unchanged.

Protocol version 1 accepts 1..100 exact `(account_id, profile_id)` pairs of canonical
lowercase nonnil UUIDs, with distinct profile IDs. Unknown protobuf fields, unsupported
versions and invalid/duplicate/oversized batches reject the entire request. The reply
has version 1 and exactly one ordered tuple echo per input. An exact ownership match
returns `NOT_DELETED` or `SOFT_DELETED` based only on User `profiles.deleted_at`;
frozen/account overlay state does not imply deletion. Missing profiles and wrong owners
return `UNKNOWN` without disclosing the actual owner. Storage failures fail the whole
request as unavailable. Clients reject unspecified/unrecognized enums, wrong version,
missing/extra/duplicate tuples and any unknown response fields.

Only pure `service:chat` and `service:file` principals are admitted: exact issuer,
audience `user`, full RPC, deterministic protobuf request SHA-256 binding, fresh
canonical UUID `x-request-id` and JTI replay protection are required. Caller-supplied
raw identity headers and delegated account/profile authority are forbidden. Activation
requires the complete prefix-owned `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`,
`REPLAY_REDIS_ADDR`, `JWKS_CLIENT_CERT_FILE` and `JWKS_CLIENT_KEY_FILE` settings,
plus both HTTPS chat/file entries in `S2S_JWKS_URLS_JSON`. Optional prefix-owned
`GRPC_LISTEN`, `REPLAY_REDIS_PASSWORD` and `JWKS_CA_FILE` follow the shared strict
listener rules; partial configuration fails startup. JWKS outbound identity is separate
from the listener certificate. Shared S2S settings alone never activate this listener.

Consumer outbound prefixes are `CHAT_CATALOG_ASSET_USER_` and `FILE_CATALOG_ASSET_USER_`
with `GRPC_ADDR`, `TLS_CA_FILE`, `TLS_SERVER_NAME`, `TLS_CLIENT_CERT_FILE` and
`TLS_CLIENT_KEY_FILE`; each consumer uses its own service signer. This read is a
snapshot, not an atomic permit, Auth account-eligibility proof, generation lease or
zero-lag cross-service deletion barrier. Consumers must combine fresh authority with
their own durable generation gates. UNKNOWN denies admission and never authorizes
permanent purge or restore. No profile-deleted event is introduced.
