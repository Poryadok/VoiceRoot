# Messaging Service

## Обзор

CRUD сообщений для всех типов чатов (DM, текстовые группы и каналы — как в спейсе, так и standalone). Треды, реакции, пины, пересылка, read receipts.

**Язык**: Go
**БД**: PostgreSQL `messaging_db`

## Ответственность

- Отправка, редактирование, удаление сообщений
- Форматирование (markdown-подмножество, см. [text-chat.md](../features/text-chat.md))
- Треды (DM replies, channel threads)
- Реакции (emoji)
- Пины
- Пересылка сообщений (с атрибуцией и без)
- @mentions (user, role; broadcast в чате — `@everyone` / `@here` в UX при наличии `TEXT_CHAT_MENTION_ALL_IN_CHAT` / `TEXT_CHAT_MENTION_ALL_ONLINE`)
- Read receipts (последнее прочитанное сообщение на пользователя на чат)
- Вложения (ссылки на File Service)
- Лимит 4000 символов
- Догрузка истории после offline / reconnect: **per `chat_id`** через `GetMessages` с курсором (`after_message_id` / `last_message_id`); правила fallback — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). Не путать с полем **`s`** в WebSocket Gateway (Realtime) — это нумерация live-событий, не курсор БД

### Идемпотентность отправки

`SendMessage` принимает опциональный **`client_message_id`** (UUID), уникальный в разрезе **`(chat_id, sender_profile_id)`** (в proto — пара `chat` + идентичность отправителя из контекста запроса). Повтор запроса с тем же ключом **не создаёт** вторую строку в `messages`. Нормативная семантика: **gRPC `OK`** и тело **`SendMessageResponse` с тем же `Message`**, что и при первом успешном сохранении (тот же `id` и полезная нагрузка). Код **`ALREADY_EXISTS`** для этого сценария **не** используем — один канонический идемпотентный успех. Без ключа при сетевых ретраях возможны дубликаты.

## API (gRPC)

Источник истины по RPC и сообщениям: [protos/voice/messaging/v1/messaging.proto](../../protos/voice/messaging/v1/messaging.proto). Ниже — краткая схема для навигации по документу (имена типов как в репозитории).

```protobuf
service MessagingService {
  rpc SendMessage(SendMessageRequest) returns (SendMessageResponse);
  rpc EditMessage(EditMessageRequest) returns (EditMessageResponse);
  rpc DeleteMessage(DeleteMessageRequest) returns (DeleteMessageResponse);
  rpc GetMessages(GetMessagesRequest) returns (GetMessagesResponse);
  rpc GetMessage(GetMessageRequest) returns (GetMessageResponse);
  rpc GetThreadMessages(GetThreadMessagesRequest) returns (GetThreadMessagesResponse);
  rpc AddReaction(AddReactionRequest) returns (AddReactionResponse);
  rpc RemoveReaction(RemoveReactionRequest) returns (RemoveReactionResponse);
  rpc PinMessage(PinMessageRequest) returns (PinMessageResponse);
  rpc UnpinMessage(UnpinMessageRequest) returns (UnpinMessageResponse);
  rpc GetPinnedMessages(GetPinnedMessagesRequest) returns (GetPinnedMessagesResponse);
  rpc ForwardMessage(ForwardMessageRequest) returns (ForwardMessageResponse);
  rpc MarkRead(MarkReadRequest) returns (MarkReadResponse);
  rpc GetReadState(GetReadStateRequest) returns (GetReadStateResponse);
  rpc GetBulkReadState(GetBulkReadStateRequest) returns (GetBulkReadStateResponse); // map chat_id -> ReadState
}
```

## Модель данных

```
messages
├── id (UUID, UUIDv7)
├── chat_id (всегда chat_db.chats.id для dm | group | channel)
├── chat_type (dm | group | channel)
├── sender_profile_id (всегда реальный автор-профиль)
├── posted_as_chat (bool, default false)
├── display_chat_id (nullable, chats.id; обязателен при posted_as_chat=true)
├── content (text, 4000 chars)
├── type (regular | system | forward)
├── thread_parent_id (nullable, logical ref → messages.id)
├── forward_from_id (nullable, logical ref → messages.id)
├── forward_from_sender (nullable, display name)
├── attachments (jsonb — [{file_id, type, url, preview_url}])
├── mentions (jsonb — [{type, target_id}])
├── edited_at (nullable)
├── deleted_at (nullable, soft delete)
├── created_at
└── INDEX(chat_id, id DESC)

reactions
├── message_id (UUID, logical ref → messages.id)
├── profile_id (UUID, logical ref → user_db.profiles.id)
├── emoji (string)
├── created_at
└── UNIQUE(message_id, profile_id, emoji)

pins
├── chat_id
├── message_id (UUID, logical ref → messages.id)
├── pinned_by (UUID, logical ref → user_db.profiles.id)
├── pinned_at
└── UNIQUE(chat_id, message_id)

message_attachments (Shared Media — целевая схема в этой же секции «Модель данных»)
├── id
├── message_id (UUID, logical ref → messages.id)
├── sort_order
├── kind (image | video | audio | voice_message | document | link | other)
├── file_id (nullable)
├── external_url (nullable)
├── title (nullable)
├── duration_seconds (nullable)
└── INDEX(message_id, sort_order)

read_receipts
├── chat_id
├── profile_id
├── last_read_message_id
├── updated_at
└── UNIQUE(chat_id, profile_id)
```

### V1 (Фаза 0-1) — детальный профиль для DDL

В первой волне миграций используются только `messages` и `read_receipts`.
`reactions`, `pins`, `thread_parent_id`, `forward_*`, `message_attachments` — target-state и внедряются позже.

```
messages
├── id UUID PRIMARY KEY -- UUIDv7 генерируется приложением Messaging
├── chat_id UUID NOT NULL -- logical ref → chat_db.chats.id
├── chat_type VARCHAR(16) NOT NULL CHECK (chat_type = 'dm')
├── sender_profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── posted_as_chat BOOLEAN NOT NULL DEFAULT false CHECK (posted_as_chat = false)
├── display_chat_id UUID NULL
├── content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000)
├── type VARCHAR(16) NOT NULL DEFAULT 'regular' CHECK (type IN ('regular','system','forward'))
├── thread_parent_id UUID NULL
├── forward_from_id UUID NULL
├── forward_from_sender TEXT NULL
├── attachments JSONB NOT NULL DEFAULT '[]'::jsonb
├── mentions JSONB NOT NULL DEFAULT '[]'::jsonb
├── edited_at TIMESTAMPTZ NULL
├── deleted_at TIMESTAMPTZ NULL
└── created_at TIMESTAMPTZ NOT NULL DEFAULT now()

read_receipts
├── chat_id UUID NOT NULL -- logical ref → chat_db.chats.id
├── profile_id UUID NOT NULL -- logical ref → user_db.profiles.id
├── last_read_message_id UUID NOT NULL -- logical ref → messages.id
├── updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
└── PRIMARY KEY (chat_id, profile_id)
```

Индексы v1:
- `INDEX messages_chat_id_id_desc_idx (chat_id, id DESC)` для истории и догрузки
- `INDEX messages_sender_profile_id_idx (sender_profile_id, created_at DESC)` для модерации и профиля
- `INDEX messages_chat_id_created_at_idx (chat_id, created_at DESC)` для fallback без курсора
- `INDEX read_receipts_profile_id_idx (profile_id)` для bulk read-state

Правило для сообщений: в аудитном следе и правах всегда используется `sender_profile_id`; отображение «от имени чата» (группа или канал) — через `posted_as_chat=true` и `display_chat_id=<chats.id>` (обычно совпадает с `chat_id`). Разрешено ли так писать и в основную ленту — **настройки чата и роли**.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`message.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                  | Данные                                       |
|--------------------------|----------------------------------------------|
| `message.sent`           | message_id, chat_id, sender_id, has_mentions |
| `message.edited`         | message_id, chat_id                          |
| `message.deleted`        | message_id, chat_id                          |
| `message.reaction_added` | message_id, profile_id, emoji                |
| `message.pinned`         | message_id, chat_id, pinned_by               |
| `message.forwarded`      | message_id, source_chat_id, target_chat_id   |

## Зависимости

- **Chat Service** — валидация членства / доступа для **DM**, **группы**, **канала** (в т.ч. standalone)
- **Space Service** — если у `chats.space_id` задан спейс: членство в спейсе (для канала и группы в спейсе)
- **Role Service** — проверка прав в спейсе для текстового чата (`chat_id` = `chats.id`, `group` \| `channel`)
- **Social Service** — проверка блокировок
- **File Service** — привязка вложений
- **Realtime Service** — (через NATS) уведомление о новых сообщениях для WebSocket fan-out

## Масштабирование

При >100M сообщений — шардинг PostgreSQL по `chat_id` (consistent hashing). Каждый шард содержит все сообщения одного чата → локальные запросы без cross-shard joins.


