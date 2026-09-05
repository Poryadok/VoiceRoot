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
- Stickers / GIF — `content_type=STICKER|GIF` + File `file_id`; composer **😊 panel only** (не 📎 attach) — § Stickers and GIF
- Send options: `send_silent`, `scheduled_at`, `send_when_online` — контракт ниже; **not yet in proto/code** — см. [todo/backend.md](../todo/backend.md)
- Лимит 4000 символов
- Догрузка истории после offline / reconnect: сначала глобальная сверка inbox через Chat `ListChats`, затем **per `chat_id`** через `GetMessages` с курсором (`after_message_id` / `last_message_id`) для выбранного чата; правила fallback — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). Не путать с полем **`s`** в WebSocket Gateway (Realtime) — это нумерация live-событий, не курсор БД

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
  rpc ListSharedMedia(ListSharedMediaRequest) returns (ListSharedMediaResponse); // media / stickers / files / links / voice tabs in chat info

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
| `PinMessage` / `UnpinMessage` / `GetPinnedMessages` | ✓ | limit **5**/chat (`MaxPinsPerChat`); 6th → `ResourceExhausted` |
| `UnpinMessagesBySenderInChats` | ✓ | bot cleanup |
| `UploadPreKeyBundle` / `GetPreKeyBundle` | ✓ | DM E2E pre-keys |
| `MarkRead` / `GetReadState` / `GetBulkReadState` | ✓ | DM/group/channel; per-member read cursor; non-members → `PERMISSION_DENIED` |
| `GetChatListMetadata` | ✓ | per-member unread + preview/content metadata; DM-only delivery ticks; non-members → `PERMISSION_DENIED` |
| `ListSharedMedia` | ✓ | shared media tabs in chat info |
| `DeleteMessage` (`DeleteScope.FOR_ME` / `FOR_EVERYONE`) | ✓ | `FOR_ME` soft-hides for caller only |
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
| `article` | `{ "url", "title", "description", "thumb_file_id?", "instant_view_html?" }` | URL https-only; OG/metadata fetch — **Messaging background worker**, not Gateway/File/client; implementation pending |
| `location` | `{ "lat", "lon", "label?", "static_map_file_id?" }` | lat ∈ [-90,90], lon ∈ [-180,180]; static map via File Service |
| `video_note` | `{ "file_id", "duration_seconds", "width", "height" }` | duration ≤ **60 s** ([file-storage.md](../features/file-storage.md)); round crop on File Service |
| `music` | `{ "file_id", "title?", "artist?", "album?", "duration_seconds?" }` | Metadata: File Service extract on upload; Messaging stores canonical copy on message |
| `sticker` | `{ "pack_id", "sticker_id", "file_id", "emoji?", "width", "height" }` | `pack_id`/`sticker_id` must exist in sender's installed packs (Chat catalog); `file_id` must match sticker row; no `content` body |
| `gif` | `{ "file_id", "provider?", "provider_id?", "preview_url?", "width?", "height?", "duration_seconds?" }` | `file_id` required (File Service `intent=gif`); provider fields for dedup/attribution when from Giphy/Tenor |

`attachments_json` в текущем коде — opaque; spec требует typed enum + validated payload (**not yet in proto/code**). **`sticker` / `gif` payloads do not require generic `attachments_json`** — `content_type` + `content_payload` is canonical; legacy clients may mirror `file_id` in `attachments` for transition.

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

Composer UX — [text-chat.md](../features/text-chat.md) § Send options; strip — [screen-controls.md](../design/screen-controls.md) §3.6 #13–17. Cross-service summary — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Push-уведомления (send options).

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

S2S enrichment для Chat `ListChats`. Код возвращает per-member unread state и preview metadata для DM/group/channel.

**Shipped fields** на `ChatListMetadata` (per `chat_id`):

| Поле | Тип | Назначение |
|------|-----|------------|
| `last_message_preview` | string | Текст или **media label** (см. ниже) |
| `last_message_content_type` | `MessageContentType` | Для client-side label без парсинга текста |
| `last_message_is_outgoing` | bool | Последнее сообщение от `profile_id` запроса |
| `last_message_delivery_state` | enum | `none` \| `sent` \| `delivered` \| `read` — **durable**, DM only; group/channel возвращают `none` |
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

**Delivery state matrix** (DM list ticks — [text-chat.md](../features/text-chat.md) § «Статусы доставки», [screen-controls.md](../design/screen-controls.md) §1.4 #15):

| State | Durable (`GetChatListMetadata`) | Ephemeral (WS) |
|-------|--------------------------------|----------------|
| `none` | not outgoing last message | — |
| `sent` | outgoing row exists; peer `last_delivered_message_id` & `last_read_message_id` both **before** this message | — |
| `delivered` | peer `last_delivered_message_id` ≥ message id **or** peer `last_read_message_id` ≥ message id | client `delivery_ack` → `message_delivered` to sender devices (+ Redis cross-instance) |
| `read` | peer `last_read_message_id` ≥ message id (`read_receipts`) | `MarkRead` REST → NATS `message.read` → WS `message_read`; client WS `mark_read` for same-profile tabs |

**Не смешивать:** WS `delivery_ack` / `message_delivered` — live fan-out для открытого bubble; list preview ticks — **только** из durable metadata после `ListChats` / `GetChatListMetadata`. WS alone после reconnect **недостаточен**.

### Durable delivery derivation (shipped for DM list ticks)

**Owner:** Messaging. **Not** Realtime Redis fan-out.

**Storage** — расширение `read_receipts` (preferred) или отдельная таблица `delivery_cursors`:

```
read_receipts (extended)
├── chat_id
├── profile_id
├── last_read_message_id
├── last_delivered_message_id   -- NEW: max message_id peer acked via delivery_ack
├── updated_at
└── UNIQUE(chat_id, profile_id)
```

| Step | Trigger | Persist |
|------|---------|---------|
| 1 | Recipient client sends WS `delivery_ack` | Realtime: ephemeral `message_delivered` fan-out **and** publish JetStream `message.delivery_ack` on `message.events` (Realtime duty — [realtime-service.md](realtime-service.md) § Ответственность) |
| 2 | Messaging consumer | `UPSERT` `last_delivered_message_id = GREATEST(existing, acked_id)` for `(chat_id, recipient_profile_id)` |
| 3 | `GetChatListMetadata` | For outgoing last message in DM: compare peer cursors → `last_message_delivery_state` enum |

**Ordering:** `last_delivered_message_id` ≤ `last_read_message_id` always (read implies delivered). Client may skip explicit `delivery_ack` if it immediately `MarkRead`s — server promotes delivered cursor to read cursor.

**Shipped:** `last_delivered_message_id` is persisted in `read_receipts`; the Messaging consumer handles `message.delivery_ack`; `GetChatListMetadata` derives the durable DM delivery state. Group/channel delivery ticks remain `none`.

### MarkRead: REST persist vs WS fan-out

| Path | API | Persist? | Fan-out |
|------|-----|----------|---------|
| **Primary** | `Messaging.MarkRead` gRPC / `POST /api/v1/messages/read` (Gateway) | ✓ `read_receipts` | publishes `message.read` → Realtime → WS `message_read` |
| **WS-only** | Client op `mark_read` on Realtime | ✗ | `message_read` to chat subscribers + same-profile tabs via Redis |
| **Client rule** | Open chat / scroll | **Must** call REST `MarkRead` (or gRPC) for durable read cursor; WS `mark_read` optional for faster multi-tab sync |

List UI **must** complete the global `ListChats` inbox snapshot after reconnect — not infer ticks from WS alone.

**Shipped scope:** `MarkRead` / `GetReadState` / `GetBulkReadState` / `GetChatListMetadata` accept **DM/group/channel** refs, scope read/unread metadata to the caller's `profile_id`, and fail closed with gRPC `PERMISSION_DENIED` for non-members. WS `mark_read` never writes `read_receipts`. Group/channel per-message view counters remain a separate future gap; the shipped A1 contract is per-member unread/read state.

После reconnect клиент завершает paginated `ListChats` snapshot для `main` / `requests` / `archive` с Messaging metadata, а не восстанавливает ticks из WS ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Reconnect). Полные сообщения остаются `GetMessages` per selected `chat_id`.

### Удалённый DM peer: durable recovery выбранной истории

Для уже известного и выбранного DM `GetMessagesResponse.dm_peer_state` — response-level состояние второго участника: `ACTIVE` или `DELETED`; для не-DM и legacy producer остаётся `UNSPECIFIED`. При `DELETED` `message_list`, его cursor и существующая история не меняются. Messaging не создаёт system `Message`, tombstone author или иной persistent marker.

Клиент добавляет ровно один локальный, неперсистентный, локализованный terminal marker «Пользователь удалён» для данного `chat_id` и запрещает новые DM sends. Состояние не является profile lookup: оно доступно только участнику уже известного выбранного DM и не раскрывает ID либо другие данные удалённого account/profile. После reconnect этот вызов `GetMessages` — durable recovery; WebSocket replay для marker не используется.

Полное 30-day erasure/tombstone/restore UX не является обязанностью этого контракта и остаётся A4.

### Message path matrix (REST / NATS / WS)

Компактная карта для implementers: что пишет durable store, что только fan-out ([realtime-service.md](realtime-service.md), [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § WS vs REST).

| Action | REST/gRPC write | NATS (`message.events`) | WS client op | WS server op | Durable store | Ephemeral only |
|--------|-----------------|-------------------------|--------------|--------------|---------------|----------------|
| Send | `SendMessage` | `message.sent` | — | `message_create` | `messages` | — |
| Edit | `EditMessage` | `message.edited` | — | `message_update` | `messages` | — |
| Delete | `DeleteMessage` | `message.deleted` | — | `message_delete` | soft-delete | — |
| Inbox reconnect catch-up | Chat `ListChats` snapshot + S2S `GetChatListMetadata` | — | — | — | Chat membership + Messaging metadata | — |
| History / reconnect gap | `GetMessages` (per selected `chat_id`) | — | — | — | `messages` | — |
| Deleted peer in selected DM | `GetMessages.dm_peer_state` | — | — | `dm_peer_deleted` (live acceleration only) | — | local client marker |
| Mark read (persist) | `MarkRead` | `message.read` | — | `message_read` (from NATS) | `read_receipts` | — |
| Mark read (multi-tab) | — (after REST) | — | `mark_read` | `message_read` | — | fan-out |
| Delivery ack | — | `message.delivery_ack` | `delivery_ack` | `message_delivered` | `last_delivered_message_id` | fan-out + Redis |
| List preview ✓/✓✓ | S2S `GetChatListMetadata` | — | — | — | read + delivery cursors | — |
| Reactions / pins | gRPC | `message.reaction_*`, `message.pinned` | — | `reaction_*`, `message_pinned` | `reactions` / `pins` | — |

**Rule:** only REST `MarkRead` mutates `read_receipts`. WS `mark_read` without REST leaves durable cursor stale. List ticks after reconnect — **never** from WS alone.

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

**Лимит pins:** не более **5** закреплённых сообщений на один `chat_id` (как Telegram). Повторный pin того же сообщения идемпотентен (обновляет `pinned_at` / `pinned_by`). Enforced in application (`MaxPinsPerChat`); migration comment documents the same limit.

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
├── last_delivered_message_id (durable ✓ ticks; DM list delivery state)
├── updated_at
└── UNIQUE(chat_id, profile_id)
```

### Shipped implementation notes (2026-08-28)

**Migrations shipped:** `messages`, `read_receipts`, `reactions`, `pins`, `thread_parent_id`, `forward_*`, `ghost_only` (platform shadow-ban column — **DB only**, not yet on `SendMessageRequest` proto), E2E columns.

**Handlers shipped beyond basic message CRUD:** threads (`GetThreadMessages`, `ListThreads`), reactions, pins (limit **5**/chat), per-member `MarkRead`/`GetReadState`/`GetBulkReadState` for DM/group/channel, `GetChatListMetadata` with per-member unread/preview metadata and non-member denial, `ListSharedMedia`, `DeleteMessage` with `DeleteScope.FOR_ME`, idempotent `client_message_id` on `SendMessage`, E2E pre-key RPCs.

**Gaps vs full spec:** `send_silent` / schedule / typed `content_type`, `message_attachments` table, `RecordMessageView`, `UpdateScheduledMessage` — см. § ниже и [todo/backend.md](../todo/backend.md).

**Attachment validation (code):** `validateAttachments` today requires `file_id` on each attachment — blocks normative `location` / `article` payloads without File row until validation branches on `content_type` (**code backlog**, R3-A06).

### Delete scope

```protobuf
enum DeleteScope {
  DELETE_SCOPE_UNSPECIFIED = 0;
  DELETE_SCOPE_FOR_EVERYONE = 1;
  DELETE_SCOPE_FOR_ME = 2;
}
```

| Scope | Behavior |
|-------|----------|
| `FOR_EVERYONE` | Soft-delete for all members; requires permission |
| `FOR_ME` | Soft-hide for caller only; message remains for others |

### `ghost_only` (platform moderation)

Column `messages.ghost_only` (migration `000010_ghost_only`): when true, message visible **only** to sender in history queries — shadow delivery for moderation. **Not yet** exposed on `SendMessageRequest` proto; set via internal/admin path only until productized.

### View count (`RecordMessageView` — spec, not yet in proto)

Group/channel view counts ([text-chat.md](../features/text-chat.md) § View count) require deduped per-`(message_id, profile_id)` storage:

```protobuf
rpc RecordMessageView(RecordMessageViewRequest) returns (Empty);

message RecordMessageViewRequest {
  string chat_id = 1;
  string message_id = 2;
}
```

**Rules:** idempotent first view per profile; excluded from DM; recount drops on message delete. **Not implemented** — use read cursor as interim UX until RPC lands.

### Stickers and GIF (wire contract — not yet in code)

Normative product scope: [text-chat.md](../features/text-chat.md) § Медиа и § Attach menu, [PLAN.md](../PLAN.md). **Zero implementation** today. Composer surface: **😊 panel** (Emoji \| Stickers \| GIFs) — **not** 📎 attach menu.

#### Service ownership

| Concern | Owner | Contract |
|---------|-------|----------|
| **Sticker pack catalog** | **Chat Service** (`chat_db`) | `sticker_packs`, `stickers`, `profile_installed_packs` — canonical DDL in [chat-service.md](chat-service.md) § Sticker packs |
| **Sticker / GIF bytes** | **File Service** | `RequestUpload(intent=sticker \| gif)` → WebP / MP4/WebM — [file-service.md](file-service.md) § Stickers and GIF assets |
| **GIF provider search** | **Chat Service** (HTTP adapter) | Proxies **Giphy or Tenor** (one provider at deploy); **`SearchGifsResponse.next_cursor` — single pagination owner** (File/Messaging do not paginate GIF search); rate-limit + cache trending server-side |
| **Send message** | **Messaging** | `SendMessage` + `content_type=STICKER \| GIF` + validated `content_payload` (§ Content types) |
| **Shared media** | **Messaging** `ListSharedMedia` | Five product tabs → `SharedMediaKind` enum — § `ListSharedMedia` filters; [search.md](../features/search.md) § Фильтры shared media |
| **Composer picker** | Flutter §3.6b | Tabs via Gateway REST wrapping Chat catalog + GIF search |
| **Events / preview** | Messaging → Realtime / Notification | `message.sent.content_type`; list label «Sticker» / «GIF» |

Premium ★ sticker packs (store browse) — optional after core packs; entitlement via Subscription, enforced on `InstallStickerPack`.

**Catalog DDL + Chat RPC sketches:** [chat-service.md](chat-service.md) § Sticker packs (single source of truth — do not fork table/column names here).

**Limits:** user-created pack ≤ **120** stickers; ≤ **50** installed packs per profile; sticker asset ≤ **512×512** px, ≤ **512 KB** after File processing ([file-service.md](file-service.md)).

#### Send flows

**Sticker:**

```
Client (😊 Stickers tab)
  → pick sticker_id from installed pack
  → Messaging.SendMessage {
       content_type: STICKER,
       content_payload: { pack_id, sticker_id, file_id, emoji?, width, height }
     }
  → validate: pack installed for sender; sticker.file_id matches payload
  → persist message; publish message.sent { content_type: STICKER }
```

**GIF:**

```
Client (😊 GIFs tab)
  → Chat.SearchGifs(query) → GifResult[] (file_id may be async)
  → if file_id pending: poll GetFileMetadata until status=ready
  → Messaging.SendMessage {
       content_type: GIF,
       content_payload: { file_id, provider, provider_id, preview_url?, width?, height? }
     }
  → validate: file_id exists, type=gif, status=ready
  → persist; publish message.sent { content_type: GIF }
```

User pack upload (Settings → Stickers): `CreateUserStickerPack` → per-sticker `RequestUpload(intent=sticker)` → `AddStickersToUserPack` → `InstallStickerPack` for self.

#### Validation rules (Messaging)

| Rule | STICKER | GIF |
|------|---------|-----|
| `content` body | Must be empty | Must be empty |
| `file_id` | Required; must match Chat `stickers.file_id` for `sticker_id` | Required; File row `type` compatible with gif/video |
| `pack_id` / `sticker_id` | Required; pack must be installed for sender | — |
| `provider` / `provider_id` | — | Optional but recommended when from search (attribution + dedup) |
| E2E DM | Allowed — sticker/GIF sent as opaque media refs same as photo | Same |
| Rate limit | Counts toward 5 msg / 5 s global limit | Same |

#### `ListSharedMedia` filters

Product UI has **five** tabs ([search.md](../features/search.md) § Фильтры shared media). Wire uses `ListSharedMediaRequest.kind` (`SharedMediaKind` enum) — **not** per-type string filters.

| Product tab | `SharedMediaKind` (spec) | `MessageContentType` values | Query predicate (normative) |
|-------------|--------------------------|----------------------------|----------------------------|
| **Медиа** | `SHARED_MEDIA_KIND_MEDIA` | `PHOTO`, `VIDEO`, `VIDEO_NOTE`, `GIF` | `messages.content_type IN (…)`; legacy transition: `message_attachments.kind IN (image, video, video_note, gif)` |
| **Стикеры** | `SHARED_MEDIA_KIND_STICKERS` | `STICKER` | `content_type = STICKER` |
| **Файлы** | `SHARED_MEDIA_KIND_FILES` | `DOCUMENT`, `MUSIC` | `content_type IN (DOCUMENT, MUSIC)` or attachment `kind IN (document, music)` |
| **Ссылки** | `SHARED_MEDIA_KIND_LINKS` | `TEXT`+link metadata, `ARTICLE` | `content_type = ARTICLE` or link/article attachment kinds |
| **Голосовые** | `SHARED_MEDIA_KIND_VOICE` | `VOICE` | `content_type = VOICE` or `kind = voice_message` |

**Proto sketch** (extends shipped four-value enum — **not yet in proto/code**):

```protobuf
enum SharedMediaKind {
  SHARED_MEDIA_KIND_UNSPECIFIED = 0;
  SHARED_MEDIA_KIND_MEDIA = 1;      // photo, video, video_note, gif — NOT stickers
  SHARED_MEDIA_KIND_STICKERS = 2;   // sticker tab — NEW vs shipped proto
  SHARED_MEDIA_KIND_FILES = 3;
  SHARED_MEDIA_KIND_LINKS = 4;
  SHARED_MEDIA_KIND_VOICE = 5;
}
```

**Code today:** shipped proto/handler expose four kinds only (`MEDIA`, `FILES`, `LINKS`, `VOICE`); `MEDIA` query uses `attachments[].type` `image|video` only — excludes `gif`, `video_note`, and entire **Стикеры** tab. Spec requires enum extension + `content_type`-first queries before five-tab UI is deliverable.

Sort: `created_at DESC` per message; sticker/GIF grid thumbs via File presigned URL on `file_id`.

#### Events and notifications

| Event | Fields |
|-------|--------|
| `message.sent` | `content_type=STICKER \| GIF`; `has_mentions=false` typical |
| Notification preview | Body label «Sticker» / «GIF» (no text body); push thumbnail via `preview_url` / file thumb when policy allows |

Implementation checklist — [todo/backend.md](../todo/backend.md) § Stickers/GIF. Live tests deferred until proto lands — [ADR 005](../adr/005-rich-media-live-tests-deferred.md).

### Deployed schema baseline (historical DM-only migration)

The first migration enforced `chat_type = 'dm'` only. Later migrations widened to group/channel, reactions, pins, threads. **Do not** treat the DM-only CHECK as current product scope — see shipped handlers above.

```
messages (deployed — simplified)
├── chat_type dm | group | channel
├── thread_parent_id, forward_*, attachments JSONB
├── ghost_only BOOLEAN (moderation)
└── reactions / pins in separate tables
```

## Конфигурация (NATS / JetStream)

- **`NATS_URL`** — URL NATS Server с включённым JetStream (порт клиента по умолчанию **4222**). В Docker Compose (внутренняя сеть): `nats://nats:4222`. С хоста при пробросе портов из [`docker-compose.yml`](../../docker-compose.yml): `nats://127.0.0.1:${NATS_PORT:-4222}`.
- Доменный поток публикации сообщений: **`message.events`** — см. ниже и [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`message.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                  | Данные                                       |
|--------------------------|----------------------------------------------|
| `message.sent`           | message_id, chat_id, sender_id, has_mentions, **content_type**, **send_silent**, **was_scheduled** (bool), **scheduled_at** (nullable — original intent time) |
| `message.read`           | chat_id, profile_id, last_read_message_id    |
| `message.delivery_ack`   | chat_id, profile_id, message_id (persist delivery cursor; publisher: Realtime on client `delivery_ack`) |
| `message.mention_added`  | message_id, chat_id, **sender_profile_id**, mentioned_profile_ids |
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
| File publishes `file.processed` | `file.events` — `file_id`, `status`, `converted_url`, `thumb_url`, `width`, `height`, `duration_seconds`, `intent` (aligned with [file-service.md](file-service.md) § NATS table) |
| Messaging consumer | Updates attachment JSON on `messages` / `message_attachments`; invalidates or recomputes `GetChatListMetadata` for affected `chat_id`s |
| Client | On `message_update` WS (or poll) refresh bubble + list row label |

**Not yet implemented** — consumer + cache invalidation contract; see [todo/backend.md](../todo/backend.md). Subject matrix: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md).

### Timestamp ownership (`chats.last_message_at`)

| Field / concern | Owner | Notes |
|-----------------|-------|-------|
| `chats.last_message_at` (sort key for `ListChats`) | **Chat Service** | Updated on `message.sent` consumer (`TouchLastMessageAt`) for `dm` / `group` / `channel` |
| `last_message_preview`, `unread_count` | **Messaging** | S2S `GetChatListMetadata` |
| `last_message_delivery_state`, `last_message_content_type` | **Messaging** | Durable list ticks/content metadata; not Realtime WS |
| Client list sort | **Chat `ListChats`** | Uses `chats.last_message_at`; do **not** sort client-side from Messaging preview timestamps alone |

Messaging does **not** own `chats.last_message_at`. Cross-ref — [chat-service.md](chat-service.md) § ListChats ownership table.

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
