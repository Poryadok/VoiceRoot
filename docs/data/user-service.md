# User Service — `user_db` (v1)

Владелец: User Service ([microservices/user-service.md](../microservices/user-service.md)). Скоуп v1: [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md).

`account_id` — UUID аккаунта из `auth_db`; **FK на `auth_db` не задаём** ([DATA_MODEL.md](../DATA_MODEL.md)).

Целевая полная схема (верификация, soft delete профиля и др.): [target/user_db.md](target/user_db.md).

---

## Таблицы

### `profiles`

Минимум для Фазы 1: один основной профиль на аккаунт (`is_primary = true`). Поля Premium / верификация как продукта — заложены nullable или дефолтами для будущих фаз.

| Колонка | Тип | Ограничения / заметки |
|---------|-----|------------------------|
| `id` | `UUID` | PK |
| `account_id` | `UUID` | NOT NULL — ссылка на `accounts.id` логически |
| `username` | `TEXT` | NOT NULL |
| `discriminator` | `TEXT` | NOT NULL, 4 цифры — формат на уровне приложения |
| `display_name` | `TEXT` | NOT NULL |
| `avatar_url` | `TEXT` | NULL — URL объекта в R2 (Фаза 1) |
| `banner_url` | `TEXT` | NULL — вне минимального MVP UI можно не заполнять |
| `bio` | `TEXT` | NULL; лимит 500 символов — приложение |
| `custom_status` | `TEXT` | NULL — Premium позже; колонка допускается NULL в v1 |
| `locale` | `TEXT` | NOT NULL, DEFAULT `'en'`, `CHECK (locale IN ('en', 'ru'))` |
| `theme` | `TEXT` | NOT NULL, DEFAULT `'dark'`, `CHECK (theme IN ('light', 'dark', 'high_contrast'))` |
| `is_primary` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `verification_type` | `TEXT` | NOT NULL, DEFAULT `'none'`, `CHECK (verification_type IN ('none', 'personal', 'organization'))` |
| `verification_badge` | `TEXT` | NULL |
| `last_seen_at` | `TIMESTAMPTZ` | NULL — **опциональная персистенция** last seen; онлайн-статус в Redis ([DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) §4.2) |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

**Индексы**

- `PRIMARY KEY (id)`
- `UNIQUE (username, discriminator)` — глобальный ник#дискриминатор (уточнить продукт: case-insensitivity → `CITEXT` или нормализация)
- `INDEX (account_id)` — список профилей аккаунта
- Частичный уникальный: **один primary на аккаунт**  
  `CREATE UNIQUE INDEX ux_profiles_one_primary_per_account ON profiles (account_id) WHERE is_primary = true;`

**Ограничения:** при появлении мульти-профиля продукта — снять/заменить `ux_profiles_one_primary_per_account` (Фаза 13).

---

### `privacy_settings`

В v1 включаем **минимум**, достаточный для DM и заявок в друзья ([DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) §4.2). Расширенные поля из [user-service.md](../microservices/user-service.md) — добавить миграцией expand, когда появится UI.

| Колонка | Тип | Ограничения / заметки |
|---------|-----|------------------------|
| `profile_id` | `UUID` | PK и **FK** `REFERENCES profiles(id) ON DELETE CASCADE` |
| `preset` | `TEXT` | NOT NULL, DEFAULT `'personal'`, `CHECK (preset IN ('personal', 'gaming', 'work'))` |
| `show_online` | `TEXT` | NOT NULL, DEFAULT `'friends'`, `CHECK (show_online IN ('everyone', 'friends', 'nobody'))` |
| `allow_dm` | `TEXT` | NOT NULL, DEFAULT `'friends'`, `CHECK (allow_dm IN ('everyone', 'friends', 'friends_of_friends', 'nobody'))` |
| `allow_friend_requests` | `TEXT` | NOT NULL, DEFAULT `'everyone'`, `CHECK (allow_friend_requests IN ('everyone', 'friends_of_friends', 'nobody'))` |
| `allow_guest_dm` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

**Индексы:** PK по `profile_id` достаточно.

**Отложено в те же таблицы позже:** `show_game_status`, `show_mm_rating`, `show_phone`, `show_stories` — миграция после появления фич.

---

## Presence

Источник истины для **текущего** онлайна — **Redis** (TTL, heartbeat), не таблицы в этом файле. `profiles.last_seen_at` — опционально для отображения после рестарта/долгого оффлайна.

---

## Отложено после v1

- Таблицы `user_settings`, `onboarding_state` — когда появятся соответствующие экраны/API в скоупе.
- Отдельная персистенция presence, если продукт потребует историю статусов.
