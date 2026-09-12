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
  rpc TransferOwnership(TransferRequest) returns (Empty);            // disabled pending production coordinator/recovery, capability activation and public vertical
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
| **RevokeInvite, ListInvites** | ✓ | ✓ | Exact `SPACE_MANAGE_INVITES` permission; owner retains the canonical Role bypass |
| JoinSpace, LeaveSpace | ✓ | ✓ | Composable AND entry policy and invite-safe verifier pipeline remain backlog — [todo/backend.md](../todo/backend.md) |
| KickMember, BanMember, UnbanMember, ListMembers, ListBans | ✓ | ✓ | |
| TimeoutMember, RemoveMemberTimeout | ✓ | ✓ | |
| TransferOwnership | ✓ | Disabled | Production still denies before dependencies. Auth proof consume/receipt lookup, Role v2, and the Space protocol-2 journal through terminal evidence, ordinary freeze and the ready-row scan are shipped foundations. Network recovery/orchestration, capability activation, public operation idempotency and the Gateway/Flutter vertical remain open. See § Ownership lifecycle principal transport. |
| AddBotMember, RemoveBotMember | ✓ | ✓ | |
| ListTemplates, CreateFromTemplate | ✓ | ✗ | |
| GetAuditLog | ✓ | Partial | Current handler has newest-first keyset paging and exact `SPACE_VIEW_AUDIT_LOG`; the additive actor/action/time proto filters are shipped, while filter execution, signed cursor v1 and REST/Flutter surfaces remain backlog |
| AppendAuditEvent | ✓ | ✗ | Protected Role/Chat audit ingestion contract; runtime verifier, idempotent store and producer outboxes remain backlog |
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

`GetAuditLog` читает только строки запрошенного `space_id` и возвращает все поля `AuditLogEntry`. Ошибка Role Service закрывает доступ (`UNAVAILABLE`), явный deny даёт `PERMISSION_DENIED`, malformed cursor — `INVALID_ARGUMENT`. Proto уже содержит additive actor/action/time filters; текущий handler их ещё не исполняет и не выпускает signed cursor v1. Writers и клиентские REST/Flutter поверхности остаются в [backend backlog](../todo/backend.md).

**Invite permissions:** shipped `CreateInvite`, `RevokeInvite` and `ListInvites`
handlers use the exact `SPACE_MANAGE_INVITES` decision. An unavailable configured
Role Service fails closed. While Role is intentionally unwired, the shared
`requireSpacePermission` compatibility path permits only the current Space owner;
the A2 target does not retain this fallback after signed Role integration is
required.

## Phase-0 Space audit ledger (accepted target; runtime not implemented)

Space owns one immutable audit registry for administrative mutations. The wire
`AuditLogEntry.id`, ingestion `audit_event_id` and `audit_log.id` are the same UUID.
`action` remains an exact lower `snake_case` string so additive writers do not
break protobuf or JSON consumers. Unknown action filters are valid and return an
empty result rather than validation failure.

### Closed action and writer registry v1

| Mutation owner | Allowed actions | Canonical target |
|---|---|---|
| Space | `space_created`, `space_updated` | `space`; the same `space_id` |
| Space | `ownership_transferred` | `profile`; the new owner profile |
| Space | `invite_created`, `invite_revoked` | `invite`; the invite ID |
| Space | `member_kicked`, `member_timed_out`, `member_timeout_removed` | `profile`; the affected profile |
| Space | `member_banned`, `member_unbanned` | `account`; the affected account |
| Space | `category_created`, `category_updated`, `category_deleted` | `category`; the category ID |
| Space | `voice_room_created`, `voice_room_updated`, `voice_room_deleted` | `voice_room`; the room ID |
| Space | `tree_node_upserted`, `tree_node_removed` | `tree_node`; the node ID |
| Space | `tree_reordered` | `space`; the same `space_id` |
| Role | `role_created`, `role_updated`, `role_deleted`, `default_join_role_set` | `role`; the role ID |
| Role | `roles_reordered` | `space`; the same `space_id` |
| Role | `role_assigned`, `role_revoked` | `profile`; the affected profile; details bind the role |
| Role | `chat_override_set`, `chat_override_removed` | `chat`; the affected Space-attached chat |
| Role | `voice_room_override_set`, `voice_room_override_removed` | `voice_room`; the affected room |
| Chat | `chat_created`, `chat_updated`, `chat_deleted` | `chat`; the Space-attached chat ID |

Join/leave are ordinary self-service membership events and are outside audit v1.
Space delete/restore/purge facts belong to BE-245. Bot mass cleanup and
`DeleteRolesCreatedByProfile` are not implicit audit actions; adding them needs a
separate product decision. Gateway never authors an audit event.

Space writes its local mutation, audit row and durable audit outbox row in one
`space_db` transaction. Failed/no-op mutations produce no new audit effect. Role
and Chat write the domain mutation plus a durable producer outbox in their own DB
transaction, then deliver the fact through protected
`AppendAuditEvent(AppendAuditEventRequest)`. Composite `POST S/chats` produces one
`chat_created` only after the complete Chat+tree outcome; its internal
`UpsertTreeNode` must not create a duplicate user-visible effect.

`AppendAuditEvent` accepts only signed `service:role` or `service:chat` principals
in the single Bearer metadata credential defined by
[ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). Caller identity is
never a request field. The principal binds the exact full RPC, `x-request-id` and
SHA-256 of deterministic serialization of all request fields; unknown request
fields are rejected before handler execution. Space applies the caller/action
allowlist above. Exact replay of the same `audit_event_id` and canonical payload
returns the empty success response; the same ID with any changed field is
`ALREADY_EXISTS`. Space-local writers never call this RPC, and Role/Chat never
write directly to `space_db`.

Every request field is semantically required: UUID fields and strings are
nonempty, `details_json` is at least `{}`, and `occurred_at` is present and a valid
Timestamp. Space persists `occurred_at` as the entry `created_at`. The request
uses ordinary proto3 strings so empty values fail validation rather than carrying
presence semantics. The empty response accepts and preserves unknown fields after
known validation for forward-compatible acknowledgement handling.

### Target and details registry v1

`target_type` is one of `space`, `invite`, `profile`, `account`, `role`, `chat`,
`voice_room`, `category`, `tree_node`; `target_id` is always a UUID. Aggregate
actions such as `space_updated`, `roles_reordered` and `tree_reordered` target the
Space itself. `details_json` is always a nonempty canonical JSON object string,
including `{}` when no details are needed. Server-owned typed builders enforce
the following allowlist:

| Action family | Allowed detail keys |
|---|---|
| update actions | `changed_fields` |
| `invite_created` | optional `max_uses`, optional `expires_at`; never the invite code |
| ban/timeout | optional normalized `reason`; timeout also has `duration_seconds` |
| `role_assigned`, `role_revoked` | `role_id` |
| chat/voice overrides | `role_id`, `allow_mask`, `deny_mask` |
| `tree_node_upserted` | `kind`, exactly one referenced `chat_id` or `voice_room_id`, optional `category_id`, `sort_order`, `is_pinned`, optional `pin_order` |
| reorder actions | `count`; never the complete UUID list |
| all other actions | `{}` or only the minimal controlled scalar documented for that action |

After JSONB normalization, `octet_length(details::text)` is at most 4096. Generic
request dumps are forbidden. Details never contain JWT/principal/proof/password,
TOTP or backup codes, session/header/IP, email/phone, invite code, URLs, or message
content. A normalized moderation reason is the sole privileged sensitive field
required by the product audit UX and remains protected by `SPACE_VIEW_AUDIT_LOG`.

### Query, cursor and retention v1

`GetAuditLogRequest` filters by optional `actor_profile_id` UUID, optional exact
nonempty `action`, inclusive `from` and exclusive `to`. Present-empty scalar
filters are `INVALID_ARGUMENT`; absent means no filter. Timestamps are valid UTC
instants and, when both are present, require `from < to`. Page size is 1..100,
default 50. Results are ordered `created_at DESC,id DESC`.

The first page fixes an upper `(created_at,id)` snapshot ceiling. Cursor v1 uses
HMAC-SHA-256 and binds `kid`, version, `space_id`, authenticated
`viewer_profile_id`, scope `space.audit.read.v1`, normalized actor/action/from/to,
exact page size, the first-page ceiling, last returned tuple, `issued_at` and
`expires_at`. UUIDs normalize to lowercase, timestamps to UTC RFC3339Nano, and
absent remains distinct from present-empty. Lifetime is 15 minutes from the first
page and ACL is rechecked on every page. Query rows are at or below the ceiling
and strictly after the last key in descending keyset order. Changed filters/page
size, foreign Space/viewer/scope, bad MAC, unknown key or expiry are
`INVALID_ARGUMENT`; later access loss remains the corresponding ACL denial. The
cutover accepts no legacy unsigned cursor; a caller starts again at the first page.

While a Space is live, ordinary audit rows retain 365 days and only the
Space-owned retention worker removes expired rows. Accepted P3 purge is the other
allowed deletion path: ordinary rows are removed after mandatory lifecycle facts
are reduced to the sole minimal tombstone, retained until `purged_at + 365 days`.
Current runtime/storage do not yet meet this target.


The current `TransferOwnershipRequest` only carries `space_id` and
`new_owner_profile_id`; its handler remains an internal backend baseline and is
not an approved public lifecycle path. The implementation change must add an
opaque Auth proof and UUID `operation_id` without accepting caller-supplied actor
identity.

Space receives authenticated owner actor plus exact request bindings. It stores a
durable idempotency record keyed by `(actor_profile_id, operation_id)` and the
canonical non-secret request bindings plus a cryptographic proof digest in the
protocol-2 journal; it never stores the proof-bearing request body. Identical
replay returns the saved outcome; changed body returns `ALREADY_EXISTS`. Before
any owner, Role, audit or event mutation, Space confirms the trusted Auth consume
receipt for actor/account/profile, `space_id`, new owner and `operation_id`, then
invokes Role Prepare. The Auth receipt must exactly match those bindings. Any
consume error, receipt mismatch, timeout or unavailable dependency fails closed
and leaves no transfer mutation.

The only valid Owner-role mutation is the dedicated trusted protocol-2 path. Space
persists one irreversible commit or abort decision: commit follows only a matching
Role Prepare and sends Finalize, while abort sends Role Abort, including as a
durable barrier before an observed Prepare. Successful audit and the ready outbox
event become visible only after the authoritative terminal Role receipt and local
completion. Every Role call remains bound to the same space, previous owner, new
owner and operation ID; it is not a bypass for generic member-role RPCs. Role must
reject direct/public `Owner` assign, revoke or reassignment. Private v1
Apply/Compensate remains test-only and is never a production fallback.

For client-visible errors, use the feature contract: proof/factor/binding failure
is `PERMISSION_DENIED` without an oracle; malformed fields are
`INVALID_ARGUMENT`; current owner/member preconditions are
`FAILED_PRECONDITION`; same idempotency key with different body is
`ALREADY_EXISTS`. Existing dependency failures remain fail-closed. The exact
Auth semantics are in [auth-service.md](auth-service.md#ownership-transfer-step-up-proof)
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

**Rules:** pinned nodes render above unpinned in same `category_id`; `ReorderSpaceTree` respects pin group; audit `tree_node_upserted` includes **`is_pinned`**, **`pin_order`** (R2-A15). ≠ Quick Access (profile rail) ≠ folder pin (Chat inbox).

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
Role is absent. Auth proof issuance/consume/receipt recovery, Role's protected v2
ledger, and Space's protocol-2 reservation, proof confirmation, PREPARED evidence,
irreversible decision, terminal evidence, ordinary freeze and ready-row scan are
present. Network orchestration and restart recovery, outbox publish/claim/ack,
capability activation, the public Space transfer contract and its Gateway/Flutter
vertical remain open. There is no environment setting to activate the private v1
saga; only same-package tests opt into it. Generic Role Owner assignment/revocation
remains forbidden; bootstrap and non-Owner member behavior are unchanged.


The private test-only v1 saga calls dedicated Role `ApplyOwnershipTransfer` and
`CompensateOwnershipTransfer` through a separate TLS client. It issues a fresh
short-lived `service:space` credential for each call, binding the exact method,
request hash and request ID. Forwarded user credentials and raw actor metadata
are not sent on this transport. One internal operation UUID identifies both
legs; compensation retains the original old/new owner ordering, detaches
cancellation and uses a bounded cleanup context. A receipt whose owner does not
match the expected result fails closed.

When Role integration is configured, missing signer or dedicated client prevents
ownership mutation. There is no generic Owner assignment fallback. A public
current+next JWKS is served at `GET /.well-known/jwks.json`; deployment exposes
that route through HTTPS to Role. The protected Role v2 surface and Space store
transitions are not yet connected by a production coordinator, and the public
confirmation flow remains unavailable.
Configuration and activation are specified in
[DEPLOYMENT.md](../DEPLOYMENT.md#ownership-lifecycle-principal-transport).

#### Durable ownership journal and completion (store layer shipped; coordinator open)

The merged Space store persists the exact operation tuple, canonical non-secret
request binding, consumed Auth receipt and decision in `space_db` before owner
mutation. Its protocol-2 journal implements `reserved`, `proof_confirmed`,
`prepared`, `commit_decided` or `abort_decided`, then `completed` or `aborted`.
Commit and abort are mutually exclusive durable decisions. Exact PREPARED and
terminal evidence, atomic terminal audit/outbox visibility, the ordinary Space
freeze and the bounded deterministic `ready=true` outbox scan are shipped. These
store APIs are not a network recovery worker or an activated public transfer.

The still-open coordinator order is: reserve journal and validate/consume proof;
invoke v2 Prepare for Role's frozen ownership operation; atomically persist the
new Space owner, audit/outbox and commit decision; finalize Role; mark the Space
operation completed and allow its audit/event visibility. Any failure before
commit decision selects durable abort and retries v2 Role Abort; failure after
commit decision retries Finalize and never switches to abort. Do not report a
terminal result until both local state and the authoritative Role receipt confirm
it.

Terminal receipt validation canonicalizes the embedded intent and the receipt
wrapper separately. After Prepare, every terminal receipt must retain the exact
stored deterministic PREPARED intent bytes, including protobuf unknown fields and
their ordering; adding, dropping, changing or reordering that intent evidence is
rejected. For Abort before Prepare, Space reconstructs the current-schema
protocol-2 intent from the immutable journal binding, and the receipt must match
those deterministic bytes without unknown intent fields.

On the first otherwise valid terminal receipt, unknown fields on the receipt
wrapper are accepted only after every known protocol, intent, state and current
owner field matches the immutable binding and selected decision. Space persists
the exact deterministic wrapper bytes and their SHA-256 hash separately from the
unchanged PREPARED evidence. Terminal replay must reproduce those bytes exactly,
including wrapper unknown fields; any changed wrapper bytes conflict and cannot
replace the stored terminal evidence.

A pending journal blocks conflicting Space mutations and ownership-sensitive
reads, including owner/role/member/audit and tree access, with `UNAVAILABLE`.
Pending audit/outbox entries remain invisible until completion. Role's matching
prepared state denies new space-scoped ACL decisions rather than transiently
granting Owner to either profile. Reads that cannot determine pending state fail
closed. A process health check does not assert operation convergence.

R20 exposes the outbox only through a bounded, deterministically ordered,
read-only query of `ready=true` rows. It neither publishes, claims, acknowledges
nor deletes rows; repeated reads return the same stable `event_id`. Unready rows
are not dispatchable through this seam. The delivery, claim and acknowledgement
protocol remains a later coordinator/dispatcher contract and is not an R20
activation claim.

The missing network recovery worker must resume exact operations after restart,
including crashes after journal reservation, Auth consume, ambiguous v2 Role
Prepare, local commit decision, Role Finalize or local completion. Once the later
delivery protocol exists, its worker also resumes outbox delivery. No timeout may
silently delete a pending operation or clear its freeze. Audit/event delivery is
idempotent; an abort publishes neither successful transfer audit nor event.
Sustained dependency failure leaves a visible pending/unavailable outcome and
operational alert, not fabricated rollback success. Space remains the owner
authority and uses Role API receipts, never cross-service database access.

Acceptance requires injected response loss/unavailability at every boundary,
restart recovery from each durable state, concurrent same-operation replay and
conflicting bodies, Finalize/Abort races obeying one durable decision,
invisible pending audit/read surfaces, and proof that no permission decision
observes two effective Owners. The shipped durable store layer and Role terminal
barrier do not yet satisfy this complete convergence contract without the network
coordinator, recovery worker and public vertical.


##### Journal exclusivity, freeze inventory and ambiguous consume

Space durably enforces one active ownership journal per space with a database
unique constraint, not only an in-process/advisory lease. The immutable global
operation record binds actor/account/original session epoch, space, old/new
owner, protocol version and canonical non-secret body/proof digest. A different
operation while one is active is `FAILED_PRECONDITION`; the same operation with
a changed body is `ALREADY_EXISTS`. Reservation precedes Auth consume. The only
allowed decisions are unset -> commit or unset -> abort, through serialized
compare-and-set; neither terminal decision can switch sides. The future network
worker must re-read the committed decision before sending its action, so stale
workers cannot issue contradictory Finalize and Abort. Abort completion requires
an authoritative Role aborted receipt even when Prepare was not observed, fencing
delayed attempts.

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

Auth's identical Consume replay requires the original opaque proof
(matched by its digest) and exact original binding, including original session epoch, and returns its durable
receipt even after proof expiry or later account security/epoch changes; it does
not create a second grant. Recovery without retaining plaintext proof therefore
uses the shipped trusted Space-only
`GetOwnershipTransferReceipt` lookup with exact `account_id`, `profile_id`
(the old-owner actor), `space_id`, `new_owner_profile_id`, `operation_id` and
original `session_epoch`, plus required `proof_digest` (the SHA-256 hex digest
of the original opaque proof). This safely persisted digest must match Auth
storage, preserving exact-proof replay without retaining plaintext proof.
It returns only a previously committed matching receipt; missing, unconsumed or
mismatched results all return coarse `PERMISSION_DENIED` and never authorize
consumption or ownership. Consumed receipts outlive ordinary proof TTL cleanup.
The production Space recovery worker that calls this lookup remains open.

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
## A2 public lifecycle and retry contract (target)

The canonical HTTP table, JSON projections, disclosure and pagination are in
[API Gateway](api-gateway.md#a2-space-rest-contract-target). It preserves existing
Space/invite/tree paths and freezes target transfer/delete/restore/audit additions.
No target status in that table means the current service implements it.

Space owns the durable mutation record `(actor_profile_id,operation_id)`, a
canonical digest binding method/resource/body, and resumable outcome for 30 days.
The Auth proof contributes only a digest; Space never persists its plaintext.
Completed replay authenticates the original actor and compares the saved binding
before applying current-owner/member preconditions: a successful transfer/leave
can therefore be retried after the actor has ceased to be owner/member. A new
operation still checks current ownership/membership before any effect. Unfinished
operations retain the same IDs through Auth consume, Role Prepare, the durable
commit/abort decision, matching Finalize/Abort and terminal audit/outbox
visibility; dependency failure cannot be converted into a success receipt.

### Deletion confirmation proof

This is an autonomous A2 contract decision, **target only**: implement after the
ownership transfer proof, as a distinct purpose. Password and second-factor checks
remain owned by Auth; Space must never receive a password or TOTP/backup code.
The separate names prevent a transfer proof from authorizing deletion.

- `POST /api/v1/auth/space-deletion-proof` (JWT) maps to future Auth
  `IssueSpaceDeletionProof`. JSON is `{space_id,confirmation_name,operation_id,
  password,totp_code?,backup_code?}`; success is `200 {proof,expires_at}`. At most
  one second-factor field is present; password is always checked and enabled 2FA
  requires TOTP or one unused backup code. Auth does not claim the name is current:
  Space makes that authoritative comparison when consuming the intent.
- Proof purpose is exactly `space_delete`, bound to verified `account_id`, active
  `profile_id`, positive `session_epoch`, `space_id`, UUID `operation_id`, exact
  UTF-8 `confirmation_name` and verified-factor set. It is opaque/high entropy,
  returned once, hash-only in Auth storage, expires in five minutes and revokes
  on epoch/password/2FA/security changes exactly as the transfer proof does.
- Only signed `service:space` may invoke future `ConsumeSpaceDeletionProof` with
  `{account_id,profile_id,session_epoch,space_id,confirmation_name,operation_id,proof}`.
  Auth atomically consumes once and stores an immutable receipt with `receipt_id`,
  exact bound fields, `verified_factors` and `consumed_at`; identical retry returns
  it and any changed binding/purpose/operation fails closed. No public consume route.
- Before consume, Space checks current owner, exact current name (case-sensitive,
  no trimming or Unicode normalization) and its operation record. It verifies every
  receipt binding before scheduling deletion. Wrong name/factor/unusable proof is
  `PERMISSION_DENIED`, invalid field format is `INVALID_ARGUMENT`; Auth outage or
  mismatched receipt leaves Space unchanged. Neither logs nor audit details contain
  name confirmation, password, factor or proof plaintext.
- Schedule commits `deletion_scheduled_at`, `purge_after = scheduled_at + 7 days`
  and audit/outbox atomically; freezes normal reads/writes/join/invite/MM and revokes
  active media access. Owner `RestoreSpace` before that deadline restores previous
  resources; it needs an authenticated current owner and operation ID, not a new
  factor proof. Restore and expiry/purge share serialization. Purge at/after the
  deadline remains retryable cross-service cleanup with attachment reference-aware
  GC and minimal audit tombstone; no delete HTTP response asserts completed purge.

Auth deletion issue/consume proto, Java storage/factors and trusted-principal
support are a separate later A2 implementation dependency. This decision does
not expand the ownership-proof implementation or reuse its tokens/receipts.

## P3 Space lifecycle coordinator (accepted target; not implemented)

Space owns the aggregate and durable participant ledger. States are `LIVE`,
`SCHEDULE_PENDING`, `FREEZE_PENDING`, `SCHEDULED`, `RESTORE_DECIDED`,
`PURGE_DECIDED`, `PURGING`, `PURGED`; generations are positive and monotonic.
The release manifest fixes ten participants: Role, Chat, Messaging, File, Voice,
Matchmaking, Search, Subscription, Bot and Notification. Service absence never
removes a participant: an empty result still has an immutable receipt.

Before Auth consume Space commits the immutable operation binding and
`SCHEDULE_PENDING`. After an exact receipt it enters `FREEZE_PENDING`, where
Chat first fences and captures the immutable sorted chat manifest; Messaging
imports it; File preliminarily fences and seals exact `SPACE`, `CHAT` and
`MESSAGING` reference declarations/pages; then all ten participants acknowledge
the same root manifest. Only after the complete barrier does fresh PostgreSQL
time under the Space lock set `SCHEDULED`, `scheduled_at` and
`purge_after = scheduled_at + interval '7 days'`. Subphases and exact manifest
bytes survive restart; no work set is recreated for the same generation.

Restore samples database time after acquiring the same lock. Strictly before
`purge_after`, it commits `RESTORE_DECIDED` at the next generation and returns
to `LIVE` only after all participant `LIVE` receipts. At equality or later it
commits irreversible `PURGE_DECIDED`, sends that fence to all participants,
then retries destructive work. Role retirement is first. Messaging deletes the
saved Chat domain and obtains File release acceptance; Chat deletes its rows
only afterward. Other participants may converge in parallel. Local `PURGED`,
the tombstone and ready `space.deleted` outbox row commit only after all ten
validated completion receipts. File receipt proves durable reference/GC
handoff, not physical R2 absence.

Each attempt uses exact stored bytes, a fresh 10-second RPC deadline and
unbounded exponential retry from one second to a five-minute cap with bounded
jitter. Fifteen minutes without progress alerts but never skips a participant.
The ledger uses `NOT_STARTED`, `IN_FLIGHT`, `COMPLETE`, `RETRYABLE_FAILURE`,
`CONTRACT_MISMATCH`; only an accepted receipt may set `COMPLETE`.

The common protected wire uses `protocol_version=1`. A lifecycle-fence request
contains canonical `space_id`, `deletion_operation_id`, positive `generation`,
desired `FROZEN`/`LIVE`/`PURGE_DECIDED` and `ManifestBinding(manifest_id,
manifest_sha256,item_count)`. Its receipt adds stable `receipt_id`, fixed
`participant_id`, applied state, request/manifest hashes and database
`applied_at`. A purge request replaces desired state with `purge_decided_at`;
its receipt accepts only `COMPLETED` and records `completed_at`. Participant IDs
are Role=1, Chat=2, Messaging=3, File=4, Voice=5, Matchmaking=6, Search=7,
Subscription=8, Bot=9, Notification=10; zero/unknown IDs or states are invalid.
Role returns the stricter retirement receipt. Exact replay returns stored bytes;
changed bytes for the same operation/generation conflict, timeouts never
manufacture a receipt, and `CONTRACT_MISMATCH` is never auto-skipped.

Completed schedule/restore replay lasts 30 days from operation `completed_at`;
nonterminal operations last until terminal. Coordinator-held full participant
request/receipt bytes last 30 days after the aggregate reaches `PURGED`; compact
completion tuples last through tombstone expiry. The no-FK tombstone contains only
Space ID, purpose-specific account HMACs, key version, `OWNER_REQUESTED`,
lifecycle timestamps and `retain_until = purged_at + 365 days`. Ordinary audit
is purged; no legal-hold field exists in P3 and public audit never exposes it.

Space tombstone HMAC input is ASCII `voice-space-tombstone-v1`, NUL, then raw
16-byte UUID. Only Space workload identity may compute it; staff and break-glass
have no access. Its distinct KMS/HSM family rotates every `P90D`, fails closed,
is audited, and uses the shared `P30D` maximum restorable-backup window. Old
versions are destroyed only after no retained row or restorable backup needs
them. The owner-approved U5-A decision explicitly accepts this purpose-specific
pseudonymous Space tombstone HMAC as compatible with account erasure.
