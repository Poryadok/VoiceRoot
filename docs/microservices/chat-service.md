# Chat Service

## Обзор

Управление сущностями чатов: DM (1:1), текстовые **группы** и **каналы** (`type = group` \| `channel`, в т.ч. вне спейса; одинаковая модель, разные дефолты), пользовательские папки.

**Язык**: Go
**БД**: PostgreSQL `chat_db`

## Ответственность

- Создание и управление DM-чатами
- Создание и управление **текстовыми групповыми чатами** (`type = group` \| `channel`, до 500 участников на чат вне спейса по продукту; одна модель API); `space_id` опционален; в спейсе узел **`space_tree_nodes`** — совместно с Space
- Участники: для чатов **без** `space_id` — `chat_members`; для чатов **в** спейсе — наследование от `space_members` + роли/оверрайды (см. [DATA_MODEL.md](../DATA_MODEL.md))
- Папки чатов (All / DM / Groups / Channels / Spaces / пользовательские) — [navigation.md](../features/navigation.md); **foundation (Batch 18):** migrations `000008`/`000009`, `ListFolders`/`CreateFolder`; **membership/pin/`ListChats.folder_id` + Gateway REST (Batch 19)**; **`UpdateFolder`/`DeleteFolder` + Gateway PATCH/DELETE (Batch 20)**
- Quick Access (до 15 чатов на профиль); таблица `quick_access_chats` + RPC — **shipped (Batch 17)**; archive removes Quick Access on `ArchiveChat` — **shipped (Batch 18)**
- Список активных чатов (mobile strip; desktop — quick access + pin)
- Мьют / архивация чатов
- Slow mode (таймер между сообщениями)

## API (gRPC)

Источник истины: [protos/voice/chat/v1/chat.proto](../../protos/voice/chat/v1/chat.proto). Ниже — инвентарь с **статусом реализации** (handler в `src/backend/chat/internal/grpcsvc/`).

```protobuf
service ChatService {
  // DM
  rpc CreateDM(CreateDMRequest) returns (CreateDMResponse);       // ✓
  rpc GetDM(GetDMRequest) returns (GetDMResponse);               // ✓ find or create

  // Текстовые групповые чаты (group | channel)
  rpc CreateChat(CreateChatRequest) returns (CreateChatResponse); // ✓
  rpc UpdateChat(UpdateChatRequest) returns (UpdateChatResponse); // ✓ name/topic/slow mode/avatar/thread settings
  rpc DeleteChat(DeleteChatRequest) returns (DeleteChatResponse); // ✓ DM soft-delete for caller; group/channel rejected

  // Участники
  rpc AddMembers(AddMembersRequest) returns (AddMembersResponse);       // ✓
  rpc RemoveMember(RemoveMemberRequest) returns (RemoveMemberResponse); // ✓
  rpc LeaveChat(LeaveChatRequest) returns (LeaveChatResponse);           // ✓
  rpc SetGroupMemberRole(...) returns (...);                              // ✓ standalone groups: admin promotion; owner demotion
  rpc TransferGroupOwnership(...) returns (...);                       // ✓
  rpc ListMembers(ListMembersRequest) returns (ListMembersResponse);     // ✓

  // Список чатов
  rpc ListChats(ListChatsRequest) returns (ListChatsResponse); // ✓ dm+group; space merge; inbox filter
  rpc GetChat(GetChatRequest) returns (GetChatResponse);     // ✓

  // Message requests (DM inbox)
  rpc AcceptDMRequest(AcceptDMRequestRequest) returns (...);   // ✓
  rpc DeclineDMRequest(DeclineDMRequestRequest) returns (...); // ✓

  // DM E2E toggle
  rpc EnableChatE2E(EnableChatE2ERequest) returns (...);   // ✓ (pre-key gate when Messaging wired)
  rpc DisableChatE2E(DisableChatE2ERequest) returns (...); // ✓

  // Папки — migrations + ListFolders/CreateFolder shipped (Batch 18); Update/Delete shipped (Batch 20)
  rpc ListFolders(ListFoldersRequest) returns (ListFoldersResponse);       // ✓ lazy system seed + custom create
  rpc CreateFolder(CreateFolderRequest) returns (CreateFolderResponse);     // ✓ custom only
  rpc UpdateFolder(UpdateFolderRequest) returns (UpdateFolderResponse);     // ✓ custom only; system immutable
  rpc DeleteFolder(DeleteFolderRequest) returns (DeleteFolderResponse);       // ✓ custom only; system immutable

  // Quick Access — shipped (Batch 17)
  rpc ListQuickAccess(ListQuickAccessRequest) returns (ListQuickAccessResponse);
  rpc AddQuickAccess(AddQuickAccessRequest) returns (AddQuickAccessResponse);
  rpc RemoveQuickAccess(RemoveQuickAccessRequest) returns (RemoveQuickAccessResponse);
  rpc ReorderQuickAccess(ReorderQuickAccessRequest) returns (ReorderQuickAccessResponse);

  // Folder membership + pin — shipped (Batch 19)
  rpc AddChatToFolder(AddChatToFolderRequest) returns (AddChatToFolderResponse);       // ✓
  rpc RemoveChatFromFolder(RemoveChatFromFolderRequest) returns (RemoveChatFromFolderResponse); // ✓
  rpc ReorderFolderChats(ReorderFolderChatsRequest) returns (ReorderFolderChatsResponse); // ✓
  rpc PinChatInFolder(PinChatInFolderRequest) returns (PinChatInFolderResponse);     // ✓
  rpc UnpinChatInFolder(UnpinChatInFolderRequest) returns (UnpinChatInFolderResponse); // ✓

  // Действия
  rpc MuteChat(MuteChatRequest) returns (MuteChatResponse);       // ✓
  rpc ArchiveChat(ArchiveChatRequest) returns (ArchiveChatResponse); // ✓ write + list archive inbox
}
```

### BE-116 Space audit producer dependency (accepted target; not implemented)

For successful create/update/delete of a Space-attached text chat, Chat writes
one canonical `chat_created`, `chat_updated` or `chat_deleted` fact into a durable
`chat_db` producer outbox in the same transaction as the domain mutation. A failed
or no-op mutation creates no new fact. A worker signs the exact deterministic
`voice.space.v1.SpaceService/AppendAuditEvent` request as `service:chat`; after
the empty acknowledgement it marks the item delivered. Retry keeps the same
`audit_event_id` and exact payload. Standalone DM/group/channel mutations do not
enter the Space ledger. Chat never writes `space_db` and cannot emit Space- or
Role-owned actions.

Composite `POST /api/v1/spaces/{space_id}/chats` exposes one `chat_created` only
after the complete Chat+tree outcome. The internal Space tree upsert in that flow
must not create a second user-visible audit effect. The proto/registry exists;
the Chat outbox, publisher and activation remain a BE-116 implementation dependency.

### `ListChatsRequest` / `ChatListItem`

```protobuf
message ListChatsRequest {
  voice.common.v1.CursorPageRequest page = 1;
  optional string inbox = 2; // main | requests | archive — ✓ shipped
  optional string folder_id = 3; // ✓ shipped (Batch 19)
}

message ChatListItem {
  Chat chat = 1;
  optional string last_message_preview = 2; // S2S Messaging when wired
  int64 unread_count = 3;
  optional string inbox = 4;              // main | requests — ✓ from chat_members.inbox_bucket
  optional bool is_stranger = 5;          // ✓ true when inbox=requests
  optional string dm_peer_profile_id = 6; // ✓ DM peer identity for authorized client lookups
  optional string dm_peer_display_name = 8; // ✓ title-only nickname for existing DM when public profile is unavailable
}
```

## Модель данных

### GIS application-managed group chats (T31 recipient slice)

Chat owns the `chats`, `chat_members`, and `managed_chat_operations` rows. A
managed chat is a standalone `group` whose `creator_profile_id` is NULL and
whose `managed_by_application_id`, `managed_environment_id`, and
`external_chat_key` are all set. The application/environment is the resource
owner; no profile receives `owner` or `admin` role. The external key is unique
within that application and environment.

GIS mutations use the separate `GameIntegrationChatService` with
`ProvisionManagedChat`, `SyncManagedChatMembers`, and
`SetManagedChatRetention` allowlisted, on the
dedicated mTLS listener. Each request carries a UUID `operation_id`. The
recipient verifies the `gameintegration` service principal for audience
`chat`, the exact full RPC, `x-request-id == operation_id`, and deterministic
protobuf SHA-256 request hash; JTI replay protection is Redis-backed. Player
roster methods reject managed chats. The GIS surface is not registered on the
ordinary player listener. Runtime configuration is all-or-nothing; its
environment variables are `CHAT_GIS_GRPC_LISTEN`, `CHAT_GIS_TLS_CERT_FILE`,
`CHAT_GIS_TLS_KEY_FILE`, `CHAT_GIS_CLIENT_CA_FILE`,
`GAME_INTEGRATION_PRINCIPAL_JWKS_URL`, optional
`GAME_INTEGRATION_PRINCIPAL_JWKS_CA_FILE`, and
`GAME_INTEGRATION_PRINCIPAL_REPLAY_REDIS_ADDR` plus optional password.
Current staging/production deployment manifests do not yet configure the
dedicated port, secret mounts, or GIS-only network policy, so they leave this
listener disabled.

`SetManagedChatRetention` records the immutable absolute `purge_after` cutoff
for a managed chat. The request binds application, environment, external chat
key, operation ID, and cutoff in the GIS principal's request hash. Exact retry
returns the original receipt; any attempt to change the cutoff conflicts. Chat
uses its PostgreSQL clock and denies message-read entitlement at
`clock_timestamp() >= purge_after`, even for a current member and for messages
created before the cutoff. GIS schedules this at the session's first terminal
transition using the frozen `terminalized_at + 30 days` deadline. It never
schedules Party chats or a child session's shared Party chat. This signal
controls read access; it does not claim that Messaging/File physical purge has
completed.


`managed_chat_operations` stores the exact app/environment/operation ID,
method, request hash, chat ID, and immutable JSON receipt. Provision and roster
responses include the persisted `receipt_id` and `request_hash` alongside the
immutable resource or roster result and `replayed` flag. `receipt_id` equals the
operation UUID, a primary key component of `managed_chat_operations`; exact
retries return the same values. GIS persists the values returned by Chat and
does not synthesize a receipt. Mutable chat projections are read through Chat's
ordinary API. A different method or request hash conflicts. Roster
synchronization atomically diffs membership, gives all members the `member`
role, and preserves per-member state for retained members. Managed resources
and receipts currently have no retention or retirement policy; product and T31
orchestration work must define one before application/environment deletion can
safely remove them.

```
chats
├── id (UUID)
├── type (dm | group | channel)
├── space_id (nullable — группа/канал в спейсе)
├── name (nullable)
├── avatar_url (nullable)
├── topic (nullable, часто канал)
├── creator_profile_id
├── slow_mode_seconds (0 = off)
├── last_message_at (activity sort — см. § Timestamp ownership)
├── threads_enabled (bool, default false; channels default true)
├── allow_user_main_feed (bool, default true; channels default false)
├── allow_guests (bool, default false — explicit chat-level guest admission)
├── e2e_enabled (bool, default false — DM opt-in E2E)
├── created_at
└── updated_at

chat_members
├── chat_id (UUID, logical ref → chats.id)
├── profile_id (UUID, logical ref → user_db.profiles.id; без меж-БД REFERENCES)
├── role (owner | admin | member)
├── joined_at
├── muted_until (nullable)
├── is_archived (bool)
├── inbox_bucket (main | requests | declined) — per-member DM request state; `social.friend_accepted` promotes existing requests for that pair to main
└── UNIQUE(chat_id, profile_id)

folders
├── id (UUID)
├── profile_id (UUID, logical ref → user_db.profiles.id)
├── name
├── folder_type (system | custom)
├── filter_config_json (jsonb) -- system: preset predicate; custom: include rules
├── sort_order (int)
├── created_at
└── updated_at

folder_chats
├── profile_id (UUID, logical ref → user_db.profiles.id)
├── folder_id (UUID, logical ref → folders.id)
├── chat_id (UUID, logical ref → chats.id)
├── sort_order (int, NOT NULL DEFAULT 0) -- manual order within folder
├── is_pinned (bool, NOT NULL DEFAULT false)
├── pin_order (int, NULL) -- lower = higher; NULL when not pinned
├── added_at (timestamptz)
└── PRIMARY KEY (profile_id, folder_id, chat_id)

quick_access_chats
├── profile_id (UUID, logical ref → user_db.profiles.id)
├── chat_id (UUID, logical ref → chats.id)
├── sort_order (int, NOT NULL DEFAULT 0)
├── added_at (timestamptz)
└── UNIQUE(profile_id, chat_id) -- max 15 rows per profile_id (enforced in service)
```

**Notes:**
- **System folders** (All, DM, Groups, Channels, Spaces): membership computed from `chats` + `filter_config_json`; rows in `folder_chats` used only for **pin/order overlay** (`is_pinned`, `pin_order`, optional `sort_order` override).
- **Custom folders**: explicit rows in `folder_chats` for every included chat; pin scoped to that folder.
- **Quick Access**: separate table; **chat_id only** (no polymorphic space target).

### Applied navigation migrations

```sql
-- 000008_folders.up.sql
CREATE TABLE folders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id UUID NOT NULL,
  name TEXT NOT NULL,
  folder_type VARCHAR(16) NOT NULL CHECK (folder_type IN ('system', 'custom')),
  filter_config_json JSONB NOT NULL DEFAULT '{}',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX folders_profile_id_idx ON folders (profile_id, sort_order);

-- 000009_folder_chats.up.sql
CREATE TABLE folder_chats (
  profile_id UUID NOT NULL,
  folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  chat_id UUID NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  is_pinned BOOLEAN NOT NULL DEFAULT false,
  pin_order INTEGER NULL,
  added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, folder_id, chat_id)
);
CREATE INDEX folder_chats_folder_pin_idx ON folder_chats (profile_id, folder_id, is_pinned DESC, pin_order NULLS LAST, sort_order);

-- 000010_quick_access_chats.up.sql
CREATE TABLE quick_access_chats (
  profile_id UUID NOT NULL,
  chat_id UUID NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, chat_id)
);
CREATE INDEX quick_access_profile_order_idx ON quick_access_chats (profile_id, sort_order);
```

### Deployed schema (migrations `000001`–`000011`) vs full spec

**Shipped today** (`chat_db` migrations): DM + group + channel types; `chat_members.inbox_bucket`; `threads_enabled` / `allow_user_main_feed`; `e2e_enabled`; slow mode; chat-level `allow_guests` (`000007`, default hardened to `false` and legacy standalone group/channel rows backfilled by `000012`); `folders` + `folder_chats`; `quick_access_chats`; per-profile `deleted_for_self` (`000011`). Standalone group/channel owners and existing admins configure future guest admission through `UpdateChat`; admission is checked against User's guest marker fail-closed, and disabling it preserves current memberships. Folder membership/pin, `ListChats.folder_id` and `UpdateFolder`/`DeleteFolder` are implemented. Incoming message activity keeps archived chats in the archive and only updates their unread badge.

### Guest admission

- Default is `allow_guests=false`; creating a chat must not silently admit guests.
- `allow_guests=true` permits an invited guest to join/use that chat but does not let
  a guest initiate a DM/call, self-discover the chat, or bypass an invite.
- For a Space-attached chat, the full effective-access contract (Space and chat
  opt-in plus membership/role checks) remains owned by Space/Role and is not
  implemented by this standalone admission slice. Chat does not write a
  standalone `allow_guests` decision for Space chats.
- DM ignores this flag because guests cannot initiate DM; receiving an allowed DM is
  governed by the guest/privacy rules in [auth-and-contacts.md](../features/auth-and-contacts.md).

**Navigation contract** (folders + Quick Access) — [navigation.md](../features/navigation.md); applied migration definitions are shown above.

Индексы (deployed):
- `chat_members_profile_id_idx (profile_id, joined_at DESC)` + `chat_members_profile_inbox_idx (profile_id, inbox_bucket)`
- `chats_last_message_at_idx (last_message_at DESC)`
- `chats_creator_profile_id_idx (creator_profile_id)`

## ListChats (список, превью, unread)

**Контракт**: `ListChatsRequest` с `voice.common.v1.CursorPageRequest` (`cursor`, `page_size`); опционально `inbox` (`main` \| `requests` \| `archive`). Ответ `ListChatsResponse.chat_list` — `ChatList` с `items: ChatListItem[]` и `next_cursor`. Каждый `ChatListItem` содержит `chat`, `last_message_preview`, `unread_count`, `inbox`, `is_stranger`, `dm_peer_profile_id` и необязательный title-only `dm_peer_display_name` (см. proto выше). Chat запрашивает nickname у User только для peer profile IDs, подтверждённых членством в DM; это не раскрывает остальные данные заблокированного профиля.

**Порядок и фильтр (shipped code)**:
- членство в `chat_members`, `is_archived = false`, `inbox_bucket` = значение `inbox` (default `main`);
- SQL: `chats.type IN ('dm', 'group', 'channel')` для membership-inbox (Batch 14);
- для `inbox=main` membership-строки и space-чаты (`group`/`channel` по `space_id`) пагинируются **единым SQL UNION** в store (`listChatsPageMainWithSpaces`); gRPC передаёт `spaceIDs` из S2S Space `ListMemberSpaceIDs` на каждой странице;
- сортировка: `COALESCE(last_message_at, created_at)` DESC, tie-break `chats.id` DESC;
- page size default 50, max 100.
- list SQL and `chatRowToProto` hydrate `space_id`, slow mode, thread flags, `e2e_enabled` and nullable `topic`.

**Full inbox spec:** `folder_id` filter — **shipped (Batch 19)**; **`inbox=archive`** + **Quick Access RPC/DDL** — shipped (Batch 15/17).

### Reconnect: global inbox snapshot

`ListChats` — durable control plane для глобальной сверки списка после reconnect, не WebSocket. Клиент запрашивает paginated snapshot `main`, `requests` и `archive`; первую страницу может отрисовать сразу, но продолжает каждый scope до `next_cursor == ""` в фоне. Пока scope не завершён успешно, клиент не удаляет старые cache rows и не заменяет их пустым ответом/нулевым `unread_count`; неуспешная страница повторяется. `Chat` остаётся owner membership, inbox bucket, archive и сортировки, а Messaging S2S обогащает каждую строку preview/unread/delivery metadata. Полные сообщения не возвращаются: после snapshot клиент вызывает `Messaging.GetMessages` только для открытого, notification-target или иного выбранного `chat_id`.

После account delete успешный новый snapshot не включает DM-строку surviving участника с удалённым peer. Для этого snapshot-only фильтра Chat вызывает User `ResolveAccountIDForProfile` как exact internal caller `chat`: lookup возвращает owner даже для soft-deleted profile и не заменяется public `GetProfile`, который сохраняет visibility semantics для group/privacy/DM creation paths. Затем Chat проверяет lifecycle account через Auth. Ошибка owner или Auth dependency делает page `UNAVAILABLE`, а не доказывает удаление строки. Уже выбранная локально загруженная история восстанавливает terminal state только через `Messaging.GetMessages.dm_peer_state`, не через `ListChats` и не через раскрытие deleted profile.

Chat consumes `user.account_deleted` through one deployment-preprovisioned `chat_account_deleted` durable with an exact `user.account_deleted` filter, explicit ACK and `DeliverAll`. Before bind it rejects an absent or drifted durable; the service never creates or updates JetStream consumers.

### Timestamp ownership (`chats.last_message_at`)

| Writer | Что обновляет | Статус |
|--------|---------------|--------|
| **Chat** | `chats.last_message_at` на потоке `message.sent` (`message_activity_consumer` → `TouchLastMessageAt`) | **dm / group / channel** (`type IN (...)`) |
| **Messaging** | `last_message_preview`, `unread_count`, durable `last_message_delivery_state` (spec) | S2S `GetChatListMetadata`; preview text only in code |
| **Target** | Chat owns activity timestamp for **all** chat types; Messaging owns preview + delivery/read metadata | shipped for chat types above |

Сортировка `ListChats` читает `chats.last_message_at` из Chat DB; превью-текст и тики доставки — из Messaging ([messaging-service.md](messaging-service.md) § `GetChatListMetadata`).

**Превью и непрочитанные: денорм в `chat_db` vs S2S Messaging**

| Подход | Плюсы | Минусы |
|--------|-------|--------|
| **Денорм в Chat** (`chats.last_message_preview`, счётчики на `chat_members` и т.п.) | Быстрый один запрос к `chat_db`; меньше зависимость от Messaging при чтении | Дублирование данных; нужны надёжные обновления из потока сообщений / триггеры / джобы; риск рассинхрона |
| **S2S Messaging (выбрано в текущем коде)** | Источник истины остаётся в `messaging_db` (`messages`, `read_receipts`); нет дублирования текста сообщения в Chat | Дополнительный round-trip (batch) при `ListChats`; требуется живой gRPC к Messaging |

Реализация: опциональный интерфейс обогащения на стороне Chat (`ListChatsEnrichment`) вызывается после выборки страницы из PostgreSQL и заполняет `last_message_preview` / `unread_count`. Если клиент Messaging не сконфигурирован, список чатов всё равно возвращается, а эти поля остаются пустыми и нулём. Конкретный набор Messaging RPC (например, расширение к `GetBulkReadState` + выборка последнего видимого сообщения пачкой по `chat_id`) задаётся при внедрении Messaging; до этого момента шлюз может собирать список из Chat и догружать превью отдельными вызовами к Messaging на своей стороне.

**Текущее состояние кода:** `src/backend/chat/main.go` подключает `ListChatsEnrichment` при наличии `MESSAGING_GRPC_ADDR`; Chat вызывает Messaging S2S `GetChatListMetadata` и заполняет `last_message_preview` / `unread_count`. Если Messaging не сконфигурирован, список чатов всё равно возвращается, а эти поля остаются пустыми / нулём.

**Индекс** `chat_members_profile_id_idx` используется для фильтрации по `profile_id`; сортировка опирается на `chats.last_message_at` / `created_at` (см. `chats_last_message_at_idx`).

## Folders

**Folder CRUD** (`ListFolders`, `CreateFolder`, `UpdateFolder`, `DeleteFolder`): **`ListFolders`/`CreateFolder` shipped (Batch 18)** — lazy system seed + custom create; **`UpdateFolder`/`DeleteFolder` shipped (Batch 20)** — custom rename/reorder/delete; system folders immutable.

**System folders** (seed per profile): All, DM, Groups, Channels, Spaces — `folder_type=system`, immutable name/delete; `filter_config_json` задаёт predicate (см. [navigation.md](../features/navigation.md)).

**Custom folders:** user-created; membership explicit via `folder_chats`.

### `ListChatsRequest.folder_id` (shipped Batch 19)

Расширение `ListChatsRequest`:

```protobuf
message ListChatsRequest {
  voice.common.v1.CursorPageRequest page = 1;
  optional string inbox = 2;   // main | requests | archive
  optional string folder_id = 3;
}
```

| `folder_id` | Filter |
|-------------|--------|
| omitted + `inbox=main` | non-archived chats for profile (current default) |
| system folder id | chats matching folder predicate ∩ `is_archived=false` |
| custom folder id | join `folder_chats` WHERE `folder_id` ∩ `is_archived=false` |
| `inbox=archive` | `is_archived=true` (ignores `folder_id`; **implemented**) |

Sort within folder: pinned first (`pin_order ASC`), then `sort_order`, then activity (`last_message_at`).

### Folder membership + pin RPCs (shipped Batch 19)

```protobuf
rpc AddChatToFolder(AddChatToFolderRequest) returns (Empty);
rpc RemoveChatFromFolder(RemoveChatFromFolderRequest) returns (Empty);
rpc ReorderFolderChats(ReorderFolderChatsRequest) returns (Empty);
rpc PinChatInFolder(PinChatInFolderRequest) returns (Empty);
rpc UnpinChatInFolder(UnpinChatInFolderRequest) returns (Empty);

message AddChatToFolderRequest {
  string folder_id = 1;
  string chat_id = 2;
  optional int32 sort_order = 3; // append if omitted
}
message RemoveChatFromFolderRequest {
  string folder_id = 1;
  string chat_id = 2;
}
message ReorderFolderChatsRequest {
  string folder_id = 1;
  repeated string chat_ids = 2; // full ordered list for profile+folder
}
message PinChatInFolderRequest {
  string folder_id = 1;
  string chat_id = 2;
  optional int32 pin_order = 3;
}
message UnpinChatInFolderRequest {
  string folder_id = 1;
  string chat_id = 2;
}
```

**Pin rules:** archived chats reject pin; system folder pin creates/updates overlay row in `folder_chats` without requiring prior membership row for custom-only chats (chat must still match system predicate).

## Quick Access

Separate from folder pin. Target: **`chat_id` only** (not space/tree polymorphic entry). Limit **15** per `profile_id`. Archiving a chat **must** remove it from Quick Access.

```protobuf
rpc ListQuickAccess(ListQuickAccessRequest) returns (ListQuickAccessResponse);
rpc AddQuickAccess(AddQuickAccessRequest) returns (Empty);
rpc RemoveQuickAccess(RemoveQuickAccessRequest) returns (Empty);
rpc ReorderQuickAccess(ReorderQuickAccessRequest) returns (Empty);

message QuickAccessItem {
  string chat_id = 1;
  int32 sort_order = 2;
  Chat chat = 3; // optional hydrate via GetChat
}
message ListQuickAccessRequest {}
message ListQuickAccessResponse {
  repeated QuickAccessItem items = 1;
}
message AddQuickAccessRequest {
  string chat_id = 1;
  optional int32 sort_order = 2;
  string replace_chat_id = 3; // existing slot to replace atomically; empty for ordinary add
}
message RemoveQuickAccessRequest {
  string chat_id = 1;
}
message ReorderQuickAccessRequest {
  repeated string chat_ids = 1; // full ordered list, max 15
}
```

**At-limit UX (normative):** When the profile already has 15 QA slots, the client shows a **replace picker** ([navigation.md](../features/navigation.md) § Quick Access, [screen-controls.md](../design/screen-controls.md) §1.1c #6): list current slots → user picks one to replace → `AddQuickAccess(replace_chat_id=<selected chat>)` atomically replaces that slot while preserving its order. On failure the original slot remains. No dedicated `ReplaceQuickAccess` RPC.

**Errors:** `AddQuickAccess` returns `FAILED_PRECONDITION` when count is already 15 and no replacement slot was supplied (server safety net), or when the selected replacement slot changed on another device. The product UX at 15/15 is **not** a hard error toast — client **must** open the replace picker instead ([navigation.md](../features/navigation.md) § Quick Access «Server at-limit error», [screen-controls.md](../design/screen-controls.md) §1.1c #6). `NOT_FOUND` if the new chat is not a membership of caller; `ALREADY_EXISTS` if the new chat already occupies another Quick Access slot. Failed replacement leaves the selected slot unchanged.

## Archive

| RPC / field | Status |
|-------------|--------|
| `ArchiveChat(chat_id, archived)` | **Implemented** — sets `chat_members.is_archived` |
| `ListChats` main inbox | **Implemented** — excludes `is_archived=true` |
| `ListChats` with `inbox=archive` | **Implemented (Batch 15)** — archived `dm` / `group` / `channel`; ignores `folder_id` |
| Side-effect: remove Quick Access on archive | **Implemented (Batch 18)** — `ArchiveChat(archived=true)` calls `RemoveQuickAccess` |
| Incoming message keeps chat archived | **Implemented** — `message.sent` updates activity but preserves per-member `is_archived=true`; unread metadata remains owned by Messaging |

**Spec:** archive write/list and Quick Access side-effect are implemented (Batch 15/18). Group/channel use the same per-member `is_archived` column and are returned by `inbox=archive`. Message activity preserves the archived state under the canonical badge-only policy; client UX status is tracked separately in [todo/client.md](../todo/client.md).

Unarchive semantics: [GLOSSARY.md](../GLOSSARY.md) § «Архив чата», [text-chat.md](../features/text-chat.md) § «Архивирование».

## Sticker packs

Catalog + per-profile install state. Binary assets — **File Service** ([file-service.md](file-service.md) § Stickers and GIF assets); send wire — **Messaging** ([messaging-service.md](messaging-service.md) § Stickers and GIF). The initial catalog slice implements installed-pack list, pack details, install, and uninstall; pack creation, asset ingest, GIF search, and message delivery remain separate work.

### Data model (spec — canonical; Messaging validates against this DDL only)

```
sticker_packs
├── id (UUID)
├── title
├── thumb_file_id (nullable — File Service)
├── is_system (bool) — shipped with app / CDN seed
├── is_premium (bool) — ★ install gate
├── creator_profile_id (nullable — user pack)
├── sticker_count (int, denormalized — maintained on sticker add/remove)
├── created_at
└── updated_at

stickers
├── id (UUID)
├── pack_id (FK)
├── file_id (FK logical → file_db; File intent=sticker)
├── emoji (string, optional shortcut / associated emoji for recents)
├── sort_order (int)
├── width (int — from File metadata at ingest)
├── height (int — from File metadata at ingest)
├── UNIQUE(pack_id, file_id)
└── UNIQUE(pack_id, sort_order)

profile_installed_packs
├── profile_id
├── pack_id
├── sort_order (int) — rail order in composer picker
├── installed_at
└── PRIMARY KEY (profile_id, pack_id)
```

**Do not duplicate this schema in other service docs** — [messaging-service.md](messaging-service.md) § Stickers and GIF cross-refs here for validation/send only.

### gRPC

```protobuf
// Sticker catalog
rpc ListInstalledStickerPacks(ListInstalledStickerPacksRequest) returns (ListInstalledStickerPacksResponse);
rpc GetStickerPack(GetStickerPackRequest) returns (GetStickerPackResponse);
rpc InstallStickerPack(InstallStickerPackRequest) returns (InstallStickerPackResponse);
rpc UninstallStickerPack(UninstallStickerPackRequest) returns (InstallStickerPackResponse);
// CreateUserStickerPack/AddStickersToUserPack follow the File ingest slice and are not part of this catalog contract.

// GIF provider search — owner: ChatService (HTTP adapter to Giphy or Tenor)
rpc SearchGifs(SearchGifsRequest) returns (SearchGifsResponse);
rpc GetTrendingGifs(GetTrendingGifsRequest) returns (SearchGifsResponse);

message SearchGifsRequest {
  string query = 1;
  int32 limit = 2;   // default 20, max 50
  string cursor = 3;
}
message GifResult {
  string provider = 1;       // "giphy" | "tenor"
  string provider_id = 2;
  string file_id = 3;        // File row after import (may be pending briefly)
  string preview_url = 4;
  int32 width = 5;
  int32 height = 6;
  float duration_seconds = 7;
}
message SearchGifsResponse {
  repeated GifResult items = 1;
  string next_cursor = 2;   // **single owner:** Chat SearchGifs only — pass opaque cursor on next request; File/Messaging MUST NOT define parallel GIF search pagination
}
```

**Pagination:** `SearchGifsRequest.cursor` ↔ `SearchGifsResponse.next_cursor` — opaque provider- or Chat-cache token; **only Chat Service** owns GIF search cursors. Empty cursor = first page.

Gateway REST (sketch): `GET /api/v1/sticker-packs`, `POST /api/v1/sticker-packs/{id}/install`, `GET /api/v1/gifs/search?q=…&cursor=…`, `GET /api/v1/gifs/trending` — full table [api-gateway.md](api-gateway.md) § Stickers and GIF.

**Send wire:** Messaging `SendMessage` + `content_type=STICKER|GIF` — full validation — [messaging-service.md](messaging-service.md) § Stickers and GIF.

**Rules:** system packs are deployment-owned rows (`is_system=true`) and are never seeded with fake product data by the service; a user pack is visible/installable only to its creator; installed state is keyed by the authenticated active profile. **`UninstallStickerPack` rejects `is_system` packs** and non-creator user packs; uninstall does not delete sent messages. Premium ★ checks, user-pack ingest, GIF search/recents, and provider caching remain outside this catalog slice.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`chat.events`** (совместно с Space для событий дерева/спейса; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие               | Данные                             |
|-----------------------|------------------------------------|
| `chat.created`        | chat_id, type, creator_id, members |
| `chat.updated`        | chat_id, changed_fields            |
| `chat.deleted`        | chat_id                            |
| `chat.member_added`   | chat_id, profile_id, added_by      |
| `chat.member_removed` | chat_id, profile_id, removed_by    |
| `chat.member_left`    | chat_id, profile_id                |
| `chat.member_changed` | chat_id, profile_id, change (`added` \| `removed` \| `left` \| `owner_transferred` \| `inbox_bucket_changed` \| `role_changed`) |
| `chat.dm_peer_deleted` | chat_id, recipient_profile_id; только surviving участнику, без deleted identity |

**Note:** JetStream payload for `chat.member_changed` may emit `removed`, `owner_transferred`, and inbox transitions beyond minimal proto comment — consumers must tolerate unknown `change` values.

## Зависимости

- **Social Service** — проверка блокировок при создании DM
- **User Service** — public получение профилей участников; отдельный internal owner lookup для lifecycle filtering свежего `ListChats` snapshot
- **Messaging Service** — для `ListChats`: превью последнего сообщения и `unread_count` по данным `messaging_db` (S2S, см. раздел «ListChats»); без интеграции список возвращается без этих полей
  - также единственный trusted S2S caller `ListDMReceiptVisibilityTargets`: paginated `chat_id` + peer profile для symmetric read-receipt revoke. RPC не маршрутизируется через Gateway; включает `main`, `requests` и `archive` DM, исключая group/channel и некорректные DMs не с двумя участниками.
- **Subscription Service** — лимиты на количество участников группы
- **Space Service** — при создании текстового чата (`group` \| `channel`) в спейсе: узел **`space_tree_nodes`** (`kind=text_chat`) после создания строки `chats`

## P3 Space lifecycle participant (target)

Chat owns a durable per-Space generation fence, immutable manifest header/pages,
request/receipt evidence and compact permanent `PURGED` fence. On `FROZEN`, its
transaction blocks Space-chat/navigation mutation and captures every
`chats.space_id` UUID sorted by raw 16-byte value in pages of at most 1000. This
is the first downstream freeze linearization point. Exact pages/count/hash are
replayed; the same page index with changed bytes conflicts.

Space-bound group and channel chat creation acquire the same PostgreSQL
transaction-scoped advisory lock before inserting. Space and Chat derive its
key through the shared `voice/backend/pkg/spacemutationlock` package. With no
lifecycle fence, or an explicit `LIVE` fence, creation proceeds; a frozen or
terminal fence rejects the insert with the lifecycle-state error. This
serializes each create against manifest capture: a create that wins the lock is
visible in the manifest, and a freeze that wins first prevents the new chat.
Ordinary resource lookups and navigation queries exclude every Chat whose
local Space fence is not `LIVE`, including the manifest-capture interval before
the participant barrier finishes. This applies to direct Chat reads, inbox and
archive pages, Space projections, custom/system folders and quick-access slots.
The stored navigation preferences remain intact and reappear after the higher
`LIVE` restore generation. A frozen shortcut does not make unrelated live or
standalone shortcuts unavailable. The authenticated lifecycle participant reads
its saved immutable manifest directly; ordinary lookup is never a purge bypass.

The Chat source manifest root is `SHA-256("voice.chat.v1.SpaceDeletionManifest\\0" ||
space_uuid_bytes || deletion_operation_uuid_bytes || schedule_generation_u64be ||
item_count_u64be || ordered_chat_uuid_bytes...)`. Each page digest is
`SHA-256("voice.chat.v1.SpaceDeletionManifestPage\\0" || root_sha256 ||
page_index_u64be || item_count_u64be || ordered_chat_uuid_bytes...)`. The
manifest ID is UUIDv5 using the deletion operation ID as namespace and the
literal UTF-8 name `voice.chat.v1.SpaceDeletionManifest/<space-id>/<generation>`.
Page tokens are unpadded base64url of `page_index_u64be || root_sha256`; page
zero has no input token. Chat validates the token against the saved root and
returns immutable page bytes only.

Space sends this canonical Chat binding unchanged to all ten lifecycle
participants. File separately verifies its private producer-reference root;
Space does not reconstruct that root. Chat's fence, page lookup, restore and
purge validate the exact saved Chat manifest ID/hash/count. The deletion
operation ID remains a separate identity and is not a replacement manifest ID.

Messaging, Search, Bot and Notification import/acknowledge those exact pages.
Search reads pages on a separate mTLS listener configured by
`CHAT_SEARCH_MANIFEST_GRPC_LISTEN`, `CHAT_SEARCH_MANIFEST_TLS_CERT_FILE`,
`CHAT_SEARCH_MANIFEST_TLS_KEY_FILE`, `CHAT_SEARCH_MANIFEST_CLIENT_CA_FILE`,
`CHAT_SEARCH_PRINCIPAL_JWKS_URL`, `CHAT_SEARCH_PRINCIPAL_JWKS_CA_FILE`, and
`CHAT_SEARCH_PRINCIPAL_REPLAY_REDIS_ADDR`. Optional JWKS client certificate/key
settings (`CHAT_SEARCH_PRINCIPAL_JWKS_CLIENT_CERT_FILE` and `_CLIENT_KEY_FILE`)
must be paired. Startup verifies the HTTPS Search rotation keys and replay
store before exposing the listener. Only Search's request-bound
`GetSpacePurgeManifestPage` is allowed there; lifecycle mutations remain
Space-only. The ordinary and GIS listeners cannot serve these page requests.
Chat's fence receipt binds the canonical Chat root manifest. Restore applies the
higher `LIVE` generation without recapture. Purge exposes only the saved pages,
waits for Messaging completion and File acceptance of Chat-owned reference
releases, then removes chats/navigation and returns an immutable completion
receipt. Full request/receipt bytes retain 30 days from this participant's
completion; compact terminal fence is permanent.

### P3 terminal purge prerequisite proof contract (accepted)

Space's generic purge request is a durable decision, not evidence that upstream
participants completed. Chat obtains each owner's evidence directly before
deleting its rows:

1. Chat makes a read-only authenticated, request-bound Messaging receipt lookup
   for the exact Space, deletion operation, purge generation, source schedule
   generation and Messaging participant request hash. Messaging returns only
   its already committed participant-3 completion receipt, after its own
   `MESSAGING` File release completed; this lookup never starts Messaging purge.
2. Chat submits the deterministic File
   `ReleaseSpaceDeletionProducerReferences` request for producer `CHAT`, using
   the producer tuple-set hash sealed for the source schedule generation. The
   current Chat schema has no File-reference producer rows, so this is the
   deterministic empty-set hash; File still verifies its durable seal and
   returns a receipt bound to the exact Space/deletion/purge/source generations,
   producer, hash and request. An identical retry returns the same receipt
   after response loss, so Chat owns retry without a separate File lookup.
3. Only after both owner responses validate does Chat atomically persist their
   deterministic bytes with its completion receipt and delete the exact saved
   Chat manifest. Missing, changed, incomplete, wrong-owner, wrong-generation
   or mismatched-hash evidence leaves rows and `PURGE_DECIDED` untouched.
   Restart reuses the saved manifest and exact request bytes; it never
   recaptures the work set.

Both owner calls use service-to-service TLS plus a short-lived request-bound
`service:chat` principal with exact audience, RPC and deterministic request
hash. Chat validates each owner response field-by-field against persisted
evidence. A Space coordinator assertion or unsigned metadata is never accepted
as proof. Messaging adds a read-only receipt lookup; File's producer release
call already provides idempotent lost-response recovery. This contract assigns
the retries to Chat and reserves no protobuf field numbers.

The outbound clients are enabled with the inbound Space lifecycle principal
listener and require all of `CHAT_SPACE_PURGE_MESSAGING_GRPC_ADDR`,
`CHAT_SPACE_PURGE_MESSAGING_TLS_CA_FILE`,
`CHAT_SPACE_PURGE_MESSAGING_TLS_SERVER_NAME`,
`CHAT_SPACE_PURGE_FILE_GRPC_ADDR`, `CHAT_SPACE_PURGE_FILE_TLS_CA_FILE`,
`CHAT_SPACE_PURGE_FILE_TLS_SERVER_NAME`, `CHAT_SPACE_PURGE_CLIENT_CERT_FILE`,
and `CHAT_SPACE_PURGE_CLIENT_KEY_FILE`. The client certificate is presented to
both owner services; each connection validates its own CA and server name.
