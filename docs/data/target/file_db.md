# `file_db` — целевая схема

**Сервис:** File ([file-service.md](../../microservices/file-service.md)). **Шаг порядка:** 8.

`uploader_profile_id` → User (**без FK**). **`chat_id`:** для `dm` / `group` / `channel` — всегда **`chat_db.chats.id`** (как в Messaging). `message_id` в `file_references` — Messaging (**без FK**).

---

## `files`

| Колонка               | Тип             | Описание                                                      |
|-----------------------|-----------------|---------------------------------------------------------------|
| `id`                  | `UUID`          | PK                                                            |
| `uploader_profile_id` | `UUID`          | NOT NULL                                                      |
| `original_name`       | `TEXT`          | NOT NULL                                                      |
| `mime_type`           | `TEXT`          | NOT NULL                                                      |
| `size_bytes`          | `BIGINT`        | NOT NULL                                                      |
| `sha256_hash`         | `TEXT`          | NOT NULL                                                      |
| `r2_key`              | `TEXT`          | NOT NULL                                                      |
| `status`              | `TEXT`          | `uploading` \ `processing` \ `ready` \ `infected` \ `expired` |
| `type`                | `TEXT`          | `image` \ `video` \ `audio` \ `document` \ `other`            |
| `width`               | `INT`           | NULL                                                          |
| `height`              | `INT`           | NULL                                                          |
| `duration_seconds`    | `NUMERIC(10,3)` | NULL                                                          |
| `thumbnail_r2_key`    | `TEXT`          | NULL                                                          |
| `converted_r2_key`    | `TEXT`          | NULL                                                          |
| `chat_id`             | `UUID`          | NULL                                                          |
| `chat_type`           | `TEXT`          | NULL — `dm` \ `group` \ `channel`                             |
| `is_e2e`              | `BOOLEAN`       | NOT NULL, DEFAULT false                                       |
| `expires_at`          | `TIMESTAMPTZ`   | NULL                                                          |
| `scan_result`         | `TEXT`          | `clean` \ `infected` \ `pending` \ `skipped`                  |
| `created_at`          | `TIMESTAMPTZ`   | NOT NULL                                                      |
| `updated_at`          | `TIMESTAMPTZ`   | NOT NULL                                                      |

**Индексы:** `(sha256_hash)` для дедупа; `(uploader_profile_id, created_at DESC)`; `(chat_id)`; `(expires_at)` для TTL job.

---

## `file_references`

Связь файл ↔ сообщение (дедупликация одного `file_id` на несколько сообщений).

| Колонка      | Тип           | Описание                                     |
|--------------|---------------|----------------------------------------------|
| `id`         | `UUID`        | PK                                           |
| `file_id`    | `UUID`        | NOT NULL, FK → `files(id)` ON DELETE CASCADE |
| `message_id` | `UUID`        | NOT NULL                                     |
| `chat_id`    | `UUID`        | NOT NULL                                     |
| `created_at` | `TIMESTAMPTZ` | NOT NULL                                     |

**Индексы:** `UNIQUE (file_id, message_id)`; `(message_id)`.


