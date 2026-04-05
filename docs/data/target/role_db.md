# `role_db` — целевая схема

**Сервис:** Role ([role-service.md](../../microservices/role-service.md)). **Шаг порядка:** 6.

`space_id` и `channel_id` принадлежат Space Service (**без FK**). `profile_id` → User (**без FK**).

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

## `channel_overrides`

| Колонка      | Тип      | Описание                                     |
|--------------|----------|----------------------------------------------|
| `channel_id` | `UUID`   | NOT NULL                                     |
| `role_id`    | `UUID`   | NOT NULL, FK → `roles(id)` ON DELETE CASCADE |
| `allow`      | `BIGINT` | NOT NULL, DEFAULT 0                          |
| `deny`       | `BIGINT` | NOT NULL, DEFAULT 0                          |

**Индексы:** `PRIMARY KEY (channel_id, role_id)`.

---

## Жизненный цикл при удалении спейса

Между `role_db` и `space_db` нет FK. При удалении спейса (hard или финальный soft по политике продукта) **Space Service** публикует доменное событие (например `space.deleted` в NATS). **Role Service** обрабатывает его **идемпотентно**:

1. Удалить все `roles` с данным `space_id` (или пометить спейс недоступным до purge — по политике).
2. Каскадно удалятся строки `member_roles` и `channel_overrides`, ссылающиеся на эти `roles` (FK внутри БД).

Повторная доставка события не должна приводить к ошибкам. Порядок относительно Messaging/File — см. саги и миграции в [OPERATIONS.md](../../OPERATIONS.md).


