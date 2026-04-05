# `search_db` — целевая схема (PostgreSQL v1)

**Сервис:** Search ([search-service.md](../../microservices/search-service.md)). **Шаг порядка:** 11.

Целевая **v1**: отдельная БД Search — проекции для глобального поиска ([search.md](../../features/search.md)): сообщения (полнотекст), профили, чаты (DM/группы), спейсы. Источники истины: `messaging_db`, `user_db`, `chat_db`, `space_db`; обновление по событиям NATS / синхронным ingest. При переходе на Meilisearch/Elasticsearch таблицы ниже могут быть выведены или оставлены как fallback.

E2E-сообщения в индекс **не попадают** (`skip` при ingest).

---

## `message_search_documents`

Денормализованная строка на одно сообщение.

| Колонка             | Тип           | Описание                                  |
|---------------------|---------------|-------------------------------------------|
| `message_id`        | `UUID`        | PK                                        |
| `chat_id`           | `UUID`        | NOT NULL                                  |
| `chat_type`         | `TEXT`        | NOT NULL                                  |
| `sender_profile_id` | `UUID`        | NOT NULL                                  |
| `content`           | `TEXT`        | NOT NULL                                  |
| `search_vector`     | `tsvector`    | GENERATED STORED или обновление триггером |
| `created_at`        | `TIMESTAMPTZ` | NOT NULL                                  |
| `deleted_at`        | `TIMESTAMPTZ` | NULL — sync с soft delete Messaging       |

**Индексы:** `GIN (search_vector)`; `GIN (content gin_trgm_ops)` при `pg_trgm`; `(chat_id, created_at DESC)` для in-chat search.

---

## `profile_search_documents`

Глобальный поиск по контактам / @username ([search.md](../../features/search.md)): денормализованная строка на профиль.

| Колонка         | Тип           | Описание                                                             |
|-----------------|---------------|----------------------------------------------------------------------|
| `profile_id`    | `UUID`        | PK                                                                   |
| `account_id`    | `UUID`        | NOT NULL — для фильтрации «свой аккаунт» без лишних вызовов User     |
| `username`      | `TEXT`        | NOT NULL                                                             |
| `discriminator` | `TEXT`        | NOT NULL                                                             |
| `display_name`  | `TEXT`        | NOT NULL                                                             |
| `search_vector` | `tsvector`    | GENERATED или триггер по `username`, `discriminator`, `display_name` |
| `deleted_at`    | `TIMESTAMPTZ` | NULL — sync с `user_db.profiles`                                     |

**Индексы:** `GIN (search_vector)`; `GIN` по выражению `(username \\ '#' \\ discriminator) gin_trgm_ops` при `pg_trgm`.

---

## `chat_search_documents`

Секция «чаты» в глобальном поиске: подпись для отображения и текст для prefix / ILIKE.

| Колонка              | Тип           | Описание                                                                  |
|----------------------|---------------|---------------------------------------------------------------------------|
| `chat_id`            | `UUID`        | PK — только `chat_db.chats` (`dm` \ `group`)                              |
| `chat_type`          | `TEXT`        | NOT NULL                                                                  |
| `search_text`        | `TEXT`        | NOT NULL — имя группы или денормализованные ники участников DM для поиска |
| `member_profile_ids` | `UUID[]`      | NULL — опционально, для ACL фильтра «вижу только свои чаты»               |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL                                                                  |

**Индексы:** `GIN (search_text gin_trgm_ops)`; `(chat_id)`.

---

## `space_search_documents`

Секция «спейсы» в глобальном поиске.

| Колонка         | Тип           | Описание                                                                                  |
|-----------------|---------------|-------------------------------------------------------------------------------------------|
| `space_id`      | `UUID`        | PK                                                                                        |
| `name`          | `TEXT`        | NOT NULL                                                                                  |
| `visibility`    | `TEXT`        | NOT NULL — чтобы не индексировать `private` в публичный каталог или фильтровать в запросе |
| `member_count`  | `INT`         | NOT NULL, DEFAULT 0                                                                       |
| `search_vector` | `tsvector`    | по `name` (и при необходимости короткому excerpt из описания)                             |
| `updated_at`    | `TIMESTAMPTZ` | NOT NULL                                                                                  |

**Индексы:** `GIN (search_vector)`; `GIN (name gin_trgm_ops)`.

---

## Расширения PostgreSQL

- `CREATE EXTENSION IF NOT EXISTS pg_trgm;` — по [search-service.md](../../microservices/search-service.md).

---

## v2+

Индексы Meilisearch / Elasticsearch не описываются строками SQL здесь; консьюмер NATS пишет во внешний движок. Таблицы `message_search_documents`, `profile_search_documents`, `chat_search_documents`, `space_search_documents` могут сохраняться для гибридного режима или быть вытеснены внешним движком по матрице из [ARCHITECTURE_REQUIREMENTS.md](../../ARCHITECTURE_REQUIREMENTS.md).


