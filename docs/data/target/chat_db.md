# `chat_db` — целевая схема

**Сервис:** Chat ([chat-service.md](../../microservices/chat-service.md)). **Шаг порядка:** 4.

`profile_id` → User; `creator_profile_id` — то же (**без FK**).

### Каналы спейса и сообщения

Сообщения в **текстовых каналах** хранятся в **Messaging** (`messaging_db`), а не в этой БД. Идентификатор потока сообщений канала — **`channels.id`** из `space_db`; в Messaging/File это поле называется **`chat_id`** при `chat_type = channel`. Строки в таблице `chats` ниже — только **DM и группы**. Подробнее: [DATA_MODEL.md](../../DATA_MODEL.md) (раздел про `chat_type = channel`).

---

## `chats`

| Колонка              | Тип           | Описание            |
|----------------------|---------------|---------------------|
| `id`                 | `UUID`        | PK                  |
| `type`               | `TEXT`        | `dm` \ `group`      |
| `name`               | `TEXT`        | NULL                |
| `avatar_url`         | `TEXT`        | NULL                |
| `creator_profile_id` | `UUID`        | NOT NULL            |
| `slow_mode_seconds`  | `INT`         | NOT NULL, DEFAULT 0 |
| `last_message_at`    | `TIMESTAMPTZ` | NULL                |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL            |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL            |

**Индексы:** `(last_message_at DESC NULLS LAST)`.

---

## `chat_members`

| Колонка       | Тип           | Описание                                     |
|---------------|---------------|----------------------------------------------|
| `chat_id`     | `UUID`        | NOT NULL, FK → `chats(id)` ON DELETE CASCADE |
| `profile_id`  | `UUID`        | NOT NULL                                     |
| `role`        | `TEXT`        | `owner` \ `admin` \ `member`                 |
| `joined_at`   | `TIMESTAMPTZ` | NOT NULL                                     |
| `muted_until` | `TIMESTAMPTZ` | NULL                                         |
| `is_archived` | `BOOLEAN`     | NOT NULL, DEFAULT false                      |

**Индексы / ограничения:** `PRIMARY KEY (chat_id, profile_id)`; `INDEX (profile_id)`.

---

## `folders`

| Колонка         | Тип           | Описание            |
|-----------------|---------------|---------------------|
| `id`            | `UUID`        | PK                  |
| `profile_id`    | `UUID`        | NOT NULL            |
| `name`          | `TEXT`        | NOT NULL            |
| `type`          | `TEXT`        | `system` \ `custom` |
| `filter_config` | `JSONB`       | NULL                |
| `sort_order`    | `INT`         | NOT NULL, DEFAULT 0 |
| `created_at`    | `TIMESTAMPTZ` | NOT NULL            |

**Индексы:** `(profile_id, sort_order)`.

---

## `folder_chats`

| Колонка     | Тип           | Описание                                       |
|-------------|---------------|------------------------------------------------|
| `folder_id` | `UUID`        | NOT NULL, FK → `folders(id)` ON DELETE CASCADE |
| `chat_id`   | `UUID`        | NOT NULL, FK → `chats(id)` ON DELETE CASCADE   |
| `added_at`  | `TIMESTAMPTZ` | NOT NULL                                       |

**Индексы:** `PRIMARY KEY (folder_id, chat_id)`.


