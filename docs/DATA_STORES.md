# Хранилища данных по микросервисам

Сводка по [MICROSERVICES.md](MICROSERVICES.md) и файлам `docs/microservices/*.md`. Принцип: **database per service** — каждая строка PostgreSQL — отдельная логическая БД (отдельная схема миграций у владельца). Общие правила идентификаторов, ссылок и полей: [DATA_MODEL.md](DATA_MODEL.md). Объём первой волны PostgreSQL по фичам: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).

**Не БД, но нужны в инфраструктуре:** NATS (шина событий, в локальном Compose — сервис **`nats`** с JetStream, см. [`docker-compose.yml`](../docker-compose.yml) и [DEPLOYMENT.md](DEPLOYMENT.md)), LiveKit (SFU для голоса/видео), объектное хранилище R2. Для локального dev их перечисляют в compose отдельно от Postgres/Redis/ClickHouse.

---

## Сводная таблица

| Сервис               | PostgreSQL        | Redis                     | Прочее                           |
|----------------------|-------------------|---------------------------|----------------------------------|
| API Gateway          | —                 | rate limit, JWT blacklist | —                                |
| Auth Service         | `auth_db`         | blacklist, limits, OTP    | —                                |
| User Service         | `user_db`         | presence cache            | —                                |
| Social Service       | `social_db`       | —                         | —                                |
| Chat Service         | `chat_db`         | —                         | —                                |
| Messaging Service    | `messaging_db`    | —                         | NATS JetStream (publish)         |
| Realtime Service     | —                 | Pub/Sub, WS registry      | NATS (не БД)                     |
| Space Service        | `space_db`        | —                         | —                                |
| Role Service         | `role_db`         | —                         | —                                |
| Voice Service        | —                 | активные сессии звонков   | LiveKit                          |
| File Service         | `file_db`         | —                         | R2, воркеры конвертации          |
| Notification Service | `notification_db` | grouping push, limits     | FCM, APNs, email                 |
| Search Service       | `search_db` (target) | —                      | Meilisearch v2, Elasticsearch v3 |
| Matchmaking Service  | `matchmaking_db`  | очереди, locks            | —                                |
| Moderation Service   | `moderation_db`   | —                         | —                                |
| Subscription Service | `subscription_db` | —                         | Paddle, CloudPayments            |
| Bot Service          | `bot_db`          | —                         | —                                |
| Federation Service   | `federation_db`   | —                         | —                                |
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

| Таблица | Примечание |
|---------|------------|
| `accounts` | учётная запись, 2FA, soft delete |
| `refresh_tokens` | opaque refresh, rotation |
| `otp_codes` | email verify / password reset |
| `e2e_key_backups` | [encryption.md](features/encryption.md) — client-encrypted key backup blob (`V4__e2e_key_backups.sql`) |

---

## Клиенты и админка

| Компонент           | Хранилище                                                  |
|---------------------|------------------------------------------------------------|
| Flutter-клиенты     | локальный кэш (SQLite/Hive), см. ARCHITECTURE_REQUIREMENTS |
| Admin Panel (React) | своей БД нет, только API к бэкендам                        |

---

## Подсчёт логических PostgreSQL БД

**16** БД: `auth_db`, `user_db`, `social_db`, `chat_db`, `messaging_db`, `space_db`, `role_db`, `file_db`, `notification_db`, `search_db`, `matchmaking_db`, `moderation_db`, `subscription_db`, `bot_db`, `federation_db`, `story_db`.

---

## Redis: один кластер или несколько

В документации зоны использования разные (Gateway, Auth, User presence, Realtime, Voice, Notification, Matchmaking, Analytics buffer). На старте обычно **один Redis** с разделением по ключам/префиксам; при росте — вынести Realtime / Matchmaking в отдельные инстансы по нагрузке.

---

## Следующие шаги к модели данных

1. Скоуп v1 и трассировка фич → сервисы: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).
2. Таблицы и связи для волны v1: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md) и секции «Модель данных» в [microservices/](microservices/) (общие правила — [DATA_MODEL.md](DATA_MODEL.md)).
3. Миграции: один сервис — один набор миграций на свою БД; инструменты и порядок — [OPERATIONS.md](OPERATIONS.md#миграции-бд-database-per-service).


