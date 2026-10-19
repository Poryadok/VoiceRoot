# Messaging Service

## Space communication timeout (BE-044-TIMEOUT-v1)

Accepted target: apply the [Space-owned communication state](space-service.md#communication-timeout-authority-be-044-timeout-v1)
to enabled SendMessage (including reply/attachment), forward destination,
EditMessage, AddReaction, PinMessage and UnpinMessage. Derive destination Space
from authoritative Chat resource data and actor from verified effective identity;
unknown Chat scope is unavailable, never a standalone shortcut. Enabled service
ingress acting as a profile follows the same check. Existing membership, Role,
privacy, author and lifecycle ACLs remain mandatory.

Obtain a fresh signed protected Space decision for each new effect. Explicit
TIMED_OUT returns PermissionDenied/403; unknown/dependency/malformed capability
returns Unavailable/503 before message/reaction/pin/File-reference/outbox or
notification side effects. Carry one two-second monotonic budget from before the
decision request through the committing transaction; cancel/expiry rolls back.
A concurrent timeout may follow an admitted snapshot; only that bounded attempt
may finish. A new attempt must resolve again. Exact persisted no-effect receipt
replay remains allowed with no repeated side effects; changed payload conflicts.

A single ForwardMessage with commentary is one admitted mutation: commentary,
forward, all outbox records and attachment intent COMMITTED share the final local
transaction. The existing durable pre-acquire File intent stays outside it;
abandoned invisible references retain exact release reconciliation. No database
transaction spans File RPC. Expired admission cannot complete in a background
context. Forward has no public idempotency key or new cross-RPC replay guarantee.
An uncertain COMMIT/response may mean the entire pair committed; a returned error
does not prove rollback and must not trigger deletion of a committed pair.
Persistence preserves DeadlineExceeded/Canceled for expired/canceled attempts,
including wrapped errors after File acquisition or while waiting on a DB lock.
Attachment intents for keyed sends bind the normalized original request digest
and RPC before File enrichment; committed-intent replay rechecks the exact receipt
under its held connection. File-owned metadata refresh cannot erase a changed
original request or grant a new timeout exemption.

Reads/history/read state, own delete/hide and own reaction removal keep their
existing ACLs. Forward source is an ordinary read. DM/standalone/other Space or
profile are outside this timeout. Disabled scheduled handlers stay disabled;
future scheduling/dispatch must check again, while post-commit retries only
replay the receipt/outbox. This section is a target, not runtime completion.

SDK `ApplyGameMessage` follows the same resource policy for **create/edit**,
using the verified Auth execution permit `ProfileID` after resource mapping and
Chat membership checks. Own SDK delete retains its existing ACL and exemption;
moderation tombstones are cleanup. Exact SDK receipts remain first, before new
permits or dependencies. A timeout/unknown decision aborts any newly issued permit
through bounded control-plane cleanup and preserves PermissionDenied/Unavailable.
The effective commit deadline is the earliest of the two-second Space attempt,
Auth permit expiry/margin and the existing 250ms transaction deadline. Currently
provisioned managed SDK chats remain standalone groups; this contract enables no
Space-owned provisioning. Bot event publication checks the Bot-owned sender
profile supplied under its verified service contract, not a player/account owner.

The Messaging consumer uses the complete `MESSAGING_SPACE_COMMUNICATION_` tuple:
`GRPC_ADDR`, `TLS_CA_FILE`, `CLIENT_CERT_FILE`, `CLIENT_KEY_FILE`. Absent settings
leave covered Space effects unavailable; any partial or explicitly empty tuple
fails startup. The shared Messaging service signer/JWKS publisher keeps its
existing `MESSAGING_PURGE_PRINCIPAL_` key and protected JWKS settings. It may run
without File/Search purge-owner clients; configuring any purge-owner setting
still requires the complete owner tuple. No signing key, JWKS publisher or
protected Space client means no Space communication grant.

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
- Send options: `send_silent` is shipped in `SendMessageRequest`, durable `messages` storage, `message.sent` and Notification push consumption; composer remains open. P-008 ships schedule wire fields, `scheduled_messages` storage and a fail-closed send gate; handlers, worker, producer usage and UI remain open — см. [todo/backend.md](../todo/backend.md)
- Лимит 4000 кодовых точек Unicode; число UTF-8 байтов не влияет на лимит.
- Догрузка истории после offline / reconnect: сначала глобальная сверка inbox через Chat `ListChats`, затем **per `chat_id`** через `GetMessages` с курсором (`after_message_id` / `last_message_id`) для выбранного чата; правила fallback — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). Не путать с полем **`s`** в WebSocket Gateway (Realtime) — это нумерация live-событий, не курсор БД

### Durable message-event publication (BE-188 A1)

Enabled Messaging mutations persist their immutable `MessageStreamEvent` bytes,
stable event ID, contract headers, and SHA-256 in `message_event_outbox` in the
same PostgreSQL transaction as the message/edit/delete/reaction/pin/read or
receipt-revocation change. A Messaging-owned dispatcher claims rows with an
expiring lease, publishes the stored bytes with the stable NATS message ID,
and records only a positive JetStream PubAck. Uncertain publishes retain the
same bytes and retry; PubAck prunes event bytes while retaining the bounded
receipt/hash evidence. Private read-position changes do not create public read
events. Scheduled-send handlers remain fail-closed/inactive and are not
included as enabled producers.

For managed Space-chat purge, the same chat/Space mutation lock freezes the
eligible message rows and their exact outbox event ID/hash set. File/Search
purge side effects wait until every frozen event has a matching positive
PubAck, and final payload cleanup rechecks that set under row locks. This is a
Messaging-to-broker barrier only: PubAck does not prove Realtime consumer
commit. End-to-end P3 privacy closure remains pending the separate A2 Realtime
durable generation fence/participant and Space coordinator integration.

### Идемпотентность отправки

`SendMessage` принимает опциональный **`client_message_id`** (UUID), уникальный в разрезе **`(chat_id, sender_profile_id)`** (в proto — пара `chat` + идентичность отправителя из контекста запроса). Это одна namespace и для immediate `Message`, и для pending `ScheduledMessage`: повтор запроса с тем же ключом не создаёт ни вторую строку в `messages`, ни вторую scheduled row. Повтор **того же нормализованного тела** возвращает gRPC `OK` и тот же вариант `SendMessageResponse` с тем же ID; pending replay возвращает current `ScheduledMessage`, включая связанный `sent_message_id` после dispatch. Тот же ключ с другим payload, `send_silent`, schedule mode/time или immediate-vs-scheduled режимом — `ALREADY_EXISTS`; сервер не меняет уже созданный объект и не публикует новый event. Нормализация игнорирует порядок ключей JSON object, но сохраняет порядок array и включает defaults/derived type в fingerprint. Проверка immediate и scheduled путей атомарна (общий idempotency ledger либо transaction advisory lock), а не две независимые unique indexes. Поэтому запрет `ALREADY_EXISTS` относится только к корректному retry. Без ключа при сетевых ретраях возможны дубликаты.

T51 provides the Bot-only S2S publication RPC
`MessagingService.SendGameEventMessage` in the proto and handler. Its canonical
request and retry contract live in
[Game Event v1](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
Messaging owns the committed message and deduplicates by the same
`(chat_id, sender_profile_id, client_message_id)` rule. Only a verified Bot
service principal may supply the Bot-owned sender identity; public game
credentials and GIS are not message senders. The RPC must retain normal
membership, Space permission, moderation, block, content and expiry checks.

Migration `messaging_db/000030` adds private `message_send_receipts` for enabled
immediate SendMessage and Bot event sends. The key is the existing
`(chat_id, sender_profile_id, client_message_id)` namespace; each receipt binds
the exact RPC, request SHA-256 and persisted message ID. Ordinary request
fingerprints include effective defaults/derived content type and canonical JSON
object order while retaining array order. Receipt insertion commits with the
message and outbox; the transaction repeats conflict detection for concurrent
first attempts, including attachment sends. A changed retained request is
AlreadyExists. Historical dedupe keys are not backfilled with inferred request
authority and cannot bypass a fresh timeout decision. Receipts expose no new
public API and are removed by the message's physical-purge cascade.

## Game SDK message ingress (T15/T16)

`MessagingService.ApplyGameMessage` accepts the signed SDK message and Auth
device-authority assertion only from the verified Gateway service principal.
For a new operation, Messaging checks the current GIS app/environment/binding/
chat mapping before asking Auth for a short-lived execution permit. It then
checks Chat membership using the permit profile, verifies File attachment
provenance, and atomically commits the message receipt, message revision and
permit-completion outbox. Exact retained receipt retries are resolved before
fresh authority or GIS calls; changed operation content conflicts. Auth permits
expire after at most 3,750 ms (also capped by the device assertion expiry);
Messaging reserves a 250 ms clock margin and caps its atomic transaction at
250 ms. An admitted operation completes within that permit window while revoke
fences new issuance.

The mapping client calls GIS's private
`POST /internal/v1/game-integrations/resource-mappings/authorize-chat` route
over a dedicated HTTPS listener. Its JSON request contains exactly the UUID
fields `application_id`, `environment_id`, `binding_id` and `chat_id`. The
Messaging client authenticates with its mTLS URI SAN
`spiffe://voice/service/messaging` and WorkloadProof v1 request proof; GIS's
signed response is verified with the same workload key used for that request.
The client pins the configured GIS CA and endpoint hostname, has a 2-second
request timeout, and fails closed on transport, TLS, signature, response-shape,
or mapping-deny errors. `mapping_revision` is required and positive on allow.

Configure the client with all five values:
`GAME_INTEGRATION_MESSAGING_RESOURCE_MAPPING_URL`,
`MESSAGING_GAME_INTEGRATION_TLS_CERT_FILE`,
`MESSAGING_GAME_INTEGRATION_TLS_KEY_FILE`,
`GAME_INTEGRATION_MESSAGING_CA_FILE`, and
`GAME_INTEGRATION_MESSAGING_WORKLOAD_KEY_B64`. If every value is absent,
Messaging starts without the optional client and game message writes remain
fail-closed before permit issuance. Partial configuration, an invalid client
certificate/SPIFFE identity, or an invalid key fails service startup; it never
enables a pass-through mapping decision.

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

  // Scheduled message wire contract. Handler implementations remain open.
  rpc ListScheduledMessages(ListScheduledMessagesRequest) returns (ListScheduledMessagesResponse);
  rpc UpdateScheduledMessage(UpdateScheduledMessageRequest) returns (UpdateScheduledMessageResponse);
  rpc CancelScheduledMessage(CancelScheduledMessageRequest) returns (CancelScheduledMessageResponse);
  rpc SendScheduledMessageNow(SendScheduledMessageNowRequest) returns (SendScheduledMessageNowResponse);
}
```

### Shipped RPC status

| RPC | Handler | Notes |
|-----|---------|-------|
| `GetThreadMessages` | ✓ | thread replies |
| `ListThreads` | unavailable | authorizes membership, then fails closed until the bounded per-viewer read model is ready; the legacy aggregate is no longer a runtime fallback |
| `PinMessage` / `UnpinMessage` / `GetPinnedMessages` | ✓ | limit **5**/chat (`MaxPinsPerChat`); 6th → `ResourceExhausted` |
| `UnpinMessagesBySenderInChats` | ✓ | bot cleanup |
| `UploadPreKeyBundle` / `GetPreKeyBundle` | ✓ | DM E2E pre-keys |
| `MarkRead` / `GetReadState` / `GetBulkReadState` | ✓ | DM/group/channel; per-member read cursor; non-members → `PERMISSION_DENIED` |
| `GetChatListMetadata` | ✓ | per-member unread + preview/content metadata; DM-only delivery ticks; non-members → `PERMISSION_DENIED` |
| `ListSharedMedia` | ✓ | shared media tabs in chat info |
| `DeleteMessage` (`DeleteScope.FOR_ME` / `FOR_EVERYONE`) | ✓ | `FOR_ME` soft-hides for caller only |
| `SendMessage.send_silent` | ✓ | durable `messages.send_silent`, `message.sent.send_silent` and Notification push consumption; composer remains open |
| `SendMessage` schedule options (`scheduled_at`, `send_when_online`) | partial | wire contract is present but populated options fail closed until the handler slice |
| Scheduled RPCs | ✗ | declarations are shipped; handlers remain open |

### ListThreads: versioned per-viewer read model (accepted successor)

`ListThreads` currently refuses authorized reads with `UNAVAILABLE "thread list unavailable"`: the legacy limit-only aggregate ignores per-viewer visibility and is disabled at the RPC boundary. The thread index is temporarily unavailable; sending replies and loading a known thread through `GetThreadMessages` remain separate operations. The bounded cursor successor is a **Messaging-owned** read model in `messaging_db`; it must not scan/aggregate `messages` while serving a page or use a candidate-limited SQL fallback. Chat remains the membership authority.

Before parsing a cursor, every call performs authoritative `EnsureMember`. A missing or `BUILDING` viewer projection returns `UNAVAILABLE` with the fixed public text `thread list unavailable`; it never returns an empty page or legacy results. Chat supplies durable, idempotently consumed membership-outbox events: `event_id`, `chat_id`, `profile_id`, monotonic `membership_revision`, and `joined|removed|left`.

Messaging stores per-viewer lifecycle/root state, a per-chat mutation journal, immutable path-copied AVL nodes in activity and parent-ID orders, HMAC-bound cursor states, and a deduplicating membership inbox. Page one captures READY roots/high-water at the mutation serialization point. The cursor is v2: a random state ID plus HMAC-bound chat/profile/effective page size/snapshot/original expiry, with no message or thread identifiers. It represents the remaining set, so pages keep `(last_reply_at DESC, thread_parent_id DESC)` and cannot duplicate a parent.

Every local root/reply/edit/delete/hide/ghost mutation appends a change and projects heads in the same transaction. `FOR_ME`, `FOR_EVERYONE`, ghost and hide revocations apply live to unexpired cursor states. Pending fan-out marks a state `UPDATING` and reads fail `UNAVAILABLE`; new visibility grants appear only in new snapshots. Public errors are `Unauthenticated`, `InvalidArgument`, `PermissionDenied`, `FailedPrecondition`, or the opaque unavailable error; implementation errors never expose `err.Error()`.

A builder enumerates authoritative memberships, takes a repeatable-read baseline, bulk-builds both trees, replays post-baseline journal changes and CASes to READY only at zero lag. Cursor states expire after 15 minutes without renewal. GC preserves nodes reachable from READY heads/unexpired cursors and retains the journal through the oldest BUILDING baseline. Activation requires readiness, zero lag, root invariants and sampled dual reads. The full execution/RED contract is [listthreads-versioned-readmodel-exec-plan.md](../testing/listthreads-versioned-readmodel-exec-plan.md).

The [effective-membership contract v1](../testing/listthreads-versioned-readmodel-exec-plan.md#effective-membership-contract-v1)
defines owner-scoped Chat/Space/Role/User revisions, complete validated snapshots,
Chat generation/journal `Q`, a separate Messaging baseline/journal `J`, and
revocation lineages. READY requires both replay barriers and a generation CAS;
every page requires exact-pair Chat admission before cursor parsing, a final fresh
proof and a local visibility-version CAS. Async events alone never authorize a
read. DM self-delete/reopen invalidates only that viewer's lineage; chat archive
preserves history, while an ineligible profile cannot be the viewer. Peer deletion
does not erase surviving history. The durable implementation and complete
admission replacement remain activation prerequisites in
[backend.md](../todo/backend.md); the unavailable guard does not implement them.
The existing first-100-members adapter cannot satisfy this contract. Social-block
and managed message-time visibility gates must also be covered before activation.
### `SendMessageRequest` (spec)

`send_silent` and `delivery_schedule` are in the current proto. `content_payload` remains a future typed-content extension:

| Поле | Тип | Семантика |
|------|-----|-----------|
| `client_message_id` | UUID, optional | Идемпотентность — см. выше |
| `send_silent` | bool, default false; **field 11, PR #310** | Push без звука; Notification Service читает флаг из события `message.sent` |
| `delivery_schedule` | `oneof { google.protobuf.Timestamp scheduled_at; bool send_when_online }`, optional | Нет варианта — immediate send. `scheduled_at` ставит отложенную отправку; `send_when_online=true` — только DM. См. § Scheduled messages |
| `content_type` | enum, optional | Тип полезной нагрузки для preview / shared media — см. § Content types |
| `content_payload` | `google.protobuf.Struct` или typed oneof | Структурированное тело для `article` / `location` / `video_note` / `music` |

**Правила:**

- `delivery_schedule` — настоящий `oneof`: одновременно timestamp и `send_when_online` задать нельзя. `send_when_online=false` не является вариантом расписания.
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

### Space text admission (accepted target; BE-067)

This operation matrix is the accepted implementation contract; it does not claim
that every listed gate is implemented. Use Chat's authoritative Space/chat scope
and the authenticated actor. Effective Role decisions own owner/admin semantics;
Messaging must not add role-name bypasses. Missing Role authority or lookup
failure fails closed before message, reaction, delete, pin or outbox writes.
Existing membership, visibility, moderation, File and lifecycle fences still apply.
DM and standalone chats retain their separate policies.

Every Space send also needs `TEXT_CHAT_SEND_MESSAGES` and the applicable thread
permissions. Additional permissions follow the actual validated payload:

| Payload | Additional permission |
|---|---|
| Text, including a bare URL without embedding metadata | None |
| Photo, video, voice, video note, GIF, sticker | `TEXT_CHAT_SEND_MEDIA` |
| Document, music | `TEXT_CHAT_ATTACH_FILES` |
| Article or an explicitly supported link-preview embedding | `TEXT_CHAT_EMBED_LINKS` |
| Location coordinates/label and derivative map thumbnail | None |

Mixed payloads require the union of their semantic permissions. A derivative
thumbnail belongs to its parent payload; a file ID alone does not turn media
into a document. Validate every attachment and its File metadata rather than
trusting only top-level `content_type`: text/unspecified plus an image cannot
bypass media permission. Reject invalid or unsupported payloads through existing
validation. This contract does not introduce a link-preview wire, contact/poll
type, Article worker or scheduled feature. Deny unauthorized explicit embedding
rather than silently stripping it; bare text URLs remain allowed.

Send, forward and copy-as-new share the classifier. Forward checks the forwarding
actor's destination rights before inserting optional commentary or any event;
source read/File authority remains required. Any enabled delayed-send dispatcher
must recheck current rights at dispatch. Text-only author edits retain their
existing policy; future rich-content edits must use the same matrix.

For `FOR_EVERYONE`, an author retains the existing own-message path; deleting
another author's Space group/channel message requires
`TEXT_CHAT_MANAGE_MESSAGES`, including thread replies. Preserve the real actor
separately from the author through storage authorization and mutation fences.
`FOR_ME` remains a self-hide. Foreign edits remain forbidden. Pin and unpin require
`TEXT_CHAT_PIN_MESSAGES` for own and foreign messages; `MANAGE_MESSAGES` and
`MANAGE_THREADS` neither replace that gate nor add an extra required permission.

`TEXT_CHAT_MANAGE_THREADS` has no defined lifecycle RPC yet and cannot substitute
for send/create-thread/send-in-thread/delete/pin permissions. Space thread
admission must fail closed when Role authority is missing. A thread lifecycle
contract remains open; this matrix does not invent archive, close or cascade delete.

Unicode reactions need `TEXT_CHAT_ADD_REACTIONS` only. Future custom reactions
need an authoritative catalog-backed identity resolving their origin Space:
same-Space assets require add permission; external assets also require
`TEXT_CHAT_USE_EXTERNAL_EMOJIS`. Unknown/deleted assets or unavailable provenance
must deny custom admission, while members may still remove their own existing
reaction after revocation. The current opaque emoji string cannot establish
origin: do not infer it from prefixes, names, URLs, UUID shape or client-supplied
origin. Catalog/wire ownership and compatibility remain an explicit prerequisite;
no external-custom-emoji enforcement is claimed here.

### Scheduled messages (lifecycle)

P-008 ships the additive wire contract and durable table foundation below. Populated schedule arms fail closed until lifecycle handlers and the worker own validation and dispatch.

**Request/response union.** После PR #310 `SendMessageRequest` сохраняет его `send_silent = 11` и добавляет optional `oneof delivery_schedule`: `scheduled_at = 12` или literal `send_when_online = 13`; отсутствие oneof означает immediate send. `scheduled_at` принимает только UTC instant строго позже server `now()` и не дальше **365 дней** от него. `send_when_online` допустим только в DM и ждёт именно live `ONLINE` получателя. Ответ использует два additive поля, но семантически является union:

```protobuf
message SendMessageResponse {
  Message message = 1;                    // immediate send
  ScheduledMessage scheduled_message = 2; // accepted pending schedule
}
```

Семантически сервер заполняет ровно одно из двух полей; schedule create никогда не маскируется под уже сохранённый `Message`, а один idempotent retry возвращает исходный вариант union. Невалидный oneof, `send_when_online=false`, non-DM online delivery, instant не в будущем или горизонт более 365 дней возвращают `INVALID_ARGUMENT`.

**Storage:** таблица `scheduled_messages` в `messaging_db` is shipped as foundation migration `000015`; lifecycle handlers and worker remain open:

```
scheduled_messages
├── id (UUID)
├── chat_id, sender_profile_id
├── payload (jsonb — mirror SendMessage fields minus schedule flags)
├── schedule_kind (at | when_online)
├── scheduled_at (TIMESTAMPTZ, UTC, nullable only for when_online)
├── status (pending | sent | cancelled | failed)
├── sent_message_id, dispatch_event_id (nullable)
├── attempt_count, next_attempt_at, last_error_code, failed_at (nullable)
├── dispatch_lease_owner, dispatch_lease_expires_at (internal)
├── created_at, updated_at
└── UNIQUE(chat_id, sender_profile_id, client_message_id) WHERE client_message_id IS NOT NULL
```

`payload` excludes `schedule`; the oneof above is represented by `schedule_kind` plus nullable `scheduled_at`. Pending create is idempotent in the same `(chat_id, sender_profile_id, client_message_id)` namespace as immediate sends, with the mismatch rule in § Идемпотентность отправки.

| Операция | Нормативное поведение |
|----------|------------------------|
| **Create** | `SendMessage` with a schedule atomically creates one `pending` row and returns `scheduled_message`; it publishes no `message.sent` or other replay event. Without a schedule it follows the existing immediate path and returns `message`. |
| **List** | `ListScheduledMessages(chat_id, page)` returns only `pending` rows for that chat **owned by the caller**. Chat membership does not reveal another sender's pending payload. |
| **Update** | `UpdateScheduledMessage(scheduled_message_id, payload?, schedule?)` replaces supplied fields only while caller owns a `pending` row. Omitted payload retains its value; an omitted `delivery_schedule` **retains the existing schedule and never means immediate dispatch**. At least one of payload or schedule must be present. It returns that `ScheduledMessage`; it cannot turn a pending row into an immediate send. `chat`, sender and `client_message_id` are immutable. |
| **Cancel** | `CancelScheduledMessage(scheduled_message_id)` changes the caller-owned `pending` row to `cancelled`. Repeating cancel on an owned cancelled row returns idempotent `OK`; sent/failed is `FAILED_PRECONDITION`; an absent/not-owned ID is `NOT_FOUND`. |
| **Send now** | `SendScheduledMessageNow(scheduled_message_id)` claims the caller-owned pending row and executes the normal message insert/outbox transaction immediately. On success it returns the linked `Message`; repeat after `sent` returns `OK` with that same Message. Cancelled/failed is `FAILED_PRECONDITION`; it is not a create retry and does not reuse the original `client_message_id`. |
| **Race** | Worker, Cancel, Update and SendNow condition their transition on `status=pending`; exactly one wins. Losers read the resulting state and return `FAILED_PRECONDITION`, never a duplicate message. |

**Worker, presence and retry boundary.** A Messaging-owned worker claims due `at` rows, or checks User's viewer-aware live `GetPresence`/`GetBulkPresence` before claiming a DM `when_online` row. It sends only exact `ONLINE`; idle, DND, invisible, offline or hidden/empty presence retain the row. Durable last-seen is never a substitute for live presence, and Messaging does not call Realtime. Before the message/outbox transaction commits, User timeout/unavailable, DB errors and other transient dispatch failures release the claim, retain `pending`, increase `attempt_count` and retry with bounded exponential backoff; absence of online presence is not an attempt. An authoritative permanent deny/not-found/invalid state, or policy-bounded exhaustion of these pre-commit attempts, records terminal `failed` with a safe failure code and publishes no `message.sent`. The successful transaction writes a preallocated message ID, `messages`, Messaging outbox, `sent_message_id` and `scheduled_messages.status=sent` together. **After that commit the schedule stays `sent`:** the stable outbox retains immutable `dispatch_event_id` and alone retries JetStream publication until PubAck. A post-commit NATS publish failure never reclaims the schedule, reinserts the message or changes it to `failed`. Sender invisibility does not block a schedule; online delivery observes the **recipient** only.

Composer UX — [text-chat.md](../features/text-chat.md) § Send options; strip — [screen-controls.md](../design/screen-controls.md) §3.6 #13–17. Cross-service summary — [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Push-уведомления (send options).

### Scheduled RPC shape (wire contract shipped; handlers open)

```protobuf
rpc UpdateScheduledMessage(UpdateScheduledMessageRequest) returns (UpdateScheduledMessageResponse);
rpc ListScheduledMessages(ListScheduledMessagesRequest) returns (ListScheduledMessagesResponse);
rpc CancelScheduledMessage(CancelScheduledMessageRequest) returns (CancelScheduledMessageResponse);
rpc SendScheduledMessageNow(SendScheduledMessageNowRequest) returns (SendScheduledMessageNowResponse);

message UpdateScheduledMessageRequest {
  string scheduled_message_id = 1;
  // Complete replacement payload; only applied while status=pending.
  optional ScheduledMessagePayload payload = 2;
  oneof delivery_schedule {
    google.protobuf.Timestamp scheduled_at = 3;
    bool send_when_online = 4;
  }
}
```

`ListScheduledMessagesRequest` carries `chat` and cursor page; it is pending-only, owner-only and stably cursored by `(created_at, id)`. A non-member is `PERMISSION_DENIED`; a missing chat is `NOT_FOUND`; there is no global scheduled inbox, recipient list, admin list, `GetScheduledMessage` RPC or Realtime API. `UpdateScheduledMessage` uses the same oneof and validation as create; omitting its `delivery_schedule` retains the stored schedule, never selects immediate send, and successful update does not publish `message.sent`. Mutation IDs that are absent or not owned return `NOT_FOUND`; apart from idempotent cancel, a non-pending lifecycle state returns `FAILED_PRECONDITION`.

### `GetChatListMetadata` / `ChatListItem` preview

S2S enrichment для Chat `ListChats`. Код возвращает per-member unread state и preview metadata для DM/group/channel.

**Viewer visibility applies to every derived read.** A profile-scoped `GetMessage`, `GetPinnedMessages`, `GetThreadMessages`, `ListSharedMedia`, and `GetChatListMetadata` must apply the same message visibility boundary as history: deleted rows are absent; `ghost_only` is visible only to its sender; `FOR_ME` hides the row only for the profile that hid it; and a directional Social profile block hides that sender's rows from the viewer. Metadata preview selects the latest remaining visible row and unread count excludes hidden rows. A missing or unavailable Chat/Social authority fails closed; clients and trusted internal callers must not use the no-profile `GetMessage` path as a viewer read.

Metadata evaluates block decisions through Social's bounded `IsProfilePairsBlocked` S2S call (at most 500 profiles per request). The canonical Chat `ListChats` page supplies at most 100 chat refs; Messaging enforces that same limit before any per-chat lookup. One request can therefore make at most 100 membership checks, 100 authoritative chat-type checks, and 100 final metadata queries. Across those chats, the unread-sender query examines at most 5,000 unread message rows plus one overflow sentinel, and preview selection reads at most 5,000 candidate rows plus one sentinel. Messaging reuses each unique profile decision across the request and caps them at 5,000; duplicate senders across chats use one decision, so at most 10 Social batch calls are needed. If an exact result would exceed a request-wide budget, `GetChatListMetadata` returns `UNAVAILABLE` without partial or approximate metadata. This explicit availability budget follows the existing fail-closed visibility-dependency contract and does not change block semantics for former senders within budget.

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

История общих групповых и channel-чатов фильтруется по направлению «viewer account заблокировал sender account». Messaging передаёт viewer/sender profile IDs внутреннему Social checker, который сам разрешает их владельцев и возвращает только bool; Messaging не делает User account lookup для non-DM history. Ошибка Social или User ownership lookup скрывает страницу целиком с `UNAVAILABLE`.

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
├── content (text, максимум 4000 кодовых точек Unicode)
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

**Migrations shipped:** `messages`, `read_receipts`, `reactions`, `pins`, `thread_parent_id`, `forward_*`, `ghost_only` (platform shadow-ban column — **DB only**, not yet on `SendMessageRequest` proto), E2E columns, `send_silent`, `scheduled_messages` foundation.

**Handlers shipped beyond basic message CRUD:** threads (`GetThreadMessages`, `ListThreads`), reactions, pins (limit **5**/chat), per-member `MarkRead`/`GetReadState`/`GetBulkReadState` for DM/group/channel, `GetChatListMetadata` with per-member unread/preview metadata and non-member denial, `ListSharedMedia`, `DeleteMessage` with `DeleteScope.FOR_ME`, idempotent `client_message_id` and durable `send_silent` producer propagation on `SendMessage`, E2E pre-key RPCs.

**Gaps vs full spec:** composer for `send_silent`; scheduled handlers, worker and event producer usage; typed `content_type`, `message_attachments` table and `RecordMessageView` — см. § ниже и [todo/backend.md](../todo/backend.md).

**Attachment validation (code):** R3-A06 is implemented: `validateParsedAttachments` branches on `content_type`; `location` / `article` use rich-payload validation without a File row, while `sticker` / `gif` / `music` / `video_note` require rich-payload and File metadata validation. This does not establish picker, Article metadata-fetch, or Music metadata-processing completion; see [AttachMenu contract gaps](../todo/client.md#attachmenu-rich-action-contracts).

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
| `message.mention_added`  | message_id, chat_id, **sender_profile_id**, mentioned_profile_ids, **send_silent** |
| `message.edited`         | message_id, chat_id                          |
| `message.deleted`        | message_id, chat_id                          |
| `message.reaction_added` | message_id, profile_id, emoji                |
| `message.pinned`         | message_id, chat_id, pinned_by               |
| `message.unpinned`       | message_id, chat_id, unpinned_by             |
| `message.forwarded`      | message_id, source_chat_id, target_chat_id   |

**`message.sent` notes:** `send_silent` drives Notification push policy and is shipped by PR #310 as `MessageSent.send_silent = 8`; the scheduling slice does not renumber or duplicate it. It adds `was_scheduled = 9` and `optional scheduled_at = 10`, which remain unset by the immediate producer until the schedule dispatch slice owns them. A pending create, idempotent replay, update, cancel or terminal failure must emit no `message.sent`; a successfully dispatched scheduled row must emit exactly one normal `message.sent` when that worker is implemented.

**`message.read`:** публикуется при `MarkRead`; Realtime fan-out как WS `message_read`. **Code-ok**.

### File → Messaging preview refresh (spec)

When File Service finishes async processing (`file.processed` on JetStream), Messaging **must** refresh list/history metadata for messages referencing that `file_id`:

| Step | Owner |
|------|-------|
| File publishes `file.processed` | `file_events` / `file.processed`: independent complete revisioned `preview` snapshot for plaintext non-Music files; no URL or storage key |
| Messaging consumer | Refreshes only private exact MESSAGE-reference/slot bindings, persists monotonic snapshots before discovery, and writes ordinary `message.edited` atomically; list/history reads use persisted attachments |
| Client | On `message_update` WS (or poll) refresh bubble + list row label |

BE-080 v1 uses protected Messaging-only `ResolveMessageAttachmentPreview` after
reference acquisition, then reconciles under local insertion fences. The resolver
checks the exact existing destination reference and existing LIVE Space fence;
it never invents viewer authority. Ready-only admission remains unchanged. The
snapshot includes status, category, persisted intent, size/scan, optional dimensions
and variant availability. Missing fields clear stale metadata; terminal snapshots
cannot be revived. Multiple slots and forwards retain private per-slot bindings.
E2E and URL-only attachments are excluded. Public URL reads still require the real
viewer and exact reference. Frozen work is retained durably and legacy rows need
bounded restartable reconciliation. See [BE-080 plan](../testing/be080-file-preview-exec-plan.md)
for implementation progress and the all-consumers-before-publication rollout gate.

**Music implementation:** explicit File Music uploads carry a persisted intent
and monotonic metadata revision. At send/forward admission, Messaging obtains
viewer-authorized File metadata, replaces caller-supplied plaintext tags and
privately binds file ID, source chat and revision. The `file_events` durable
`messaging_music_metadata` consumes the existing `file.processed` subject with
`DeliverNew`; File is the sole publisher. It validates that binding and updates
eligible original/forwarded attachments plus the normal `message.edited` outbox
atomically under the chat/Space mutation fence. List/history reads use the
updated attachment data; the event reaches the existing `message_update` client
path. It does not change message identity, creation time or Chat-owned sort keys.

Messaging persists the latest per-file snapshot in
`messaging_music_file_snapshots` before enumerating destinations. The
send/forward insert transaction locks that snapshot and reconciles its admitted
tags against a newer stored revision before inserting. This covers a
`file.processed` tombstone delivered after File reference acquisition but before
the first message row exists.

Only ready plaintext Music with the existing clean-or-skipped scan policy can
gain tags. Missing fields clear previous tags. Delete/expiry emits a newer
metadata-free snapshot; Messaging retains that revision so replayed ready events
cannot restore tags. This is ordered eventual revocation, not a claim that File
state has been synchronously re-read before every background update. File still
authorizes downloads. E2E ciphertext is never extracted server-side and receives
no public metadata; the client derives tags from authenticated decrypted bytes.

Activation requires the versioned four-consumer preservation/credential proof
before deploying the new File producer; see the [NATS activation runbook](../../scripts/staging/nats-rollout-preservation/README.md).
Other attachment processing consumers and their cache invalidation remain open;
see [todo/backend.md](../todo/backend.md). Subject matrix:
[CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md).

### Timestamp ownership (`chats.last_message_at`)

| Field / concern | Owner | Notes |
|-----------------|-------|-------|
| `chats.last_message_at` (sort key for `ListChats`) | **Chat Service** | Updated on `message.sent` consumer (`TouchLastMessageAt`) for `dm` / `group` / `channel` |
| `last_message_preview`, `unread_count` | **Messaging** | S2S `GetChatListMetadata` |
| `last_message_delivery_state`, `last_message_content_type` | **Messaging** | Durable list ticks/content metadata; not Realtime WS |
| Client list sort | **Chat `ListChats`** | Uses `chats.last_message_at`; do **not** sort client-side from Messaging preview timestamps alone |

Messaging does **not** own `chats.last_message_at`. Cross-ref — [chat-service.md](chat-service.md) § ListChats ownership table.

## Зависимости

`AddReaction` in a Space chat requires `TEXT_CHAT_ADD_REACTIONS` for the
authenticated profile and the message's authoritative Space/chat scope before
writing either the reaction or its event outbox row. Missing or unavailable Role
authority returns `UNAVAILABLE`; an explicit deny returns `PERMISSION_DENIED`.
DM and standalone chats do not use Space permissions. `RemoveReaction` still
allows a member to remove their own reaction after losing add permission.

- **Chat Service** — валидация членства / доступа для **DM**, **группы**, **канала** (в т.ч. standalone)
- **Space Service** — если у `chats.space_id` задан спейс: членство в спейсе (для канала и группы в спейсе)
- **Role Service** — проверка прав в спейсе для текстового чата (`chat_id` = `chats.id`, `group` \| `channel`)
- **Social Service** — проверка блокировок
- **User Service** — internal `ResolveAccountIDForProfile` для DM lifecycle: fresh S2S metadata с единственным caller `messaging`, без viewer identity; lookup возвращает owner soft-deleted peer только для следующей Auth account-status проверки
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

## P3 Space lifecycle participant (target)

The participant implementation has Space-authenticated manifest-page import,
lifecycle-fence, and purge RPCs on a dedicated mutual-TLS listener. Manifest pages
are validated for domain-separated hashes, canonical sorted Chat IDs, page
size, sequence, and final seal position. PostgreSQL retains deterministic page,
request, and receipt bytes, and seals only after the ordered root matches.
Exact retries return the first receipt; changed bytes and gaps fail closed.
The fence accepts only a sealed matching manifest, records monotonic FROZEN and
LIVE transitions with immutable operation receipts, and acknowledges stale
generations with the current saved receipt. Database triggers currently block
message, reaction, pin, read-position/read-receipt, and scheduled-message
mutations while the Space is frozen or terminal, under the shared Space
mutation lock. Additional triggers cover message hides, game-message
revisions/operation receipts, game cards, and game-action results. Thread edits
flow through the guarded message row; read and delivery cursors share the
guarded receipt tables. Purge pages the sealed Chat manifest, derives a stable
child operation per chat, and invokes the managed-chat purge coordinator. Each
child must return valid File and Search receipt hashes before Messaging commits
its exact participant receipt and terminal fence. The Space coordinator and a
separate File Space-producer release receipt remain outstanding.

The app Compose profile configures `MESSAGING_SPACE_LIFECYCLE_GRPC_LISTEN`,
the Messaging server certificate/client CA, and the Space principal JWKS
client. The listener defaults to `:9093` when enabled and startup fails on
partial TLS or principal configuration.

Enabled Chat/Space lifecycle and managed-chat purge also require clean Messaging
migration `000026_attachment_send_intents` or later and all nine durable-intent
columns with their expected types/nullability. The preflight runs before purge
JWKS, lifecycle listeners and recovery workers serve. Missing, dirty, old or
partial schema refuses startup with a fixed migration diagnostic. Migration
metadata is trusted; this runtime check does not replace migration execution.
Disabled lifecycle keeps the baseline startup path. Rollback of 000026 refuses
while any saved intent exists; drain and export its evidence before rollback.
Its DOWN requires READ COMMITTED and holds a write-excluding table lock before
the current-snapshot guard through DROP in one atomic statement. A concurrent
first intent must survive rollback refusal; an older repeatable-read snapshot
is rejected rather than treated as evidence that the table is empty.

Messaging stores its generation fence, imported immutable Chat manifest pages,
and exact request/receipt evidence. `FROZEN` imports and seals the exact Chat
binding and blocks message/reaction/thread/schedule/read/delivery mutations for
that Space work set. Each managed-chat purge snapshots its frozen attachment
references and submits them to File as the `MESSAGING` producer. A separate
Space-level producer manifest is still outstanding.

Reads and attachment refresh authorize the message domain first, then request a
subject-bound File capability for its exact live reference. Attachment visibility
waits for `AcquireFileReferences`. Each managed-chat purge deletes messages,
reactions, read and delivery state, pins, hides, threads and scheduled rows in
bounded FK order, then waits for File reference-release and Search purge
receipts. Space-level Messaging completion follows only after every manifest
chat returns both owner receipts. Full request/receipt bytes retain 30 days from
this participant's completion and the compact `PURGED` fence is permanent.

After validating the exact Space service principal and complete request binding,
`PurgeSpace` reads its committed response before requiring File or managed-chat
purge dependencies. A response-loss retry returns the original receipt and
preserves unknown response fields without repeating child effects. Changed
request hashes fail closed; unavailable or corrupt receipt lookup cannot restart
destructive work. The read uses the original database completion timestamp and
the same 30-day window as Chat's committed receipt lookup.

Public read-receipt opt-out is serialized against the same per-chat purge lock.
If any receipt the operation would revoke points to a message already included
in a `PENDING` purge manifest, `ClearPublicReadReceiptsWithOutbox` rejects the
whole transaction with the retryable `ErrMessageMutationPurging` result before
changing receipts or inserting revocation events. The consumer retries after
the purge completes; revocations committed before the freeze are part of the
immutable event set and must receive PubAck before purge side effects. Existing
`PENDING` operations whose event-set hash is null remain fail-closed and need
explicit operator recovery; Messaging does not backfill their event set or
silently resume them.

The child uses the existing T33 `PurgeManagedChatContent` protocol:
`request_sha256` is SHA-256 of its deterministic protobuf request bytes, matching
its service-principal request binding. The P3 adapter verifies that digest;
it does not replace the saved child evidence with the parent's message-name
domain. The Space participant request/receipt remains domain-separated P3.
