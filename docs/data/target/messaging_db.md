# `messaging_db` — целевая схема

**Сервис:** Messaging ([messaging-service.md](../../microservices/messaging-service.md)). **Шаг порядка:** 9.

`sender_profile_id` → User (**без FK**).

**Семантика `chat_id`:** при `chat_type` **`dm` \| `group`** — UUID строки `chat_db.chats.id`; при **`channel`** — UUID **`space_db.channels.id`** (отдельной строки в `chats` нет). См. [DATA_MODEL.md](../../DATA_MODEL.md).

**Идентификаторы сообщений:** `messages.id` — **UUIDv7**, генерирует сервис при INSERT ([DATA_MODEL.md](../../DATA_MODEL.md)).

---

## `messages`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK, UUIDv7 |
| `chat_id` | `UUID` | NOT NULL |
| `chat_type` | `TEXT` | `dm` \| `group` \| `channel` |
| `sender_profile_id` | `UUID` | NOT NULL |
| `content` | `TEXT` | NOT NULL |
| `type` | `TEXT` | `regular` \| `system` \| `forward` |
| `thread_parent_id` | `UUID` | NULL, FK → `messages(id)` ON DELETE SET NULL |
| `forward_from_id` | `UUID` | NULL, FK → `messages(id)` ON DELETE SET NULL |
| `forward_from_sender` | `TEXT` | NULL |
| `attachments` | `JSONB` | NULL — дублирует/дополняет строки `message_attachments` для совместимости API; источник для выборок Shared Media — таблица ниже |
| `mentions` | `JSONB` | NULL |
| `edited_at` | `TIMESTAMPTZ` | NULL |
| `deleted_at` | `TIMESTAMPTZ` | NULL |
| `is_e2e` | `BOOLEAN` | NOT NULL, DEFAULT false — не индексировать в Search |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(chat_id, id DESC)`; частичный `(chat_id, id DESC) WHERE deleted_at IS NULL`; `(thread_parent_id)` при тредах.

---

## `read_receipts`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `chat_id` | `UUID` | NOT NULL |
| `profile_id` | `UUID` | NOT NULL |
| `last_read_message_id` | `UUID` | NOT NULL |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `PRIMARY KEY (chat_id, profile_id)`; `(profile_id)`.

---

## `reactions`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `message_id` | `UUID` | NOT NULL, FK → `messages(id)` ON DELETE CASCADE |
| `profile_id` | `UUID` | NOT NULL |
| `emoji` | `TEXT` | NOT NULL |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (message_id, profile_id, emoji)`; `(message_id)`.

---

## `pins`

| Колонка | Тип | Описание |
|---------|-----|----------|
| `chat_id` | `UUID` | NOT NULL |
| `message_id` | `UUID` | NOT NULL, FK → `messages(id)` ON DELETE CASCADE |
| `pinned_by_profile_id` | `UUID` | NOT NULL |
| `pinned_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `UNIQUE (chat_id, message_id)`; `(chat_id, pinned_at DESC)`.

---

## `message_attachments`

Нормализованные вложения и ссылки для **Shared Media** (медиа, файлы, ссылки, голосовые) и тяжёлых запросов по типу контента без полного скана `JSONB`.

| Колонка | Тип | Описание |
|---------|-----|----------|
| `id` | `UUID` | PK |
| `message_id` | `UUID` | NOT NULL, FK → `messages(id)` ON DELETE CASCADE |
| `sort_order` | `SMALLINT` | NOT NULL, DEFAULT 0 |
| `kind` | `TEXT` | `image` \| `video` \| `audio` \| `voice_message` \| `document` \| `link` \| `other` |
| `file_id` | `UUID` | NULL — `file_db.files.id`, если файл в объектном хранилище |
| `external_url` | `TEXT` | NULL — URL для вкладки «Ссылки» и превью |
| `title` | `TEXT` | NULL — заголовок ссылки / подпись |
| `duration_seconds` | `NUMERIC(10,3)` | NULL — для аудио/голосовых |
| `created_at` | `TIMESTAMPTZ` | NOT NULL |

**Индексы:** `(message_id, sort_order)`; частичный `(file_id) WHERE file_id IS NOT NULL`; частичный GIN или B-tree по `kind` + join на `messages` по `chat_id` для выборок по чату (в миграции — композитный путь через `messages(chat_id, id)`).
