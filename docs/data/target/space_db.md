# `space_db` — целевая схема

**Сервис:** Space ([space-service.md](../../microservices/space-service.md)). **Шаг порядка:** 5.

`owner_profile_id`, `profile_id` в членстве, `account_id` в банах → внешние UUID (**без FK**).

---

## `spaces`

| Колонка             | Тип           | Описание                                              |
|---------------------|---------------|-------------------------------------------------------|
| `id`                | `UUID`        | PK                                                    |
| `name`              | `TEXT`        | NOT NULL                                              |
| `description`       | `TEXT`        | NULL                                                  |
| `icon_url`          | `TEXT`        | NULL                                                  |
| `banner_url`        | `TEXT`        | NULL                                                  |
| `visibility`        | `TEXT`        | `public` \ `invite_only` \ `private`                  |
| `owner_profile_id`  | `UUID`        | NOT NULL                                              |
| `member_count`      | `INT`         | NOT NULL, DEFAULT 0                                   |
| `is_verified`       | `BOOLEAN`     | NOT NULL, DEFAULT false                               |
| `verification_type` | `TEXT`        | `none` \ `personal` \ `organization`                  |
| `entry_requirement` | `TEXT`        | `none` \ `phone` \ `captcha` \ `questions` \ `manual` |
| `entry_questions`   | `JSONB`       | NULL                                                  |
| `mm_config`         | `JSONB`       | NULL                                                  |
| `created_at`        | `TIMESTAMPTZ` | NOT NULL                                              |
| `updated_at`        | `TIMESTAMPTZ` | NOT NULL                                              |

**Индексы:** `(visibility, name)` для каталога; `(owner_profile_id)`.

---

## `categories`

| Колонка      | Тип           | Описание                                      |
|--------------|---------------|-----------------------------------------------|
| `id`         | `UUID`        | PK                                            |
| `space_id`   | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE |
| `name`       | `TEXT`        | NOT NULL                                      |
| `sort_order` | `INT`         | NOT NULL, DEFAULT 0                           |
| `created_at` | `TIMESTAMPTZ` | NOT NULL                                      |

**Индексы:** `(space_id, sort_order)`.

---

## `voice_rooms`

Сущность **голосовой комнаты** в спейсе (медиа — Voice / LiveKit). **Не** путать с текстовым чатом: сообщения идут в Messaging с `chat_id` из **`chat_db.chats`**, см. [DATA_MODEL.md](../../DATA_MODEL.md).

Здесь только **идентичность и владелец** комнаты (`space_id`, имя). **Категория, порядок в sidebar и соседство с текстовыми чатами** задаются в **`space_tree_nodes`** — один слой дерева для голоса и текста.

| Колонка      | Тип           | Описание                                       |
|--------------|---------------|------------------------------------------------|
| `id`         | `UUID`        | PK — в Role оверрайдах и Voice как `voice_room_id` |
| `space_id`   | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE  |
| `name`       | `TEXT`        | NOT NULL                                       |
| `created_at` | `TIMESTAMPTZ` | NOT NULL                                       |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL                                       |

**Индексы:** `(space_id)`; опционально `(space_id, name)`.

---

## `space_tree_nodes`

**Упорядоченные узлы дерева** спейса в боковой панели: и **текстовые** чаты (`group` \| `channel`), и **голосовые** комнаты. Одна строка — один видимый узел; `sort_order` и `category_id` относятся ко **всем** типам одинаково.

| Колонка         | Тип           | Описание                                                                 |
|-----------------|---------------|--------------------------------------------------------------------------|
| `id`            | `UUID`        | PK                                                                       |
| `space_id`      | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE                            |
| `category_id`   | `UUID`        | NULL, FK → `categories(id)` ON DELETE SET NULL                           |
| `kind`          | `TEXT`        | NOT NULL, `text_chat` \| `voice_room`                                    |
| `chat_id`       | `UUID`        | NULL — при `kind = text_chat`: `chat_db.chats.id` (**без FK** наружу)    |
| `voice_room_id` | `UUID`        | NULL — при `kind = voice_room`: FK → `voice_rooms(id)` ON DELETE CASCADE |
| `sort_order`    | `INT`         | NOT NULL, DEFAULT 0 — порядок среди узлов (в категории / в корне)        |
| `is_system`     | `BOOLEAN`     | NOT NULL, DEFAULT false — только для `text_chat` (системный чат объявлений) |
| `created_at`    | `TIMESTAMPTZ` | NOT NULL                                                                 |
| `updated_at`    | `TIMESTAMPTZ` | NOT NULL                                                                 |

**Инварианты (CHECK в миграции):**

- ровно одна ссылка: либо (`kind = text_chat` и `chat_id` NOT NULL и `voice_room_id` IS NULL), либо (`kind = voice_room` и наоборот);
- `is_system = true` допустимо только при `kind = text_chat`.

**Индексы / ограничения:** частичный `UNIQUE (space_id, chat_id) WHERE chat_id IS NOT NULL`; частичный `UNIQUE (space_id, voice_room_id) WHERE voice_room_id IS NOT NULL`; `(space_id, category_id, sort_order)` для сортировки и reorder.

---

## `space_members`

| Колонка      | Тип           | Описание                                      |
|--------------|---------------|-----------------------------------------------|
| `space_id`   | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE |
| `profile_id` | `UUID`        | NOT NULL                                      |
| `joined_at`  | `TIMESTAMPTZ` | NOT NULL                                      |
| `nickname`   | `TEXT`        | NULL                                          |

**Индексы:** `PRIMARY KEY (space_id, profile_id)`; `(profile_id)`.

---

## `space_bans`

| Колонка                | Тип           | Описание                                      |
|------------------------|---------------|-----------------------------------------------|
| `space_id`             | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE |
| `account_id`           | `UUID`        | NOT NULL                                      |
| `banned_by_profile_id` | `UUID`        | NOT NULL                                      |
| `reason`               | `TEXT`        | NULL                                          |
| `banned_at`            | `TIMESTAMPTZ` | NOT NULL                                      |

**Индексы:** `PRIMARY KEY (space_id, account_id)`; `(account_id)`.

---

## `invites`

| Колонка              | Тип           | Описание                                      |
|----------------------|---------------|-----------------------------------------------|
| `id`                 | `UUID`        | PK                                            |
| `space_id`           | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE |
| `code`               | `TEXT`        | NOT NULL, UNIQUE                              |
| `creator_profile_id` | `UUID`        | NOT NULL                                      |
| `max_uses`           | `INT`         | NULL                                          |
| `use_count`          | `INT`         | NOT NULL, DEFAULT 0                           |
| `expires_at`         | `TIMESTAMPTZ` | NULL                                          |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL                                      |
| `revoked_at`         | `TIMESTAMPTZ` | NULL                                          |

**Индексы:** `(space_id, created_at DESC)`.

---

## `audit_log`

| Колонка            | Тип           | Описание                                      |
|--------------------|---------------|-----------------------------------------------|
| `id`               | `UUID`        | PK                                            |
| `space_id`         | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE |
| `actor_profile_id` | `UUID`        | NOT NULL                                      |
| `action`           | `TEXT`        | NOT NULL                                      |
| `target_type`      | `TEXT`        | NOT NULL                                      |
| `target_id`        | `UUID`        | NULL                                          |
| `details`          | `JSONB`       | NULL                                          |
| `created_at`       | `TIMESTAMPTZ` | NOT NULL                                      |

**Индексы:** `(space_id, created_at DESC)`.

---

## `space_templates`

Шаблоны для `CreateFromTemplate` ([space-service.md](../../microservices/space-service.md)).

| Колонка           | Тип           | Описание                               |
|-------------------|---------------|----------------------------------------|
| `id`              | `UUID`        | PK                                     |
| `name`            | `TEXT`        | NOT NULL                               |
| `description`     | `TEXT`        | NULL                                   |
| `template_config` | `JSONB`       | NOT NULL — категории и узлы дерева (`space_tree_nodes`: текст + голос), см. [DATA_MODEL.md](../../DATA_MODEL.md) |
| `is_system`       | `BOOLEAN`     | NOT NULL, DEFAULT false                |
| `created_at`      | `TIMESTAMPTZ` | NOT NULL                               |


