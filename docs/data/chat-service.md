# Chat Service — `chat_db` (v1)

Владелец: Chat Service ([microservices/chat-service.md](../microservices/chat-service.md)). Скоуп v1: [DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) — **тип чата `dm`**, группы и папки сознательно не обязательны в первой миграции.

`creator_profile_id` и `profile_id` в участниках — UUID из `user_db`; **FK наружу нет**.

Текстовые каналы — строки **`chats`** с `type = channel` (в т.ч. вне спейса и с `space_id`); сообщения в Messaging с тем же `chat_id`. Плейсмент в дереве спейса — `space_db.space_text_channel_placements`. См. [DATA_MODEL.md](../DATA_MODEL.md) и [data/target/chat_db.md](target/chat_db.md).

---

## Таблицы

### `chats`

| Колонка              | Тип           | Ограничения / заметки                                                                                    |
|----------------------|---------------|----------------------------------------------------------------------------------------------------------|
| `id`                 | `UUID`        | PK                                                                                                       |
| `type`               | `TEXT`        | NOT NULL, `CHECK (type IN ('dm', 'group', 'channel'))` — в v1 создаём только `dm`; `group` / `channel` зарезервированы под [data/target/chat_db.md](target/chat_db.md) |
| `name`               | `TEXT`        | NULL — для DM обычно NULL (отображаемое имя с клиента / User)                                            |
| `avatar_url`         | `TEXT`        | NULL                                                                                                     |
| `creator_profile_id` | `UUID`        | NOT NULL — кто инициировал создание пары                                                                 |
| `slow_mode_seconds`  | `INT`         | NOT NULL, DEFAULT 0, `CHECK (slow_mode_seconds >= 0)`                                                    |
| `last_message_at`    | `TIMESTAMPTZ` | NULL — обновляется по событию из Messaging / приложения                                                  |
| `created_at`         | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                                                  |
| `updated_at`         | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                                                  |

**Индексы**

- `PRIMARY KEY (id)`
- `INDEX (last_message_at DESC NULLS LAST)` — сортировка списка чатов (в связке с членством)

---

### `chat_members`

| Колонка       | Тип           | Ограничения / заметки                                                                                     |
|---------------|---------------|-----------------------------------------------------------------------------------------------------------|
| `chat_id`     | `UUID`        | NOT NULL, **FK** `REFERENCES chats(id) ON DELETE CASCADE`                                                 |
| `profile_id`  | `UUID`        | NOT NULL                                                                                                  |
| `role`        | `TEXT`        | NOT NULL, `CHECK (role IN ('owner', 'admin', 'member'))` — для DM достаточно `member`/`owner` по политике |
| `joined_at`   | `TIMESTAMPTZ` | NOT NULL, DEFAULT now()                                                                                   |
| `muted_until` | `TIMESTAMPTZ` | NULL                                                                                                      |
| `is_archived` | `BOOLEAN`     | NOT NULL, DEFAULT false                                                                                   |

**Индексы и ограничения**

- `PRIMARY KEY (chat_id, profile_id)` или суррогат `id` + `UNIQUE (chat_id, profile_id)` — предпочтительно **составной PK** для простоты.
- `INDEX (profile_id, is_archived, last_message_at)` — не хранится здесь; для списка чатов пользователя:  
  `INDEX (profile_id)` + join на `chats` по `last_message_at` (или денормализация превью позже).

**Уникальность DM (ровно два участника, один чат на пару):** обеспечивается **прикладным слоем** (идемпотентный `GetOrCreateDM`) или отдельной таблицей пар в будущем; в сырой схеме v1 достаточно правила в Chat Service.

---

## Отложено после v1 (отдельные миграции)

- **`folders`**, **`folder_chats`** — когда появится UX папок в плане или клиенте ([DATA_SCOPE_V1.md](../DATA_SCOPE_V1.md) §2).
- Поля групп (имя, лимиты участников) — активация при Фазе 4–5.
- Индекс/таблица для стабильного «найти DM между двумя профилями» без скана — при росте нагрузки.


