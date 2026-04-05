# `auth_db` — целевая схема

**Сервис:** Auth ([auth-service.md](../../microservices/auth-service.md)). **Шаг порядка:** 1.

---

## `accounts`

| Колонка         | Тип           | Описание                                             |
|-----------------|---------------|------------------------------------------------------|
| `id`            | `UUID`        | PK, `DEFAULT gen_random_uuid()`                      |
| `email`         | `CITEXT`      | NULL; уникальность среди активных — частичный индекс |
| `phone`         | `TEXT`        | NULL; E.164 в приложении                             |
| `password_hash` | `TEXT`        | NOT NULL где требуется пароль                        |
| `type`          | `TEXT`        | `regular` \ `guest`                                  |
| `status`        | `TEXT`        | `active` \ `suspended` \ `deleted`                   |
| `totp_secret`   | `BYTEA`       | NULL, шифрование в приложении                        |
| `totp_enabled`  | `BOOLEAN`     | NOT NULL, DEFAULT false                              |
| `deleted_at`    | `TIMESTAMPTZ` | NULL                                                 |
| `created_at`    | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                              |
| `updated_at`    | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                              |

**Индексы:** `PRIMARY KEY (id)`; `UNIQUE` частичные на `email`, `phone` при `deleted_at IS NULL`.

---

## `refresh_tokens`

| Колонка       | Тип           | Описание                                        |
|---------------|---------------|-------------------------------------------------|
| `id`          | `UUID`        | PK                                              |
| `account_id`  | `UUID`        | NOT NULL, FK → `accounts(id)` ON DELETE CASCADE |
| `token_hash`  | `TEXT`        | NOT NULL, UNIQUE                                |
| `device_info` | `JSONB`       | NULL                                            |
| `expires_at`  | `TIMESTAMPTZ` | NOT NULL                                        |
| `created_at`  | `TIMESTAMPTZ` | NOT NULL                                        |
| `revoked_at`  | `TIMESTAMPTZ` | NULL                                            |

**Индексы:** `(account_id)`, `(expires_at)`.

---

## `otp_codes`

| Колонка          | Тип           | Описание                                        |
|------------------|---------------|-------------------------------------------------|
| `id`             | `UUID`        | PK                                              |
| `account_id`     | `UUID`        | NOT NULL, FK → `accounts(id)` ON DELETE CASCADE |
| `code_encrypted` | `BYTEA`       | NOT NULL                                        |
| `type`           | `TEXT`        | `email_verify` \ `password_reset`               |
| `expires_at`     | `TIMESTAMPTZ` | NOT NULL                                        |
| `used_at`        | `TIMESTAMPTZ` | NULL                                            |
| `created_at`     | `TIMESTAMPTZ` | NOT NULL                                        |

**Индексы:** `(account_id, type, created_at DESC)`, `(expires_at)`.

---

## Ссылки наружу

Нет. Корневая БД идентичности.


