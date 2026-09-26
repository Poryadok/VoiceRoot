# User Service

## Обзор

Управление профилями пользователей, настройками, приватностью и онлайн-статусом.

**Язык**: Go
**БД**: PostgreSQL `user_db`, Redis (presence cache)

## Ответственность

- Профили пользователей (аватар, имя, био, баннер, статус)
- Мульти-профили (2 бесплатно / 5 Premium, независимые контакты и настройки)
- Переключение активного профиля
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
  rpc ResolvePrimaryProfileIDs(ResolvePrimaryProfileIDsRequest) returns (ResolvePrimaryProfileIDsResponse); // S2S Auth: batch account_id -> existing primary profile_id
  rpc MarkAccountRegular(MarkAccountRegularRequest) returns (MarkAccountRegularResponse); // S2S Auth: guest -> regular profile marker
  rpc GetSdkProfileEligibility(GetSdkProfileEligibilityRequest) returns (GetSdkProfileEligibilityResponse); // Auth-only signed listener: exact read-only SDK target profile state
  rpc RecordSdkAuthorTombstone(RecordSdkAuthorTombstoneRequest) returns (RecordSdkAuthorTombstoneResponse); // Auth-only signed listener: immutable SDK historical-author receipt
  rpc GetProfile(GetProfileRequest) returns (Profile);
  rpc GetProfiles(GetProfilesRequest) returns (ProfileList); // batch
  rpc UpdateProfile(UpdateProfileRequest) returns (Profile);
  rpc CreateProfile(CreateProfileRequest) returns (Profile); // мульти-профиль
  rpc DeleteProfile(DeleteProfileRequest) returns (Empty);
  rpc SwitchProfile(SwitchProfileRequest) returns (Profile);
  rpc ListMyProfiles(Empty) returns (ProfileList);
  rpc SearchProfiles(SearchProfilesRequest) returns (SearchProfilesResponse);

  // Приватность
  rpc GetPrivacySettings(GetPrivacyRequest) returns (PrivacySettings);
  rpc UpdatePrivacySettings(UpdatePrivacyRequest) returns (PrivacySettings);

  // Presence
  rpc UpdatePresence(UpdatePresenceRequest) returns (Empty);
  rpc GetPresence(GetPresenceRequest) returns (PresenceStatus);
  rpc GetBulkPresence(GetBulkPresenceRequest) returns (GetBulkPresenceResponse); // map profile_id -> PresenceStatus

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

### Internal Auth profile seam

`EnsurePrimaryProfile`, `ResolvePrimaryProfileIDs` и `MarkAccountRegular` доступны только internal callers; они не являются Gateway REST API. `ResolvePrimaryProfileIDs` — read-only batch lookup: возвращает только существующие non-deleted primary profiles, пропускает unknown/no-primary/deleted записи и не создаёт профиль. Frozen primary остаётся каноническим и возвращается. После successful guest→regular conversion в Auth `MarkAccountRegular` снимает `is_guest_account` у всех профилей account, включая soft-deleted; повторный вызов или неизвестный account успешен без изменений.

SDK conversion uses two additional Auth-only methods on a dedicated signed-principal listener at `:9094`; they are not registered on the ordinary User gRPC listener or exposed as Gateway routes. The service accepts only issuer `auth`, audience `user`, subject `service:auth`, and the exact RPC in the signed principal. `request_hash` binds the full deterministic protobuf request as `sha256:<lowercase SHA-256 hex>`; each call also has a unique `request_id`. Service principals omit account/profile IDs and use session epoch zero. Redis replay protection is shared across User instances, and an incomplete listener configuration fails closed.

`GetSdkProfileEligibility` performs a read-only lookup of the exact `(account_id, profile_id)` pair. It returns that pair, the positive `profile_revision`, and the `deleted` and `frozen` flags; it never creates a profile. `RecordSdkAuthorTombstone` records an immutable historical SDK author alias without changing message authors or granting history. Its version-1 `request_hash` is lowercase SHA-256 hex over compact UTF-8 canonical JSON excluding `request_hash`, with keys sorted lexicographically: `expected_profile_revision`, `freeze_receipt_id`, `frozen_authority_epoch`, `frozen_binding_id`, `operation_id`, `source_account_id`, `source_actor_id`, `target_account_id`, `target_profile_id`, `version`. UUID values are canonical lowercase non-nil RFC 4122 strings; integer values use base-10 notation. A retry with the same operation and body returns the original receipt; a changed body or reused source actor conflicts. PostgreSQL rejects update, delete, and truncate of committed receipts.

`ResolveAccountIDForProfile` — отдельный read-only internal lookup только для exact caller `messaging` или `chat`: возвращает только `account_id` владельца указанного `profile_id`, включая soft-deleted profile. Messaging использует его для lifecycle account check до DM write; Chat — для fresh `ListChats` snapshot, чтобы убрать DM с deleted peer. Он не возвращает `Profile`, не применяет public visibility/block filters и не имеет Gateway REST route. Missing, wrong, padded или multiple internal caller metadata отвергается; invalid UUID — `INVALID_ARGUMENT`, unknown profile — `NOT_FOUND`, ошибка store — `INTERNAL`.

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
`GetProfile` and `ListProfileIDsForAccount`; each requires a verified
request-bound `service:social` credential and shared Redis replay admission
before store access. The domain entrypoint verifies the Social identity and
exact RPC/request binding again. On ordinary 9090, a raw Social caller marker
is rejected and Social has no ordinary path for these lookups. The protected
`GetProfile` retains its public/viewerless projection; the protected account
lookup returns only profile IDs for Social's account-level block cascade.
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
└── TTL 30 days; refreshed on every heartbeat (until PG lands)

last_seen_at (PostgreSQL — target durable)
├── profile_id (UUID PK, logical ref → profiles.id)
├── last_seen_at (TIMESTAMPTZ, UTC)
└── updated on session end, heartbeat boundary, explicit offline transition
```

**Interim vs target:**

| Store | Назначение | Статус |
|-------|------------|--------|
| **Redis session hash** | Live `status`, game, custom status, call | **Implemented** — TTL 5 min |
| **Redis `last_seen` string** | Interim «был(а) N назад» when session expired | **Implemented** — TTL 30 d; refreshed every heartbeat |
| **PostgreSQL `last_seen_at`** | Durable last seen for header / profile | **Not yet** — no PG column |

Redis-only interim **недостаточен** для long-tail «был 2 недели назад» (30 d cap) и privacy enforcement at scale — target is PG + read-time filter ([presence.md](../features/presence.md) § Interim storage).

При реализации PG: запись `last_seen_at` при transition to offline / idle boundary / graceful disconnect; `GetBulkPresence` / `GetPresence` **merge** Redis live status + PG `last_seen_at` when offline; apply **`show_last_seen`** filter per viewer.

### Heartbeat (`UpdatePresence`)

| Step | Action |
|------|--------|
| 1 | Upsert session hash; EXPIRE 5 min |
| 2 | SET `last_seen` key = now unix; EXPIRE 30 d |
| 3 | If status enum changed → publish `user.presence_changed` with `old_status`, `new_status`; for the first live observation `old_status` is empty and `new_status` is the canonical current status. Same-enum heartbeats still complete steps 1–2 but publish nothing. |
| 4 | Realtime fan-out `presence_update` to friends/subscribers **after** privacy filter (spec) |

**`show_last_seen` enforcement:** when `show_last_seen = nobody` (or viewer is outside its allowed audience), `GetPresence` / `GetBulkPresence` **omit** `last_seen` timestamp (live online independently respects `show_online`). Self may read it; viewerless calls, missing privacy rows and audience dependency errors fail closed. Invisible is shown as offline to others and never leaks a hidden timestamp. Header «был(а)…» in DM — [presence.md](../features/presence.md). Durable PostgreSQL `last_seen_at` remains separate — [todo/backend.md](../todo/backend.md).

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

**`last_seen_at`:** spec — таблица в PostgreSQL (см. схема выше). **Code gap:** только Redis hash, без durable persistence — [todo/backend.md](../todo/backend.md).

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

**`user.presence_changed`:** the event is published for an enum transition in `UpdatePresence` (`user/internal/userevents/jetstream.go`): its first observation uses an empty `old_status`, and a same-enum heartbeat updates Redis activity but emits nothing. The legacy `status` value remains the current status for existing consumers, and the JetStream payload carries additive `old_status` / `new_status` fields.

## Зависимости

- **Auth Service** — account_id валидация
- **Subscription Service** — проверка лимитов (мульти-профили, кастомный статус)
- **Space Service** — `AreCoMembers` для проверки privacy-аудитории `space_members` (User подключает S2S-клиент через `SPACE_GRPC_ADDR`)
- **Redis** — presence кэш (TTL 5 мин, heartbeat)
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
