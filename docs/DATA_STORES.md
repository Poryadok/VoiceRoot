# Хранилища данных по микросервисам

Сводка по [MICROSERVICES.md](MICROSERVICES.md) и файлам `docs/microservices/*.md`. Принцип: **database per service** — каждая строка PostgreSQL — отдельная логическая БД (отдельная схема миграций у владельца). Общие правила идентификаторов, ссылок и полей: [DATA_MODEL.md](DATA_MODEL.md). Объём первой волны PostgreSQL по фазам плана: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).

**Не БД, но нужны в инфраструктуре:** NATS (шина событий), LiveKit (SFU для голоса/видео), объектное хранилище R2. Для локального dev их перечисляют в compose отдельно от Postgres/Redis/ClickHouse.

---

## Сводная таблица

| Сервис                 | PostgreSQL      | Redis                                | Прочее                               |
|------------------------|-----------------|--------------------------------------|--------------------------------------|
| API Gateway            | —               | rate limit, JWT blacklist            | —                                    |
| Auth Service           | `auth_db`       | blacklist, limits, OTP               | —                                    |
| User Service           | `user_db`       | presence cache                       | —                                    |
| Social Service         | `social_db`     | —                                    | —                                    |
| Chat Service           | `chat_db`       | —                                    | —                                    |
| Messaging Service      | `messaging_db`  | —                                    | —                                    |
| Realtime Service       | —               | Pub/Sub, WS registry                 | NATS (не БД)                         |
| Space Service          | `space_db`      | —                                    | —                                    |
| Role Service           | `role_db`       | —                                    | —                                    |
| Voice Service          | —               | активные сессии звонков              | LiveKit                              |
| File Service           | `file_db`       | —                                    | R2, воркеры конвертации              |
| Notification Service   | `notification_db` | grouping push, limits             | FCM, APNs, email                     |
| Search Service         | `search_db` v1  | —                                    | Meilisearch v2, Elasticsearch v3     |
| Matchmaking Service    | `matchmaking_db` | очереди, locks                       | —                                    |
| Moderation Service     | `moderation_db` | —                                    | —                                    |
| Subscription Service   | `subscription_db` | —                                    | Paddle, CloudPayments                |
| Bot Service            | `bot_db`        | —                                    | —                                    |
| Federation Service     | `federation_db` | —                                    | —                                    |
| Story Service          | `story_db`      | —                                    | медиа через File, R2                 |
| Analytics Service      | —               | буфер батчей                         | ClickHouse                           |

Разделение Redis между Gateway и Auth: [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) («Redis: API Gateway и Auth Service»).

---

## Клиенты и админка

| Компонент           | Хранилище                                                |
|---------------------|----------------------------------------------------------|
| Flutter-клиенты     | локальный кэш (SQLite/Hive), см. ARCHITECTURE_REQUIREMENTS |
| Admin Panel (React) | своей БД нет, только API к бэкендам                      |

---

## Подсчёт логических PostgreSQL БД

**17** БД: `auth_db`, `user_db`, `social_db`, `chat_db`, `messaging_db`, `space_db`, `role_db`, `file_db`, `notification_db`, `search_db`, `matchmaking_db`, `moderation_db`, `subscription_db`, `bot_db`, `federation_db`, `story_db`.

---

## Redis: один кластер или несколько

В документации зоны использования разные (Gateway, Auth, User presence, Realtime, Voice, Notification, Matchmaking, Analytics buffer). На старте обычно **один Redis** с разделением по ключам/префиксам; при росте — вынести Realtime / Matchmaking в отдельные инстансы по нагрузке.

---

## Следующие шаги к модели данных

1. Скоуп v1 и трассировка фич → сервисы: [DATA_SCOPE_V1.md](DATA_SCOPE_V1.md).
2. Таблицы и связи для волны v1: [data/README.md](data/README.md) и `docs/data/*-service.md` (общие правила — [DATA_MODEL.md](DATA_MODEL.md)).
3. Миграции: один сервис — один набор миграций на свою БД ([OPERATIONS.md](OPERATIONS.md)); стек — [data/README.md](data/README.md#db-migrations).
