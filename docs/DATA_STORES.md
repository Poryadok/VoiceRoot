# Хранилища данных по микросервисам

Сводка по [MICROSERVICES.md](MICROSERVICES.md) и файлам `docs/microservices/*.md`. Принцип: **database per service** — каждая строка PostgreSQL — отдельная логическая БД (отдельная схема миграций у владельца). Общие правила идентификаторов, ссылок и полей: [DATA_MODEL.md](DATA_MODEL.md). Объём первой волны PostgreSQL по фичам: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).

**Не БД, но нужны в инфраструктуре:** NATS (шина событий, в локальном Compose — сервис **`nats`** с JetStream, см. [`docker-compose.yml`](../docker-compose.yml) и [DEPLOYMENT.md](DEPLOYMENT.md)), LiveKit (SFU для голоса/видео), объектное хранилище R2. Для локального dev их перечисляют в compose отдельно от Postgres/Redis/ClickHouse.

---

## Сводная таблица

| Сервис               | PostgreSQL        | Redis                     | Прочее                           |
|----------------------|-------------------|---------------------------|----------------------------------|
| API Gateway          | —                 | rate limit, JWT blacklist; session-epoch floor | —                  |
| Auth Service         | `auth_db`         | blacklist, session-epoch floor, principal replay, limits, OTP | —            |
| User Service         | `user_db` (profiles and immutable SDK author tombstones) | presence cache; Social and Auth principal replay | — |
| Social Service       | `social_db`       | —                         | `friend_accept_outbox` retries accepted-friend events; `friend_request_outbox` durably publishes friend invitations |
| Chat Service         | `chat_db`         | MatchSquad operation/receipt evidence (full request/receipt bytes retained until aggregate teardown completion can be proven); protected RPC replay guard in `chat:match-squad:principal:replay:<hex(SHA-256(issuer || NUL || JTI))>` | — |
| Messaging Service    | `messaging_db`    | —                         | NATS JetStream (publish)         |
| Realtime Service     | —                 | Pub/Sub, WS registry; session-epoch floor read/check | NATS (не БД)          |
| Space Service        | `space_db`        | Social principal replay   | —                                |
| Role Service         | `role_db`         | Shared principal replay Redis | —                                |
| Voice Service        | `voice_db`        | active-call compatibility projection + lifecycle admission/receipt mirror; `voice_match_squad_operations` and `voice_room_instances` own MatchSquad operation/resource state | Redis call/session projection; LiveKit room `match-squad-<room UUID>` |
| File Service         | `file_db`         | Shared principal replay Redis | R2, воркеры конвертации       |
| Notification Service | `notification_db` | grouping push, limits     | FCM, APNs, email                 |
| Search Service       | `search_db` (target) | —                      | Meilisearch v2, Elasticsearch v3 |
| Matchmaking Service  | `matchmaking_db`  | очереди, locks            | —                                |
| Moderation Service   | `moderation_db`   | —                         | —                                |
| Subscription Service | `subscription_db` | —                         | Paddle, CloudPayments            |
| Bot Service          | `bot_db`          | —                         | —                                |
| Game Integration Service | `game_integration_db` | — | Current app/env registry, credentials, installations, registry operations; target bindings, sessions, resource mappings, managed grants |
| Federation Service   | `federation_db` (planned, **not provisioned**) | —                         | Nodes, placements, snapshots, lease nonces, and append-only Q11 denial audit |
| Story Service        | `story_db`        | —                         | медиа через File, R2; durable archive-media deletion outbox |
| Analytics Service    | —                 | —                           | JetStream durable backlog + ClickHouse (`voice` DB) |

Разделение Redis между Gateway и Auth: [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).

### Game Integration Service (`game_integration_db`): T05 ownership target

This inventory describes the target owner boundary; it is not a shipment claim.
The current migrations implement registry/security, credentials, installations,
and registry operations. Session/resource mappings and managed grants remain
future GIS-owned records under T30/T31 and later community work.

| GIS record family | GIS-owned data | Logical references; owner keeps the resource |
|---|---|---|
| Applications, environments, installations, credentials | Registry state, policy/revision, credential verifier, installation lifecycle | Auth owns accounts and SDK identity/device keys; Bot owns Bot lifecycle. |
| Player/character bindings | App/environment-scoped binding state and opaque external game keys | Auth owns identity/device proof; User owns profiles/privacy; game owns external character facts. |
| Sessions/resource mappings | External key, session state, Chat/Voice/Space resource IDs | Chat owns `chat_db.chats`/membership; Voice owns `voice_db` room/lifecycle; Space owns `space_db` Space; Role owns effective permission state. |
| Desired managed grants | External origin/reason, subject reference, desired bounded permission and projection status | GIS owns desired intent; Role owns applied/effective permission. |
| Operations/retirement fences | Request hash, durable stage/result references, reconciliation state, retired external-key fence | Each domain service owns its side effect and local receipt. No direct cross-database SQL or cross-service FK. |

GIS schema changes belong in `src/backend/migrations/game_integration_db/`.
The target contract does not yet fix G04 operation/tombstone retention, G09 roster
freshness, or G12 app-visible alias/privacy details. Keep these explicit until
their owning decisions are recorded; do not infer retention durations from other
services.

### ClickHouse (Analytics Service)

| Компонент | Compose (`--profile app`) | Staging |
|-----------|---------------------------|---------|
| Сервис | `clickhouse` (`clickhouse/clickhouse-server:24.8`) | `voice-clickhouse` StatefulSet или managed endpoint |
| Init DDL | `docker/clickhouse/init/001_events.sql` | тот же SQL (job / idempotent apply) |
| HTTP / native | `8123` / `9000` | из Secret `CLICKHOUSE_DSN` |

Переменные окружения **Analytics** (`voice-analytics`):

| Переменная | Назначение |
|------------|------------|
| `CLICKHOUSE_DSN` | DSN для `clickhouse-go` (например `clickhouse://default@clickhouse:9000/voice`) |
| `NATS_URL` | JetStream consumers (domain streams + `analytics_events`) |
| `ANALYTICS_ID_HASH_KEY` | HMAC-соль для `account_id` / `profile_id` (не коммитить) |
| `ANALYTICS_BATCH_MAX_EVENTS` | Размер батча (default 1000) |
| `ANALYTICS_BATCH_FLUSH_INTERVAL` | Интервал flush (default `5s`) |

Для локального compose задайте `ANALYTICS_ID_HASH_KEY` в `.env` или используйте dev default из `docker-compose.yml`.

Для `API Gateway` канонично **нет service-owned PostgreSQL**. Политика версий клиента (`/api/v1/version`) может храниться либо в managed config store, либо в отдельной control-plane БД/таблице (`client_versions`) под владением Gateway как edge-политики; это не означает появление отдельной доменной БД Gateway в inventory.

### BE-116 Space audit durable ownership (accepted target; not implemented)

| Store owner | Durable responsibility |
|---|---|
| `space_db` | immutable audit registry, local mutation/audit/outbox atomicity, idempotent Role/Chat ingestion and Space-owned 365-day retention |
| `role_db` | role mutation and audit producer outbox in one transaction; stable event ID/payload until protected Space acknowledgement |
| `chat_db` | Space-attached chat mutation and audit producer outbox in one transaction; stable event ID/payload until protected Space acknowledgement |

No service writes another service database. Role and Chat deliver their outboxes
through signed `SpaceService.AppendAuditEvent`; Gateway is neither a producer nor
a persistence owner. These rows describe the accepted runtime dependency. The
proto is shipped, while the stores/workers remain unimplemented.

### `auth_db` (Auth Service)

Инвентарь таблиц — [auth-service.md](microservices/auth-service.md); миграции Flyway в `src/backend/auth/src/main/resources/db/migration/`.

Auth principal replay admission uses the existing Auth Redis connection:
`auth:principal:replay:<issuer>:<SHA-256-of-jti>`, written atomically with SET NX
and the credential's remaining lifetime as TTL. Missing/unavailable replay
admission fails closed; this ephemeral key does not replace durable ownership
proof receipts. Runtime configuration is documented in
[Auth README](../src/backend/auth/README.md#ownership-proof-principal-listener).

| Таблица | Примечание |
|---------|------------|
| `accounts` | учётная запись, 2FA, soft delete; `session_epoch` — durable Auth source |
| `ownership_transfer_proofs` | Auth-only hash, resource/factor binding and atomic durable transfer receipt (Flyway V12); `accounts.security_revision` revokes pending proofs |
| `refresh_tokens` | opaque refresh, rotation; nullable `profile_id` binds new sessions to the active profile so refresh preserves profile switches; `NULL` supports legacy rows |
| `otp_codes` | email verify / password reset |
| `e2e_key_backups` | [encryption.md](features/encryption.md) — client-encrypted key backup blob (`V4__e2e_key_backups.sql`) |

### `chat_db` (Chat Service)

Инвентарь ядра + backlog — [chat-service.md](microservices/chat-service.md) § «Модель данных»; scope matrix — [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md) §4.4 / §8.

| Таблица | Примечание |
|---------|------------|
| `chats`, `chat_members` | Shipped — DM/group/channel, `is_archived`, `inbox_bucket` |
| `folders`, `folder_chats` | **Shipped** — migrations `000008`/`000009`; folder CRUD and membership handlers are implemented |
| `quick_access_chats` | **Shipped** — migration `000010` (Batch 17) |
| `sticker_packs` | Catalog metadata (`is_system`, `is_premium`, `creator_profile_id`) — **0 code** |
| `stickers` | Rows per asset; `file_id` → File `intent=sticker` |
| `profile_installed_packs` | Per-profile install + composer rail `sort_order` |
| `chat_match_squad_operations` | One permanent match-owned row binds creation and teardown operation/receipt IDs and request/manifest hashes. Full request/receipt protobuf bytes remain retained while the aggregate teardown-completion trigger is unresolved; permanent operation digests/fences must remain. Migration DOWN refuses while pending, replay-required, or fence evidence exists. |

The Chat MatchSquad replay guard uses `chat:match-squad:principal:replay:<hex(SHA-256(issuer || NUL || JTI))>` with Redis `SET NX` and expiry bounded by token expiry. Its create/teardown request and receipt bytes currently remain durable; the provider has no aggregate teardown-completion signal, so no local compaction clock is applied. The compact terminal fence remains permanent.

Sticker/GIF bytes live in **`file_db`** (`files`); send payloads in **`messaging_db`** (`messages.content_type`). Do not duplicate catalog DDL outside Chat Service docs.

---

### `messaging_db` (Messaging Service): ListThreads successor (accepted target)

The bounded `ListThreads` successor keeps all projection state in `messaging_db` and has **no cross-database foreign keys**. Chat remains the membership authority and delivers durable membership events through an outbox; Messaging records consumed events idempotently.

| Durable data | Purpose |
|---|---|
| `thread_list_viewers` | Per `(chat_id, profile_id)` membership revision, `BUILDING|READY|REVOKED` state, baseline/applied journal sequence, immutable root IDs, high-water, revision and timestamps. |
| `thread_list_changes` | Ordered per-chat journal emitted in the same Messaging transaction as each root/reply/edit/delete/hide/ghost mutation. |
| immutable activity/id AVL nodes | Path-copied visible-thread trees, keyed by activity and parent ID; nodes carry exact count/latest reply/preview. |
| `thread_list_cursor_states` | Unexpired immutable roots and remaining-set page state, bound to chat/profile/page size/snapshot/original expiry/revision. |
| membership inbox | Durable Chat event deduplication and monotonic revision fence. |

The build worker retains journal entries through the minimum `BUILDING` baseline watermark. Cursor/node GC after 15 minutes preserves nodes reachable from READY heads or unexpired cursor states. This accepted target is not yet migrated or activated; execution details live in [listthreads-versioned-readmodel-exec-plan.md](testing/listthreads-versioned-readmodel-exec-plan.md).
### `voice_db` (Voice Service)

Voice owns `voice_room_instances`, `voice_room_memberships`,
`voice_lifecycle_operations`, `voice_lifecycle_effects`,
`voice_media_epoch_denials`, `voice_event_outbox`, and
`voice_lifecycle_redis_divergences`. PostgreSQL is the sole durable lifecycle
source. Divergence incidents are orthogonal evidence: completed receipts do not
regress, and Redis orphans remain representable without an operation row.

Migration `000004_game_session_rooms` adds ownerless `GAME_SESSION` rooms and
`voice_game_session_operations`. The latter stores the application/environment
resource mapping, deterministic request hash, Chat ID and Chat creation operation
ID, Voice room/receipt IDs and serialized immutable response. A transaction
creates the room and receipt together. Operation retries return the stored
response; operation/input divergence and attempts to remap one external resource
conflict. The Chat operation ID is the identity of Chat's durable create receipt;
there is no cross-service foreign key. A `GAME_SESSION` room cannot have an
owner, while `MATCH_SQUAD` still requires one.
Redis is a rebuildable, non-authoritative mirror; Voice stores no cross-service
foreign keys to profile/account owners.

### MatchSquad provider state

`voice_match_squad_operations` stores the exact create/teardown operation bytes,
receipts and bindings, with pending/active/closing/closed state and permanent
operation/match/resource/receipt fences. `voice_room_instances` remains the
canonical current room resource row. The Redis `call:<room UUID>` and
`session:<profile UUID>` keys are disposable room and participant projections;
LiveKit names the room `match-squad-<room UUID>`. Teardown records DB `closing`,
projects terminal Redis state and closes LiveKit, then commits DB `closed` and
its receipt. Full request/receipt bytes remain retained while the aggregate
teardown-completion signal is unresolved; the provider applies no local purge
clock. Pending evidence and permanent fences are not aged out.

The source-disabled A3 schema extension adds immutable room kind/purpose and
match creation bindings to `voice_room_instances`, plus verified account/session
epoch, lifecycle state and bounded reconnect interval to `voice_room_memberships`.
Legacy membership identity remains unknown; no backfill is inferred from Redis
or profile IDs. This is storage evidence, not an activated party snapshot source.

## Клиенты и админка

| Компонент           | Хранилище                                                  |
|---------------------|------------------------------------------------------------|
| Flutter-клиенты     | локальный кэш (SQLite/Hive), см. ARCHITECTURE_REQUIREMENTS |
| Admin Panel (React) | своей БД нет, только API к бэкендам                        |

---

## Подсчёт логических PostgreSQL БД

**17** current service PostgreSQL databases are listed above; **16** are provisioned in staging.
The staging manifests still include legacy `gateway_db` and lack `game_integration_db`; the local
Compose initializer also carries the planned `federation_db` scaffold. The game sprint in
[PLAN](PLAN.md) includes implementing its new authority store and deployment;
the old deferred-runtime description is superseded.

---

## Redis: один кластер или несколько

В документации зоны использования разные (Gateway, Auth, User presence, Realtime, Voice, Notification, Matchmaking). На старте обычно **один Redis** с разделением по ключам/префиксам; при росте — вынести Realtime / Matchmaking в отдельные инстансы по нагрузке.

For the Voice namespace, Redis loss is repairable from PostgreSQL only when no
open divergence incident blocks the operation. TTL expiry, flush, equality, or
manual Redis correction never resolves durable evidence automatically.

### Минимальный epoch сессии (T056-P1)

`auth_db.accounts.session_epoch` — durable source of truth, а Redis хранит
монотонный minimum-epoch floor для чтения Gateway и Realtime. Auth — единственный
writer: значение положительного `int64` не имеет TTL и обновляется только
операцией `max`; Redis не может понизить floor при сбое или откате Auth DB. В
strict-режиме ошибка чтения, отсутствие ключа или невалидное значение приводят к
fail-closed отказу. Compose strict proof не завершает rollout во всех окружениях;
немедленное account-targeted закрытие через Redis Pub/Sub не реализовано, а
authority остаётся за JWT/floor validation.

---

## Следующие шаги к модели данных

1. Скоуп v1 и трассировка фич → сервисы: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).
2. Таблицы и связи для волны v1: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md) и секции «Модель данных» в [microservices/](microservices/) (общие правила — [DATA_MODEL.md](DATA_MODEL.md)).
3. Миграции: один сервис — один набор миграций на свою БД; инструменты и порядок — [OPERATIONS.md](OPERATIONS.md#миграции-бд-database-per-service).


### File Story-media principal replay

File's protected Story-media listener uses its File-owned shared Redis
namespace `file:principal:replay:<sha256(issuer + NUL + jti)>` with SET NX and
the remaining credential lifetime. Redis/JWKS failure or a hard-expired JWKS
cache fails closed; this ephemeral replay key is not File-reference authority.

### Role ownership principal replay

Social privacy consumers also use shared Redis configured by
`USER_PRINCIPAL_REPLAY_REDIS_ADDR` and `SPACE_PRINCIPAL_REPLAY_REDIS_ADDR`.
Keys are `user:principal:replay:<sha256(issuer + NUL + jti)>` and
`space:principal:replay:<sha256(issuer + NUL + jti)>`. Each verified attempt
requires atomic SET NX with a relative TTL covering the remaining JWT lifetime,
rounded up to milliseconds. Redis failure denies the request before the domain
store; no process-local fallback replaces shared replay admission.

The dedicated ownership transport uses shared Redis configured by
`ROLE_PRINCIPAL_REPLAY_REDIS_ADDR`. Keys are
`role:principal:replay:<sha256(issuer + NUL + jti)>`; atomic create-if-absent rejects
replay across Role instances. Entries use the remaining lifetime to the verifier's credential `exp`, rounded
up to the next millisecond, as an atomic relative TTL; Redis wall-clock skew
cannot expire replay protection early.
Unavailable storage fails closed; it is not replaced by a process-local cache.
Role ownership operation receipts remain durable in `role_db` and implement
business idempotency independently of per-attempt JWT replay rejection.

## P3 Space lifecycle storage inventory (accepted target)

| Store owner | Additive durable data |
|---|---|
| `auth_db` | separate Space-deletion proofs, immutable consume receipts, persistence acknowledgement and purpose-specific post-erasure HMAC lookup index |
| `space_db` | lifecycle operation/state, immutable root/chat/File bindings, ten-participant ledger and receipts, leased outbox/inbox evidence, no-FK minimal deletion tombstone |
| `role_db` | permanent Space retirement fence, compact retirement receipt and bounded full request/receipt evidence |
| `chat_db` | lifecycle fence, immutable Space-chat manifest header/pages, operation/receipt evidence and permanent compact PURGED fence |
| `messaging_db` | lifecycle fence, imported Chat pages, Messaging File-reference producer pages, purge/release operation evidence and compact PURGED fence |
| `file_db` | `file_blobs`, exact `file_references`, subject-bound access capabilities, Space lifecycle fences, reference operations, producer declarations/chunks/seals and GC operations |
| `voice_db` | lifecycle fence/operation receipts and compact terminal fence; Redis remains a projection, never the durable deletion authority |
| Matchmaking/Search/Bot/Notification DBs | service-owned lifecycle fence, exact imported Chat pages where required, cleanup operation/receipt evidence and compact terminal fence |
| `subscription_db` | lifecycle fence/receipts, purged Space billing detail and permanent provider-event HMAC dedup fences |
| Moderation DB | atomic `TARGET_DELETED` resolution, sanction snapshot/detached report relation and deletion of Space report evidence |
| Analytics ClickHouse | existing 90-day raw HMAC events and de-identified aggregates; no raw deleted Space key |

Space's shared `space_lifecycle_operations` ledger admits distinct `DELETE` and
`RESTORE` operation IDs. Restore rows bind the original deletion ID and recovery
generation separately from the public restore ID; authenticated account/profile/
session epoch and request digest remain immutable. Restore has no proof or Auth
receipt columns populated. Its completed result retains the exact serialized
`RestoreSpaceResponse`, domain-separated SHA-256 and database completion time,
without a foreign key to Space rows, for the same 30-day replay window. Migration
rollback refuses to remove any admitted restore evidence.

Unconsumed Auth proof expires at five minutes; unacknowledged consumed receipt
does not time out; acknowledged receipt keeps through
`max(consumed + 30 days, acknowledged + 24 hours)`. A completed Space operation
keeps 30 days from `completed_at`; coordinator-held participant bytes keep 30
days after aggregate `PURGED`; each participant's own full request/receipt bytes
keep 30 days from that participant's completion. Delivered outbox rows keep 30
days from `delivered_at`, and processed inbox rows keep 30 days from
`processed_at`. Role retirement and participant PURGED fences are permanent.
The Space tombstone keeps 365 days from `purged_at`.

## A7 subscription lifecycle storage inventory (accepted target)

| Store owner | Additive durable data |
|---|---|
| `subscription_db` | permanent personal/Space aggregate revision, normalized provider outcome/order evidence, immutable provider-neutral lifecycle journal, leased event/reminder outbox, current-entitlement snapshot rows |
| `auth_db` | subscription event inbox and durable personal tier/revision/deadline projection used for JWT issuance |
| `user_db` | subscription event inbox/projection, downgrade pending selection and subscription-specific profile freeze provenance |
| `file_db` | immutable `retention_account_id` per exact reference, subscription event inbox/projection and revision-serialized retention deadlines |
| `space_db` | subscription event inbox and revisioned Space Pro projection including grace |
| `voice_db` | subscription event inbox and revisioned Space Pro admission projection; personal quality uses Auth's short-lived trusted entitlement claim |
| `notification_db` | subscription event inbox/projection, unique in-app reminder and leased push dispatch outbox |
| Analytics ClickHouse ingest | canonical event-ID dedupe and complete lifecycle reason mapping; no Analytics PostgreSQL database |

Account deletion adds an opaque `deletion_fence_id`, one-use cycle/`purge_at`, a
Subscription participant receipt and permanent purpose-scoped HMAC tombstone.
Every service schedules its own DB-time purge and erases raw
personal/purchaser identifiers and payload bytes at `P30D` even if another
participant is offline; receipts are asynchronous evidence, not a gate. This
applies to A7 stores and Analytics even when `P400D` has not
elapsed; recorded JetStream sequences carrying those raw IDs are deleted and
receipted. Offline/backup restore must apply the protected permanent purge-fence
snapshot before serving, then emit a late receipt. Only replay
hashes/IDs/revisions/terminal outcomes remain. The HMAC row
is excluded from raw entitlement snapshots and cannot recreate billing detail.
A still-paid Space aggregate remains under `space_id` with
`purchaser_deleted=true`; only its raw payer field is cleared, so period-end
convergence does not depend on the purged account identity.

An unconfirmed provider cancellation never delays privacy purge. A minimal
random-ID cancellation escrow may retain only provider + opaque subscription/
cancel handle, with no raw account/purchaser link or payment detail. Only the
cancellation worker identity can read it; terminal provider receipt immediately
crypto-shreds the handle, while the permanent HMAC fence retains the outcome.

For a pre-delete backup that knows only the old aggregate ID, an allowlisted
participant uses Subscription's non-logging batch legacy-key lookup. Subscription
computes purpose HMACs internally and returns only purge fence/cycle/time; a match
is locally erased before serving. No consumer receives HMAC keys or a reversible
raw-ID mapping.

These rows are target inventory, not a shipment claim. No consumer writes
`subscription_db`, and Subscription writes no consumer database. Stable event
bytes cross the boundary at least once; each local transaction commits its
effect before ACK. Bootstrap/restore uses the protected current-entitlement
snapshot because JetStream MaxAge is not source of truth. Details:
[subscription-lifecycle-convergence-exec-plan.md](testing/subscription-lifecycle-convergence-exec-plan.md).

Authoritative lifecycle journal rows and delivered event outbox rows retain
`P400D` after delivery; processed enforcement inbox IDs/hashes and canonical
Analytics dedupe rows retain at least the same `P400D`. Undelivered entitlement
rows are retained without deadline. Reminder schedule and notification dispatch
rows that miss their window become terminal `SUPPRESSED`/`EXPIRED`, not deleted;
their logical IDs/terminal outcomes also retain `P400D`, while verbose provider
attempt bodies may be redacted after `P30D`. Durable poison quarantine stores the
exact bytes/hash/error for `P400D`; after bounded delivery attempts the stream
message is terminated/moved to DLQ with an alert, never ACKed as successfully
applied. Billing/provider records keep their separate legal retention only in
the pseudonymized/minimized form allowed by the account-deletion contract; that
policy overrides generic `P400D` raw-payload retention.
