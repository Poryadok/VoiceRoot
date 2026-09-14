# Хранилища данных по микросервисам

Сводка по [MICROSERVICES.md](MICROSERVICES.md) и файлам `docs/microservices/*.md`. Принцип: **database per service** — каждая строка PostgreSQL — отдельная логическая БД (отдельная схема миграций у владельца). Общие правила идентификаторов, ссылок и полей: [DATA_MODEL.md](DATA_MODEL.md). Объём первой волны PostgreSQL по фичам: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).

**Не БД, но нужны в инфраструктуре:** NATS (шина событий, в локальном Compose — сервис **`nats`** с JetStream, см. [`docker-compose.yml`](../docker-compose.yml) и [DEPLOYMENT.md](DEPLOYMENT.md)), LiveKit (SFU для голоса/видео), объектное хранилище R2. Для локального dev их перечисляют в compose отдельно от Postgres/Redis/ClickHouse.

---

## Сводная таблица

| Сервис               | PostgreSQL        | Redis                     | Прочее                           |
|----------------------|-------------------|---------------------------|----------------------------------|
| API Gateway          | —                 | rate limit, JWT blacklist; session-epoch floor | —                  |
| Auth Service         | `auth_db`         | blacklist, session-epoch floor, principal replay, limits, OTP | —            |
| User Service         | `user_db`         | presence cache; Social principal replay | —                         |
| Social Service       | `social_db`       | —                         | —                                |
| Chat Service         | `chat_db`         | —                         | —                                |
| Messaging Service    | `messaging_db`    | —                         | NATS JetStream (publish)         |
| Realtime Service     | —                 | Pub/Sub, WS registry; session-epoch floor read/check | NATS (не БД)          |
| Space Service        | `space_db`        | Social principal replay   | —                                |
| Role Service         | `role_db`         | Shared principal replay Redis | —                                |
| Voice Service        | `voice_db`        | active-call compatibility projection + lifecycle admission/receipt mirror | LiveKit |
| File Service         | `file_db`         | Shared principal replay Redis | R2, воркеры конвертации       |
| Notification Service | `notification_db` | grouping push, limits     | FCM, APNs, email                 |
| Search Service       | `search_db` (target) | —                      | Meilisearch v2, Elasticsearch v3 |
| Matchmaking Service  | `matchmaking_db`  | очереди, locks            | —                                |
| Moderation Service   | `moderation_db`   | —                         | —                                |
| Subscription Service | `subscription_db` | —                         | Paddle, CloudPayments            |
| Bot Service          | `bot_db`          | —                         | —                                |
| Federation Service   | `federation_db` (planned, **not provisioned**) | —                         | —                                |
| Story Service        | `story_db`        | —                         | медиа через File, R2; durable archive-media deletion outbox |
| Analytics Service    | —                 | —                           | JetStream durable backlog + ClickHouse (`voice` DB) |

Разделение Redis между Gateway и Auth: [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).

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
| `refresh_tokens` | opaque refresh, rotation |
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
Redis is a rebuildable, non-authoritative mirror; Voice stores no cross-service
foreign keys to profile/account owners.

## Клиенты и админка

| Компонент           | Хранилище                                                  |
|---------------------|------------------------------------------------------------|
| Flutter-клиенты     | локальный кэш (SQLite/Hive), см. ARCHITECTURE_REQUIREMENTS |
| Admin Panel (React) | своей БД нет, только API к бэкендам                        |

---

## Подсчёт логических PostgreSQL БД

**17** planned PostgreSQL databases (see table above). **16** are provisioned by current deployment tooling, including the Voice Service `voice_db`. **`federation_db`** is documented for the deferred Federation Service but is **not** created in `docker/postgres/initdb.d/`, `deploy/templates/`, or migrate jobs until federation implementation starts ([PLAN.md](PLAN.md)).

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
