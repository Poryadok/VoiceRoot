# `chat_db` — целевая схема

**Сервис:** Chat ([chat-service.md](../../microservices/chat-service.md)). **Шаг порядка:** 4.

`profile_id` → User; `creator_profile_id` — то же (**без FK**). `space_id` → Space (**без FK**), когда чат привязан к спейсу.

### Сообщения

Все тексты (DM, группа, **канал**) хранятся в **Messaging** (`messaging_db`). Поле **`chat_id` в сообщениях** для любого текстового типа — **`chats.id`**. Семантика `chat_type` в Messaging: `dm` \| `group` \| `channel`. Подробнее: [DATA_MODEL.md](../../DATA_MODEL.md).

---

## `chats`

| Колонка              | Тип           | Описание |
|----------------------|---------------|----------|
| `id`                 | `UUID`        | PK       |
| `type`               | `TEXT`        | `dm` \| `group` \| `channel` |
| `space_id`           | `UUID`        | NULL — для `group`/`channel` в спейсе; для standalone канала или группы вне спейса NULL |
| `name`               | `TEXT`        | NULL — для DM обычно NULL; для группы/канала — отображаемое имя |
| `avatar_url`         | `TEXT`        | NULL     |
| `topic`              | `TEXT`        | NULL — краткое описание (часто для `channel`) |
| `creator_profile_id` | `UUID`        | NOT NULL |
| `slow_mode_seconds`  | `INT`         | NOT NULL, DEFAULT 0 |
| `last_message_at`    | `TIMESTAMPTZ` | NULL     |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(last_message_at DESC NULLS LAST)`; `(space_id)` где не NULL; `(type)`.

---

## `chat_members`

Участники для **DM** и **группы**; для **канала** — по продукту (явные подписчики, модераторы и т.д.) или минимальный набор; для больших каналов в спейсе доступ может опираться на `space_members` + Role без строки в `chat_members` — политика Chat Service.

| Колонка       | Тип           | Описание                                     |
|---------------|---------------|----------------------------------------------|
| `chat_id`     | `UUID`        | NOT NULL, FK → `chats(id)` ON DELETE CASCADE |
| `profile_id`  | `UUID`        | NOT NULL                                     |
| `role`        | `TEXT`        | `owner` \| `admin` \| `member`               |
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
| `type`          | `TEXT`        | `system` \| `custom` |
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

