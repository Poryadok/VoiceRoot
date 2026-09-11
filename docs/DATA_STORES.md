# Хранилища данных по микросервисам

Сводка по [MICROSERVICES.md](MICROSERVICES.md) и файлам `docs/microservices/*.md`. Принцип: **database per service** — каждая строка PostgreSQL — отдельная логическая БД (отдельная схема миграций у владельца). Общие правила идентификаторов, ссылок и полей: [DATA_MODEL.md](DATA_MODEL.md). Объём первой волны PostgreSQL по фичам: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).

**Не БД, но нужны в инфраструктуре:** NATS (шина событий, в локальном Compose — сервис **`nats`** с JetStream, см. [`docker-compose.yml`](../docker-compose.yml) и [DEPLOYMENT.md](DEPLOYMENT.md)), LiveKit (SFU для голоса/видео), объектное хранилище R2. Для локального dev их перечисляют в compose отдельно от Postgres/Redis/ClickHouse.

---

## Сводная таблица

| Сервис               | PostgreSQL        | Redis                     | Прочее                           |
|----------------------|-------------------|---------------------------|----------------------------------|
| API Gateway          | —                 | rate limit, JWT blacklist; session-epoch floor | —                  |
| Auth Service         | `auth_db`         | blacklist, session-epoch floor, principal replay, limits, OTP | —            |
| User Service         | `user_db`         | presence cache            | —                                |
| Social Service       | `social_db`       | —                         | —                                |
| Chat Service         | `chat_db`         | —                         | —                                |
| Messaging Service    | `messaging_db`    | —                         | NATS JetStream (publish)         |
| Realtime Service     | —                 | Pub/Sub, WS registry; session-epoch floor read/check | NATS (не БД)          |
| Space Service        | `space_db`        | —                         | —                                |
| Role Service         | `role_db`         | Shared principal replay Redis | —                                |
| Voice Service        | `voice_db`        | активные сессии звонков (projection) | LiveKit                 |
| File Service         | `file_db`         | —                         | R2, воркеры конвертации          |
| Notification Service | `notification_db` | grouping push, limits     | FCM, APNs, email                 |
| Search Service       | `search_db` (target) | —                      | Meilisearch v2, Elasticsearch v3 |
| Matchmaking Service  | `matchmaking_db`  | очереди, locks            | —                                |
| Moderation Service   | `moderation_db`   | —                         | —                                |
| Subscription Service | `subscription_db` | —                         | Paddle, CloudPayments            |
| Bot Service          | `bot_db`          | —                         | —                                |
| Federation Service   | `federation_db` (planned, **not provisioned**) | —                         | —                                |
| Story Service        | `story_db`        | —                         | медиа через File, R2             |
| Analytics Service    | —                 | in-memory batch buffer      | ClickHouse (`voice` DB)          |

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

В документации зоны использования разные (Gateway, Auth, User presence, Realtime, Voice, Notification, Matchmaking, Analytics buffer). На старте обычно **один Redis** с разделением по ключам/префиксам; при росте — вынести Realtime / Matchmaking в отдельные инстансы по нагрузке.

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


### Role ownership principal replay

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
`max(consumed + 30 days, acknowledged + 24 hours)`. Completed Space operation,
participant full evidence, delivered outbox and processed inbox evidence keep
30 days at their specified terminal timestamps. Role retirement and participant
PURGED fences are permanent. The Space tombstone keeps 365 days from purge.
