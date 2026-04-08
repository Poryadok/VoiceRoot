# Messaging Service

## Обзор

CRUD сообщений для всех типов чатов (DM, группы, каналы пространств). Треды, реакции, пины, пересылка, read receipts.

**Язык**: Go
**БД**: PostgreSQL `messaging_db`

## Ответственность

- Отправка, редактирование, удаление сообщений
- Форматирование (Discord-style markdown)
- Треды (DM replies, channel threads)
- Реакции (emoji)
- Пины
- Пересылка сообщений (с атрибуцией и без)
- @mentions (user, role, @everyone, @here)
- Read receipts (последнее прочитанное сообщение на пользователя на чат)
- Вложения (ссылки на File Service)
- Лимит 4000 символов
- Догрузка истории после offline / reconnect: **per `chat_id`** через `GetMessages` с курсором (`after_message_id` / `last_message_id`); правила fallback — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). Не путать с полем **`s`** в WebSocket Gateway (Realtime) — это нумерация live-событий, не курсор БД

## API (gRPC)

```protobuf
service MessagingService {
  // Сообщения
  rpc SendMessage(SendMessageRequest) returns (Message);
  rpc EditMessage(EditMessageRequest) returns (Message);
  rpc DeleteMessage(DeleteMessageRequest) returns (Empty);
  rpc GetMessages(GetMessagesRequest) returns (MessageList); // paginated, cursor-based
  rpc GetMessage(GetMessageRequest) returns (Message);

  // Треды
  rpc GetThreadMessages(GetThreadRequest) returns (MessageList);

  // Реакции
  rpc AddReaction(ReactionRequest) returns (Empty);
  rpc RemoveReaction(ReactionRequest) returns (Empty);

  // Пины
  rpc PinMessage(PinRequest) returns (Empty);
  rpc UnpinMessage(PinRequest) returns (Empty);
  rpc GetPinnedMessages(GetPinnedRequest) returns (MessageList);

  // Пересылка
  rpc ForwardMessage(ForwardRequest) returns (Message);

  // Read receipts
  rpc MarkRead(MarkReadRequest) returns (Empty);
  rpc GetReadState(GetReadStateRequest) returns (ReadState);
  rpc GetBulkReadState(GetBulkReadStateRequest) returns (BulkReadState);
}
```

## Модель данных

```
messages
├── id (UUID, UUIDv7)
├── chat_id (всегда chat_db.chats.id для dm | group | channel)
├── chat_type (dm | group | channel)
├── sender_profile_id (всегда реальный автор-профиль)
├── posted_as_channel (bool, default false)
├── display_channel_id (nullable, chats.id; обязателен при posted_as_channel=true)
├── content (text, 4000 chars)
├── type (regular | system | forward)
├── thread_parent_id (nullable, FK → messages)
├── forward_from_id (nullable, FK → messages)
├── forward_from_sender (nullable, display name)
├── attachments (jsonb — [{file_id, type, url, preview_url}])
├── mentions (jsonb — [{type, target_id}])
├── edited_at (nullable)
├── deleted_at (nullable, soft delete)
├── created_at
└── INDEX(chat_id, id DESC)

reactions
├── message_id (FK)
├── profile_id (FK)
├── emoji (string)
├── created_at
└── UNIQUE(message_id, profile_id, emoji)

pins
├── chat_id
├── message_id (FK)
├── pinned_by (profile_id)
├── pinned_at
└── UNIQUE(chat_id, message_id)

message_attachments (Shared Media — целевая схема, см. docs/data/target/messaging_db.md)
├── id
├── message_id (FK → messages)
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

Правило для канала: в аудитном следе и правах всегда используется `sender_profile_id`; отображение "пост от имени канала" задаётся через `posted_as_channel=true` и `display_channel_id=<channel chats.id>`.

## Публикуемые события (→ NATS)

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
- **Role Service** — проверка прав в спейсе для **текстового канала** (`channel_id` = `chats.id`) и согласованных сценариев для группы в спейсе (если включено в продукт)
- **Social Service** — проверка блокировок
- **File Service** — привязка вложений
- **Realtime Service** — (через NATS) уведомление о новых сообщениях для WebSocket fan-out

## Масштабирование

При >100M сообщений — шардинг PostgreSQL по `chat_id` (consistent hashing). Каждый шард содержит все сообщения одного чата → локальные запросы без cross-shard joins.


