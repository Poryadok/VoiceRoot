# Space Service

## Обзор

Управление пространствами (аналог Discord-серверов): **дерево sidebar** — единая таблица **`space_tree_nodes`** (текстовые чаты `group`/`channel` и **голосовые комнаты** в одном порядке сортировки), категории, инвайты, участники, шаблоны.

**Язык**: Go
**БД**: PostgreSQL `space_db`

Пул advisory lease должен подключаться напрямую к PostgreSQL или через session pooling; transaction-mode PgBouncer не поддерживается, потому что lock привязан к сессии.

## Ответственность

- CRUD пространств
- Видимость: public / invite-only / private
- Категории; **голосовые комнаты** (`voice_rooms` — сущность); **дерево** — `space_tree_nodes` (`kind`: текстовый чат → `chat_id` из Chat, или голос → `voice_room_id`)
- Системный канал (welcome, rules)
- Инвайт-ссылки (expiry, usage limits)
- Проверка при входе (phone / CAPTCHA / вопросы / ручное одобрение)
- Участники (join, leave, ban, kick)
- Лимиты: **50 узлов дерева** (текст + голос в сумме) free / 500 Pro; 50 участников free / 5000 Pro
- Каталог публичных пространств (поиск, ранжирование)
- Space-level matchmaking конфигурация
- Шаблоны пространств
- Аудит-лог действий (узлы дерева, голосовые комнаты, баны, изменение ролей)
- Передача владения
- Slow mode на уровне текстового чата (`group` \| `channel`) — в данных **Chat** (`chats.slow_mode_seconds`); Space может дублировать отображение/кэш при необходимости
- Бан пользователя (с сохранением сообщений)

## API (gRPC)

```protobuf
service SpaceService {
  // Пространства
  rpc CreateSpace(CreateSpaceRequest) returns (Space);
  rpc UpdateSpace(UpdateSpaceRequest) returns (Space);
  rpc UpdateSpaceMmConfig(UpdateSpaceMmConfigRequest) returns (Space); // ✓ shipped
  rpc DeleteSpace(DeleteSpaceRequest) returns (Empty);               // current hard-delete handler; must become schedule-delete
  rpc RestoreSpace(RestoreSpaceRequest) returns (Space);             // target: owner restore within 7 days
  rpc GetSpace(GetSpaceRequest) returns (Space);
  rpc ListMySpaces(ListMySpacesRequest) returns (SpaceList);
  rpc SearchPublicSpaces(SearchRequest) returns (SpaceList);         // ✗ unimplemented

  // Голосовые комнаты (сущность) + дерево sidebar (текст и голос в одном слое)
  rpc CreateVoiceRoom(CreateVoiceRoomRequest) returns (VoiceRoom);
  rpc UpdateVoiceRoom(UpdateVoiceRoomRequest) returns (VoiceRoom);
  rpc DeleteVoiceRoom(DeleteVoiceRoomRequest) returns (Empty); // каскад на узел в space_tree_nodes
  rpc UpsertTreeNode(UpsertTreeNodeRequest) returns (SpaceTreeNode); // text_chat (chat_id) или voice_room (voice_room_id)
  rpc RemoveTreeNode(RemoveTreeNodeRequest) returns (Empty);
  rpc ListSpaceTree(ListSpaceTreeRequest) returns (SpaceTreeList);   // ✓ shipped
  rpc CreateCategory(CreateCategoryRequest) returns (Category);
  rpc UpdateCategory(UpdateCategoryRequest) returns (Category);
  rpc DeleteCategory(DeleteCategoryRequest) returns (Empty);
  rpc ReorderSpaceTree(ReorderRequest) returns (Empty); // только space_tree_nodes: порядок и категории для текста и голоса
  rpc PinTreeNode(PinTreeNodeRequest) returns (SpaceTreeNode);   // ✓ shipped
  rpc UnpinTreeNode(UnpinTreeNodeRequest) returns (SpaceTreeNode); // ✓ shipped

  // Инвайты
  rpc CreateInvite(CreateInviteRequest) returns (Invite);
  rpc RevokeInvite(RevokeInviteRequest) returns (Empty);
  rpc GetInvite(GetInviteRequest) returns (Invite);
  rpc ListInvites(ListInvitesRequest) returns (InviteList);
  rpc JoinByInvite(JoinByInviteRequest) returns (SpaceMembership);

  // Участники
  rpc JoinSpace(JoinSpaceRequest) returns (SpaceMembership);
  rpc LeaveSpace(LeaveSpaceRequest) returns (Empty);
  rpc KickMember(KickMemberRequest) returns (Empty);
  rpc BanMember(BanMemberRequest) returns (Empty);
  rpc UnbanMember(UnbanMemberRequest) returns (Empty);
  rpc ListMembers(ListMembersRequest) returns (MemberList);
  rpc ListBans(ListBansRequest) returns (BanList);
  rpc TimeoutMember(TimeoutMemberRequest) returns (Empty);           // ✓ shipped
  rpc RemoveMemberTimeout(RemoveMemberTimeoutRequest) returns (Empty); // ✓ shipped
  rpc TransferOwnership(TransferRequest) returns (Empty);            // disabled in production pending durable v2 + Auth proof
  rpc AddBotMember(AddBotMemberRequest) returns (SpaceMembership);   // ✓ shipped
  rpc RemoveBotMember(RemoveBotMemberRequest) returns (Empty);       // ✓ shipped

  // Шаблоны
  rpc ListTemplates(Empty) returns (TemplateList);                   // ✗ unimplemented
  rpc CreateFromTemplate(CreateFromTemplateRequest) returns (Space); // ✗ unimplemented

  // Аудит
  rpc GetAuditLog(GetAuditLogRequest) returns (AuditLogList);        // ✓ backend gRPC

  // S2S / internal
  rpc AreCoMembers(AreCoMembersRequest) returns (AreCoMembersResponse); // ✓ shipped
  rpc SyncSpaceProSubscription(SyncSpaceProSubscriptionRequest) returns (Empty); // ✓ shipped
}
```

### Implementation status (proto vs handlers)

Источник истины: [protos/voice/space/v1/space.proto](../../protos/voice/space/v1/space.proto). Сводка по `src/backend/space/internal/grpcsvc/`:

| RPC | Proto | Handler | Notes |
|-----|-------|---------|-------|
| CreateSpace, UpdateSpace, GetSpace, ListMySpaces | ✓ | ✓ | |
| UpdateSpaceMmConfig | ✓ | ✓ | MM config on space |
| DeleteSpace | ✓ | ✓ | Current owner-only hard delete is obsolete; target schedules 7-day hidden/frozen recovery window |
| RestoreSpace | ✗ | ✗ | Target owner-only restore during recovery window |
| SearchPublicSpaces | ✓ | ✗ | Catalog search backlog |
| Create/Update/Delete VoiceRoom | ✓ | ✓ | |
| UpsertTreeNode, RemoveTreeNode, ReorderSpaceTree | ✓ | ✓ | Tree pin fields shipped in migration `000007_tree_pin` |
| **ListSpaceTree** | ✓ | ✓ | **Omitted from earlier doc inventory** |
| Create/Update/Delete Category | ✓ | ✓ | |
| PinTreeNode, UnpinTreeNode | ✓ | ✓ | Migration `000007_tree_pin`; handlers and event payload shipped |
| CreateInvite, GetInvite, JoinByInvite | ✓ | ✓ | |
| **RevokeInvite, ListInvites** | ✓ | ✓ | **Owner-only** today (`requireSpaceOwner`) — normative target: role with `MANAGE_INVITES` |
| JoinSpace, LeaveSpace | ✓ | ✓ | Composable AND entry policy and invite-safe verifier pipeline remain backlog — [todo/backend.md](../todo/backend.md) |
| KickMember, BanMember, UnbanMember, ListMembers, ListBans | ✓ | ✓ | |
| TimeoutMember, RemoveMemberTimeout | ✓ | ✓ | |
| TransferOwnership | ✓ | Disabled | Production denies before dependencies. The signed, compensated v1 foundation is test-only; durable v2 journal/freeze/finalization, Auth proof and public operation idempotency must land before activation through Gateway/Flutter. See § Ownership lifecycle principal transport. |
| AddBotMember, RemoveBotMember | ✓ | ✓ | |
| ListTemplates, CreateFromTemplate | ✓ | ✗ | |
| GetAuditLog | ✓ | ✓ | `created_at DESC, id DESC`; opaque timestamp+UUID keyset cursor; default 50/max 100; exact `SPACE_VIEW_AUDIT_LOG` check, owner-only fallback only when Role Service is unwired. Filters and REST/Flutter surfaces remain backlog |
| AreCoMembers | ✓ | ✓ | S2S co-membership check |
| SyncSpaceProSubscription | ✓ | ✓ | Subscription sync |

### Phase-0 S2S decision callers (target)

`AreCoMembers`, subscription sync и новые internal decision RPC не получают actor
из metadata. Для `RoleService.CheckPermission` Space предъявляет только signed
`service:space` principal в единственном Bearer header: exact RPC, `space_id`,
decision `profile_id`, permission и отсутствие необязательных node fields входят
в deterministic protobuf request hash. `profile_id` остаётся decision data,
подтверждённым самим Space, но не становится identity authority. Разрешённый
permission subset и Owner lifecycle paths определены в
[role-service.md](role-service.md#space-checkpermission-decision-contract): generic
`AssignRole`/`RevokeRole` не меняют Owner; это могут сделать только
`ApplyOwnershipTransfer`/`CompensateOwnershipTransfer` с проверенным
`operation_id`.

`GetAuditLog` читает только строки запрошенного `space_id` и возвращает все поля `AuditLogEntry`. Ошибка Role Service закрывает доступ (`UNAVAILABLE`), явный deny даёт `PERMISSION_DENIED`, malformed cursor — `INVALID_ARGUMENT`. Наличие RPC не означает полноту аудита: writers для части действий, фильтры по actor/action и клиентские REST/Flutter поверхности остаются в [backend backlog](../todo/backend.md).

**Invite permissions (code vs spec):** shipped handlers gate `RevokeInvite` / `ListInvites` on **space owner** only. Product spec allows admins with invite-management permission — align handlers when Role Service integration lands; until then document owner-only as **partial shipment**.

## Phase-0 Space audit ledger (target; not implemented)

Space owns an immutable audit registry for every administrative mutation, including
invite lifecycle, membership/moderation, role and owner transfer, tree/room changes
and settings. The mutation transaction writes an outbox row with the same
`audit_event_id`; the publisher delivers exactly once to the logical audit effect
(idempotent consumers dedupe by that ID). A failed mutation produces no audit row.
Records retain 365 days; `details` is canonical, redacted and capped at 4 KiB.
Deletion/purge never cascades audit rows: it leaves the required minimal tombstone.

`GetAuditLog` adds action/actor/time filters and uses an HMAC-signed cursor bound to
`space_id` and the full filter set. A changed filter/cursor, invalid signature or
expired cursor is `INVALID_ARGUMENT`; no cursor may be replayed against another
Space or disclosure scope. Existing current RPC/storage do not yet meet this target.


The current `TransferOwnershipRequest` only carries `space_id` and
`new_owner_profile_id`; its handler remains an internal backend baseline and is
not an approved public lifecycle path. The implementation change must add an
opaque Auth proof and UUID `operation_id` without accepting caller-supplied actor
identity.

Space receives authenticated owner actor plus exact request bindings. It stores a
durable idempotency record keyed by `(actor_profile_id, operation_id)` and the
exact request body before performing the existing compensated transfer. Identical
replay returns the saved outcome; changed body returns `ALREADY_EXISTS`. Before
any owner, Role, audit or event mutation, Space calls the trusted Auth consume
operation with actor/account/profile, `space_id`, new owner and `operation_id`.
The Auth receipt must exactly match those bindings. Any consume error, receipt
mismatch, timeout or unavailable dependency fails closed and leaves no transfer
mutation.

The only valid Owner-role mutation is the dedicated trusted transfer path. It
retains Space-owned compensation, serialization and post-success audit/event
behavior; Role must reject direct/public `Owner` assign, revoke or reassignment.
The future authenticated Space→Role transfer operation must be bound to the same
space, previous owner, new owner and operation ID; it is not a bypass for generic
member-role RPCs.

For client-visible errors, use the feature contract: proof/factor/binding failure
is `PERMISSION_DENIED` without an oracle; malformed fields are
`INVALID_ARGUMENT`; current owner/member preconditions are
`FAILED_PRECONDITION`; same idempotency key with different body is
`ALREADY_EXISTS`. Existing dependency failures remain fail-closed. The exact
Auth semantics are in [auth-service.md](auth-service.md#ownership-transfer-step-up-proof-target-contract-not-implemented)
and product contract in [spaces.md](../features/spaces.md#контракт-подтверждения-передачи-владения).

## Ownership-transfer contract (target; not implemented)

## Модель данных

```
spaces
├── id (UUID)
├── name
├── description (text)
├── icon_url
├── banner_url
├── visibility (public | invite_only | private)
├── owner_profile_id
├── member_count (denormalized counter)
├── is_verified (bool)
├── verification_type (none | personal | organization)
├── allow_guests (bool, default false)
├── entry_policy_version (int)
├── entry_policy (jsonb: enabled phone/captcha/questions/manual + versioned config)
├── mm_config (jsonb — space-level matchmaking settings)
├── deletion_scheduled_at (nullable)
├── purge_after (nullable; deletion_scheduled_at + 7 days)
├── created_at
└── updated_at

voice_rooms
├── id (UUID)
├── space_id (FK)
├── name
├── created_at
└── updated_at

space_tree_nodes
├── id (UUID)
├── space_id (FK)
├── category_id (FK, nullable)
├── kind (text_chat | voice_room)
├── chat_id (nullable — Chat, group|channel)
├── voice_room_id (nullable — FK → voice_rooms)
├── sort_order (int)
├── is_pinned (bool, default false) — pinned nodes sort above unpinned within same category
├── pin_order (int, nullable) — ordering among pinned nodes
├── is_system (bool — только text_chat)
├── created_at
└── updated_at

categories
├── id (UUID)
├── space_id (FK)
├── name
├── sort_order (int)
└── created_at

space_members
├── space_id (FK)
├── profile_id (FK)
├── joined_at
├── nickname (nullable, space-specific)
└── UNIQUE(space_id, profile_id)

space_bans
├── space_id (FK)
├── account_id (FK)
├── banned_by (profile_id)
├── reason (text, nullable)
├── banned_at
└── UNIQUE(space_id, account_id)

invites
├── id (UUID)
├── space_id (FK)
├── code (string, unique)
├── creator_profile_id
├── max_uses (nullable)
├── use_count (int)
├── expires_at (nullable)
├── created_at
└── revoked_at (nullable)

audit_log
├── id (UUID)
├── space_id (FK)
├── actor_profile_id
├── action (string — voice_room_created, tree_node_upserted, tree_node_removed, member_banned, role_updated, ...)
├── target_type (string)
├── target_id (UUID)
├── details (jsonb)
└── created_at
```

### Pin tree node

Закреп узла sidebar — UX [spaces.md](../features/spaces.md) § Pin элемента дерева. Поля и handlers shipped в migration `000007_tree_pin`.

```protobuf
message PinTreeNodeRequest {
  string space_id = 1;
  string node_id = 2;
}
message UnpinTreeNodeRequest {
  string space_id = 1;
  string node_id = 2;
}
```

**Rules:** pinned nodes render above unpinned in same `category_id`; `ReorderSpaceTree` respects pin group; audit `space.tree_node_upserted` includes **`is_pinned`**, **`pin_order`** (R2-A15). ≠ Quick Access (profile rail) ≠ folder pin (Chat inbox).

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`chat.events`** (совместно с Chat; события спейса и дерева — те же потребители; матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                          |
|-------------------------|---------------------------------|
| `space.created`         | space_id, owner_id              |
| `space.updated`         | space_id, changed_fields        |
| `space.deletion_scheduled` | space_id, owner_id, purge_after |
| `space.restored`        | space_id, owner_id              |
| `space.deleted`         | space_id (only after converged irreversible purge) |
| `space.member_joined`   | space_id, profile_id            |
| `space.member_left`     | space_id, profile_id            |
| `space.member_banned`   | space_id, account_id, banned_by |
| `space.voice_room_created`   | space_id, voice_room_id         |
| `space.voice_room_deleted`   | space_id, voice_room_id         |
| `space.tree_node_upserted`   | space_id, node_id, kind, chat_id?, voice_room_id?, **is_pinned**, **pin_order** |
| `space.tree_node_removed`    | space_id, node_id               |
| `space.invite_created`  | space_id, invite_code           |

## Зависимости

- **Chat Service** — создание/удаление строки текстового чата (`chats`, `group` \| `channel`); Space ведёт **`space_tree_nodes`** (`kind = text_chat`)
- **Role Service** — проверка прав при операциях (в т.ч. `chat_overrides` по `chat_id` и `voice_room_overrides`)
- **Subscription Service** — лимиты узлов дерева (текст + голос) и участников (free vs Pro)
- **User Service** — профили участников
- **Social Service** — проверка блокировок при join

### Ownership lifecycle principal transport

**Staged foundation:** production Space TransferOwnership is disabled before any
lock, database or Role access, including when all signing/TLS settings exist or
Role is absent. There is no environment setting to activate the v1 saga; only
same-package test fixtures opt into its private test switch. Generic Role Owner
assignment/revocation remains forbidden; bootstrap and non-Owner member behavior
is unchanged. V2 durable convergence and Auth proof consumption must be enabled
atomically before production transfer is exposed.


Space calls dedicated Role `ApplyOwnershipTransfer` and
`CompensateOwnershipTransfer` through a separate TLS client. It issues a fresh
short-lived `service:space` credential for each call, binding the exact method,
request hash and request ID. Forwarded user credentials and raw actor metadata
are not sent on this transport. One internal operation UUID identifies both
legs of the current saga; compensation retains the original old/new owner
ordering, detaches cancellation and uses a bounded cleanup context. A receipt
whose owner does not match the expected result fails closed.

When Role integration is configured, missing signer or dedicated client prevents
ownership mutation. There is no generic Owner assignment fallback. A public
current+next JWKS is served at `GET /.well-known/jwks.json`; deployment exposes
that route through HTTPS to Role. This slice does not yet implement client
operation idempotency, Auth proof consumption or the public confirmation flow.
Configuration and activation are specified in
[DEPLOYMENT.md](../DEPLOYMENT.md#ownership-lifecycle-principal-transport).

#### Target durable ownership journal and recovery (not implemented)

A follow-on slice persists the exact operation tuple, canonical non-secret
request binding, consumed Auth receipt and decision in `space_db` before owner
mutation. Under the space mutation lease it advances a journal through prepared,
commit-decided or abort-decided, then completed or aborted. Commit and abort are
mutually exclusive durable decisions; clients retry the same operation body to
observe its outcome, while a changed body returns `ALREADY_EXISTS`.

The target order is: reserve journal and validate/consume proof; invoke v2 Prepare for Role's
frozen ownership operation; atomically persist the new Space owner, audit/outbox
and commit decision; finalize Role; mark the Space operation completed and allow
its audit/event visibility. Any failure before commit decision selects durable
abort and retries v2 Role Abort; failure after commit decision retries
Finalize and never switches to abort. Do not report a terminal result until both
local state and the authoritative Role receipt confirm it.

A pending journal blocks conflicting Space mutations and ownership-sensitive
reads, including owner/role/member/audit and tree access, with `UNAVAILABLE`.
Pending audit/outbox entries remain invisible until completion. Role's matching
prepared state denies new space-scoped ACL decisions rather than transiently
granting Owner to either profile. Reads that cannot determine pending state fail
closed. A process health check does not assert operation convergence.

A recovery worker resumes exact operations after restart, including crashes after
journal reservation, Auth consume, ambiguous v2 Role Prepare, local commit decision,
Role Finalize, local completion or outbox delivery. No timeout silently deletes a
pending operation or clears its freeze. Audit/event delivery is idempotent; an
abort publishes neither successful transfer audit nor event. Sustained dependency
failure leaves a visible pending/unavailable outcome and operational alert, not
fabricated rollback success. Space remains the owner authority and uses Role API
receipts, never cross-service database access.

Acceptance requires injected response loss/unavailability at every boundary,
restart recovery from each durable state, concurrent same-operation replay and
conflicting bodies, Finalize/Abort races obeying one durable decision,
invisible pending audit/read surfaces, and proof that no permission decision
observes two effective Owners. The current compensated saga and terminal Role
abort barrier do not yet satisfy this complete convergence contract.


##### Journal exclusivity, freeze inventory and ambiguous consume

Space durably enforces one active ownership journal per space with a database
unique constraint, not only an in-process/advisory lease. The immutable global
operation record binds actor/account/original session epoch, space, old/new
owner, protocol version and canonical non-secret body/proof digest. A different
operation while one is active is `FAILED_PRECONDITION`; the same operation with
a changed body is `ALREADY_EXISTS`. Reservation precedes Auth consume. The only
allowed decisions are unset -> commit or unset -> abort, through serialized
compare-and-set; neither terminal decision can switch sides. A worker re-reads
the committed decision before sending its action, so stale workers cannot issue
contradictory Finalize and Abort. Abort completion requires an authoritative Role
aborted receipt even when Prepare was not observed, fencing delayed attempts.

Space freeze applies to GetSpace/owner enrichment, ListMySpaces/search/templates
that include the space, members/bans/audit/tree/category/node reads, invite reads
and redemption, join/leave/bot/subscription/member lifecycle and every space,
chat-node or voice-room mutation. A request or batch involving a pending space
fails atomically with `UNAVAILABLE`; no partial authority-bearing result is
returned. Indirect owner lookups and all owner/admin fallback paths consult the
journal under the same store serialization boundary as the protected action.
Permission/owner caches cannot turn unavailable state into an allow. Ordinary
absence of an active journal permits normal behavior; lookup errors deny. The
operation status/identical-request outcome may remain readable only to its
verified initiating actor without disclosing pending member or audit data.

Auth's current identical Consume replay requires the original opaque proof
(matched by its digest) and exact original binding, including original session epoch, and returns its durable
receipt even after proof expiry or later account security/epoch changes; it does
not create a second grant. Recovery without retaining plaintext proof therefore
requires a separate target trusted Space-only
`GetOwnershipTransferReceipt` lookup with exact `account_id`, `profile_id`
(the old-owner actor), `space_id`, `new_owner_profile_id`, `operation_id` and
original `session_epoch`, plus required `proof_digest` (the SHA-256 hex digest
of the original opaque proof). This safely persisted digest must match Auth
storage, preserving exact-proof replay without retaining plaintext proof.
It returns only a previously committed matching receipt; missing, unconsumed or
mismatched results all return coarse `PERMISSION_DENIED` and never authorize
consumption or ownership. Consumed receipts outlive ordinary proof TTL cleanup.
This additive Auth contract is a dependency of v2, not a change to the current
Consume implementation.

After ambiguous consume, recover that exact receipt and persist it before
Prepare or a commit decision. Never consume a different proof to resolve the
same operation and never infer success from timeout. If no receipt can be
confirmed, recovery may choose durable abort (ownership stays unchanged); an
already abort-decided operation cannot revive when a late consume succeeds.
A later new ownership intent requires a new operation and proof. The journal
stores proof digest and receipt only, never opaque plaintext proof. Terminal
journal tombstones follow Role's durable lifetime/retired-space retention rule.
A missing journal after a previously issued operation is an operational failure,
not permission to create a second transfer.
