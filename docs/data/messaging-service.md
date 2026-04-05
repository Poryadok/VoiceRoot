# Messaging Service — `messaging_db` (v1)

Владелец: Messaging Service ([microservices/messaging-service.md](../microservices/messaging-service.md)). Скоуп v1: [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) — сообщения + read receipts; **без** реакций, пинов, тредов как функционала.

`chat_id` — при **DM/группе** UUID из `chat_db.chats`; при **канале спейса** — UUID **`space_db.channels.id`** (строки в `chats` нет). **FK наружу нет**. `sender_profile_id` — UUID из `user_db`; **FK наружу нет**. Целевая полная схема: [data/target/messaging_db.md](target/messaging_db.md).

Идентификатор сообщения: **только UUIDv7** (тип PostgreSQL `UUID`). Значение генерирует **Messaging Service на Go** при создании сообщения (библиотека с поддержкой UUIDv7); в БД не полагаемся на `DEFAULT` для этого PK. Иные форматы (ULID и т.д.) не используем — см. [DATA_MODEL.md](../DATA_MODEL.md).

---

## Таблицы

### `messages`

| Колонка               | Тип           | Ограничения / заметки                                                                              |
|-----------------------|---------------|----------------------------------------------------------------------------------------------------|
| `id`                  | `UUID`        | PK — **UUIDv7**, задаётся сервисом при `INSERT` (в БД без `DEFAULT` для этого поля)                |
| `chat_id`             | `UUID`        | NOT NULL — см. семантику DM/group vs channel в начале файла                                        |
| `chat_type`           | `TEXT`        | NOT NULL, DEFAULT `'dm'`, `CHECK (chat_type IN ('dm', 'group', 'channel'))` — в v1 фактически `dm` |
| `sender_profile_id`   | `UUID`        | NOT NULL                                                                                           |
| `content`             | `TEXT`        | NOT NULL — лимит 4000 символов в приложении                                                        |
| `type`                | `TEXT`        | NOT NULL, DEFAULT `'regular'`, `CHECK (type IN ('regular', 'system', 'forward'))`                  |
| `thread_parent_id`    | `UUID`        | NULL — **колонка допускается**, FK внутри таблицы включать только когда треды в скоупе             |
| `forward_from_id`     | `UUID`        | NULL — отложено                                                                                    |
| `forward_from_sender` | `TEXT`        | NULL — отложено                                                                                    |
| `attachments`         | `JSONB`       | NULL — в v1 обычно NULL (вложения Фаза 3+)                                                         |
| `mentions`            | `JSONB`       | NULL — опционально для @ в тексте                                                                  |
| `edited_at`           | `TIMESTAMPTZ` | NULL                                                                                               |
| `deleted_at`          | `TIMESTAMPTZ` | NULL — soft delete ([PLAN.md](../PLAN.md) Фаза 1)                                                  |
| `created_at`          | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                                            |

**Индексы**

- `PRIMARY KEY (id)`
- **`INDEX (chat_id, id DESC)`** — история и курсор `GetMessages` per `chat_id` (порядок по времени задаёт UUIDv7) ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md))
- Частичный индекс для «не удалённые»: опционально `INDEX (chat_id, id DESC) WHERE deleted_at IS NULL` если запросы всегда исключают удалённые

**FK внутри БД:** при включении тредов — `thread_parent_id REFERENCES messages(id)` отдельной миграцией.

---

### `read_receipts`

Последнее прочитанное сообщение участника в чате.

| Колонка                | Тип           | Ограничения / заметки                                                              |
|------------------------|---------------|------------------------------------------------------------------------------------|
| `chat_id`              | `UUID`        | NOT NULL                                                                           |
| `profile_id`           | `UUID`        | NOT NULL                                                                           |
| `last_read_message_id` | `UUID`        | NOT NULL — должен указывать на сообщение с тем же `chat_id` (инвариант приложения) |
| `updated_at`           | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                            |

**Индексы и ограничения**

- `PRIMARY KEY (chat_id, profile_id)`
- `INDEX (profile_id)` — bulk read state для списка диалогов

---

## Не создаём в первой волне

Таблицы **`reactions`**, **`pins`** — по [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) §2; добавить позже expand-миграцией.

---

## Отложено после v1

- Шардирование по `chat_id` при объёме >100M сообщений ([messaging-service.md](../microservices/messaging-service.md)).
- Полнотекст / поиск — `search_db`, не эта БД.
- Таблица **`message_attachments`** (Shared Media) — в целевой схеме [target/messaging_db.md](target/messaging_db.md); в v1 достаточно `attachments` JSONB при необходимости API.


