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
- Вложения (ссылки на File Service): photo, video, document, voice, video_note, music, article, location — см. [text-chat.md](../features/text-chat.md) § Attach menu
- Send options: `send_silent`, `scheduled_at`, `send_when_online` — контракт ниже; **not yet in proto/code** — см. [todo/backend.md](../todo/backend.md)
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
  rpc ListThreads(ListThreadsRequest) returns (ListThreadsResponse);
  rpc AddReaction(AddReactionRequest) returns (AddReactionResponse);
  rpc RemoveReaction(RemoveReactionRequest) returns (RemoveReactionResponse);
  rpc PinMessage(PinMessageRequest) returns (PinMessageResponse);
  rpc UnpinMessage(UnpinMessageRequest) returns (UnpinMessageResponse);
  rpc GetPinnedMessages(GetPinnedMessagesRequest) returns (GetPinnedMessagesResponse);
  // BOT-B: bulk unpin for bot uninstall cleanup (sender_profile_id + chat_ids).
  rpc UnpinMessagesBySenderInChats(UnpinMessagesBySenderInChatsRequest) returns (UnpinMessagesBySenderInChatsResponse);

  rpc ForwardMessage(ForwardMessageRequest) returns (ForwardMessageResponse);

  rpc MarkRead(MarkReadRequest) returns (MarkReadResponse);
  rpc GetReadState(GetReadStateRequest) returns (GetReadStateResponse);
  rpc GetBulkReadState(GetBulkReadStateRequest) returns (GetBulkReadStateResponse);
  rpc GetChatListMetadata(GetChatListMetadataRequest) returns (GetChatListMetadataResponse); // S2S: preview + unread for Chat ListChats
  rpc ListSharedMedia(ListSharedMediaRequest) returns (ListSharedMediaResponse); // media / files / links / voice tabs in chat info

  // Signal pre-key directory for DM E2E — docs/features/encryption.md.
  rpc UploadPreKeyBundle(UploadPreKeyBundleRequest) returns (UploadPreKeyBundleResponse);
  rpc GetPreKeyBundle(GetPreKeyBundleRequest) returns (GetPreKeyBundleResponse);

  // Scheduled messages — not yet in proto
  // rpc ListScheduledMessages / CancelScheduledMessage / SendScheduledMessageNow / UpdateScheduledMessage
}
```

### Shipped RPC status

| RPC | Handler | Notes |
|-----|---------|-------|
| `GetThreadMessages` | ✓ | thread replies |
| `ListThreads` | ✓ | channel thread index |
| `PinMessage` / `UnpinMessage` / `GetPinnedMessages` | ✓ | limit **5**/chat (spec); code `MaxPinsPerChat=50` — backlog |
| `UnpinMessagesBySenderInChats` | ✓ | bot cleanup |
| `UploadPreKeyBundle` / `GetPreKeyBundle` | ✓ | DM E2E pre-keys |
| `MarkRead` / `GetReadState` / `GetBulkReadState` | ✓ | DM-typed validation today |
| `GetChatListMetadata` | ✓ partial | preview text + unread only |
| `SendMessage` send options (`send_silent`, `scheduled_at`, `send_when_online`) | ✗ | spec below; not in proto |
| `UpdateScheduledMessage` | ✗ | spec below |

### `SendMessageRequest` (spec)

Поля поверх текущего proto (**not yet in proto/code** — [todo/backend.md](../todo/backend.md)):

| Поле | Тип | Семантика |
|------|-----|-----------|
| `client_message_id` | UUID, optional | Идемпотентность — см. выше |
| `send_silent` | bool, default false | Push без звука; Notification Service читает флаг из события `message.sent` |
| `scheduled_at` | `google.protobuf.Timestamp`, optional | Отложенная отправка; см. § Scheduled messages |
| `send_when_online` | bool, default false | Только **DM**: держать в очереди до `online` у получателя; игнорируется если задан `scheduled_at` |
| `content_type` | enum, optional | Тип полезной нагрузки для preview / shared media — см. § Content types |
| `content_payload` | `google.protobuf.Struct` или typed oneof | Структурированное тело для `article` / `location` / `video_note` / `music` |

**Правила:**

- `scheduled_at` и `send_when_online=true` **взаимоисключающие** (валидация `INVALID_ARGUMENT`).
- `send_when_online` только для `chat_type = dm`; для group/channel — `INVALID_ARGUMENT`.
- Timezone: клиент передаёт **UTC instant**; UI показывает локаль профиля отправителя.
- Silent + scheduled: silent применяется в момент фактической отправки worker'ом.

### Content types

Расширение `MessageKind` **или** отдельное поле `content_type` на `Message` / `SendMessageRequest`. Канон UX — [text-chat.md](../features/text-chat.md) § Attach menu.

```protobuf
enum MessageContentType {
  MESSAGE_CONTENT_TYPE_UNSPECIFIED = 0;
  MESSAGE_CONTENT_TYPE_TEXT = 1;      // plain / markdown text in `content`
  MESSAGE_CONTENT_TYPE_PHOTO = 2;
  MESSAGE_CONTENT_TYPE_VIDEO = 3;
  MESSAGE_CONTENT_TYPE_DOCUMENT = 4;
  MESSAGE_CONTENT_TYPE_VOICE = 5;
  MESSAGE_CONTENT_TYPE_STICKER = 6;
  MESSAGE_CONTENT_TYPE_GIF = 7;
  MESSAGE_CONTENT_TYPE_ARTICLE = 8;
  MESSAGE_CONTENT_TYPE_LOCATION = 9;
  MESSAGE_CONTENT_TYPE_VIDEO_NOTE = 10;
  MESSAGE_CONTENT_TYPE_MUSIC = 11;
}
```

**Payload sketches** (хранение: `messages.attachments` JSONB до миграции `message_attachments`; нормативная форма — `kind` + поля ниже):

| `content_type` | Payload (JSON sketch) | Валидация / лимиты |
|----------------|----------------------|---------------------|
| `article` | `{ "url", "title", "description", "thumb_file_id?", "instant_view_html?" }` | URL https-only; OG/metadata fetch — **not yet implemented** (owner TBD: Gateway vs File vs worker) |
| `location` | `{ "lat", "lon", "label?", "static_map_file_id?" }` | lat ∈ [-90,90], lon ∈ [-180,180]; static map via File Service |
| `video_note` | `{ "file_id", "duration_seconds", "width", "height" }` | duration ≤ **60 s** ([file-storage.md](../features/file-storage.md)); round crop on File Service |
| `music` | `{ "file_id", "title?", "artist?", "album?", "duration_seconds?" }` | Metadata: File Service extract on upload; Messaging stores canonical copy on message |

`attachments_json` в текущем коде — opaque; spec требует typed enum + validated payload (**not yet in proto/code**).

### Scheduled messages (lifecycle)

**Storage:** таблица `scheduled_messages` в `messaging_db` (**not yet in proto/code**):

```
scheduled_messages
├── id (UUID)
├── chat_id, sender_profile_id
├── payload (jsonb — mirror SendMessage fields minus schedule flags)
├── scheduled_at (TIMESTAMPTZ, UTC)
├── send_when_online (bool)
├── status (pending | sent | cancelled)
├── created_at, updated_at
└── UNIQUE(chat_id, sender_profile_id, client_message_id) WHERE client_message_id IS NOT NULL
```

| Операция | Поведение |
|----------|-----------|
| **Create** | `SendMessage` с `scheduled_at` и/или `send_when_online` → row `pending`; **не** публикует `message.sent` до dispatch |
| **Worker dispatch** | Cron/worker каждые ~30s: `scheduled_at <= now()` OR (`send_when_online` AND recipient `online` via User `GetPresence`) → insert `messages`, publish `message.sent`, mark `sent` |
| **Cancel** | `CancelScheduledMessage` → `cancelled`; race с worker: first commit wins; повтор cancel → `NOT_FOUND` / idempotent OK |
| **Send now** | `SendScheduledMessageNow` → немедленный dispatch |
| **Edit** | `UpdateScheduledMessage` — replace `payload` / `scheduled_at` while `status=pending`; reject if worker already dispatched |
| **Max horizon** | **365 days** от `now()`; beyond → `INVALID_ARGUMENT` |
| **Invisible sender** | Отправитель `invisible` **не** блокирует dispatch scheduled; `send_when_online` ждёт **получателя** |

Composer UX — [text-chat.md](../features/text-chat.md) § Send options; strip — [screen-controls.md](../design/screen-controls.md) §3.6 #13–17.

### `UpdateScheduledMessage` (spec — not yet in proto)

```protobuf
rpc UpdateScheduledMessage(UpdateScheduledMessageRequest) returns (UpdateScheduledMessageResponse);

message UpdateScheduledMessageRequest {
  string scheduled_message_id = 1;
  // Fields mirror SendMessage payload; only applied while status=pending.
  optional string content = 2;
  optional google.protobuf.Struct content_payload = 3;
  optional google.protobuf.Timestamp scheduled_at = 4;
}
```

**Rules:** `NOT_FOUND` if id missing or not owned by caller; `FAILED_PRECONDITION` if `status != pending` (worker race); `INVALID_ARGUMENT` for horizon / mutual exclusion with `send_when_online` (same as create). Successful edit does **not** publish `message.sent` until dispatch.

### `GetChatListMetadata` / `ChatListItem` preview

S2S enrichment для Chat `ListChats`. **Код сегодня:** только `last_message_preview` text + `unread_count`.

**Spec fields** на `ChatListMetadata` (per `chat_id`; **not yet in proto/code**):

| Поле | Тип | Назначение |
|------|-----|------------|
| `last_message_preview` | string | Текст или **media label** (см. ниже) |
| `last_message_content_type` | `MessageContentType` | Для client-side label без парсинга текста |
| `last_message_is_outgoing` | bool | Последнее сообщение от `profile_id` запроса |
| `last_message_delivery_state` | enum | `none` \| `sent` \| `delivered` \| `read` — **durable**, DM only |
| `unread_count` | int32 | Без изменений |
| `last_message_at` | timestamp | Для сортировки |

**Media label rules** (server-side string when no text body) — precedence как [text-chat.md](../features/text-chat.md) § Preview:

`Photo` \| `Video` \| `Voice` \| `File` \| `Sticker` \| `GIF` \| `Article` \| `Location` \| `Music` \| `Video message` (video note) \| call labels.

**Ownership delivery state:**

| Layer | Роль |
|-------|------|
| **Messaging** | Durable owner: `read_receipts`, derivation of `last_message_delivery_state` for list metadata |
| **Realtime** | Ephemeral WS `delivery_ack` → `message_delivered` fan-out (incl. Redis cross-instance); **не** источник истины для list preview |
| **Chat** | Merge Messaging metadata в `ChatListItem` при `ListChats` |

**Delivery state matrix** (DM list ticks):

| State | Durable (`GetChatListMetadata`) | Ephemeral (WS) |
|-------|--------------------------------|----------------|
| Sent | outgoing row exists; peer offline / no ack yet | — |
| Delivered | derived from peer read cursor **or** last known delivery ack persisted (spec) | client `delivery_ack` → server `message_delivered` to sender devices |
| Read | `read_receipts.last_read_message_id` | `MarkRead` REST → NATS → WS `message_read`; also client WS `mark_read` fan-out |

**MarkRead dual path:** (1) **Persist** — `Messaging.MarkRead` gRPC/REST writes `read_receipts`, publishes `message.read` on `message.events`; (2) **Fan-out** — Realtime consumes `message.read` → WS `message_read` to chat subscribers; client may also send WS `mark_read` for same-profile multi-device sync (Realtime → Redis → other tabs). List UI **must** refresh metadata after reconnect — not infer ticks from WS alone.

После reconnect клиент **перезапрашивает** `ListChats` / metadata, а не восстанавливает ticks из WS alone ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md)).

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

**Лимит pins:** не более **5** закреплённых сообщений на один `chat_id` (как Telegram). Повторный pin того же сообщения идемпотентен (обновляет `pinned_at` / `pinned_by`).

> **Code gap:** `MaxPinsPerChat = 50` в коде и migration `000006_pins` — bug; align to **5** ([todo/backend.md](../todo/backend.md)).

message_attachments (Shared Media — spec; not yet in proto/code)
├── id
├── message_id (UUID, logical ref → messages.id)
├── sort_order
├── kind (image | video | audio | voice_message | video_note | document | article | location | link | music | other)
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

### Current code (DM-only) vs full spec

**Deployed migrations** используют только `messages` и `read_receipts`.
`reactions`, `pins`, `thread_parent_id`, `forward_*`, `message_attachments` — **not yet in proto/code**.

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

Индексы:
- `INDEX messages_chat_id_id_desc_idx (chat_id, id DESC)` для истории и догрузки
- `INDEX messages_sender_profile_id_idx (sender_profile_id, created_at DESC)` для модерации и профиля
- `INDEX messages_chat_id_created_at_idx (chat_id, created_at DESC)` для fallback без курсора
- `INDEX read_receipts_profile_id_idx (profile_id)` для bulk read-state

Правило для сообщений: в аудитном следе и правах всегда используется `sender_profile_id`; отображение «от имени чата» (группа или канал) — через `posted_as_chat=true` и `display_chat_id=<chats.id>` (обычно совпадает с `chat_id`). Разрешено ли так писать и в основную ленту — **настройки чата и роли**.

## Конфигурация (NATS / JetStream)

- **`NATS_URL`** — URL NATS Server с включённым JetStream (порт клиента по умолчанию **4222**). В Docker Compose (внутренняя сеть): `nats://nats:4222`. С хоста при пробросе портов из [`docker-compose.yml`](../../docker-compose.yml): `nats://127.0.0.1:${NATS_PORT:-4222}`.
- Доменный поток публикации сообщений: **`message.events`** — см. ниже и [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`message.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                  | Данные                                       |
|--------------------------|----------------------------------------------|
| `message.sent`           | message_id, chat_id, sender_id, has_mentions, **content_type**, **send_silent**, **was_scheduled** (bool), **scheduled_at** (nullable — original intent time) |
| `message.read`           | chat_id, profile_id, last_read_message_id    |
| `message.mention_added`  | message_id, chat_id, sender_id, mentioned_profile_ids |
| `message.edited`         | message_id, chat_id                          |
| `message.deleted`        | message_id, chat_id                          |
| `message.reaction_added` | message_id, profile_id, emoji                |
| `message.pinned`         | message_id, chat_id, pinned_by               |
| `message.unpinned`       | message_id, chat_id, unpinned_by             |
| `message.forwarded`      | message_id, source_chat_id, target_chat_id   |

**`message.sent` notes:** `send_silent` drives Notification push policy. `was_scheduled` / `scheduled_at` — audit и client strip cleanup. **Code gap:** JetStream proto сегодня без silent/schedule/content_type — **not yet in proto** (`jetstream_events.proto`).

**`message.read`:** публикуется при `MarkRead`; Realtime fan-out как WS `message_read`. **Code-ok**.

### File → Messaging preview refresh (spec)

When File Service finishes async processing (`file.processed` on JetStream), Messaging **must** refresh list/history metadata for messages referencing that `file_id`:

| Step | Owner |
|------|-------|
| File publishes `file.processed` | `file.events` — `file_id`, `status`, derived `preview_url`, dimensions |
| Messaging consumer | Updates attachment JSON on `messages` / `message_attachments`; invalidates or recomputes `GetChatListMetadata` for affected `chat_id`s |
| Client | On `message_update` WS (or poll) refresh bubble + list row label |

**Not yet implemented** — consumer + cache invalidation contract; see [todo/backend.md](../todo/backend.md). Subject matrix: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md).

### Timestamp ownership (`chats.last_message_at`)

Chat Service writes `chats.last_message_at` on `message.sent` for sort order. **Today:** `TouchLastMessageAt` updates **DM only**; group/channel activity timestamp gap — Chat backlog. Messaging does **not** own this column; it owns preview text, unread, and durable delivery state — см. [chat-service.md](chat-service.md) § ListChats.

## Зависимости

- **Chat Service** — валидация членства / доступа для **DM**, **группы**, **канала** (в т.ч. standalone)
- **Space Service** — если у `chats.space_id` задан спейс: членство в спейсе (для канала и группы в спейсе)
- **Role Service** — проверка прав в спейсе для текстового чата (`chat_id` = `chats.id`, `group` \| `channel`)
- **Social Service** — проверка блокировок
- **File Service** — привязка вложений
- **Realtime Service** — (через NATS) уведомление о новых сообщениях для WebSocket fan-out

## Масштабирование

При >100M сообщений — шардинг PostgreSQL по `chat_id` (consistent hashing). Каждый шард содержит все сообщения одного чата → локальные запросы без cross-shard joins.

## E2E / pre-keys / edit policy ([encryption.md](../features/encryption.md))

Нормативное поведение: [encryption.md](../features/encryption.md). Поиск по E2E-телу на сервере не индексируется ([search-service.md](search-service.md)).

### gRPC (дополнение к `MessagingService`)

| RPC | Назначение |
|-----|------------|
| `UploadPreKeyBundle` | Сохранить Signal pre-key bundle для `sender_profile_id` из контекста |
| `GetPreKeyBundle` | Выдать bundle собеседника; при fetch один OTPK потребляется из пула |

REST через Gateway: `POST/GET /api/v1/messages/prekeys` (см. [api-gateway.md](api-gateway.md)).

### Модель и миграции

- `messages.is_e2e` — ciphertext-only payload при `true`
- `e2e_prekey_bundles` — opaque wire bundle per `profile_id` (`messaging_db` migration `000009_e2e`)

### Политика отправки (`validateE2ESend`)

- Только **DM** (`chat_type = dm`)
- Когда у чата `e2e_enabled = true`: `is_e2e` обязателен, `content` — opaque ciphertext
- Plaintext send отклоняется после включения E2E в чате

### Политика редактирования (`validateE2EEdit`)

- Plaintext edit **запрещён**, если чат `e2e_enabled`
- Сообщение с `is_e2e = true`: разрешён edit с новым ciphertext (тот же `message_id`)
- Событие `message.edited` в NATS несёт `is_e2e` — Search indexer пропускает E2E upsert

### Лимиты (Gateway / сервис)

| Маршрут / операция | Лимит (дефолт Gateway) |
|--------------------|-------------------------|
| `PUT /api/v1/auth/e2e-key-backup` | 5 / мин на user (Auth Service) |
| `GET /api/v1/auth/e2e-key-backup` | 30 / мин |
| `POST /api/v1/messages/prekeys` | 10 / мин |
| `GET /api/v1/messages/prekeys` | 60 / мин |
| Max pre-key bundle wire | 64 KiB (Messaging) |
| Max key backup blob | 512 KiB (Auth) |

Key backup хранится в **Auth Service** (`PutE2EKeyBackup` / `GetE2EKeyBackup`), не в Messaging.

Включение E2E в DM — **Chat Service** (`E2EPreKeyGate`: оба участника должны иметь pre-key bundle).

