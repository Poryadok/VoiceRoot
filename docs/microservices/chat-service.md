# Chat Service

## Standalone channel publication authority

For a stored `channel` with `space_id IS NULL`, Chat owns the current
`chat_members` role, membership-scoped official grant and `allow_user_main_feed`
setting. Current `owner`/`admin` and ordinary members with an explicit current
`post_as_chat` grant may publish as the channel (`posted_as_chat=true`). User-identity
thread replies retain the existing thread policy; adding `thread_parent_id`
never grants an ordinary member the channel's official identity. User-identity
main-feed publication requires both a known current role (`owner`, `admin` or
`member`) and explicit `allow_user_main_feed=true`; the disabled setting also
denies owner/admin user-identity posts. Standalone owner/admin `UpdateChat`
controls that setting. No channel promotion API is implied by this contract.

Messaging uses the existing Chat boundary (fresh `GetChatPermissions` via the
production `GRPCChatGuard`) for official Send, Forward and copy-as-new. MemberRole
alone never proves an ordinary member's official capability.
Forward chooses official mode when user main-feed posts are disabled and user
mode when enabled; commentary uses the same identity. Stored type/scope wins
over caller `ChatRef.type`. Missing membership, unknown role, missing policy or
unavailable authority never grants publication. Denials precede commentary,
message and event/outbox writes. Space-attached destinations continue to require
the exact-chat Role `TEXT_CHAT_SEND_MESSAGES` decision; local roles cannot
replace it. See [forward contract](../features/forward-messages.md#права-основного-потока-канала).

## Обзор

Управление сущностями чатов: DM (1:1), текстовые **группы** и **каналы** (`type = group` \| `channel`, в т.ч. вне спейса; одинаковая модель, разные дефолты), пользовательские папки.

**Язык**: Go
**БД**: PostgreSQL `chat_db`

Accepted Space-attached Chat audit facts use the frozen
[external codec](space-service.md#external-audit-details-codec-v1) and
[replay identity](space-service.md#durable-external-replay-codec-v1), shared through
`src/backend/pkg/spaceaudit`. Build actual normalized changes once inside the
mutation/outbox transaction; retries preserve those bytes. Composite creation
emits its fact only after the complete Chat/tree result. Producer runtime remains
BE-131 work; this codec contract does not activate its transport.

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

### Native Space-chat scope authority

`ChatScopeAuthorityService.GetSpaceChatScopeAuthority` is a narrow own-`chat_db`
read for signed `service:role` and `service:space`. Its request binds protocol
version 1, canonical `chat_id`, `expected_space_id` and `operation_id`. A known
matching group/channel returns only echoed scope, existence/liveness, stored
type/Space, UUID attachment incarnation, positive monotonic Chat authority
revision, independent lifecycle generation/state and managed kind. It returns
no content, title, membership or Role decision and calls no other service.
Missing and cross-Space requests use the same NotFound outcome; unavailable or
inconsistent source state never becomes a permissive default. Consumers accept
only an ordinary LIVE scope and retain their own fresh authorization checks.

Migration `chat_db/000025` maintains `chats.scope_incarnation` and permanent
`chat_scope_authority_heads` in the source transaction. Reparenting, type or
ownership/managed binding reassignment replaces the incarnation. Cosmetic
title/topic/category ordering does not. Delete preserves the head and advances
its revision; same-ID recreation receives a new incarnation and higher revision.
Lifecycle state/generation transitions advance the same observation revision
without replacing the independent lifecycle generation. Revision overflow aborts
the mutation; the head and fences cannot be truncated or reset. This migration
has no provenance-safe DOWN. Existing native creation and P3 mutation barriers
remain in force; no new attachment/reparent API is introduced.

All returned facts come from one PostgreSQL statement snapshot of the native
row, its head and lifecycle fence. Source triggers cover consumed fields;
the MatchSquad operation insert covers managed-kind ownership. No read lock is
held across network I/O. Space owns its tree relation separately: this Chat
snapshot does not replace the exact Space-tree authority observation.

Role and Space each use a dedicated listener configured by
`CHAT_SCOPE_ROLE_*` or `CHAT_SCOPE_SPACE_*`: `GRPC_LISTEN`, `TLS_CERT_FILE`,
`TLS_KEY_FILE`, `CLIENT_CA_FILE`, `JWKS_URL`, `JWKS_CA_FILE`,
`REPLAY_REDIS_ADDR` and optional `REPLAY_REDIS_PASSWORD` plus paired
`JWKS_CLIENT_CERT_FILE`/`JWKS_CLIENT_KEY_FILE`. Partial configuration fails
startup. The listener verifies mTLS and exact issuer, subject, Chat audience,
full RPC, request ID, complete protobuf hash and fresh JTI through the existing
principal verifier/replay store. Ordinary and other protected Chat listeners
do not register this service. Federation-only authority remains unchanged.
Serving this producer is not activation of the downstream permission/admission
vertical or permission to deploy.

`GetBotChatScopeAuthority` is a separate Bot-only read under BE170. Its dedicated
request has the same four scope fields/tags and returns `scope` plus the raw32 SHA256 of the deterministic
complete request. Only `service:bot` may call this exact method through its own
`CHAT_SCOPE_BOT_*` configuration, using the same mTLS/JWKS/replay configuration
keys above. Bot cannot call the Role/Space read; their listeners cannot call the
Bot read. No user, bot-actor or mixed identity supplies this authority.

The Bot read may also return a retained head whose chat row is absent,
`present=false`, last Space matches and incarnation/revision remain valid.
Such a response has `exists=false`, `live=false` and unspecified type/managed
kind; the head does not retain those historical facts. Missing/foreign/DM or
standalone resources yield indistinguishable NotFound. Contradictory head or
lifecycle state yields Unavailable. Current resource liveness reports the
independent lifecycle state; managed kind remains an explicit separate fact.
No new schema, source mutation, public reparent or identity reuse API is added.

This is a point-in-time observation, not a reservation or cleanup receipt. Bot
must persist exact request/response bytes and source tuple before newly adding
or enabling whitelist targets, retaining existing owner/manageBots decisions.
New targets require a present matching group/channel and LIVE lifecycle. A
retired observation can qualify historical cleanup scope, never prove effects
completed. Messaging independently validates current native scope through its
existing authorized Chat data source before first cleanup; exact completed
replays preserve their original result and later human repins. Source revision
changes require fresh qualification while preserving original evidence. A read
does not serialize unsupported concurrent cross-database parent/identity rewrites.

**Effective-membership revision/export target.**

The [ListThreads effective-membership contract v1](../testing/listthreads-versioned-readmodel-exec-plan.md#effective-membership-contract-v1)
defines Chat's additive internal target, not a shipped API. Chat composes complete
Space membership, Role-owned `TEXT_CHAT_VIEW` decisions and User eligibility at a
validated common source cut. Local membership, actual DM self-delete/reopen,
Space binding and lifecycle changes participate in the transactional source
revision/outbox. Chat persists effective generations, monotonic pair revisions,
revocation lineages and committed journal `Q`; it exports pinned complete
snapshots plus replay to signed caller `messaging` and supplies exact-pair fresh
admission. First-N enumeration, unsigned caller metadata and a Chat-local table
trigger alone cannot establish authority. DM self-delete retires only its viewer
lineage, actual reopen creates a new one, and chat archive remains navigation
state. The linked contract owns the full source/caller/recovery and race matrix.

### Chat management permissions

This target extends the existing standalone owner/admin official-publication
baseline; it does not invalidate the BE-187 official-thread authorization fix.
The standalone persistence, handlers, HTTP routes and member-management controls
implement this contract. The Space authority/admission/navigation vertical below
remains an accepted target; standalone support does not complete that scope.
For ordinary human-owned standalone groups and channels Chat owns
`member_additions_enabled` (default true) and a membership-scoped
`post_as_chat` grant (default false). Current owner/admin implicitly have both
capabilities and may manage these settings. An ordinary member can add only
while the toggle is enabled and use the chat identity only with an explicit
grant. Send/settings permissions never imply either capability.

`GetChatPermissions(chat_id)` returns caller-relative
`can_post_as_chat`, `can_add_members`, `can_manage_permissions`,
`member_additions_enabled`, and `explicit_post_as_chat`.
`UpdateChat.member_additions_enabled` is optional; omission preserves the
setting. `SetChatMemberPostAsChat(chat_id, target_profile_id, allowed)` is an
idempotent owner/admin operation on a current ordinary member. No actor is
selected by an untrusted request body. Member-management projections expose
the explicit grant only to authorized roster readers.

HTTP: `GET /api/v1/chats/{chat_id}/permissions`,
`PUT /api/v1/chats/{chat_id}/members/{profile_id}/post-as-chat` with
`{"allowed": boolean}`, and `PATCH /api/v1/chats/{chat_id}` with optional
`member_additions_enabled`. Route identifiers override body identifiers.
The roster editor exposes the addition policy and per-member official grant
separately. Unknown/unavailable capability state disables those controls.

All mutations recheck current actor/target membership and role in the same
Chat transaction as the write. AddMembers rechecks its caller and toggle in
the roster transaction. Leave/kick/profile cleanup removes the explicit grant
with membership; rejoin starts without it. Promotion never creates a grant.
Demotion/ownership transfer removes implicit authority but preserves a
separately explicit grant throughout that same membership. Toggle false does
not remove existing people. Chat-row then membership locks serialize changes.
No historical send/settings/user-feed value is migrated into an explicit grant.

Messaging obtains a fresh Chat decision for the authenticated sender and actual
stored destination before official main-feed/thread/send/forward/commentary
writes. Missing/unavailable authority fails closed; no cached UI grant, forged
ChatRef, or local owner field is proof. Cross-service authorization linearizes
at the completed fresh decision, not at a fictional shared transaction.
Other privacy, guest/account, media, moderation, E2E, thread and feed-mode gates
still apply. False policy is PermissionDenied; malformed IDs InvalidArgument;
unsupported scope/configuration FailedPrecondition; dependency failure
Unavailable, with existing missing-resource visibility semantics preserved.

DM, GIS-managed and MatchSquad chats reject ordinary grant/toggle operations.
Guest policy retains its existing gates; this target adds no blanket ban or
guest initiation permission. Space chats never use standalone setters or local
AddMembers. Their membership belongs to Space and authority to Role.

For a chat in a Space, adding a person means inviting them into its parent Space
through the existing admission rules, then opening the specific invited chat
after successful authorized entry. Preserve the target binding across pending
admission and retry. Current members are not inserted twice and still require
current target-chat visibility/access. Never disclose or open unauthorized
content, bypass bans/caps/entry policy/hierarchy/channel ACL, or create chat-only
Space membership. Distinct Role authority, durable invitation binding and
Gateway/client continuation are part of the full target; standalone support
alone is not completion of this requirement.

Rollout must establish a compatible serving set of Chat and Messaging writers
before exposing these policy mutations: an older Chat writer cannot enforce
the new addition toggle. Migration defaults preserve the previous enabled-add
behavior and create no explicit grants. This is an operator qualification gate,
not evidence of deployment or a permission to deploy.

`CreateChatRequest.request_id` необязателен для совместимости существующих
клиентов. Канонический UUID связывается с аутентифицированным
`creator_profile_id`; в той же транзакции Chat сохраняет хэш нормализованных
полей create и созданный `chat_id`. Тот же ключ и те же поля возвращают тот же
чат без повторного insert или повторной публикации `chat.created`; тот же ключ
с другими полями возвращает `AlreadyExists`. При timeout клиент сверяет исход
повтором с тем же ключом. Запросы без ключа используют прежний путь создания.

Источник истины: [protos/voice/chat/v1/chat.proto](../../protos/voice/chat/v1/chat.proto). Ниже — инвентарь с **статусом реализации** (handler в `src/backend/chat/internal/grpcsvc/`).

```protobuf
service ChatService {
  // DM
  rpc CreateDM(CreateDMRequest) returns (CreateDMResponse);       // ✓
  rpc GetDM(GetDMRequest) returns (GetDMResponse);               // ✓ find or create
  rpc CanCreateDM(CanCreateDMRequest) returns (CanCreateDMResponse); // ✓ read-only {allowed}

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
  rpc GetChatPermissions(...) returns (...);                              // ✓ standalone caller-relative
  rpc SetChatMemberPostAsChat(...) returns (...);                         // ✓ standalone owner/admin grant

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

`CanCreateDM` evaluates the authenticated caller against the selected active target, account blocks, and the target's `allow_dm` audience. A policy denial, blocked pair, guest-ineligible caller, self-target, or missing/inactive target returns `allowed=false` without revealing the specific reason or privacy settings. Missing authentication, malformed identifiers, and unavailable authorization dependencies retain typed errors. This RPC does not depend on the DM store and never creates/promotes/clears a conversation or publishes chat events. `CreateDM` repeats its existing checks and remains authoritative when opening a chat.

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

### Public composite replay authority (BE-131)

Every public Space-attached `CreateChat` result, including completed keyed
replay, uses a fresh signed `service:chat` outcome lookup with a typed
`public_actor` copied from the current verified initiator. Space checks current
membership, lifecycle and guest admission before private receipt/expiry lookup,
then binds disclosure to the original profile/account or bot installation.
Current actor facts never replace immutable creation intent. Guest disclosure
also checks the current Chat-owned `allow_guests`; a saved response is not current
authority. Dependency failure denies disclosure. Private recovery omits
`public_actor` and does not acquire this later ACL gate.

Chat main composes the protected listener and durable recovery/effect worker
together with complete `CHAT_CREATE_*` configuration: `GRPC_LISTEN`,
`TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`, `PRINCIPAL_JWKS_URL` (Gateway),
`BOT_PRINCIPAL_JWKS_URL`, `PRINCIPAL_JWKS_CA_FILE`,
`PRINCIPAL_JWKS_CLIENT_CERT_FILE`, `PRINCIPAL_JWKS_CLIENT_KEY_FILE`,
`PRINCIPAL_REDIS_ADDR` and optional `PRINCIPAL_REDIS_PASSWORD`. Both HTTPS JWKS
endpoints require distinct current/next RSA keys. `CHAT_CREATE_AUTH_*` and
`CHAT_CREATE_SPACE_*` and `CHAT_CREATE_AUDIT_*` each require `GRPC_ADDR`, `TLS_CA_FILE`, `TLS_SERVER_NAME`,
`CLIENT_CERT_FILE` and `CLIENT_KEY_FILE`. The runtime additionally requires Chat
signing keys, composite schema and JetStream publisher. Partial configuration
stops startup; the ordinary listener always denies CreateChat. This source
composition does not prove deployment secret configuration or activation.
`CHAT_CREATE_SPACE_*` targets Space's commit/outcome listener;
`CHAT_CREATE_AUDIT_*` targets its distinct protected audit-ingestion listener.
The composite method allowlist is never widened to deliver audit facts.
Legacy snapshots without a Space composite receipt fail closed on this public
path pending separate legacy reconciliation; replay cannot invent a receipt or
historical audit.

### Space chat creation: durable composite target (BE131-COMPOSITE-v1)

This is the accepted target; the current two-call Gateway create/tree path is
not proof of this behavior. Chat owns the durable create operation and audit
payload in `chat_db`; Space owns the authoritative tree outcome in `space_db`.
Gateway is a stateless transport and never authors an audit fact or receipt.

Every new Space-attached `CreateChat`, including the protected direct RPC and
generic HTTP entrypoint, follows the same composite. Standalone creation is
unchanged. The existing `(creator_profile_id, request_id)` key and normalized
create digest bind one execution kind and one operation, Chat, node and audit
UUID allocated once. Changed bindings conflict. Requests without a client key
receive a durable server operation, but independent unkeyed retries do not gain
a deduplication guarantee. Existing successful keyed creates are explicitly
legacy: replay preserves their original Chat without inventing a past audit.

Chat first atomically writes a prepared Chat, its operation/reservation and the
immutable `chat_created` producer payload with eligibility `HELD`. The original
verified initiating actor, Space, target Chat, audit UUID, `{}` details and
original creation Timestamp are frozen here. The prepared Chat is unavailable
to ordinary reads, lists, use, mutations and tree links, and emits no ordinary
`chat.created` notification.

A dedicated protected Chat-to-Space commit/outcome protocol binds every stored
operation/creator/Space/Chat/type/node/audit/digest field. Space rechecks current
membership, `TEXT_CHAT_CREATE_IN_SPACE`, guest/bot scope, limits and lifecycle
before the first tree commit. Service authentication alone is not user
authorization. Space serializes its node insertion with an immutable
`COMMITTED` receipt, or a deterministic refusal with an `ABORTED` receipt. These
outcomes are mutually exclusive and terminal. This private tree insertion does
not create a duplicate `tree_node_upserted` audit; ordinary tree APIs keep their
audits and accept no generic audit-suppression flag.

Chat authenticates the outcome, compares all bindings, then atomically marks
the operation complete, activates its prepared Chat, changes the same outbox
payload to `READY`, and schedules the ordinary `chat.created` event with stable
deduplication identity. Success waits for that local finalization. The Gateway
returns the original Space node outcome and does not issue a second unbound
`UpsertTreeNode`. Delivery is at least once with stable identities; a transport
exactly-once guarantee is not implied. Tree visibility after Space commit and
before Chat activation may briefly precede Chat usability; the two databases do
not provide atomic global read visibility.

A Chat recovery worker resumes incomplete durable operations without Gateway
or the original client. Each attempt uses fresh bound service credentials and
holds no database transaction across network I/O. Timeout, response loss,
missing receipt, expired lease and process crash mean unknown outcome, never
permission to delete a prepared Chat. Only an authenticated exact `ABORTED`
receipt permits guarded local compensation and cancellation of the held fact.
After `COMMITTED`, recovery rolls forward even if permissions later change; a
later node move/removal cannot be undone by replay. Public result disclosure
still checks current caller access independently of historical completion.

Full operation/outcome evidence and creator/request replay indexes have no
ordinary TTL before terminal purge. BE131-COMPOSITE-PURGE-v1 bounds Chat's full
copy to its own successful local P3 completion plus 30 days; Space's copy uses
aggregate PURGED plus 30 days. Each owner fixes its deadline once, checks expiry
using database time even before physical cleanup, and erases all duplicated
personal fields, replay indexes and identity/content digests at expiry. Protected
reads then return `FAILED_PRECONDITION / COMPOSITE_RESULT_EXPIRED` without a
fabricated outcome; an expired fence never authorizes finalize or compensation.
Permanent compact fences retain only the original server resource IDs, protocol
and execution kind, terminal state/safe failure code, deletion operation and
generation, completion timestamp and fixed expiry. They retain no creator,
account/session, bot/installation, client key, content or derivative hash/HMAC.
Permanent Space/resource nonreuse remains after owner tombstone expiry; public
purged-resource admission remains undiscoverable.

Freeze first commits the local admission fence under the preparation lock, then
durably enters `DRAINING_COMPOSITES`. Recovery settles every PREPARED/HELD
operation using exact authenticated Space outcomes. COMMITTED finalizes while
publicly hidden by FROZEN; ABORTED compensates; UNKNOWN blocks progress. Only
after a locked zero-incomplete recheck may Chat seal its immutable manifest and
acknowledge freeze. No transaction spans RPC. Purge additionally requires actual
delivery evidence for pre-purge audit and ordinary chat.created obligations,
alongside existing participant prerequisites; send attempts or expired leases
are insufficient. Unexpected incomplete work after terminal purge is a
quarantined invariant failure and cannot extend retention or recreate resources.
This contract does not activate deletion or purge. BE-131 owns producer
eligibility/recovery; BE-008 consumes the complete Gateway routing/node-result
and negative transport vertical. Required evidence includes crash recovery
after Chat prepare and after Space tree commit, with no Gateway, one logical
audit and no duplicate internal tree event.

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
├── member_additions_enabled (bool, default true — standalone ordinary-member additions)
├── e2e_enabled (bool, default false — DM opt-in E2E)
├── created_at
└── updated_at

chat_members
├── chat_id (UUID, logical ref → chats.id)
├── profile_id (UUID, logical ref → user_db.profiles.id; без меж-БД REFERENCES)
├── role (owner | admin | member)
├── post_as_chat (bool, default false — explicit grant for this membership)
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

### Deployed schema (migrations `000001`–`000021`) vs full spec

**Shipped today** (`chat_db` migrations): DM + group + channel types; `chat_members.inbox_bucket`; `threads_enabled` / `allow_user_main_feed`; `e2e_enabled`; slow mode; chat-level `allow_guests` (`000007`, default hardened to `false` and legacy standalone group/channel rows backfilled by `000012`); `folders` + `folder_chats`; `quick_access_chats`; per-profile `deleted_for_self` (`000011`). Standalone group/channel owners and existing admins configure future guest admission through `UpdateChat`; admission is checked against User's guest marker fail-closed, and disabling it preserves current memberships. Folder membership/pin, `ListChats.folder_id` and `UpdateFolder`/`DeleteFolder` are implemented. Incoming message activity keeps archived chats in the archive and only updates their unread badge.

Migration `000021` adds `chat_create_requests`, which stores the authenticated
creator, request UUID, canonical request hash and original create-result snapshot
in the same transaction as a keyed `CreateChat`. Rows currently have no expiry;
the repository has no documented retry-retention window or cleanup policy.

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
rpc UninstallStickerPack(UninstallStickerPackRequest) returns (UninstallStickerPackResponse);
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

**Pagination:** `SearchGifsRequest.cursor` ↔ `SearchGifsResponse.next_cursor` — opaque provider- or Chat-cache token; **only Chat Service** owns GIF search cursors. Empty cursor = first page. The shipped proto uses a separately named `GetTrendingGifsResponse` with the same `items`/`next_cursor` fields.

**Search metadata implementation:** `SearchGifs` and `GetTrendingGifs` authenticate the active profile, enforce default 20 / maximum 50, and delegate to the provider-neutral `gifcatalog.Catalog`. Empty search queries and invalid limits/cursors return `INVALID_ARGUMENT`; provider rate exhaustion maps to `RESOURCE_EXHAUSTED`, other provider failures to sanitized `UNAVAILABLE`. An absent provider returns `UNAVAILABLE`, never fake product results. No live provider or runtime activation switch is configured in this slice; H1/H2 and the release capability check remain open. Deterministic fake results are for tests/local development only; their cursors are bound to search query or trending mode. The optional bounded trending metadata cache requires an explicit positive TTL and page capacity from its activation consumer, copies returned pages, and never caches errors. Search pages are not cached by this wrapper; provider-specific quotas and cache policy remain part of later provider activation.

Search metadata grants no File access or send authority. `file_id` stays empty until a genuine import/binding exists; a nonempty ID may still be pending and must pass authoritative File readiness and Messaging admission. Protected source resolution/import, shared-asset provenance, E2E handling, capability-manifest wiring and selected-asset → ready File → Messaging proof remain separate prerequisites.

### Protected catalog source authority (non-E2E implementation in progress)

The separate `voice.chat.v1.CatalogAssetService` schema in `catalog_assets.proto`
has no ordinary-listener or raw-header fallback. `ResolveGifAsset` accepts only a
verified Gateway delegated active account/profile; `ValidateGifAsset` is a pure
Messaging read of the exact admitted source/profile/File tuple.
`AuthorizeCatalogDestination` is a pure File read: it independently checks current
membership, authoritative chat type and plaintext state, returning the exact
request echo, observed type and current opaque Chat actor generations. File must
match these generations to its original import before fetching/retrying.

The operation-status lookup in protected File precedes any new provider lookup.
It preserves original import identity after a lost Chat response or CDN URL
rotation; compacted operations deny rather than recreate an old File. Chat keeps
only active admitted File projections, with no raw source URL, full request body
or second permanent operation ledger. A source-generation UUID is an identity,
never an access capability. Source URLs exist only transiently on the protected
Chat-to-File request. The complete producer/consumer protocol and its remaining
integration work are in [the catalog asset contract](../testing/be085-catalog-asset-contract.md).

Migration `000028_catalog_assets` adds account/profile purpose-HMAC gates and
active source bindings. Admission consults fresh protected Auth account/session
and User ownership/deletion facts. Save locks account then profile gates, checks
both captured generations and inserts the binding in that same transaction.
Forward migration `000029_catalog_selection_cache` preserves unordered v1 rows
as cleanup-only history and adds a disposable selector-slot incarnation captured
before remote File work. File orders selections by first successful pending
admission, not request/response time or UUID. A new admitted choice supersedes
the prior choice even while pending or subsequently failed. Save updates only
the captured incarnation: greater revision advances, equal requires an identical
tuple, lower denies. Retirement/eviction deletes that incarnation; a delayed
response cannot reinstall into a newly created slot. File retains the current
watermark, so cache deletion never makes an older READY choice current again.

Public Resolve uses envelope1 but translates only to private File protocol2.
Protected source Validate requires protocol2 and a fresh positive File revision
equal to the current head, then rechecks the exact cached slot; a cache miss
fails closed. Destination-only observation remains version1. Missing/zero/v1
ordering proof never grants admission. Only qualified asset-terminal cleanup
may omit both generations and use both zero revisions with exact tuple and
nonzero terminal fence; this exception grants no send/import authority.

Selector metadata is bounded to 16384 slots globally and 256 per profile.
Eviction prefers empty slots, including crashed or in-flight captures; a bounded
reconciliation pass also evicts empty slots. Failed/cancelled requests remove
only their own still-empty incarnation. This is disposable cache maintenance,
not a TTL or proof about File execution. A retry obtains fresh File status.
Purge closes the same gates and removes raw actor/catalog projections; old
operations cannot attach to a restored generation. Gate/event aliases are
restricted pseudonymous anti-resurrection evidence, not anonymous data or public
lookup keys. Historical HMAC keys remain available; an unknown/changed registered
key fails closed, and nonempty rollback refuses to erase the negative state.

User snapshots are not atomic cross-service permits. `UNKNOWN` denies admission
without authorizing erasure. The account consumer closes its local catalog gate
before external User lookup/fanout and does not ACK failed effects. A bounded
reconciler scans at most 100 existing File projections per 30-second pass, with a
15-second pass budget. Only explicit owner `SOFT_DELETED` closes the scanned
profile generation; a stale scan cannot close a newly restored generation.
Protected File status may retire only the exact cache tuple. Compacted status
requires the qualified `terminal_fence_id` proof; `NotFound`, unavailable or a
generic failed precondition never proves erasure. Binding retirement does not
delete committed messages, references or their retained File bytes.

The dedicated listener is disabled when all `CHAT_CATALOG_ASSET_*` configuration
is absent. Any recognized setting requires the complete configuration:
`GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`, issuer endpoints
`GATEWAY_JWKS_URL`, `MESSAGING_JWKS_URL`, `FILE_JWKS_URL`, `JWKS_CA_FILE`,
`JWKS_CLIENT_CERT_FILE`, `JWKS_CLIENT_KEY_FILE`, `REPLAY_REDIS_ADDR` (optional
`REPLAY_REDIS_PASSWORD`), and `ACTOR_KEYS_FILE`. Each protected owner prefix
`CHAT_CATALOG_ASSET_AUTH_`, `CHAT_CATALOG_ASSET_USER_` and
`CHAT_CATALOG_ASSET_FILE_` requires `GRPC_ADDR`, `TLS_CA_FILE`, `TLS_SERVER_NAME`,
`TLS_CLIENT_CERT_FILE` and `TLS_CLIENT_KEY_FILE`. The actor key file is a bounded
JSON object with `active_key_id` and `keys: [{id, secret_base64}]`; each secret is
exactly 32 random bytes in canonical standard Base64. Historical registered keys
cannot be removed or replaced. Startup requires the catalog migration, signing
issuer, existing account-deletion durable and consumer dependencies. Space
destinations require native Space membership and the protected Role
`TEXT_CHAT_VIEW` decision; no fabricated user headers are forwarded.

Runtime composition keeps the production resolver absent. Gateway selection
transport uses the dedicated listener; physical File import and Messaging integration proof remain open at this
source checkpoint. This does not activate a live provider, E2E GIFs or
sticker-source admission.

Gateway REST (sketch): `GET /api/v1/sticker-packs`, `POST /api/v1/sticker-packs/{id}/install`, `GET /api/v1/gifs/search?q=…&cursor=…`, `GET /api/v1/gifs/trending` — full table [api-gateway.md](api-gateway.md) § Stickers and GIF.

**Send wire:** Messaging `SendMessage` + `content_type=STICKER|GIF` — full validation — [messaging-service.md](messaging-service.md) § Stickers and GIF.

**Rules:** system packs are deployment-owned rows (`is_system=true`) and are never seeded with fake product data by the service; a user pack is visible/installable only to its creator; installed state is keyed by the authenticated active profile. **`UninstallStickerPack` rejects `is_system` packs** and non-creator user packs; uninstall does not delete sent messages. Premium ★ checks, user-pack ingest, GIF search/recents, and provider caching remain outside this catalog slice.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`chat.events`** (совместно с Space для событий дерева/спейса; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие               | Данные                             |
|-----------------------|------------------------------------|
| `chat.created`        | chat_id, type, creator_id, members |
| `chat.updated`        | chat_id, changed_fields — поля успешного `UpdateChat` |
| `chat.deleted`        | chat_id, space_id, deletion_operation_id, generation, manifest_id, manifest_sha256 — только Chat participant exact P3 purge после commit |
| `chat.member_changed` | chat_id, profile_id, change (`added` \| `removed` \| `left` \| `owner_transferred` \| `inbox_bucket_changed` \| `role_changed`) |
| `chat.dm_peer_deleted` | chat_id, recipient_profile_id; только surviving участнику, без deleted identity |

`chat.member_changed` is the single membership-change subject; producers do not emit parallel `chat.member_added`, `chat.member_removed`, or `chat.member_left` subjects. Consumers must tolerate unknown `change` values. Caller-relative DM `DeleteChat` is a `left` membership change and is never a permanent `chat.deleted` event. P3 `chat.deleted` envelopes retain a stable event ID, occurrence time, exact Space/operation/generation/manifest binding, and immutable bytes in `chat_db` until JetStream confirms publication; retries publish those same bytes and deduplication ID.

## BE-065 Role override target validation (accepted target)

Add `/voice.chat.v1.ChatService/ValidateRoleOverrideTarget` with package-local
`ValidateRoleOverrideTargetRequest { string space_id = 1; string chat_id = 2; }`
and empty `ValidateRoleOverrideTargetResponse`. The source implementation uses
a dedicated protected listener; runtime acceptance is separate. Configuration
is documented with the complete caller/error/consistency contract in
[Role's BE-065 contract](role-service.md#be-065-protected-override-target-validation-accepted-target);
the [execution plan](../testing/role-override-target-validation-exec-plan.md)
defines required verification.

Only a verified Role workload (`principal_type=service`, `iss=role`,
`sub=service:role`, `aud=chat`, exact full RPC and deterministic whole-body hash)
may call this method. It requires protected TLS/mTLS, Role JWKS and replay
verification, fresh JTI, canonical signed request ID matching the sole
`x-request-id`, and Phase-0 <=30 s lifetime. Reject raw identity metadata,
duplicate credentials and all delegated-user/end-user/bot/other-service callers.
The handler itself requires the verified Role principal even on direct calls.
Do not broaden existing GIS/Search/MatchSquad listeners; ordinary/public
listeners refuse with `UNAVAILABLE` before domain access. No Gateway route exists.

Under one consistent local store/lifecycle serialization boundary, validate
canonical non-nil IDs and an active `chats` row whose type is `group|channel`,
whose `space_id` is non-null and exactly matches, and whose local Space fence
permits ordinary admission. DM, standalone, absent, foreign-Space and deleted
rows all return indistinguishable `NOT_FOUND`; malformed IDs return
`INVALID_ARGUMENT`. Tree placement is not an additional predicate. Known
non-LIVE/ownership/terminal fences and unavailable/corrupt fence reads return
`UNAVAILABLE`; absent legacy fence follows existing ordinary-admission semantics.
No members, owner, name or permission data are returned, and this path never
calls Role or another ACL-enriched provider API.

Each attempt reads current state; success is a point-in-time existence result,
not a reservation or capability. Deletion after this snapshot may leave an inert
Role reference; actual Chat operations must still reject missing/deleted rows.
No UUID reuse/reparenting is permitted; group/channel conversion preserves
eligibility. Restoration requires a fresh read. Existing overrides cannot grant
access across Chat lifecycle fences.

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

An exact authenticated `PurgeSpace` retry first reads Chat's committed response
within that original retention window, before requiring Messaging/File clients.
It returns the saved wrapper including unknown protobuf fields, without repeating
owner calls. Changed request bytes/hash, corrupt response bindings and expired
evidence fail closed; they cannot restart cleanup or extend receipt retention.

The same successful local PURGE transaction writes one immutable
`chat.deleted` envelope to `chat_deleted_event_outbox` for each manifest chat.
Each envelope binds the Space, operation, purge generation and Chat manifest
ID/hash. A deterministic event UUID is both the envelope ID and JetStream
`Nats-Msg-Id`; replay during retained evidence never mints different bytes.
The worker leases rows with a fresh token, publishes outside the transaction,
and records PubAck only if that token is still current. Failures persist a
bounded backoff and continue retrying without dropping the event. Pending rows
are never cleaned up. A confirmed row can be removed only after the parent
PURGE receipt's `retain_until`; the parent receipt/manifest have no cleaner in
this change. Migration DOWN refuses while any outbox evidence remains.

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

## MatchFound temporary Chat ownership

Matchmaking is the sole authority that creates and tears down temporary match
chats. It calls `MatchSquadChatService.CreateMatchSquadChat`,
`TeardownMatchSquadChat`, and `CompactMatchSquadChat` on the dedicated
protected gRPC listener. The service is not registered on Chat's public or
ordinary gRPC listeners. All three methods accept only the verified
`service:matchmaking` principal with audience `chat`,
the exact full RPC, one bearer credential and one `x-request-id` bound to the
operation UUID and deterministic request hash. Raw identity metadata, unknown
protobuf fields, replayed JWT IDs and requests outside the three-method allowlist
are rejected before persistence.

Enable the listener by setting every variable with prefix
`CHAT_MATCH_SQUAD_PRINCIPAL_`: `GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`,
`CLIENT_CA_FILE`, `JWKS_URL`, `JWKS_CA_FILE`, `REPLAY_REDIS_ADDR`, and
`REPLAY_REDIS_PASSWORD`. With all variables absent the capability stays off;
any partial or invalid configuration fails Chat startup. The listener requires
mTLS, a trusted HTTPS JWKS endpoint for issuer `matchmaking`, and reachable
Redis for JTI replay protection.

All protected methods use the same replay guard and key namespace:
`chat:match-squad:principal:replay:<hex(SHA-256(issuer || NUL || JTI))>`. The
operation method and request hash do not partition this key, so a credential
replayed across any two protected methods is rejected.

Creation atomically writes one ordinary group chat, its exact participant
membership and an immutable operation/receipt row. The match ID, chat ID,
receipt ID, sorted participant manifest and request digest are unique durable
ownership bindings. An identical retry returns the stored receipt bytes;
reusing an operation or match with changed request bytes fails. Teardown checks
the exact creation receipt, match, chat, participant-manifest digest and
creation-request digest, then removes all participant memberships and stores
its immutable completion receipt in the same transaction. Chat and message
history rows remain durable, while the empty membership denies further
participant access. Ordinary group APIs cannot change MatchSquad participants,
and a database trigger prevents deleting a Chat protected by its permanent
ownership fence.

Full creation and teardown request/receipt bytes remain until Chat receives a
valid Matchmaking aggregate-compaction command. The command binds the exact
creation and teardown receipts, owner IDs, participant manifest, teardown
aggregate ID, and immutable aggregate completion and authorization timestamps.
It is accepted only when authorization is at least 30 days after aggregate
completion; Chat does not infer eligibility from its local clock or teardown
timestamp. One SQL transaction stores the exact command and stable compaction
receipt, then clears only the full creation/teardown request and receipt bytes.
The compact command/receipt and permanent ownership, operation, resource,
receipt, digest, aggregate, and timestamp fences remain. Exact command retries
return the saved receipt; changed commands conflict. Creation/teardown retries
after compaction return terminal precondition failures instead of synthesizing
old success receipts. SQL guards reject unauthorized evidence clearing,
reopening, or fence deletion, and migration DOWN locks and refuses while any
operation evidence remains.
