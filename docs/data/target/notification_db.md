# `notification_db` — целевая схема

**Сервис:** Notification ([notification-service.md](../../microservices/notification-service.md)). **Шаг порядка:** 10.

`profile_id` → User (**без FK**). Группировка push и rate limit — Redis, не таблицы ниже.

---

## `device_tokens`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `profile_id` | `UUID` | NOT NULL |
| `platform` | `TEXT` | `android` \| `ios` \| `web` \| `desktop` |
| `token` | `TEXT` | NOT NULL |
| `push_service` | `TEXT` | `fcm` \| `apns` \| `voip_apns` |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (token)`; `(profile_id)`; `(profile_id, platform)`.

---

## `notification_settings`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `profile_id` | `UUID` | NOT NULL |
| `scope_type` | `TEXT` | `global` \| `space` \| `channel` \| `chat` |
| `scope_id` | `UUID` | NULL — space / channel / chat |
| `enabled` | `BOOLEAN` | NOT NULL |
| `mute_until` | `TIMESTAMPTZ` | NULL |
| `suppress_types` | `JSONB` | NULL |

**Индексы:** `UNIQUE (profile_id, scope_type, scope_id)` — для `scope_id` NULL использовать уникальный индекс с выражением или sentinel; на практике: отдельная строка global с `scope_id` NULL и частичный уникальный индекс на `(profile_id) WHERE scope_type = 'global'`.

---

## `quiet_hours`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `profile_id` | `UUID` | PK, логически 1:1 с профилем |
| `enabled` | `BOOLEAN` | NOT NULL |
| `start_time` | `TIME` | NOT NULL |
| `end_time` | `TIME` | NOT NULL |
| `timezone` | `TEXT` | NOT NULL |
| `override_mentions` | `BOOLEAN` | NOT NULL, DEFAULT true |

**Индексы:** PK `(profile_id)`.
