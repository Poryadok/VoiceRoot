# Целевые схемы БД (полное приложение)

Здесь зафиксирована **целевая** структура каждой PostgreSQL-БД и ClickHouse для Analytics по состоянию на документацию [MICROSERVICES.md](../../MICROSERVICES.md), [DATA_STORES.md](../../DATA_STORES.md) и `docs/microservices/*.md`. Общие правила ID и ссылок между БД: [DATA_MODEL.md](../../DATA_MODEL.md).

**Не входят в этот каталог (нет PostgreSQL):** API Gateway (только Redis), Realtime (Redis + NATS), Voice (Redis + LiveKit).

**Волна MVP (урезанный скоуп):** [DATA_SCOPE_V1.md](../../DATA_SCOPE_V1.md) и файлы в [parent `../`](../README.md).

---

## Порядок генерации БД и микросервисов

Порядок отражает **логические зависимости** (какие UUID из чужих сервисов появляются в данных раньше), а не обязательный порядок разработки команд. **Между кластерами FK не строим** — только внутри одной БД.

| Шаг | Микросервис  | БД / хранилище            | Зависимости (логические)                              |
|-----|--------------|---------------------------|-------------------------------------------------------|
| 1   | Auth         | `auth_db`                 | —                                                     |
| 2   | User         | `user_db`                 | `account_id` из Auth                                  |
| 3   | Social       | `social_db`               | `profile_id` / `account_id`                           |
| 4   | Chat         | `chat_db`                 | `profile_id`                                          |
| 5   | Space        | `space_db`                | `profile_id`                                          |
| 6   | Role         | `role_db`                 | `space_id`, `channel_id` (текст → Chat, голос → Space) |
| 7   | Subscription | `subscription_db`         | `account_id`; `space_id` для Space Pro                |
| 8   | File         | `file_db`                 | `profile_id`; опционально `chat_id`                   |
| 9   | Messaging    | `messaging_db`            | `chat_id`, `profile_id`                               |
| 10  | Notification | `notification_db`         | `profile_id`                                          |
| 11  | Search       | `search_db`               | копия/проекция сообщений для индекса (v1: PostgreSQL) |
| 12  | Matchmaking  | `matchmaking_db`          | `profile_id`                                          |
| 13  | Moderation   | `moderation_db`           | цели в других сервисах по UUID                        |
| 14  | Bot          | `bot_db`                  | `account_id`, `channel_id`, `space_id`                |
| 15  | Federation   | `federation_db`           | `user_id` = account (master)                          |
| 16  | Story        | `story_db`                | `profile_id`, `file_id`                               |
| 17  | Analytics    | ClickHouse + Redis buffer | консьюмер событий из NATS                             |

Параллельно с шагами 4–16 могут развиваться независимые сервисы, но **миграции «первичного» наполнения справочников** (например игры в Matchmaking) удобно вводить после User.

---

## Файлы по БД

| БД                     | Документ                                           |
|------------------------|----------------------------------------------------|
| `auth_db`              | [auth_db.md](auth_db.md)                           |
| `user_db`              | [user_db.md](user_db.md)                           |
| `social_db`            | [social_db.md](social_db.md)                       |
| `chat_db`              | [chat_db.md](chat_db.md)                           |
| `space_db`             | [space_db.md](space_db.md)                         |
| `role_db`              | [role_db.md](role_db.md)                           |
| `subscription_db`      | [subscription_db.md](subscription_db.md)           |
| `file_db`              | [file_db.md](file_db.md)                           |
| `messaging_db`         | [messaging_db.md](messaging_db.md)                 |
| `notification_db`      | [notification_db.md](notification_db.md)           |
| `search_db`            | [search_db.md](search_db.md)                       |
| `matchmaking_db`       | [matchmaking_db.md](matchmaking_db.md)             |
| `moderation_db`        | [moderation_db.md](moderation_db.md)               |
| `bot_db`               | [bot_db.md](bot_db.md)                             |
| `federation_db`        | [federation_db.md](federation_db.md)               |
| `story_db`             | [story_db.md](story_db.md)                         |
| ClickHouse (Analytics) | [analytics_clickhouse.md](analytics_clickhouse.md) |

Миграции: [../README.md](../README.md#db-migrations).


