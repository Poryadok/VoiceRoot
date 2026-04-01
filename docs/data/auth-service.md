# Auth Service — `auth_db` (v1)

Владелец: Auth Service ([microservices/auth-service.md](../microservices/auth-service.md)). Скоуп v1: [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md).

JWT: claim `user_id` = `accounts.id` (= логический `account_id`).

---

## Таблицы

### `accounts`

Учётная запись. Soft delete: `deleted_at IS NOT NULL` означает удалённый аккаунт (grace period и политика — продукт/операции).

| Колонка | Тип | Ограничения / заметки |
|---------|-----|------------------------|
| `id` | `UUID` | PK, `DEFAULT gen_random_uuid()` |
| `email` | `CITEXT` или `TEXT` | NULL; уникальность среди **активных** — см. индекс |
| `phone` | `TEXT` | NULL; формат E.164 на уровне приложения; уникальность среди активных — см. индекс |
| `password_hash` | `TEXT` | NOT NULL для `type = regular` с паролем; для гостя может быть заглушка по правилю приложения |
| `type` | `TEXT` | NOT NULL, `CHECK (type IN ('regular', 'guest'))` |
| `status` | `TEXT` | NOT NULL, `CHECK (status IN ('active', 'suspended', 'deleted'))`; согласовать с `deleted_at` |
| `totp_secret` | `BYTEA` или `TEXT` | NULL; шифрование at rest — приложение |
| `totp_enabled` | `BOOLEAN` | NOT NULL, DEFAULT false |
| `deleted_at` | `TIMESTAMPTZ` | NULL = не удалён |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

**Индексы**

- PK: `PRIMARY KEY (id)`
- Частичные уникальные индексы (только «живые» строки для логина), например:
  - `CREATE UNIQUE INDEX ux_accounts_email_active ON accounts (email) WHERE deleted_at IS NULL AND email IS NOT NULL;`
  - `CREATE UNIQUE INDEX ux_accounts_phone_active ON accounts (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL;`
- Опционально: `INDEX (deleted_at) WHERE deleted_at IS NOT NULL` для джобов очистки.

**FK внешние:** нет (корневая сущность БД).

---

### `refresh_tokens`

Одноразовая ротация refresh; в БД только хэш.

| Колонка | Тип | Ограничения / заметки |
|---------|-----|------------------------|
| `id` | `UUID` | PK |
| `account_id` | `UUID` | NOT NULL, **FK** `REFERENCES accounts(id) ON DELETE CASCADE` |
| `token_hash` | `TEXT` или `BYTEA` | NOT NULL, UNIQUE |
| `device_info` | `JSONB` | NULL |
| `expires_at` | `TIMESTAMPTZ` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |
| `revoked_at` | `TIMESTAMPTZ` | NULL |

**Индексы**

- `PRIMARY KEY (id)`
- `UNIQUE (token_hash)`
- `INDEX (account_id)` — список сессий / отзыв всех
- `INDEX (expires_at)` — TTL-очистка

---

### `otp_codes`

Email OTP (верификация, сброс пароля).

| Колонка | Тип | Ограничения / заметки |
|---------|-----|------------------------|
| `id` | `UUID` | PK |
| `account_id` | `UUID` | NOT NULL, **FK** `REFERENCES accounts(id) ON DELETE CASCADE` |
| `code_encrypted` | `TEXT` или `BYTEA` | NOT NULL — хранить зашифрованным |
| `type` | `TEXT` | NOT NULL, `CHECK (type IN ('email_verify', 'password_reset'))` |
| `expires_at` | `TIMESTAMPTZ` | NOT NULL |
| `used_at` | `TIMESTAMPTZ` | NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() |

**Индексы**

- `PRIMARY KEY (id)`
- `INDEX (account_id, type, created_at DESC)` — выбор актуального кода
- `INDEX (expires_at)` — очистка просроченных

---

## Отложено после v1

- Отдельные таблицы под аудит IP / security events (если не хранится вне БД).
- Расширения под SMS / дополнительные провайдеры — по продукту.
