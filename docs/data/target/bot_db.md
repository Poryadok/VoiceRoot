# `bot_db` — целевая схема

**Сервис:** Bot ([bot-service.md](../../microservices/bot-service.md)). **Шаг порядка:** 14.

`owner_account_id` → Auth; `channel_id` → Space (**без FK**).

---

## `bots`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `owner_account_id` | `UUID` | NOT NULL |
| `name` | `TEXT` | NOT NULL |
| `description` | `TEXT` | NULL |
| `avatar_url` | `TEXT` | NULL |
| `token_hash` | `TEXT` | NOT NULL, UNIQUE |
| `webhook_url` | `TEXT` | NULL |
| `webhook_secret` | `TEXT` | NULL |
| `is_polling_mode` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `scopes` | `JSONB` | NOT NULL |
| `status` | `TEXT` | `active` \| `suspended` |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(owner_account_id)`; `(status)`.

---

## `bot_commands`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `bot_id` | `UUID` | NOT NULL, FK → `bots(id)` ON DELETE CASCADE |
| `name` | `TEXT` | NOT NULL |
| `description` | `TEXT` | NULL |
| `parameters` | `JSONB` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (bot_id, name)`; `(bot_id)`.

---

## `bot_channel_whitelist`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `bot_id` | `UUID` | NOT NULL, FK → `bots(id)` ON DELETE CASCADE |
| `channel_id` | `UUID` | NOT NULL |
| `added_by_profile_id` | `UUID` | NOT NULL |
| `added_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `PRIMARY KEY (bot_id, channel_id)`; `(channel_id)`.

---

## `bot_event_log`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `bot_id` | `UUID` | NOT NULL, FK → `bots(id)` ON DELETE CASCADE |
| `event_type` | `TEXT` | NOT NULL |
| `payload` | `JSONB` | NULL |
| `delivery_status` | `TEXT` | `pending` \| `delivered` \| `failed` \| `timeout` |
| `attempts` | `INT` | NOT NULL, DEFAULT 0 |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `delivered_at` | `TIMESTAMPTZ` | NULL |

**Индексы:** `(bot_id, created_at DESC)`; `(delivery_status, created_at)` для ретраев.
