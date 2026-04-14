# `role_db` — целевая схема

**Сервис:** Role ([role-service.md](../../microservices/role-service.md)). **Шаг порядка:** 6.

`space_id` — спейс (**без FK**). В оверрайдах используем **раздельные идентификаторы**: `chat_id` для текстового чата в спейсе (`chat_db.chats.id`, `type` = `group` или `channel`) и `voice_room_id` для голоса (`space_db.voice_rooms.id`). Валидация — Role Service через вызовы Chat / Space (**без FK**). `profile_id` → User (**без FK**).

---

## `roles`

| Колонка          | Тип           | Описание                |
|------------------|---------------|-------------------------|
| `id`             | `UUID`        | PK                      |
| `space_id`       | `UUID`        | NOT NULL                |
| `name`           | `TEXT`        | NOT NULL                |
| `color`          | `TEXT`        | NULL                    |
| `is_system`      | `BOOLEAN`     | NOT NULL, DEFAULT false |
| `position`       | `INT`         | NOT NULL                |
| `permissions`    | `BIGINT`      | NOT NULL, bitmask       |
| `is_mentionable` | `BOOLEAN`     | NOT NULL, DEFAULT true  |
| `created_at`     | `TIMESTAMPTZ` | NOT NULL                |
| `updated_at`     | `TIMESTAMPTZ` | NOT NULL                |

**Индексы:** `(space_id, position)`; `UNIQUE (space_id, name)` — опционально, если имена уникальны в спейсе.

---

## `member_roles`

| Колонка                  | Тип           | Описание                                     |
|--------------------------|---------------|----------------------------------------------|
| `space_id`               | `UUID`        | NOT NULL                                     |
| `profile_id`             | `UUID`        | NOT NULL                                     |
| `role_id`                | `UUID`        | NOT NULL, FK → `roles(id)` ON DELETE CASCADE |
| `assigned_at`            | `TIMESTAMPTZ` | NOT NULL                                     |
| `assigned_by_profile_id` | `UUID`        | NULL                                         |

**Индексы:** `UNIQUE (space_id, profile_id, role_id)`; `(profile_id)`; `(role_id)`.

---

## `chat_overrides`

| Колонка   | Тип      | Описание                                     |
|-----------|----------|----------------------------------------------|
| `chat_id` | `UUID`   | NOT NULL — `chat_db.chats.id`, `type` = `group` или `channel` |
| `role_id` | `UUID`   | NOT NULL, FK → `roles(id)` ON DELETE CASCADE |
| `allow`   | `BIGINT` | NOT NULL, DEFAULT 0                          |
| `deny`    | `BIGINT` | NOT NULL, DEFAULT 0                          |

**Индексы:** `PRIMARY KEY (chat_id, role_id)`.

---

## `voice_room_overrides`

| Колонка         | Тип      | Описание                                    |
|-----------------|----------|---------------------------------------------|
| `voice_room_id` | `UUID`   | NOT NULL — `space_db.voice_rooms.id`        |
| `role_id`       | `UUID`   | NOT NULL, FK → `roles(id)` ON DELETE CASCADE |
| `allow`         | `BIGINT` | NOT NULL, DEFAULT 0                         |
| `deny`          | `BIGINT` | NOT NULL, DEFAULT 0                         |

**Индексы:** `PRIMARY KEY (voice_room_id, role_id)`.

---

## Жизненный цикл при удалении спейса

Между `role_db` и `space_db` нет FK. При удалении спейса (hard или финальный soft по политике продукта) **Space Service** публикует доменное событие (например `space.deleted` в NATS). **Role Service** обрабатывает его **идемпотентно**:

1. Удалить все `roles` с данным `space_id` (или пометить спейс недоступным до purge — по политике).
2. Каскадно удалятся строки `member_roles`, `chat_overrides` и `voice_room_overrides`, ссылающиеся на эти `roles` (FK внутри БД).

Повторная доставка события не должна приводить к ошибкам. Порядок относительно Messaging/File — см. саги и миграции в [OPERATIONS.md](../../OPERATIONS.md).


