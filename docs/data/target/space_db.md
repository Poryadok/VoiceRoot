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

## `channels`

| Колонка             | Тип           | Описание                                       |
|---------------------|---------------|------------------------------------------------|
| `id`                | `UUID`        | PK                                             |
| `space_id`          | `UUID`        | NOT NULL, FK → `spaces(id)` ON DELETE CASCADE  |
| `category_id`       | `UUID`        | NULL, FK → `categories(id)` ON DELETE SET NULL |
| `name`              | `TEXT`        | NOT NULL                                       |
| `type`              | `TEXT`        | `text` \ `voice`                               |
| `topic`             | `TEXT`        | NULL                                           |
| `slow_mode_seconds` | `INT`         | NOT NULL, DEFAULT 0                            |
| `sort_order`        | `INT`         | NOT NULL, DEFAULT 0                            |
| `is_system`         | `BOOLEAN`     | NOT NULL, DEFAULT false                        |
| `created_at`        | `TIMESTAMPTZ` | NOT NULL                                       |
| `updated_at`        | `TIMESTAMPTZ` | NOT NULL                                       |

**Индексы:** `(space_id, sort_order)`; `(space_id, type)`.

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
| `template_config` | `JSONB`       | NOT NULL — структура каналов/категорий |
| `is_system`       | `BOOLEAN`     | NOT NULL, DEFAULT false                |
| `created_at`      | `TIMESTAMPTZ` | NOT NULL                               |


