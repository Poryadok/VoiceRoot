# Role Service

### Owning source catalog qualification

The existing fixed RoleState v1 mask projection serves exact clean16/17 and
independently qualified clean20. Intermediate18/19, dirty and unknown versions
remain unavailable. Migration17 restores Admin manage-role bit2;19 adds bit42;
20 adds the explicit built-in43/44 deltas. Actual16→20 qualification seeds
built-ins/custom roles and overrides, checks all eight row invalidations and
unchanged SDK revision, and compares the executed snapshot fold with native
GetEffectiveMask. The original source DDL/function hashes remain unchanged;
catalog20 additionally guards both new SpaceChat clock inputs and four triggers.
This projection does not represent the separate binding-aware SpaceChat policy
decision. The separate development native21/source2 cohort implements protected EnsureDefaultMemberRole/RemoveMemberRoles with fresh Space authority, immutable receipts and generation floors; ordinary listeners deny these methods. Component transport/RolePG/SpacePG proof does not establish complete all-writer cutover, current owner-channel readiness or full runtime activation.

### Admission readiness target (ADMISSION-ROLE-READINESS-v1)

The development native/state2 cohort also qualifies exact clean25 through the
same native21 membership facts and pure mask fold. Its additional Bot removal
tables and immutable guards are mandatory catalog inputs, not permission grants
or membership sources. Cleanup removes typed contributions/preparations before
definitions and invalidates the existing Role revision; the independent SDK
revision is unchanged. State1 is never served for21 or25. Intermediate22-24,
future/dirty schemas and altered guards remain unavailable. Qualified admission
receipts are guarded by the same exact catalog inside their operation/Space
transaction. Bot actor mutation readiness and the durable Assign/Revoke core
remain separate gates; these component proofs do not activate them.

Accepted BE-010 c12/c13 separates the ordinary membership incarnation from an
independent community source. Space stores pending admission outside
`space_members`, allocates a monotonic per-member generation, obtains an inert
Role READY receipt, then publishes membership/count/invite use/outcome/event in
one local transaction after fresh final admission checks. Role preparation is
not membership and cannot authorize an ordinary role grant by row presence.

`EnsureDefaultMemberRole` takes protocol1, canonical Space/profile UUIDs,
positive membership generation, admission operation UUID and exact32-byte
admission binding digest. Role resolves its current configured default role;
the caller cannot select a role, permission or human actor. An immutable receipt
echoes the request and selected role UUID. Exact replay returns the same receipt;
changed operation binding conflicts and never chooses a new default role.
`RemoveMemberRoles` binds protocol1, Space/profile, a monotonic removal floor,
removal operation UUID and exact intent digest. It persists the floor even when
no role rows exist; late Ensure at/below that floor cannot materialize rights.
Both methods use fresh exact-request service:space credentials on the dedicated
Role principal listener and are denied on every ordinary listener.

Receipt SHA256 framing is fixed by `pkg/memberadmission`, independently from
protobuf authentication hashes. Ensure hashes the ASCII domain
`voice.member-admission.ensure-receipt.v1` plus NUL, protocol BE32, Space UUID16,
profile UUID16, membership generation BE64, operation UUID16, original binding
SHA25632 and selected Role UUID16. Remove uses domain
`voice.member-admission.remove-receipt.v1` plus NUL and the same fixed fields
through the original removal binding digest, without a role UUID. IDs must be
canonical nonzero UUIDs; generation is positive and at most signed int64 max.
The exported receipt fingerprints have independent Python golden vectors.

Role-owned storage distinguishes ordinary generation-tagged contributions,
immutable operation receipts, generation floors and independent community
baseline provenance. It does not insert an inert READY projection into the
unqualified legacy `member_roles` relation. A default-role receipt remains
historical evidence after removal, role edits or generation supersession; its
replay cannot restore a contribution. Owner/bootstrap/ownership/P3 and typed
community Member-only projection remain separate authorities.

Before preparation/effective authorization, Role fetches protected Space
`GetMemberAdmissionAuthority` outside its transaction/Space lock. Expected
generation+operation are supplied together for Ensure. Ordinary rights require
current LIVE/ownership availability and matching published generation,
operation and exact Role receipt digest; PREPARING authorizes inert Ensure
only. No positive membership cache, caller assertion, cross-DB query or missing
row fallback can substitute. A wait beyond the accepted5s observation budget
requires a fresh owner read. Space's authority lookup never calls Role.

The same source eligibility guard must cover effective-role/permission/hierarchy
reads, generic Assign/Revoke actor and target incarnation, authority snapshots
and derived voice-policy grants. Unknown owner/transport/storage state denies
or remains unavailable. A completed local leave observed by a new authorization
cannot leave an orphan Role grant effective; the accepted overlapping-operation
snapshot limitation remains. Legacy generation0 needs explicit verified Space
authority and proven provenance backfill, never zero-on-error. Retirement
dominates the new ledgers and prevents identity reuse.

The c15/c16 clarification preserves unknown legacy grant evidence as
`UNRESOLVED`; even one current membership source does not prove the old grant's
origin. Such grants contribute no rights after qualified-reader cutover until a
new normally authorized assignment reconciles them. New Assign with two eligible
sources requires an explicit verified membership binding; with one source the
server may pin that exact incarnation. Never infer both or rebind a retry.
COMMUNITY eligibility binds positive int64-compatible owner generation and
per-profile membership generation plus a never-reused membership episode UUID;
roster revision or row replacement is not episode identity. Generic Revoke pins
a finite set of legal contribution IDs, never a wildcard against future joins.

The native21/25 implementation supplies protected admission receivers, durable
inert receipts and floors, qualified permission/hierarchy/Voice reads and
explicit state2 exports. Its consumer is Space's admission/publication flow.
Configure the fresh owner client with `ROLE_SPACE_MEMBER_AUTHORITY_GRPC_ADDR`,
`ROLE_SPACE_MEMBER_AUTHORITY_TLS_CA_FILE`,
`ROLE_SPACE_MEMBER_AUTHORITY_TLS_SERVER_NAME`,
`ROLE_SPACE_MEMBER_AUTHORITY_CLIENT_CERT_FILE` and
`ROLE_SPACE_MEMBER_AUTHORITY_CLIENT_KEY_FILE`, plus Role's signing identity.
Missing owner authority remains unavailable. Ordinary listeners reject protected
admission and raw actor calls. Generic Assign/Revoke at the qualified catalog
remain unavailable until their durable operation protocol is implemented;
unknown legacy grants are not reclassified. Bot actor mutation routes remain
closed. Component evidence does not establish deployment or complete
cross-service acceptance.

For the BE-131 composite vertical, the existing dedicated Role principal
listener additionally accepts signed `service:space` `CheckPermission` for
`TEXT_CHAT_CREATE_IN_SPACE` only. The exact request binds canonical nonzero
Space and subject profile UUIDs, has no Chat or Voice room override, and uses
fresh method/body/request-ID credentials and single-use JTI. Space derives the
subject from its verified immutable composite intent; it cannot substitute the
owner. The protected interceptor rejects all other principal kinds, issuers,
permissions and scope expansion before the ordinary permission evaluator.
Unrelated legacy decision reads keep their existing migration boundary. A
transport/authentication failure is unknown; only a valid authenticated negative
decision can be treated as permission denial for composite admission.

## Обзор

Управление ролями и правами в пространствах. Иерархия ролей, гранулярные права, оверрайды по текстовым чатам (`group` / `channel`) и голосовым комнатам.

**Язык**: Go
**БД**: PostgreSQL `role_db`

Accepted audit producer details use the frozen
[external codec](space-service.md#external-audit-details-codec-v1) and
[replay identity](space-service.md#durable-external-replay-codec-v1), shared through
`src/backend/pkg/spaceaudit`. Build actual normalized changes once inside the
mutation/outbox transaction; retries preserve those bytes. Producer runtime is
tracked separately by BE-131; this codec contract does not activate its transport.

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

The additive [ListThreads membership source contract](../testing/listthreads-versioned-readmodel-exec-plan.md#effective-membership-contract-v1)
requires a Space-scoped transactional revision covering every `TEXT_CHAT_VIEW`
input, including role assignments, text overrides, Owner/Admin effects and
ownership/lifecycle fences. Role supplies pinned effective decisions and
authoritative head validation to signed caller `chat`; Chat does not duplicate
permission precedence. Potential restrictions also increment a durable revocation
counter. A voice-only epoch or best-effort event cannot satisfy this contract.
This target is not yet implemented and does not bypass the existing ownership
freeze or change authorization's completed-decision linearization point.

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

  // GIS-owned managed game-session grants (T31 target contract)
rpc ApplyGameSessionGrants(ApplyGameSessionGrantsRequest) returns (ApplyGameSessionGrantsResponse);
rpc RevokeGameSessionGrants(RevokeGameSessionGrantsRequest) returns (RevokeGameSessionGrantsResponse);
  rpc CheckGameSessionGrant(CheckGameSessionGrantRequest) returns (CheckGameSessionGrantResponse);
}
```

### T31 managed game-session grants (target owner contract)

The grant RPCs and ledger in this section are required T31 additions, not
implemented APIs at base `c31a8e0b771f99d00f00a760a2c6e2f7ec67cbcd`. Role has
no T31 grant proto/handlers, grant ledger, or migration at that base. T31
includes the protobuf contract, private server implementation, PostgreSQL
ledger/migration, and owner-level integration tests. GIS and Voice integration
must not ship until those owner receipts and admission checks are executable.

Role owns a separate `game_session_grants` ledger; GIS never writes it
directly. Its private gRPC listener requires mTLS and a signed workload principal.
Configure the listener with `ROLE_PRINCIPAL_TLS_CERT_FILE`,
`ROLE_PRINCIPAL_TLS_KEY_FILE`, and `ROLE_PRINCIPAL_CLIENT_CA_FILE`; the last
file is the trusted client-certificate CA bundle. `ROLE_PRINCIPAL_GRPC_LISTEN`
defaults to `:9091`. `S2S_JWKS_URLS_JSON` must contain the exact `space`,
`gameintegration`, and `voice` issuers. The server requires and verifies a
client certificate before principal verification.
GIS authenticates as exact issuer `gameintegration`, audience `role`, exact
RPC full name and deterministic protobuf request hash; `x-request-id` equals
the request's operation ID for Apply/Revoke. The distinct Voice caller uses
exact issuer `voice`, audience `role`, exact `CheckGameSessionGrant` RPC and
request hash; its request ID is a fresh canonical UUID. Role accepts only
Apply/Revoke from GIS and only Check from Voice on the game-session grant
surface. Voice also calls the Voice-only `ResolveVoiceRoomGrants` RPC on this
private listener using the same exact issuer/audience/RPC/request-hash binding
and a fresh canonical request ID. Role validates the Space, Voice room, and
profile UUIDs, then returns the existing per-Space fenced permission decision
and policy epoch. The ordinary listener continues to reject this RPC. For
game-session Apply/Revoke, each JTI authorizes one attempt; retries use a
fresh JTI and the same request ID/hash. An apply request binds
`application_id`, `environment_id`, `session_id`, `voice_room_id`,
`operation_id`, `roster_revision`, and the complete sorted, unique list of
canonical `profile_id` UUIDs. It grants only `VOICE_JOIN` for that exact
managed Voice room. Role atomically replaces the session's complete grant set
and writes a durable receipt containing the request hash, revision, applied
profile-set hash and receipt ID. The same operation ID and request hash returns
the same receipt; changed input conflicts. A lower roster revision is a stale
no-op, equal revision/equal set replays without mutation, equal revision with a
different set conflicts, and only a complete higher revision replaces the set.
An empty set is valid only as an explicit complete higher revision.

The apply receipt is readiness evidence only after Role commits every grant in
its database transaction. Voice checks `CheckGameSessionGrant` for the
authenticated profile, application/environment, session and room on JoinCall
and token issuance in addition to current Chat membership; missing, revoked,
or mismatched grants fail closed. A grant remains valid while its durable
session ledger row is active. Role has no time-based session expiry; durable
Revoke immediately removes the session's grants and terminally fences later
Apply calls. GIS closes Voice first, then revokes Role grants; Voice close
independently fences admission. `RevokeGameSessionGrants` takes
app/environment/session, close operation ID and request hash, atomically
removes only that session's managed grants, and returns a durable receipt. It
cannot revoke another session's grant or a player's ordinary Role assignment.
Exact revoke retry returns the original receipt; conflicting reuse rejects.
GIS reports a roster or close stage ready only after validating this Role
receipt. These private RPCs do not widen `AssignRole`/`RevokeRole` authority.

### BE-116 Space audit producer dependency (accepted target; not implemented)

For every successful role definition, assignment or override mutation listed in
the closed [Space audit registry](space-service.md#closed-action-and-writer-registry-v1),
Role writes one canonical audit fact into a durable `role_db` producer outbox in
the same transaction as the domain mutation. A failed or no-op mutation creates
no new fact. A worker signs the exact deterministic
`voice.space.v1.SpaceService/AppendAuditEvent` request as `service:role`; after
the empty acknowledgement it marks the outbox item delivered. Retry keeps the
same `audit_event_id` and exact payload. Role never writes `space_db`, never puts
caller identity in the request body and cannot emit Space- or Chat-owned actions.

The proto/registry exists; the Role outbox, publisher and activation remain a
BE-116 implementation dependency.

## Идентификаторы прав (bitmask)

Один канонический набор имён для **ролей в спейсе**, **оверрайдов** и проверок в сервисах. В манифестах ботов (`scopes`) используются **в основном те же строки**, плюс исключение `DM_SEND` (только боты; см. [features/bots.md](../features/bots.md)).

### Спейс (глобально по `space_id`)

| Константа | Назначение |
|-----------|------------|
| `SPACE_VIEW` | Видеть спейс и базовую информацию |
| `SPACE_MANAGE_SETTINGS` | Название, иконка, видимость, правила входа и пр. |
| `SPACE_MANAGE_ROLES` | Создавать/редактировать/удалять роли ниже своей позиции (политика иерархии) |
| `SPACE_MANAGE_INVITES` | Создавать и отзывать инвайт-ссылки |
| `SPACE_APPROVE_JOIN_REQUESTS` | Принятый target: читать заявки/ответы, approve/reject; отдельное делегируемое право, по умолчанию Owner/Admin/Moderator |
| `SPACE_VIEW_AUDIT_LOG` | Читать аудит-лог |
| `SPACE_MANAGE_CUSTOM_EMOJIS` | Кастомные эмодзи спейса |
| `SPACE_MANAGE_BOTS` | Добавлять/удалять ботов, их scopes |
| `SPACE_MANAGE_MATCHMAKING` | Настройки матчмейкинга спейса |
| `SPACE_VIEW_MEMBER_LIST` | Видеть список участников спейса (ростер, не контент чатов) |

### Участники спейса

`SPACE_APPROVE_JOIN_REQUESTS` относится к Space, а не к отдельному текстовому
чату или голосовой комнате. Admission API должен проверять его для текущего
actor и точного Space при каждом чтении/решении. `SPACE_MANAGE_INVITES`,
`SPACE_MANAGE_SETTINGS`, `MEMBER_BAN` и имя `Moderator` не заменяют effective
permission. Member и Guest не получают его по умолчанию; custom roles могут
делегировать его по обычным правилам Role. Runtime registry, defaults,
миграция существующих системных ролей и admission consumers ещё входят в
реализацию BE-010; строка таблицы не означает, что они уже реализованы.

| Константа | Назначение |
|-----------|------------|
| `MEMBER_KICK` | Исключить участника |
| `MEMBER_BAN` | Бан / разбан |
| `MEMBER_MANAGE_NICKNAMES` | Менять ник в спейсе |
| `MEMBER_ASSIGN_ROLES` | Назначать и снимать с участников роли **ниже своей** позиции в иерархии (без создания/удаления определений ролей — это `SPACE_MANAGE_ROLES`) |

Space requests the exact global `MEMBER_BAN` decision over the protected Role
listener with a request-bound `service:space` principal. Role checks current
Space membership and the effective mask; the ordinary listener cannot grant this
decision. Missing protected transport fails closed.

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
| `TEXT_CHAT_MANAGE_MESSAGES` | Удалять чужие сообщения; закрепление требует отдельного `TEXT_CHAT_PIN_MESSAGES` |
| `TEXT_CHAT_READ_HISTORY` | Читать историю (если выключено — виден только «с момента входа») |
| `TEXT_CHAT_PIN_MESSAGES` | Закреплять сообщения |
| `TEXT_CHAT_CREATE_THREADS` | Создавать треды |
| `TEXT_CHAT_SEND_IN_THREADS` | Писать в тредах |
| `TEXT_CHAT_MANAGE_THREADS` | Модерировать треды |
| `TEXT_CHAT_SET_SLOW_MODE` | Выставлять slow mode на чате |
| `TEXT_CHAT_POST_AS_CHAT` | Отдельная official-publication capability; ordinary member дополнительно требует текущий individual grant |
| `TEXT_CHAT_ADD_MEMBERS` | Инициировать chat-targeted приглашение в родительский Space; отдельно требуется `SPACE_MANAGE_INVITES` |

### Space-chat individual capabilities (accepted implementation contract)

The registry reserves bit 43 for `TEXT_CHAT_POST_AS_CHAT` and bit 44 for
`TEXT_CHAT_ADD_MEMBERS`, without renumbering existing permissions. New system
Member/Moderator seeds include only the latter. Guest seeds acquire neither;
existing guest/admission policy remains conjunctive. Owner/Admin include both
ordinary bits, but dedicated grant management requires actual current system
Owner/Admin authority, not an all-bits custom role or a role with the same name.

For an ordinary current Space member, official publication requires a separate
Role-owned grant bound to this exact chat and current membership incarnation;
a role-wide allow alone is insufficient, and an explicit node deny wins.
Addition requires the chat's `member_additions_enabled` policy (default true),
effective `TEXT_CHAT_ADD_MEMBERS` and separately authorized Space invitation.
Neither capability implies visibility, send/settings rights, admission bypass
or local Chat membership. Current Owner/Admin bypass the toggle/individual
grant only after current membership, exact scope and lifecycle validation.

Only current system Owner/Admin manages the toggle and individual grant.
Promotion invents no explicit grant; demotion drops implicit authority while
preserving a separately explicit grant in the same membership incarnation.
Leave/rejoin, scope move and retirement cannot resurrect or migrate old grants.
Role owns this state for Space chats; Chat owns equivalent state only for
standalone chats. Signed caller-relative reads and delegated-user setters,
authority revision revalidation, additive existing-role migration and the
targeted-invite consumer are tracked in the
[implementation plan](../testing/space-chat-permissions-exec-plan.md).
Registry/seed support alone does not activate those endpoints or prove the
complete parent-Space admission and exact-chat navigation flow.

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

BE-273 accepted target extends the same protected `ResolveVoiceRoomGrants`
projection with explicit versioned audio/video/share-source capability. Voice
must not infer `VOICE_VIDEO` or `VOICE_SCREEN_SHARE` from `VOICE_SPEAK`, or
authorize an unproven source from a legacy/malformed response. Role retains the
policy epoch and sole Owner bypass; Voice intersects its durable moderator
restriction with those grants without mutating Role state. Target moderation
requires `VOICE_MUTE_OTHERS` in the canonical room and `SPACE_VIEW_MEMBER_LIST`;
the target must be a current Space member. Source construction and the distinct
Voice-owned moderation version are defined in the
[Voice contract](voice-service.md#moderator-mute-accepted-target-be-273).

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
   Base |= node_override.allow
   Base &= ~node_override.deny
5. Admin использует обычное объединение ролей и оверрайды; финального bypass нет
```

`SPACE_MANAGE_ROLES` — обычное делегируемое право на управление кастомными
ролями строго ниже максимальной позиции вызывающего. Новая позиция при
`UpdateRole` тоже обязана оставаться ниже неё; проверка и запись выполняются
в одной fenced Space-транзакции. `MEMBER_ASSIGN_ROLES` отдельно управляет
назначением/снятием нижестоящих ролей. Node override может менять mask узла,
но не разрешает глобальное управление ролями: для него всегда нужен unscoped
permission decision без `chat_id` / `voice_room_id`.

Системный Admin по умолчанию имеет все ordinary permission bits, включая
`SPACE_MANAGE_ROLES`. Его mask может совпадать с mask Owner; Owner-specific
authority не является permission bit и не возникает из mask. Только настоящий
Owner имеет существующий bypass после lifecycle fences; generic назначение и
снятие Owner запрещено даже самому Owner. Передача и owner-only lifecycle
сохраняют отдельные identity/proof gates.

Миграция `role_db/000017_admin_role_management` добавляет только отсутствующий
bit 2 реальным system Admin rows, сохраняя остальные bits, custom roles и
overrides. Существующие policy triggers записывают durable invalidation.
Повторный UP не меняет данные; DOWN отказывает вместо отзыва ранее выданных
прав без доказательства их происхождения. Для исправления нужен отдельный
reviewed forward migration. Authority snapshot reader принимает clean schema
16 и 17 с одним и тем же полным catalog/trigger/function proof; unknown/dirty
версии и изменённый catalog по-прежнему fail closed.

## Phase-0 principal и caller matrix (target; не реализовано)

Role принимает actor и service authority только из верифицированного Phase-0
interceptor ([ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md));
`x-voice-profile-id` и похожие metadata не являются identity. Gateway передаёт
только derived delegated-user principal для client surface.

| Caller principal | Exact allowed Role RPCs | Allowed actor / subject fields |
|---|---|---|
| `gateway` delegated user | `CreateRole`, `UpdateRole`, `DeleteRole`, `ListRoles`, `ReorderRoles`, `AssignRole`, `RevokeRole`, `GetMemberRoles`, `SetChatOverride`, `RemoveChatOverride`, `GetChatOverrides`, `SetVoiceRoomOverride`, `RemoveVoiceRoomOverride`, `GetVoiceRoomOverrides`, `SetDefaultJoinRole`, `GetDefaultJoinRole`, `GetEffectivePermissions` | actor is only derived `sub`/`profile_id`; request `profile_id` is a target subject where that RPC has one and is ACL-checked by Role |
| `space` service | `BootstrapSpaceRoles`, `GetDefaultJoinRole`, `ListRoles`, `GetMemberRoles`, `EnsureDefaultMemberRole` (**target dedicated lifecycle RPC**), `RemoveMemberRoles` (**target dedicated lifecycle RPC**), `CheckPermission` | `owner_profile_id` only for bootstrap; dedicated lifecycle RPCs bind the joining/leaving `profile_id`, `space_id` and Role-resolved default/member roles. Generic `AssignRole`/`RevokeRole` are not accepted from Space. `CheckPermission` is limited to the global permission names enumerated below. |
| `space` trusted transfer | `GetOwnershipTransferCapabilities`, `PrepareOwnershipTransfer`, `FinalizeOwnershipTransfer`, `AbortOwnershipTransfer` | protocol 2 and exact `space_id`, `old_owner_profile_id`, `new_owner_profile_id`, `operation_id` and request binding; same action is idempotent; no generic Role RPC may mutate `Owner` |
| `voice`, `chat`, `messaging` service | `CheckPermission` | explicit `profile_id` only as the decision subject, plus requested `space_id`/node scope; no actor mutation authority |
| `bot` service | `CheckPermission`, `GetMemberRoles`, `RevokeRole`, `DeleteRolesCreatedByProfile` | only signed `service:bot` lifecycle/decision calls below; no actor-mutation authority; cleanup cannot touch `Owner` or system roles |
| `bot` signed `bot_actor` capability | `CreateRole`, `AssignRole`, `RevokeRole` | only the exact scope and claim-bound space below; no generic Gateway principal or metadata fallback; `Owner` and every system-role mutation are rejected |

### Scoped Messaging and Voice decisions

The protected listener accepts signed `service:messaging` decisions only for
`TEXT_CHAT_SEND_MESSAGES`, `TEXT_CHAT_PIN_MESSAGES`, `TEXT_CHAT_ADD_REACTIONS`,
`TEXT_CHAT_MENTION_ALL_IN_CHAT`, `TEXT_CHAT_MENTION_ALL_ONLINE`,
`TEXT_CHAT_CREATE_THREADS`, and `TEXT_CHAT_SEND_IN_THREADS`. Each request requires
canonical Space/profile/chat IDs, an explicit GROUP or CHANNEL type, and no voice
room. Configuring the `messaging` HTTPS endpoint in `S2S_JWKS_URLS_JSON` enables
only this closed decision surface; it adds no actor or mutation authority.

Signed `service:voice` decisions accept only `VOICE_JOIN`, `VOICE_SPEAK`,
`VOICE_SCREEN_SHARE`, `VOICE_MUTE_OTHERS`, and `VOICE_MOVE_OTHERS`, with explicit
canonical Space/profile/voice-room IDs and no chat. Voice remains responsible for
the asserted room's ownership. Both services bind audience `role`, exact
`CheckPermission` RPC, fresh UUID request ID and deterministic full request hash.
The permission selector chooses one verifier without issuer fallback. Unknown
fields, mixed identity and node scopes, expired credentials and replay deny.
The native reader obtains current Space membership outside Role locks; Messaging
also requires the exact current Space-tree chat observation. It rechecks the
original observation vector and credential after the owning read transaction
commits. Ordinary listener calls do not provide this authority.

### Space `CheckPermission` decision contract

The protected Bot installation decision is a separate exact branch: only
`service:bot`, `SPACE_MANAGE_BOTS`, canonical Space/profile IDs and no chat/voice
node. `ROLE_BOT_LIFECYCLE_ENABLED=true` adds only the configured `bot` HTTPS JWKS
issuer to the closed listener trust set. Missing/partial configuration fails
closed; the ordinary listener rejects this decision. This branch uses current
native membership qualification, never a raw Owner/Admin label. Cleanup RPC
activation additionally requires its durable participant adapter and fences;
enabling the decision flag alone does not establish cleanup readiness.

Space may call only `/voice.role.v1.RoleService/CheckPermission` with a signed
service JWT whose `principal_type=service`, `iss=space`, `sub=service:space`,
`aud=role`, and `rpc` is that exact full RPC name. The credential also requires
`iat`, `nbf`, `exp` (TTL at most **30 s**), `jti`, `kid`, `request_id` and
`request_hash`. `request_hash` binds exactly `space_id`, `profile_id` and
`permission_name`, and binds the absence of both a chat and a voice-room node.

The verified Space principal may request only these global permission names:
`SPACE_VIEW_AUDIT_LOG`, `SPACE_MANAGE_BOTS`, `SPACE_MANAGE_INVITES`,
`MEMBER_KICK`, `MEMBER_BAN`, `MODERATION_TIMEOUT_MEMBERS`,
`SPACE_MANAGE_SETTINGS`, `SPACE_MANAGE_MATCHMAKING`, `SPACE_APPROVE_JOIN_REQUESTS`, and
`TEXT_CHAT_CREATE_IN_SPACE`. This is the accepted target allowlist. The current
protected runtime's narrow `CheckPermission` routing supports
`TEXT_CHAT_CREATE_IN_SPACE`, `SPACE_APPROVE_JOIN_REQUESTS`,
`SPACE_MANAGE_SETTINGS`, `SPACE_MANAGE_MATCHMAKING`, and `SPACE_MANAGE_INVITES`;
adding an allowlist name here does not expand that listener. Matchmaking-only
Space updates request `SPACE_MANAGE_MATCHMAKING`; mixed updates require both
settings and matchmaking decisions. `profile_id` remains
decision data asserted by verified Space; it is never identity authority derived
from request metadata.

For Space member lifecycle, `EnsureDefaultMemberRole` binds exactly
`space_id`, joining `profile_id` and the Role-resolved default member role;
`RemoveMemberRoles` binds exactly `space_id` and leaving `profile_id`, removes
only non-system non-Owner memberships, and cannot invoke a generic role mutation.
Both use the standard signed `service:space` claims and request hash. The concrete
RPC additions are target-only until their proto and handler migration lands.

For ownership transfer, the same verified Space principal calls only the v2
capability, Prepare, Finalize and Abort RPCs on the protected listener. Each
transition binds protocol 2, `space_id`, `old_owner_profile_id`,
`new_owner_profile_id`, `operation_id` and the exact request in its signed
request binding. Role records the operation so an identical action replay returns
the same result; it rejects a different body under the same `operation_id`.
These are the only routed paths for Owner-role mutation.

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

BE-234-nickname-v2-transport adds one executable exception to the otherwise
target-only Space permission allowlist: the protected mTLS listener accepts
`CheckPermission` only for exact `MEMBER_MANAGE_NICKNAMES`, `service:space`,
audience `role`, exact RPC/request ID/hash binding and absent `chat` and
`voice_room_id` (present-empty is forbidden). Space vouches for the verified
actor profile in the request, not the edited member. The service JWT carries no
user authority. The Role handler repeats the gate before `WithinSpaces` and
effective-mask calculation, preserving retirement/ownership freeze. Ordinary
Role listener rejects only this nickname selector; other legacy permission
branches keep their existing routing. No wildcard, default-role grant or global
CheckPermission cutover is introduced.
`Owner` нельзя назначить, снять или переназначить через generic client/member RPC.

### Audit producer initiating authority (BE131-INITIATING-AUTHORITY-v1 target)

This accepted cutover is part of the consumed audit vertical, not a claim that
the current ordinary listener verifies user or bot identity. Audit-producing
interactive methods are `CreateRole`, `UpdateRole`, `DeleteRole`, `ReorderRoles`,
`AssignRole`, `RevokeRole`, both set/remove override pairs, and
`SetDefaultJoinRole`. Gateway uses the delegated-user matrix above; Bot may use
only its three existing `bot_actor` methods/scopes. A service principal cannot
impersonate an interactive actor. Every migrated exact method is denied on all
ordinary/alternate listeners, including when protected configuration is absent.

The signed initiating profile, rather than the target, Space owner, worker or
service subject, becomes immutable `actor_profile_id` in the transaction's audit
fact. User admission verifies current Auth session-epoch floor and the existing
account/profile eligibility policy; removal of raw metadata must not default a
guest to regular. Internal adapters may project verified identity for existing
helpers only after the interceptor rejects raw identity metadata and the
handler requires the verified context. Fresh delivery credentials assert only
producer provenance and cannot repair an unverified initiating actor.

Space bootstrap, default membership/community baseline projection and member
cleanup remain distinct service lifecycle operations. Space cannot use generic
`AssignRole`/`RevokeRole`; Role selects the bounded default or baseline Member
role, preserves independently retained community membership, and never widens
`RemoveMemberRoles` to Owner/system-role cleanup. Bootstrap owner is a subject
of the persisted Space creation, not an audit actor. Existing bootstrap
`role.created` notifications are not hooks for synthetic user audit entries.
Join/leave and Bot mass cleanup remain outside audit v1. Signed Bot uninstall
cleanup uses its exact durable installation intent and cannot enter interactive
handlers. Bot Install/Uninstall must verify the initiating Gateway user, current
epoch and `SPACE_MANAGE_BOTS` before asserting service cleanup authority.

Configuration, JWKS refresh/rotation, replay and epoch checks, Gateway/Bot/Space
caller wiring, listener denial and producer transaction hooks ship as one
consumed cutover. Existing ownership/GIS/Voice principals retain their exact
authority. Do not globally lock unrelated `CheckPermission` callers before
their own migration; remaining broad S2S work belongs to BE-181. The producer
is not enabled by a future auth-migration promise.
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
| `role.chat_override_set`  | space_id, chat_id, role_id       |
| `role.chat_override_removed` | space_id, chat_id, role_id |
| `role.voice_override_set` | space_id, voice_room_id, role_id |
| `role.voice_override_removed` | space_id, voice_room_id, role_id |
| `role.voice_policy_invalidated` | event_id, space_id, policy_epoch, optional voice_room_id/profile_id; typed `RoleStreamEvent` |

Role persists the immutable policy epoch snapshot in its own transactional
outbox. Voice consumes the dedicated `role_events` subject with durable
`voice_space_media_role` delivery and ACKs only after the active Space room
reconciliation succeeds. Optional room/profile IDs are hints; they do not
replace the full current-authority scan. Existing Role subjects and their
payloads are unchanged.

### Ordinary notification wire (ROLE-ORDINARY-WIRE-v1)

Ordinary notifications use nine additive `RoleStreamEvent` arms. The exact
subject must match the selected arm; a subject cannot supply a missing action.
`RoleMutation` contains `space_id=1`, `role_id=2`, `profile_id=3`, `name=4`,
`chat_id=5`, `voice_room_id=6`, and repeated `changed_fields=7`.

| Subject | Arm | Number |
|---|---|---:|
| `role.created` | `role_created` | 13 |
| `role.updated` | `role_updated` | 14 |
| `role.deleted` | `role_deleted` | 15 |
| `role.assigned` | `role_assigned` | 16 |
| `role.revoked` | `role_revoked` | 17 |
| `role.chat_override_set` | `role_chat_override_set` | 18 |
| `role.chat_override_removed` | `role_chat_override_removed` | 19 |
| `role.voice_override_set` | `role_voice_override_set` | 20 |
| `role.voice_override_removed` | `role_voice_override_removed` | 21 |

The generic arms 10/11 remain unspecified legacy schemas: never infer a
snapshot, delta or action from their `role_ids` or NATS subject. Policy arm12
and its Voice outbox/reconciliation contract remain separate. Ordinary events
are notifications, not commands to alter authorization or replace a role set.

Deploy dual readers in Realtime and Analytics before any binary ordinary writer.
The current ordinary writer remains flat JSON in the reader release. Historical
flat JSON preserves its missing-field and unknown-property tolerance, while
arrays, null and envelope-shaped JSON are not ordinary flat records. New binary
requires action data, a valid event UUID/time and matching subject/arm. Unknown
protobuf fields on a recognized arm are compatible; unsupported/malformed events
produce no ordinary effect. The binary writer must forward `changed_fields`;
missing legacy lists mean unspecified changes, not an empty authoritative set.

Realtime keeps exactly the existing six fields plus `subject` in `role_update`.
Only assigned/revoked target a profile; only chat override events target a chat.
The other five actions have no recipient index and never broadcast, including
when a payload contains an unrelated `chat_id`. `changed_fields` stays in NATS.
Analytics keeps existing action names, hashes profiles and stores only Space/Role
properties; typed source UUID/time pass through its actual Runner. Legacy
identity remains the stream/sequence fallback. ACK follows persistence.

Keep dual readers while JSON can be retained, replayed or restored. Producer
rollback to JSON is compatible with dual readers; JSON-only reader rollback is
unsafe after any binary is retained, even after producer rollback. Do not purge
or recreate streams/durables to enable the migration. Source checks do not prove
actual serving-reader upgrade or authorize binary cutover/deployment.

## Публикуемые события (→ NATS)

## Зависимости

- **Space Service** — валидация `space_id`, `voice_room_id`
- **Chat Service** — валидация `chat_id` для текстового чата (`group` \| `channel`) при оверрайдах
- **Federation Service** — синхронизация ролей при S2S (SyncSnapshot)


### Ownership lifecycle principal runtime

**Staged foundation:** production Space TransferOwnership is disabled before any
lock, database or Role access, including when all signing/TLS settings exist or
Role is absent. There is no environment setting to activate the v1 saga. Generic
Role Owner assignment/revocation remains forbidden; bootstrap and non-Owner
member behavior is unchanged. Role's v2 ledger and protected transport are
present, but capability advertisement remains fail-closed until the legacy drain,
complete v2 method set and retired-space fence are explicitly proven. The Space
coordinator and public transfer remain disabled.


The legacy `ApplyOwnershipTransfer` and `CompensateOwnershipTransfer` handlers
retain their immutable v1 receipts for historical recovery, but neither Role
listener routes new calls to them. Generic `AssignRole` and `RevokeRole` reject
`Owner` mutations.

The ordinary Role listener (`ROLE_GRPC_LISTEN`, default `:9090`) rejects the
capability RPC and all v1/v2 ownership RPCs before handler or verifier access. A
separate TLS listener (`ROLE_PRINCIPAL_GRPC_LISTEN`, default `:9091` when enabled)
routes only authenticated `GetOwnershipTransferCapabilities`,
`PrepareOwnershipTransfer`, `FinalizeOwnershipTransfer` and
`AbortOwnershipTransfer`; it rejects legacy and unrelated methods. Capability
returns `UNAVAILABLE` until the explicit activation gate passes. See the runtime
settings and activation order in
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

#### Role-side durable ownership commit protocol

Role implements the disjoint v2 `PrepareOwnershipTransfer`,
`FinalizeOwnershipTransfer` and `AbortOwnershipTransfer` surface with the same
Space principal and exact operation/space/old/new binding. The Space caller and
recovery coordinator remain a separate activation dependency; Role capability
does not advertise until the legacy drain and retired-space hold are resolved.

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
listener allow-lists atomically. Preexisting v1 operations must reach a durable
terminal and owner-converged state while the old fleet still serves its bounded
recovery path, before listener cutover. After cutover the retained v1 receipts
and handlers are historical/internal evidence only; neither Role listener routes
Apply or Compensate. A fleet unable to prove drain/convergence remains in
maintenance rather than mixing ownership protocols.

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

## BE-065 protected override target validation (accepted target)

`BE-065-target-v1` is the accepted implementation contract. The source setters
require fresh protected validation; rollout and real-store acceptance remain
separate evidence. The executable plan and required
negative-case matrix are in
[role-override-target-validation-exec-plan.md](../testing/role-override-target-validation-exec-plan.md).

Every `SetChatOverride` attempt freshly calls both
`/voice.space.v1.SpaceService/ValidateRoleOverrideTarget` (without a room) and
`/voice.chat.v1.ChatService/ValidateRoleOverrideTarget`. Every
`SetVoiceRoomOverride` attempt freshly calls Space's same method with the room
present. A missing client/configuration, optional environment skip, positive
cache, legacy `GetChat`/`GetSpace`/tree lookup or cross-database query cannot
authorize a write. Provider success validates existence and local lifecycle;
it grants no actor permission and returns no owner/member projection.

Both owner methods accept only verified `principal_type=service`, `iss=role`,
`sub=service:role`, exact `aud=chat|space` and exact full RPC name. The Phase-0
credential binds the whole deterministic protobuf request (including optional
field presence), canonical `request_id`, fresh `jti`, `kid` and bounded temporal
claims (TTL <=30 s). One bearer and one matching `x-request-id` are required;
raw identity metadata and duplicate authority headers reject before handler.
Use protected TLS/mTLS with Role JWKS and replay verification. Retry signs a
fresh JTI and performs a fresh owner read; validation is not a durable receipt
or reusable capability. Gateway delegated-user, end-user, bot and other service
principals are forbidden. There is no public Gateway route.

The lock order is mandatory: first perform actor/role-scope/permission/hierarchy
checks in a short existing fenced Role transaction, release that transaction,
then make bounded owner RPCs with **no Role transaction or advisory lock held**.
Finally open the normal fenced Role mutation transaction, repeat all current
actor/role/hierarchy checks and write in that same transaction. Parent
cancellation and deadline propagate; each owner call has a bounded deadline.
Events occur only after successful Role commit. Providers use only their own
store and lifecycle/journal authority; they never call Role or an ACL-enriched
provider lookup. This avoids Role -> provider -> Role recursion and lock inversion.

Space validates a live Space, plus a live room in that exact Space if supplied.
Chat validates an active group/channel row with non-null matching Space ID and
ordinary local admission. DM, standalone, foreign, deleted and missing targets
are indistinguishable `NOT_FOUND`. Both providers require canonical non-nil
UUIDs; a present-empty optional room is `INVALID_ARGUMENT`. Tree placement is
not resource authority. Nonterminal Space deletion/recovery state, active
ownership journal and non-LIVE Chat fence deny with `UNAVAILABLE`; known
terminal fences also deny. Missing legacy fence permits admission only where
the existing owner's canonical semantics permit it; lookup failure never means
LIVE. Role's own prepared/frozen/purge/retired fences always win before Owner
shortcuts. A higher permitted LIVE restoration requires new validation.

This is point-in-time reference validation, **not distributed referential
integrity**. Deletion after provider snapshot and before Role commit may leave
an inert logical override row. Actual resource operations must still enforce
existence/lifecycle; the row cannot resurrect or authorize a deleted resource.
IDs are not reusable or reparentable; recreation uses a fresh server UUID.
Group <-> channel conversion preserves identity and remains eligible.
`RemoveChatOverride` and `RemoveVoiceRoomOverride` remain locally authorized
cleanup paths and never require live-target lookup. P3 retirement removes Space
overrides. No `voice.room_deleted` consumer is presumed: that subject is reserved.

Provider errors: malformed input `INVALID_ARGUMENT`; ineligible/missing target
`NOT_FOUND`; invalid signature/claims/binding/replay `UNAUTHENTICATED`;
authenticated wrong caller `PERMISSION_DENIED`; known lifecycle fence or
DB/JWKS/replay dependency failure `UNAVAILABLE`. TLS failure precedes handler.
Role preserves its own actor/ACL errors and owner `NOT_FOUND`/`INVALID_ARGUMENT`;
owner transport/config/verifier failure, `UNIMPLEMENTED` or unavailable validation
becomes `UNAVAILABLE` on the user mutation. Parent cancellation/deadline remains
typed. Every failed validation leaves existing rows and mutation events unchanged.

### BE-065 runtime configuration

Role signs with `ROLE_PRINCIPAL_SIGNING_KEYS_DIR` and
`ROLE_PRINCIPAL_ACTIVE_KID` (the standard current/next PKCS#8 key directory).
Its separate HTTPS JWKS endpoint is `/.well-known/jwks.json`, configured by
`ROLE_PRINCIPAL_JWKS_TLS_CERT_FILE`, `ROLE_PRINCIPAL_JWKS_TLS_KEY_FILE` and
`ROLE_PRINCIPAL_JWKS_HTTPS_LISTEN` (default `:9092`); health HTTP does not expose
JWKS. Publish only public current/next keys through that TLS endpoint.

Set both `ROLE_CHAT_TARGET_GRPC_ADDR` and `ROLE_SPACE_TARGET_GRPC_ADDR`, each
owner's `ROLE_CHAT_TARGET_TLS_CA_FILE` / `ROLE_SPACE_TARGET_TLS_CA_FILE` and
`ROLE_CHAT_TARGET_TLS_SERVER_NAME` / `ROLE_SPACE_TARGET_TLS_SERVER_NAME`, plus
Role's `ROLE_OVERRIDE_TARGET_CLIENT_CERT_FILE` and
`ROLE_OVERRIDE_TARGET_CLIENT_KEY_FILE`. Partial configuration fails startup;
absent configuration never permits a setter write and returns `UNAVAILABLE`.
No plaintext or ordinary-listener fallback exists. Owner calls each inherit the
parent deadline and have a maximum two-second call budget.

Each owner has a dedicated listener prefix:
`CHAT_ROLE_OVERRIDE_PRINCIPAL_` or `SPACE_ROLE_OVERRIDE_PRINCIPAL_`, with required
`GRPC_LISTEN`, `TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`, and
`REPLAY_REDIS_ADDR`; `REPLAY_REDIS_PASSWORD` is optional. The client CA verifies
Role's mTLS certificate before the workload JWT reaches the handler. Shared
`S2S_JWKS_URLS_JSON` must include Role's HTTPS endpoint; `S2S_JWKS_CA_FILE` supplies
an optional private issuer CA. Standard Phase-0 refresh/hard-expiry/unknown-kid
settings apply. Each owner periodically refreshes Role's complete key set and
fails closed after hard expiry. Other configured service issuers may be verified
only to deny their caller authority; none gains this RPC.

Redis replay keys are owner-scoped
`chat:role-override:principal:replay:<sha256(issuer + NUL + jti)>` and the matching
`space:` prefix, atomically admitted with `SET NX` for the remaining credential
lifetime rounded up to milliseconds. Redis failure denies before domain access;
there is no process-local fallback or reusable positive-validation cache.

## P3 permanent Space retirement

Role is participant 1 of the common Space lifecycle barrier. Before retirement,
`ApplySpaceLifecycleFence` accepts only a request-bound authenticated Space
service principal on the protected listener. The durable head binds canonical
Space/deletion IDs, monotonic generation and the exact common manifest. FROZEN
and PURGE_DECIDED block ordinary Role reads, permission decisions, mutations,
bootstrap and new ownership admission under the same advisory Space lock.
LIVE at the next generation restores ordinary admission only before irreversible
purge. A later deletion cycle starts with a new operation at the next generation
after LIVE; old exact receipt replay cannot reopen current authority. Prepared
ownership operations prevent a new deletion fence. Exact immutable receipt
replay preserves the original ID and database application time; changed bindings
conflict. Permanent retirement takes precedence over every stale LIVE request.
Migration `000014_space_deletion_fence` persists the head and immutable receipt
tuple. Receipts bind the full typed participant RPC with a domain-separated
SHA-256, independently of transport credential hashing. Startup checks the
schema and runs bounded compaction; full bytes compact only 30 days after
permanent retirement, while the semantic tuple and stable receipt remain.
Focused real PostgreSQL, mTLS/JWKS and restart/negative checks verify this
participant; integrated all-participant acceptance remains tracked in T40.

Version 13 is reserved for `000013_game_session_grants`; its original UP bytes
remain unchanged. Its DOWN requires READ COMMITTED isolation and refuses a
transaction that could retain a stale empty snapshot. It acquires exclusive locks on operation receipts,
sessions and grants before checking emptiness, retaining them through DROP in
the same migration transaction. A concurrent first writer is either observed
after the lock wait and prevents rollback, or the bounded lock wait fails;
saved authority and receipts must never be discarded. Version 14 also handles the pre-repair local version-13 fence
catalog: exact legacy fences/receipts are retained and canonical grant tables
are added. A canonical grant-only v13 receives the fence schema. A combined
exact catalog is adopted without row changes. The upgrade requires one recorded
clean v13 marker, or the dirty v14 marker set by the executing golang-migrate
driver. Missing/old/dirty v13 state, partial schemas, different columns/defaults/
checks/keys/indexes/FKs, or an altered immutable function/trigger are refused.
RLS (including forced RLS), saved policies, rewrite rules, inheritance and
unlogged tables also prevent adoption, even when columns and keys match.
This includes schema validation against temporary canonical tables before any
upgrade can commit; `IF NOT EXISTS` is not used to trust saved objects.

The actual pinned golang-migrate acceptance loads the complete unique source
catalog, runs to clean v13, then upgrades canonical and legacy fixture states
to clean v14. Direct-SQL store fixtures record their input marker separately;
they do not replace that deployment evidence. DOWN locks both fence tables,
rechecks after concurrent writes complete and refuses while either holds saved
evidence. The locked catalog check rejects filtering or rewriting behavior;
`row_security=off` makes hidden-row access fail instead of authorizing an empty
rollback. A non-superuser forced-RLS regression verifies saved receipts survive.
Empty DOWN removes only fence objects and retains canonical v13 game
grants. Never force a dirty version marker to bypass a refused catalog; preserve
the database and diagnose/recover the exact source state before rollout.

`RetireSpace` is protected-listener only and accepts `protocol_version=1`,
canonical Space/deletion IDs, positive generation, `purge_decided_at` and exact
root manifest binding from authenticated Space workload identity. Unknown
request fields are rejected. Role refuses retirement while any ownership-v2
operation is `PREPARED`, or unless the durable `PURGE_DECIDED` head matches
the exact Space, deletion operation, generation and complete manifest tuple.
One transaction writes the permanent retirement fence
and immutable receipt, removes ordinary Role rows and emits final policy
invalidation. Exact replay returns stored bytes; changed operation, generation,
manifest or request for the retired Space is `FAILED_PRECONDITION`.

Every delayed ownership v1/v2, ordinary role/permission and bootstrap call
checks retirement before old receipt replay or mutation, so identifier reuse
cannot recreate authority. Ownership receipts may compact only after retirement
and no `PREPARED` operation; the compact retirement receipt and fence are
permanent. Full request/receipt bytes retain 30 days after completion. The
receipt binds request and manifest SHA-256 and is Role's participant completion
evidence; restore cannot reach this path because it is decided only before
`PURGE_DECIDED`.

The immutable request fields are `protocol_version`, `space_id`,
`deletion_operation_id`, positive `generation`, `purge_decided_at` and
`ManifestBinding`. The receipt fields are `protocol_version`, stable
`receipt_id`, the same Space/operation/generation, state exactly `RETIRED`,
`request_sha256`, `manifest_sha256` and database `retired_at`. Receipt-wrapper
unknown fields are preserved after known-field validation; request and nested
unknown fields, including fields inside `purge_decided_at` Timestamp, are
rejected before mutation.

The implementation lives behind Role's authenticated TLS listener. Migration
`000012_space_retirement` extends `role_space_lifecycle` and stores permanent
compact evidence in `role_space_retirement_receipts`; its database-derived
`full_bytes_retain_until` permits one-way request/receipt byte compaction only
after 30 days. The ordinary listener rejects `RetireSpace` before dispatch.
The compact row retains every immutable request field, because
[`ARCHITECTURE_REQUIREMENTS.md`](../ARCHITECTURE_REQUIREMENTS.md) requires
semantic and byte/hash equality and explicitly says that a hash alone is
insufficient.
