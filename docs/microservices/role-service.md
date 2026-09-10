# Role Service

## Обзор

Управление ролями и правами в пространствах. Иерархия ролей, гранулярные права, оверрайды по текстовым чатам (`group` / `channel`) и голосовым комнатам.

**Язык**: Go
**БД**: PostgreSQL `role_db`

## Ответственность

- Предустановленные роли: Owner, Admin, Moderator, Member, Guest
- Кастомные роли
- Иерархия ролей (позиция определяет приоритет)
- Права как **набор именованных флагов** в `bigint` bitmask (`roles.permissions`); идентификаторы — `SCREAMING_SNAKE_CASE`, без имён из сторонних продуктов (см. раздел ниже)
- Оверрайды прав для узла спейса: отдельно для текстового чата (`chat_id`) и голосовой комнаты (`voice_room_id`)
- Назначение ролей участникам пространства
- Верификационные роли (автоматические по статусу верификации)
- Voice chat organizer роль
- Проверка прав (вычисление effective permissions)

## API (gRPC)

```protobuf
service RoleService {
  // Роли
  rpc CreateRole(CreateRoleRequest) returns (Role);
  rpc UpdateRole(UpdateRoleRequest) returns (Role);
  rpc DeleteRole(DeleteRoleRequest) returns (Empty);
  rpc ListRoles(ListRolesRequest) returns (RoleList);
  rpc ReorderRoles(ReorderRolesRequest) returns (Empty);

  // Назначение
  rpc AssignRole(AssignRoleRequest) returns (Empty);
  rpc RevokeRole(RevokeRoleRequest) returns (Empty);
  rpc GetMemberRoles(GetMemberRolesRequest) returns (RoleList);

  // Оверрайды прав узла спейса (текстовый чат = group | channel)
  rpc SetChatOverride(SetChatOverrideRequest) returns (Empty);
  rpc RemoveChatOverride(RemoveChatOverrideRequest) returns (Empty);
  rpc GetChatOverrides(GetChatOverridesRequest) returns (OverrideList);
  rpc SetVoiceRoomOverride(SetVoiceRoomOverrideRequest) returns (Empty);
  rpc RemoveVoiceRoomOverride(RemoveVoiceRoomOverrideRequest) returns (Empty);
  rpc GetVoiceRoomOverrides(GetVoiceRoomOverridesRequest) returns (OverrideList);

  // Проверка прав (internal, вызывается другими сервисами)
  rpc CheckPermission(CheckPermissionRequest) returns (CheckPermissionResponse);
  rpc GetEffectivePermissions(GetEffectiveRequest) returns (PermissionSet);
}
```

## Идентификаторы прав (bitmask)

Один канонический набор имён для **ролей в спейсе**, **оверрайдов** и проверок в сервисах. В манифестах ботов (`scopes`) используются **в основном те же строки**, плюс исключение `DM_SEND` (только боты; см. [features/bots.md](../features/bots.md)).

### Спейс (глобально по `space_id`)

| Константа | Назначение |
|-----------|------------|
| `SPACE_VIEW` | Видеть спейс и базовую информацию |
| `SPACE_MANAGE_SETTINGS` | Название, иконка, видимость, правила входа и пр. |
| `SPACE_MANAGE_ROLES` | Создавать/редактировать/удалять роли ниже своей позиции (политика иерархии) |
| `SPACE_MANAGE_INVITES` | Создавать и отзывать инвайт-ссылки |
| `SPACE_VIEW_AUDIT_LOG` | Читать аудит-лог |
| `SPACE_MANAGE_CUSTOM_EMOJIS` | Кастомные эмодзи спейса |
| `SPACE_MANAGE_BOTS` | Добавлять/удалять ботов, их scopes |
| `SPACE_MANAGE_MATCHMAKING` | Настройки матчмейкинга спейса |
| `SPACE_VIEW_MEMBER_LIST` | Видеть список участников спейса (ростер, не контент чатов) |

### Участники спейса

| Константа | Назначение |
|-----------|------------|
| `MEMBER_KICK` | Исключить участника |
| `MEMBER_BAN` | Бан / разбан |
| `MEMBER_MANAGE_NICKNAMES` | Менять ник в спейсе |
| `MEMBER_ASSIGN_ROLES` | Назначать и снимать с участников роли **ниже своей** позиции в иерархии (без создания/удаления определений ролей — это `SPACE_MANAGE_ROLES`) |

### Создание текстовых чатов в спейсе

| Константа | Назначение |
|-----------|------------|
| `TEXT_CHAT_CREATE_IN_SPACE` | Создавать новые строки `chats` с `type = group` \| `channel` и узлы дерева (совместно с Space) |

### Текстовый чат (`chat_id`, `group` \| `channel`)

| Константа | Назначение |
|-----------|------------|
| `TEXT_CHAT_VIEW` | Видеть чат в списке и открывать |
| `TEXT_CHAT_MANAGE_SETTINGS` | Тема, slow mode, настройки чата |
| `TEXT_CHAT_SEND_MESSAGES` | Писать сообщения (если политика чата разрешает от своего имени) |
| `TEXT_CHAT_SEND_MEDIA` | Вложения медиа |
| `TEXT_CHAT_EMBED_LINKS` | Превью / встраивание ссылок |
| `TEXT_CHAT_ATTACH_FILES` | Файлы |
| `TEXT_CHAT_ADD_REACTIONS` | Реакции |
| `TEXT_CHAT_USE_EXTERNAL_EMOJIS` | Внешние эмодзи |
| `TEXT_CHAT_MENTION_ALL_ONLINE` | Упоминание всех онлайн в этом чате (`@here`) |
| `TEXT_CHAT_MENTION_ALL_IN_CHAT` | Упоминание всех участников чата (`@everyone`) |
| `TEXT_CHAT_MANAGE_MESSAGES` | Удалять/закреплять чужие сообщения |
| `TEXT_CHAT_READ_HISTORY` | Читать историю (если выключено — виден только «с момента входа») |
| `TEXT_CHAT_PIN_MESSAGES` | Закреплять сообщения |
| `TEXT_CHAT_CREATE_THREADS` | Создавать треды |
| `TEXT_CHAT_SEND_IN_THREADS` | Писать в тредах |
| `TEXT_CHAT_MANAGE_THREADS` | Модерировать треды |
| `TEXT_CHAT_SET_SLOW_MODE` | Выставлять slow mode на чате |

### Голосовая комната (`voice_room_id`)

| Константа | Назначение |
|-----------|------------|
| `VOICE_JOIN` | Подключаться к комнате |
| `VOICE_SPEAK` | Аудио от себя |
| `VOICE_VIDEO` | Видео |
| `VOICE_SCREEN_SHARE` | Демонстрация экрана |
| `VOICE_MUTE_OTHERS` | Мьютить других |
| `VOICE_DEAFEN_OTHERS` | Deafen других |
| `VOICE_MOVE_OTHERS` | Переносить между комнатами |
| `VOICE_USE_PTT` | Push-to-talk, если включён режим |
| `VOICE_PRIORITY_SPEAKER` | Приоритетный говорящий |

### Модерация (спейс)

| Константа | Назначение |
|-----------|------------|
| `MODERATION_MANAGE_REPORTS` | Жалобы по контенту спейса |
| `MODERATION_TIMEOUT_MEMBERS` | Таймаут участника |

**Владелец спейса** обходит проверки (или эквивалент «все флаги»); точные биты и порядок фиксируются в коде при первой миграции bitmask — здесь зафиксированы **имена**, а не номера битов.

### Манифест бота (`scopes` в JSON)

Те же строковые константы, плюс **только для ботов** (не хранятся в bitmask роли участника):

| Константа | Назначение |
|-----------|------------|
| `DM_SEND` | Писать пользователю в DM — только в ответ на его действие (v1; см. [bots.md](../features/bots.md)) |

Остальные возможности бота задаются теми же именами, что и права участника, например `TEXT_CHAT_SEND_MESSAGES`, `TEXT_CHAT_CREATE_IN_SPACE`, `MEMBER_ASSIGN_ROLES`, `SPACE_VIEW_MEMBER_LIST`. Привилегированное чтение истории для бота — строка `TEXT_CHAT_READ_HISTORY` с отдельной политикой в Bot Service (предупреждение в UI при установке).

## Модель данных

```
roles
├── id (UUID)
├── space_id (FK)
├── name
├── color (hex, nullable)
├── is_system (bool) -- Owner, Admin, Moderator, Member, Guest
├── position (int) -- иерархия, выше = больше приоритет
├── permissions (bigint bitmask)
├── is_mentionable (bool)
├── created_at
└── updated_at

member_roles
├── space_id (FK)
├── profile_id (FK)
├── role_id (FK)
├── assigned_at
├── assigned_by (profile_id)
└── UNIQUE(space_id, profile_id, role_id)

chat_overrides
├── chat_id (FK) -- chats.id, type = group | channel
├── role_id (FK)
├── allow (bigint bitmask) -- явно разрешённые права
├── deny (bigint bitmask)  -- явно запрещённые права
└── UNIQUE(chat_id, role_id)

voice_room_overrides
├── voice_room_id (FK)
├── role_id (FK)
├── allow (bigint bitmask) -- явно разрешённые права
├── deny (bigint bitmask)  -- явно запрещённые права
└── UNIQUE(voice_room_id, role_id)
```

## Вычисление effective permissions

```
1. Если Owner → все права
2. Base = права дефолтной роли участника спейса (роль «участник» / «все»)
3. Для каждой роли пользователя (по позиции):
   Base |= role.permissions
4. Применить оверрайды целевого узла (chat или voice_room):
   Base &= ~node_override.deny
   Base |= node_override.allow
5. Admin → все права кроме Owner-specific
```

## Phase-0 principal и caller matrix (target; не реализовано)

Role принимает actor и service authority только из верифицированного Phase-0
interceptor ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md));
`x-voice-profile-id` и похожие metadata не являются identity. Gateway передаёт
только derived delegated-user principal для client surface.

| Caller principal | Exact allowed Role RPCs | Allowed actor / subject fields |
|---|---|---|
| `gateway` delegated user | `CreateRole`, `UpdateRole`, `DeleteRole`, `ListRoles`, `ReorderRoles`, `AssignRole`, `RevokeRole`, `GetMemberRoles`, `SetChatOverride`, `RemoveChatOverride`, `GetChatOverrides`, `SetVoiceRoomOverride`, `RemoveVoiceRoomOverride`, `GetVoiceRoomOverrides`, `SetDefaultJoinRole`, `GetDefaultJoinRole`, `GetEffectivePermissions` | actor is only derived `sub`/`profile_id`; request `profile_id` is a target subject where that RPC has one and is ACL-checked by Role |
| `space` service | `BootstrapSpaceRoles`, `GetDefaultJoinRole`, `ListRoles`, `GetMemberRoles`, `EnsureDefaultMemberRole` (**target dedicated lifecycle RPC**), `RemoveMemberRoles` (**target dedicated lifecycle RPC**), `CheckPermission` | `owner_profile_id` only for bootstrap; dedicated lifecycle RPCs bind the joining/leaving `profile_id`, `space_id` and Role-resolved default/member roles. Generic `AssignRole`/`RevokeRole` are not accepted from Space. `CheckPermission` is limited to the global permission names enumerated below. |
| `space` trusted transfer | `ApplyOwnershipTransfer` and `CompensateOwnershipTransfer` (**target dedicated lifecycle RPCs**) | exact `space_id`, `old_owner_profile_id`, `new_owner_profile_id`, `operation_id` and request binding; same operation is idempotent; no generic Role RPC may mutate `Owner` |
| `voice`, `chat`, `messaging` service | `CheckPermission` | explicit `profile_id` only as the decision subject, plus requested `space_id`/node scope; no actor mutation authority |
| `bot` service | `CheckPermission`, `GetMemberRoles`, `RevokeRole`, `DeleteRolesCreatedByProfile` | only signed `service:bot` lifecycle/decision calls below; no actor-mutation authority; cleanup cannot touch `Owner` or system roles |
| `bot` signed `bot_actor` capability | `CreateRole`, `AssignRole`, `RevokeRole` | only the exact scope and claim-bound space below; no generic Gateway principal or metadata fallback; `Owner` and every system-role mutation are rejected |

### Space `CheckPermission` decision contract

Space may call only `/voice.role.v1.RoleService/CheckPermission` with a signed
service JWT whose `principal_type=service`, `iss=space`, `sub=service:space`,
`aud=role`, and `rpc` is that exact full RPC name. The credential also requires
`iat`, `nbf`, `exp` (TTL at most **30 s**), `jti`, `kid`, `request_id` and
`request_hash`. `request_hash` binds exactly `space_id`, `profile_id` and
`permission_name`, and binds the absence of both a chat and a voice-room node.

The verified Space principal may request only these global permission names:
`SPACE_VIEW_AUDIT_LOG`, `SPACE_MANAGE_BOTS`, `SPACE_MANAGE_INVITES`,
`MEMBER_KICK`, `MEMBER_BAN`, `MODERATION_TIMEOUT_MEMBERS`,
`SPACE_MANAGE_SETTINGS`, and `TEXT_CHAT_CREATE_IN_SPACE`. `profile_id` remains
decision data asserted by verified Space; it is never identity authority derived
from request metadata.

For Space member lifecycle, `EnsureDefaultMemberRole` binds exactly
`space_id`, joining `profile_id` and the Role-resolved default member role;
`RemoveMemberRoles` binds exactly `space_id` and leaving `profile_id`, removes
only non-system non-Owner memberships, and cannot invoke a generic role mutation.
Both use the standard signed `service:space` claims and request hash. The concrete
RPC additions are target-only until their proto and handler migration lands.

For ownership transfer, the same verified Space principal calls only
`ApplyOwnershipTransfer` or `CompensateOwnershipTransfer`. Each call binds
`space_id`, `old_owner_profile_id`, `new_owner_profile_id`, `operation_id` and
the exact request in its signed request binding. Role records the operation so an
identical replay returns the same result; it never applies or compensates a
different body under the same `operation_id`. These are the only constrained
paths for Owner-role mutation, including the compensating leg of the Space saga.

### Bot service lifecycle and decision contract

The Bot Service may use a normal signed `principal_type=service`,
`iss=bot`, `sub=service:bot`, `aud=role` principal only for the existing
lifecycle/decision calls. It carries the exact RPC, `request_id`, `request_hash`,
`iat`, `nbf`, `exp` (at most **30 s**), `jti` and `kid`.

- `CheckPermission` is only the `InstallBotInSpace` decision: its hash binds
  `space_id`, installer `profile_id`, `permission_name=SPACE_MANAGE_BOTS`, and
  absence of chat/voice node.
- `GetMemberRoles`, `RevokeRole` and `DeleteRolesCreatedByProfile` are only the
  `UninstallBotFromSpace` cleanup. Their hash binds every request field and the
  installed bot's recorded actor profile; Role checks that it matches the
  verified installation context. `RevokeRole` and delete cleanup reject `Owner`
  and all system roles.

This service principal has no authority for interactive bot actor mutations;
those require the capability below.

### Bot actor mutation contract

A bot does not use a Gateway delegated-user principal for Role mutations. Bot
Service issues a separate signed `principal_type=bot_actor` capability with
`iss=bot`, `sub=bot:<bot_id>`, `aud=role`, exact `rpc`, `request_id`,
`request_hash`, `actor_profile_id`, `bot_id`, `space_id`, `installation_id`,
`bot_scope`, and normal temporal/anti-replay claims (`iat`, `nbf`, `exp`, `jti`,
`kid`). Role derives the actor solely from the verified `actor_profile_id` claim
and requires `request.space_id == capability.space_id`.

`request_hash` binds every field of the exact RPC: `space_id`, `name`,
`permissions_mask` and `position` for `CreateRole`; `space_id`, `profile_id` and
`role_id` for `AssignRole` or `RevokeRole`. The capability permits only
`CreateRole` when `bot_scope=SPACE_MANAGE_ROLES`, and `AssignRole` or
`RevokeRole` when `bot_scope=MEMBER_ASSIGN_ROLES`. Existing hierarchy and
permission checks still apply. Every attempt to mutate `Owner` or any system role
is rejected, regardless of scope; a bot capability cannot call the dedicated
ownership-transfer path.

### Phase-0 errors and per-RPC migration

Malformed requests return `INVALID_ARGUMENT`; invalid signature, temporal
claims, audience, RPC or request binding return `UNAUTHENTICATED`; a verified
caller with a wrong allowed caller, scope, actor or business ACL returns
`PERMISSION_DENIED`; unavailable verifier dependencies return `UNAVAILABLE`.

Migration is per exact RPC. Once a protected RPC has its signed-principal
cutover, it has no header or metadata fallback. Bot generic mutations must move
to `bot_actor` in the same cutover that enforces their Role allow-list.
`CheckPermission` cannot become globally strict until every caller of that RPC
uses a signed principal; each migrated caller is enforced independently while
remaining callers follow the explicitly tracked migration path.
`Owner` нельзя назначить, снять или переназначить через generic client/member RPC.
Только dedicated authenticated Space transfer lifecycle меняет Owner и обязан быть
idempotent по `operation_id`. Unknown caller/RPC, caller с неправильной audience
или read, раскрывающий member data вне своей ACL, fail closed.


Доменный поток JetStream: **`role.events`** (матрица: [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие             | Данные                            |
|---------------------|-----------------------------------|
| `role.created`      | space_id, role_id, name           |
| `role.updated`      | space_id, role_id, changed_fields |
| `role.deleted`      | space_id, role_id                 |
| `role.assigned`     | space_id, profile_id, role_id     |
| `role.revoked`      | space_id, profile_id, role_id     |
| `role.chat_override_set`  | chat_id, role_id       |
| `role.voice_override_set` | voice_room_id, role_id |

## Публикуемые события (→ NATS)

## Зависимости

- **Space Service** — валидация `space_id`, `voice_room_id`
- **Chat Service** — валидация `chat_id` для текстового чата (`group` \| `channel`) при оверрайдах
- **Federation Service** — синхронизация ролей при S2S (SyncSnapshot)


### Ownership lifecycle principal runtime

**Staged foundation:** production Space TransferOwnership is disabled before any
lock, database or Role access, including when all signing/TLS settings exist or
Role is absent. There is no environment setting to activate the v1 saga; only
same-package test fixtures opt into its private test switch. Generic Role Owner
assignment/revocation remains forbidden; bootstrap and non-Owner member behavior
is unchanged. V2 durable convergence and Auth proof consumption must be enabled
atomically before production transfer is exposed.


The dedicated `ApplyOwnershipTransfer` and `CompensateOwnershipTransfer` paths
verify `service:space` and the complete deterministic protobuf request hash,
including unknown wire fields. The apply and compensating leg retain the same
`operation_id` and original old/new owner ordering. Generic `AssignRole` and
`RevokeRole` reject `Owner` mutations.

The ordinary Role listener (`ROLE_GRPC_LISTEN`, default `:9090`) rejects both
ownership RPCs even if a signed credential is supplied. A separate TLS listener
(`ROLE_PRINCIPAL_GRPC_LISTEN`, default `:9091` when enabled) accepts only these two
RPCs; unrelated methods are denied there. Other callers remain on their existing
per-method migration path. See the exact runtime settings and activation order in
[DEPLOYMENT.md](../DEPLOYMENT.md#ownership-lifecycle-principal-transport).

This is the Role transport/receipt cutover, not completion of the Auth proof or
public Space operation-idempotency contract.

#### Terminal compensation and delayed apply

A compensated ownership operation is terminal. Under the same operation lock,
Role records a durable compensation receipt even when an in-flight Apply has not
yet recorded its receipt, provided the expected old owner is still the sole
Owner. This receipt is an abort barrier, not evidence that the remote Apply was
canceled. Every later Apply for that operation is denied before mutation or
replaying a stale successful Apply receipt. Identical compensation replays the
recorded old-owner result; changed tuple or request binding conflicts. This
prevents a timed-out Apply from changing Role ownership after Space has restored
its database owner and released its mutation lease.

#### Target durable ownership commit protocol (not implemented)

The next ownership slice adds a disjoint v2 `PrepareOwnershipTransfer`,
`FinalizeOwnershipTransfer` and `AbortOwnershipTransfer` surface with the same
Space principal and exact operation/space/old/new binding. It must land atomically
with the Space durable journal and caller, generated contracts, Role ACL freeze
and recovery worker; the current two-method listener does not enable it.

Role records one serialized operation state per tuple: `prepared`, `finalized`
or `aborted`. In v2, Prepare records `prepared` and freezes
space-scoped authorization without granting Owner to the new profile. Finalize
atomically moves the sole Owner membership and records `finalized`; Abort
records `aborted`, preserving/restoring the old owner. An abort before Prepare is
a durable barrier. Terminal decisions cannot be reversed: late Prepare cannot
resurrect abort, Finalize after abort and Abort after finalize are denied,
and identical same-action retries replay the durable terminal outcome.

While `prepared`, Role rejects space-scoped permission decisions and role/member
reads or mutations with `UNAVAILABLE`; no Owner, Admin, bot or lifecycle shortcut
may bypass the freeze. Only the exact trusted ownership lifecycle may resolve it.
Authorization reads and state transitions must share a database serialization
boundary so permission decisions cannot observe two effective Owners. Already
authorized requests linearize at their completed permission decision; no new
old-owner permission decision may pass after finalization. Missing state or
lookup failure fails closed, never unfreezes via a timeout or process restart.

Role never reads Space's database. Only a Space operation with a durable commit
decision may invoke Finalize; a durable abort decision invokes Abort. Space
must never issue both decisions. Replays/concurrency are serialized by the
operation and space locks, including an absent-operation abort.


##### V2 capability, durable state and rollout contract

The current Apply/Compensate pair retains v1 immediate-mutation semantics and is
never reinterpreted as provisional. V2 uses the disjoint RPCs above plus a trusted
`GetOwnershipTransferCapabilities` read. Before reserving a new operation, Space
requires the authenticated Role endpoint to advertise protocol 2 and all three
v2 methods. Every v2 request and immutable receipt explicitly carries version 2;
unknown versions or operation IDs belonging to v1 are denied before mutation.
No old-server or network-error fallback to v1 is permitted.

The Role ledger enforces one durable active prepared operation per space, in
addition to the operation lock and immutable global operation tuple/hash/version.
An unrelated operation cannot prepare, finalize or abort over that prepared
operation. The complete v2 transition matrix is:

| Existing state | Prepare, exact binding | Finalize, exact binding | Abort, exact binding |
|---|---|---|---|
| Absent | Validate sole old Owner and no active operation; persist prepared/freeze | `FAILED_PRECONDITION` | Validate sole old Owner and no active operation; persist aborted barrier |
| Prepared | Replay prepared receipt | Atomically move sole Owner and persist finalized/unfreeze | Preserve old Owner and persist aborted/unfreeze |
| Finalized | `FAILED_PRECONDITION` | Replay finalized receipt | `FAILED_PRECONDITION` |
| Aborted | `FAILED_PRECONDITION` | `FAILED_PRECONDITION` | Replay aborted receipt |

A changed tuple, canonical body or version for an existing operation returns
`ALREADY_EXISTS` for every action. A normal absence of any active operation allows
ordinary service; a failed active-state lookup returns `UNAVAILABLE`. Missing an
expected receipt during recovery never implies success and never unlocks Space.
Compact immutable terminal receipts remain for the lifetime of the space; hard
space deletion must retain a retired-space fence so late calls cannot recreate
roles or reuse old operation IDs. Timed receipt expiry is not an unlock mechanism.

Freeze enforcement includes every Role RPC/store path: CheckPermission,
GetEffectivePermissions, role/member/override/default-role reads and writes,
BootstrapSpaceRoles, DeleteRolesCreatedByProfile, cleanup and batch paths, plus
owner/admin/bot shortcuts. A request touching any frozen space fails atomically
with `UNAVAILABLE`; it neither returns partial cached authority nor mutates the
unfrozen subset. Reads and writes check the durable freeze in the same database
serialization boundary as permission calculation/mutation. Caller caches cannot
serve a new allow/Owner decision while this fence is uncertain; cached authority
is disabled for the v2 decision path until a versioned invalidation fence is
implemented. Clients must propagate `UNAVAILABLE`, never convert it to Owner or
cached permission fallback.

Rollout first deploys v2-capable Role and Space code without starting v2 transfers.
Place ownership entrypoints in maintenance (`UNAVAILABLE`), drain generic/v1
requests and their bounded RPC contexts/transactions, and prove current Space and
Role owners converge through service-owned inspection. All old Space replicas
must stop issuing generic/v1 mutations before v2 activation. Then enable v2 only
after capability checks pass for the serving fleet and update exact caller and
listener allow-lists atomically. Old v1 receipts keep their original semantics;
only exact recovery/abort of preexisting v1 operations remains during drain, and
new v1 Apply is rejected after cutover. A fleet unable to prove drain/convergence
remains in maintenance rather than mixing ownership protocols.

##### Ordinary-operation transaction boundary and scoped integrity

The v2 ordinary-operation fence holds the same Role database transaction from
actor permission/hierarchy evaluation through the resulting read or mutation.
Nested store helpers reuse that transaction; a separate permission preflight
followed by an independent write is insufficient. Prepared or retired spaces,
and unavailable fence lookups, return `UNAVAILABLE` before Owner shortcuts,
empty-member results, default-role fallback, bootstrap, cleanup or events.
Events are emitted only after a successful transaction commit.

A role ID supplied together with a space ID must resolve to that space under the
same transaction before assignment, revocation, reordering or override mutation.
Role-ID-only updates and deletes discover their space and revalidate the role
under that space's fence. A foreign role ID must never authorize or mutate a
different space through chat/voice override removal. Member-role reads cannot
import authority from a role belonging to another space.

Bootstrap is idempotent for an existing sole Owner with the same profile. It must
not add a second Owner or replace an existing Owner through generic bootstrap;
only the dedicated trusted ownership protocol can change that membership.
Retired spaces cannot be recreated by bootstrap. These scoped-integrity rules
also apply while transfer entrypoints remain disabled; implementing them does
not activate the v2 ownership feature.
