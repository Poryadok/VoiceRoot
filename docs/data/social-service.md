# Social Service — `social_db` (v1)

Владелец: Social Service ([microservices/social-service.md](../microservices/social-service.md)). Скоуп v1: [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md).

- `requester_profile_id` / `target_profile_id` / `owner_profile_id` — UUID из `user_db`; **FK наружу нет**.
- `blocker_account_id` / `blocked_account_id` — UUID из `auth_db`; **FK наружу нет**.

---

## Таблицы

### `friendships`

Заявки и дружба между **профилями**.

| Колонка                | Тип           | Ограничения / заметки                                             |
|------------------------|---------------|-------------------------------------------------------------------|
| `id`                   | `UUID`        | PK                                                                |
| `requester_profile_id` | `UUID`        | NOT NULL                                                          |
| `target_profile_id`    | `UUID`        | NOT NULL, `CHECK (target_profile_id <> requester_profile_id)`     |
| `status`               | `TEXT`        | NOT NULL, `CHECK (status IN ('pending', 'accepted', 'declined'))` |
| `created_at`           | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                           |
| `updated_at`           | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                           |

**Индексы**

- `PRIMARY KEY (id)`
- Запрет дубликата пары (упорядочить UUID в приложении или хранить каноническую пару):
  - Рекомендация: уникальность по **направленной** паре (кто кого пригласил):  
    `UNIQUE (requester_profile_id, target_profile_id)`
- `INDEX (target_profile_id, status)` — входящие заявки
- `INDEX (requester_profile_id, status)` — исходящие

**Бизнес-правило:** повторная заявка после `declined` — либо новая строка, либо обновление существующей (зафиксировать в коде).

---

### `blocks`

Блокировка на уровне **аккаунта** (ко всем профилям заблокированного).

| Колонка              | Тип           | Ограничения / заметки                                        |
|----------------------|---------------|--------------------------------------------------------------|
| `id`                 | `UUID`        | PK                                                           |
| `blocker_account_id` | `UUID`        | NOT NULL                                                     |
| `blocked_account_id` | `UUID`        | NOT NULL, `CHECK (blocked_account_id <> blocker_account_id)` |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                      |

**Индексы**

- `PRIMARY KEY (id)`
- `UNIQUE (blocker_account_id, blocked_account_id)`
- `INDEX (blocked_account_id)` — обратный поиск «кто меня заблокировал» (если нужен internal API)

---

## Опционально в v1: `contacts`

Таблица из [social-service.md](../microservices/social-service.md) **не обязательна** для ядра «друзья + DM» ([DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) §2). Вводить миграцией, если в той же волне нужны телефонная синхронизация или односторонние контакты без дружбы.

| Колонка             | Тип           | Ограничения / заметки                                                          |
|---------------------|---------------|--------------------------------------------------------------------------------|
| `id`                | `UUID`        | PK                                                                             |
| `owner_profile_id`  | `UUID`        | NOT NULL                                                                       |
| `target_profile_id` | `UUID`        | NOT NULL                                                                       |
| `source`            | `TEXT`        | NOT NULL, `CHECK (source IN ('manual', 'phone_sync', 'space', 'matchmaking'))` |
| `is_favorite`       | `BOOLEAN`     | NOT NULL, DEFAULT false                                                        |
| `created_at`        | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                        |

**Индексы:** `UNIQUE (owner_profile_id, target_profile_id)`, `INDEX (owner_profile_id)`.

---

## Отложено после v1

- Индексы/материализации под friends-of-friends при масштабе.
- Избранное, если моделируется отдельно от `contacts`.


