# `social_db` — целевая схема

**Сервис:** Social ([social-service.md](../../microservices/social-service.md)). **Шаг порядка:** 3.

`profile_id` → User; `blocker_account_id` / `blocked_account_id` → Auth (**без FK**).

---

## `friendships`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `requester_profile_id` | `UUID` | NOT NULL |
| `target_profile_id` | `UUID` | NOT NULL |
| `status` | `TEXT` | `pending` \| `accepted` \| `declined` |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (requester_profile_id, target_profile_id)`; `(target_profile_id, status)`; `(requester_profile_id, status)`.

---

## `contacts`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `owner_profile_id` | `UUID` | NOT NULL |
| `target_profile_id` | `UUID` | NOT NULL |
| `source` | `TEXT` | `manual` \| `phone_sync` \| `space` \| `matchmaking` |
| `is_favorite` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (owner_profile_id, target_profile_id)`; `(owner_profile_id)`.

---

## `blocks`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `blocker_account_id` | `UUID` | NOT NULL |
| `blocked_account_id` | `UUID` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (blocker_account_id, blocked_account_id)`; `(blocked_account_id)`.
