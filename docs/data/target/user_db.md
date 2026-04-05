# `user_db` — целевая схема

**Сервис:** User ([user-service.md](../../microservices/user-service.md)). **Шаг порядка:** 2.

`account_id` — логическая ссылка на `auth_db.accounts.id` (**без FK**).

---

## `profiles`

| Колонка              | Тип           | Описание                                                          |
|----------------------|---------------|-------------------------------------------------------------------|
| `id`                 | `UUID`        | PK                                                                |
| `account_id`         | `UUID`        | NOT NULL                                                          |
| `username`           | `TEXT`        | NOT NULL                                                          |
| `discriminator`      | `TEXT`        | NOT NULL                                                          |
| `display_name`       | `TEXT`        | NOT NULL                                                          |
| `avatar_url`         | `TEXT`        | NULL                                                              |
| `banner_url`         | `TEXT`        | NULL                                                              |
| `bio`                | `TEXT`        | NULL                                                              |
| `custom_status`      | `TEXT`        | NULL (Premium)                                                    |
| `locale`             | `TEXT`        | `en` \ `ru`                                                       |
| `theme`              | `TEXT`        | `light` \ `dark` \ `high_contrast`                                |
| `is_primary`         | `BOOLEAN`     | NOT NULL                                                          |
| `verification_type`  | `TEXT`        | `none` \ `personal` \ `organization`                              |
| `verification_badge` | `TEXT`        | NULL                                                              |
| `last_seen_at`       | `TIMESTAMPTZ` | NULL (доп. к Redis presence)                                      |
| `deleted_at`         | `TIMESTAMPTZ` | NULL — soft delete профиля ([DATA_MODEL.md](../../DATA_MODEL.md)) |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL                                                          |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL                                                          |

**Индексы:** `UNIQUE (username, discriminator) WHERE deleted_at IS NULL`; `INDEX (account_id)`; частичный UNIQUE один активный `is_primary` на аккаунт: `(account_id) WHERE is_primary = true AND deleted_at IS NULL`.

---

## `privacy_settings`

| Колонка                 | Тип           | Описание                                                 |
|-------------------------|---------------|----------------------------------------------------------|
| `profile_id`            | `UUID`        | PK, FK → `profiles(id)` ON DELETE CASCADE                |
| `preset`                | `TEXT`        | `personal` \ `gaming` \ `work`                           |
| `show_online`           | `TEXT`        | `everyone` \ `friends` \ `nobody`                        |
| `show_game_status`      | `TEXT`        | `everyone` \ `friends` \ `nobody`                        |
| `show_mm_rating`        | `TEXT`        | `everyone` \ `friends` \ `nobody`                        |
| `show_phone`            | `TEXT`        | `friends` \ `nobody`                                     |
| `show_stories`          | `TEXT`        | `everyone` \ `friends` \ `nobody`                        |
| `allow_dm`              | `TEXT`        | `everyone` \ `friends` \ `friends_of_friends` \ `nobody` |
| `allow_friend_requests` | `TEXT`        | `everyone` \ `friends_of_friends` \ `nobody`             |
| `allow_guest_dm`        | `BOOLEAN`     | NOT NULL                                                 |
| `updated_at`            | `TIMESTAMPTZ` | NOT NULL                                                 |

---

## `user_settings`

| Колонка              | Тип           | Описание                                  |
|----------------------|---------------|-------------------------------------------|
| `profile_id`         | `UUID`        | PK, FK → `profiles(id)` ON DELETE CASCADE |
| `notification_prefs` | `JSONB`       | NULL — ключи по продукту                  |
| `language`           | `TEXT`        | NULL                                      |
| `client_flags`       | `JSONB`       | NULL                                      |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL                                  |

---

## `onboarding_state`

| Колонка           | Тип           | Описание                                  |
|-------------------|---------------|-------------------------------------------|
| `profile_id`      | `UUID`        | PK, FK → `profiles(id)` ON DELETE CASCADE |
| `completed_steps` | `JSONB`       | NOT NULL, DEFAULT `'[]'`                  |
| `current_step`    | `TEXT`        | NULL                                      |
| `updated_at`      | `TIMESTAMPTZ` | NOT NULL                                  |

---

## `linked_identities`

Привязки внешних аккаунтов (OAuth) для **верификации** личности и будущих интеграций. Секреты — в приложении (шифрование), не в открытом виде.

| Колонка                   | Тип           | Описание                                        |
|---------------------------|---------------|-------------------------------------------------|
| `id`                      | `UUID`        | PK                                              |
| `profile_id`              | `UUID`        | NOT NULL, FK → `profiles(id)` ON DELETE CASCADE |
| `provider`                | `TEXT`        | `twitch` \ `youtube` \ …                        |
| `provider_user_id`        | `TEXT`        | NOT NULL                                        |
| `access_token_encrypted`  | `BYTEA`       | NULL                                            |
| `refresh_token_encrypted` | `BYTEA`       | NULL                                            |
| `token_expires_at`        | `TIMESTAMPTZ` | NULL                                            |
| `provider_snapshot`       | `JSONB`       | NULL — кэш статуса Partner / YPP для крона      |
| `created_at`              | `TIMESTAMPTZ` | NOT NULL                                        |
| `updated_at`              | `TIMESTAMPTZ` | NOT NULL                                        |

**Индексы:** `UNIQUE (provider, provider_user_id)`; `(profile_id)`; `(profile_id, provider)`.

---

## `dns_verification_challenges`

Организационная верификация через DNS TXT ([verification.md](../../features/verification.md)).

| Колонка        | Тип           | Описание                                                              |
|----------------|---------------|-----------------------------------------------------------------------|
| `id`           | `UUID`        | PK                                                                    |
| `profile_id`   | `UUID`        | NOT NULL, FK → `profiles(id)` ON DELETE CASCADE                       |
| `domain`       | `TEXT`        | NOT NULL                                                              |
| `txt_expected` | `TEXT`        | NOT NULL — полное значение записи (например `voice-verify=…`), UNIQUE |
| `status`       | `TEXT`        | `pending` \ `verified` \ `expired` \ `failed`                         |
| `verified_at`  | `TIMESTAMPTZ` | NULL                                                                  |
| `expires_at`   | `TIMESTAMPTZ` | NOT NULL                                                              |
| `created_at`   | `TIMESTAMPTZ` | NOT NULL                                                              |

**Индексы:** `(profile_id, status)`; `(expires_at)` для очистки.

---

## `verification_manual_requests`

Ручной fallback верификации организаций (email-заявка).

| Колонка                  | Тип           | Описание                                        |
|--------------------------|---------------|-------------------------------------------------|
| `id`                     | `UUID`        | PK                                              |
| `profile_id`             | `UUID`        | NOT NULL, FK → `profiles(id)` ON DELETE CASCADE |
| `organization_name`      | `TEXT`        | NOT NULL                                        |
| `contact_email`          | `TEXT`        | NOT NULL                                        |
| `notes`                  | `TEXT`        | NULL                                            |
| `status`                 | `TEXT`        | `pending` \ `approved` \ `denied`               |
| `reviewed_by_profile_id` | `UUID`        | NULL                                            |
| `reviewed_at`            | `TIMESTAMPTZ` | NULL                                            |
| `review_notes`           | `TEXT`        | NULL                                            |
| `created_at`             | `TIMESTAMPTZ` | NOT NULL                                        |
| `updated_at`             | `TIMESTAMPTZ` | NOT NULL                                        |

**Индексы:** `(status, created_at)`; `(profile_id)`.

---

## Ссылки наружу

`account_id` → Auth.


