# Space Service

## Local-report read context (BE-066 accepted target)

NEW `ResolveSpaceReportReadContext` accepts only exact signed
`service:messaging`, audience `space`, bound to protocol1, operation ID,
`space_id`, canonical `ChatRef` and viewer profile. Use normal Phase-0
method/body/request/JTI/JWKS/epoch where applicable and protected mTLS guards.
The existing communication/timeout RPC does not grant this read capability.

One native Space snapshot reads live/unfrozen lifecycle, exact Chat tree
attachment and current active published membership. Return the request binding,
`eligible`, Space authority revision, lifecycle generation, tree attachment
incarnation, membership binding and `history_visible_since` with explicit
known/unknown state. Do not return roster, owner IDs or other members.
Pending/revoked/left membership is ineligible. A timeout alone does not deny read.

The history boundary is the canonical continuous effective membership episode
across ordinary/community sources. Rejoining cannot revive a previous episode;
consumers cannot choose the oldest historical join. Missing exact history
provenance is unknown, never time zero; Messaging fails UNAVAILABLE in the
since-entry branch. It separately requires current Role TEXT_CHAT_VIEW and, for
older history, TEXT_CHAT_READ_HISTORY, plus native message visibility and Social.
ListMembers page100 and unmanaged Chat entitlement are not substitutes for this
producer. No foreign database reads, raw viewer identity metadata or public
caller registration is introduced by this target contract.

The owner-local `ReadSpaceReportReadContext` source stage reads real qualified41
native witnesses and the exact effective head under a fenced READ COMMITTED
transaction, checking revision and final DB time after SQL waits. It returns
positioning facts, not a permission or completed all-writer cutover capability.
Missing/stale heads never repair themselves. Protected serving additionally
requires the native guard-epoch/completed per-Space cutover producer; current
table presence, head-local cutover ID and a fresh Creator head do not supply it.
The protected RPC/full downstream chain remains unactivated until that producer
and its real qualified cohort are composed.

The local `ReadProtectedSpaceReportReadContext` producer now composes the real
native44 `RequireScopedNativeMembershipReadiness` with those positioning facts
in the SAME fenced READ COMMITTED transaction. A genuine serving LOGIN needs
the exact catalog and per-Space COMPLETE registry before any facts are returned;
installer privileges, a fresh Creator head or schema44 alone do not suffice.
It then validates actual native witnesses, exact tree attachment, episode and
final DB clock, retaining UNKNOWN history without reader repair. Native producer
pin d410305b and its independent SOURCE CLEAR423 qualify this exact44 local
composition; they do not qualify45 or other unfinished writer routes. This is
not request authentication, Role/content authorization or an exposed RPC. The
earlier facts-only tests retain explicit schema41 fixtures; new protected
consumer tests run the actual44 cohort and current eleven-participant P3.

Accepted410 (BE010 c76-c79, decision SHA256
`83baa14994d9b5f63f0ae7c24956d0a1a373360111c6735be9d590322e64c0b4`)
requires both an enforced exact DB guard epoch and a completed full-cohort
per-Space UNKNOWN cutover. Forward44 implements protected transaction/backend
frames, BEFORE capture enforcement, semantic source-loss observation, dirty
sequence and deferred completion. It includes actual OLD member/owner finite
coverage and a common final clock after waiting SQL. Ban/unban ABA cannot be
hidden by final witness equality. Permanent retirement is immutable. Private
capture rows are deleted at commit or real terminal cleanup; the retained
registry tombstone contains only Space/protocol facts.

`beginNativeMembershipDBMutation` returns the DB-derived complete cohort, not
the caller's subset. Native production writers must finalize that entire set
and call `completeNativeMembershipDBMutation` after their last source SQL.
`InitializeScopedNativeMembership` is a separate administrator-only full native
source validation and UNKNOWN successor initializer. It preserves immutable
head-local cutover IDs and ONE-source publication identity. A Space created
under the installed guard can establish a genuinely empty baseline before its
first source publication. Reader repair and repeated administrative recutover
are prohibited.

`RequireScopedNativeMembershipReadiness` additionally requires the producer's
exact native catalog qualification. Its SQL gate rejects a serving login with
owner/DDL/superuser/replication/role-escalation privileges, missing per-Space
COMPLETE or a changed guard catalog. Tests use an actual independent login,
not an elevated session with SET ROLE. Installation or a standalone SQL guard
positive does not qualify the entire native provider. Current source requires
186's exact44 catalog and production writer placements; protected serving and
the full report chain remain unavailable until their real composition. The
administrative initializer follows410 maintenance decision
`1d5ea8e9ced79bf9279c3f8a8dfabc129bc1f579369fba4992fd02ffc6604488`
(BE010 c89-c92): coherent FROZEN/PREPARED and preterminal PURGE_DECIDED/PURGING
may first initialize UNKNOWN before a permanent fence. Validate actual typed
sources independently of serving availability; do not collapse them to NONE.
Commit the administrative transaction before a separate native negative/P3
transaction. Empty derived cohorts require no fabricated profile. Interactive
serving remains LIVE plus AVAILABLE; malformed/retired/missing facts refuse.
The real native P3 transaction opens capture before its first permanent-fence
SQL. After actual current-eleven-participant/native terminal cleanup, it uses
`completeNativeMembershipDBPurge` to verify a same-transaction Space/token-bound
terminal receipt and erased private frames/cohort/heads. A missing normal frame
is not successful completion. This placement belongs to186's existing lifecycle
transaction owner; no outer ambient transaction or caller cleanup flag is valid.

The continuous effective binding is distinct from the ONE-source Role/admission
binding. BE-066 c23 selects a compact generation/episode UUID/retirement floor and
ledger revision, exact private ordinary/community witnesses and their coverage
horizon. Capture OLD published support before changing either member or owner
lease, under the native sorted Space transaction fence. Preserve an episode only
through an exact surviving source. Community continuity ends at the minimum of
both strict leases; a later renewal cannot bridge expiry without a sweeper.
Final fresh DB time must cover all captured profiles after waiting SQL. New entry
is the owner publication completion time; clock regression and overflow fail.

Explicit all-writer cutover initializes every existing eligible source with
UNKNOWN cutoff, including community-only and positive ordinary publications.
An overlapping source cannot upgrade UNKNOWN; a complete break followed by a
new publication may create a KNOWN boundary. Reads never repair missing heads.

Forward ledger41 follows native39 and admission40; the historical prototype38
was not integrated and is not rewritten. 41 qualifies the exact native39 terminal
predicate and canonical34 eleven-participant predicate. Its Space terminal
cleanup calls that real native predicate, removes profile-bearing effective
heads and denies late replay behind the permanent Space fence. Reversible freeze
is not itself a membership-history cut. Destructive DOWN locks Space and ledger
before checking emptiness, preserving a first concurrent writer. This storage
composition does not activate native all-writer capture/finalization or protected
ingress; actual native provider/cutover/catalog and account-erasure composition
remain required. Head minimization alone is not global physical-erasure proof.

## Composite Chat creation (accepted implementation contract)

`CommitSpaceChatCreation` admits only a freshly authenticated `service:chat`
credential bound to exact RPC, operation/request ID and protobuf hash over mTLS;
shared replay storage and rotating HTTPS JWKS fail closed. Ordinary listeners
deny this method. `GetSpaceChatCreationOutcome` accepts private Chat recovery or
the protected Gateway delegated-user path. Gateway additionally proves original
creator/account ownership and current Space membership/lifecycle admission;
service authentication alone never authorizes public disclosure.

Space rechecks current membership, guest policy, Role
`TEXT_CHAT_CREATE_IN_SPACE`, lifecycle/ownership fencing and capacity before the
first node commit. Bot creation additionally requires Bot-owned current exact
bot/actor/installation/scope admission. Missing authority is UNKNOWN, never an
ABORTED result. Network checks hold no Space DB lock; local transaction checks
repeat under the shared Space advisory lock. The same lock serializes composite
node/result writes with freeze, audit ingress and terminal purge. The operation
lock is acquired before the Space lock. Node plus immutable COMMITTED result,
or deterministic domain refusal plus ABORTED result, commit atomically; ordinary
tree audit is not emitted for this internal step. Ordinary Upsert keeps its audit.

Chat owns prepared/HELD work and autonomous recovery. Exact replay returns the
original typed snapshot even after node movement/removal and never reinserts it.
Gateway waits for Chat completion and reads that snapshot instead of issuing a
second ordinary tree Upsert. An absent receipt/timeout does not prove abort.
The complete tuple, terminal retention and pre-manifest drain contract are in
[DATA_MODEL.md](../DATA_MODEL.md#p3-space-deletion-ownership-and-file-references-accepted-target).

During freeze/purging, protected Chat audit ingress may finish only the original
`chat_created` fact matching a saved COMMITTED composite's audit/Space/Chat/creator
binding and exact empty details codec. It does not rerun interactive ACL or admit
new ordinary commits. Ingress takes the shared Space transaction lock before
receipt lookup, including duplicate/retry paths. Terminal purge records a
permanent Space fence atomically with fixed evidence deadlines and local deletion;
late signed deliveries cannot insert audit, outbox or node rows. Reserved composite
audit IDs remain unavailable for a different fact after payload minimization.
Actual participant drain/completion proof is required before aggregate purge.

Migration `000031_composite_audit_reservation` serializes composite reservation,
protected ingress and audit INSERT through one transaction advisory lock per
audit UUID. The order is Space, audit UUID, then reservation/receipt rows.
Composite commit rejects any pre-existing receipt or different composite fence;
the audit trigger rejects local/different facts using a reserved ID while allowing
the original COMMITTED Chat tuple. The protected ingress additionally verifies
the complete saved creator binding. No provisional receipt/fingerprint is created.
Receipt minimization takes no audit UUID lock and runs after Space cleanup locks.
Startup of the composite receiver requires migrations `000028` and `000031`.
The migration refuses every pre-existing receipt/composite audit-ID overlap:
producer/Space equality and a hash alone cannot prove the full original creator
binding, particularly after audit payload retention. Even apparently matching
historical Chat overlaps require explicit verified reconciliation; the migration
preserves both records and supplies no bypass or automatic evidence deletion.

The receiver config prefix is `SPACE_CHAT_COMPOSITE_`: `GRPC_LISTEN`,
`TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`, `PRINCIPAL_JWKS_URL`,
`PRINCIPAL_JWKS_CA_FILE`, `PRINCIPAL_JWKS_CLIENT_CERT_FILE`,
`PRINCIPAL_JWKS_CLIENT_KEY_FILE`, `PRINCIPAL_REDIS_ADDR` and optional
`PRINCIPAL_REDIS_PASSWORD`. JWKS fetching itself uses mTLS against Chat's existing
HTTPS principal endpoint; Space never copies Chat signing keys. Partial config
fails startup. Enabled composite requires the protected Gateway listener and
Role principal client/Space signer. Its narrow Role decision is a fresh signed
`service:space` `CheckPermission`, restricted to the immutable actor/Space and
`TEXT_CHAT_CREATE_IN_SPACE`; the legacy ordinary client is never a fallback.
Bot's current installation decision uses `SPACE_BOT_COMPOSITE_GRPC_ADDR`,
`SPACE_BOT_COMPOSITE_TLS_CA_FILE`, `SPACE_BOT_COMPOSITE_TLS_SERVER_NAME`,
`SPACE_BOT_COMPOSITE_CLIENT_CERT_FILE` and `SPACE_BOT_COMPOSITE_CLIENT_KEY_FILE`,
pointing to Bot's existing protected Space lifecycle listener. It verifies the
exact echoed tuple and treats missing/malformed/unauthenticated responses as
UNKNOWN. Only a valid authenticated `allowed=false` is a domain refusal.

## Communication timeout authority (BE-044-TIMEOUT-v1)

Accepted implementation target; not a shipped/runtime-acceptance claim. The
[product action matrix](../features/spaces.md#communication-timeout-be-044-timeout-v1-accepted-target)
is an additional profile/Space deny. Space owns timeout and evaluation time.
It evaluates live membership/lifecycle, timeout state and positive `access_epoch`
under the existing serialized authority snapshot, using its DB clock. Missing or
expired timeout is CLEAR only for an eligible current member; equality is expired.

`ResolveMemberCommunicationState` is a read-only protected RPC for
`service:messaging` only: protocol1, exact Space/profile, explicit
UNSPECIFIED/CLEAR/TIMED_OUT enum, echoed IDs, positive access epoch and evaluation
timestamp. TIMED_OUT requires an until strictly after evaluation; CLEAR omits it.
No moderator/reason/audit details or REST route. Voice alone uses the existing
`ResolveVoiceRoomAccess`, extended with the same versioned evidence from the same
snapshot; membership/active/discoverable keep their existing meanings.

Both covered decisions require signed exact-RPC/hash/audience/request-bound
service authority, mTLS, HTTPS trusted JWKS and shared replay protection. Ordinary
listeners and legacy bearer/header fallback cannot provide this capability.
All-absent consumer capability leaves covered Space writes/grants unavailable;
partial configuration fails startup. Unknown version/enum, default/missing data,
wrong IDs, epoch0 or impossible timestamps are unavailable, never CLEAR.
Nonmember/undiscoverable resources preserve their generic discovery response.

Timeout set/replace/remove atomically updates the existing access epoch and
immutable `space.voice_room_access_invalidated` outbox with timeout/audit.
Migration24 already supplies the timeout mutation trigger; do not double-increment.
No positive/negative decision cache or downstream SQL into `space_db` is allowed.
Each new effect obtains a fresh decision within one two-second monotonic
check-to-commit deadline; a concurrent already-admitted effect may finish within
that budget. Consumer-local time cannot convert a previous deny into allow.

The provider has two separately configured narrow listeners, both addressed to
audience `space`: `SPACE_VOICE_COMMUNICATION_` (default `:9094`, Voice room
resolver only) and `SPACE_MESSAGING_COMMUNICATION_` (default `:9095`, member
communication resolver only). Each prefix requires `REPLAY_REDIS_ADDR`,
`TLS_CERT_FILE`, `TLS_KEY_FILE`, `CLIENT_CA_FILE`, `JWKS_CLIENT_CERT_FILE` and
`JWKS_CLIENT_KEY_FILE`; `GRPC_LISTEN` and `REPLAY_REDIS_PASSWORD` are optional.
`S2S_JWKS_URLS_JSON` supplies the exact trusted caller's HTTPS endpoint, with
`S2S_JWKS_CA_FILE` extending server trust when necessary. A listener may instead
set its own `<prefix>JWKS_CA_FILE`; absent retains the shared CA, while an
explicitly blank, unreadable or invalid override fails without falling back.
Local trust configuration alone is partial listener configuration and fails;
shared trust settings alone enable no listener. This permits separate Social
and Voice CAs without a combined trust bundle or sharing private keys. Replicas use shared
replay storage; each listener authenticates its one caller and exact method.
All-absent provider settings leave that listener disabled. Partial settings
fail startup. The ordinary listener rejects both methods, and the legacy
`SPACE_VOICE_S2S_TOKEN` cannot authenticate either decision.

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

The additive [ListThreads membership source contract](../testing/listthreads-versioned-readmodel-exec-plan.md#effective-membership-contract-v1)
requires Space-scoped transactional membership/lifecycle revisions, revocation
counters, exact replay/outbox records and complete pinned snapshots/head
validation for signed caller `chat`. Every join/leave/kick/ban and allowed
restoration participates. A paginated response is complete only with its sealed
terminal proof; retention pins preserve replay across Chat reconciliation.
This is an unimplemented activation prerequisite, not a grant on an existing
ordinary listener or a claim of runtime delivery.

The Social privacy transport exposes only `AreCoMembers` on a dedicated TLS
listener (9091). Signature, request binding, expiry, Redis replay admission and
the `service:social` allow-list are checked before domain/store access. The
domain wrapper also requires the verified principal. An ordinary 9090 call with
a raw Social marker is rejected; other existing callers migrate separately.
See [deployment and rotation](../DEPLOYMENT.md#social-privacy-principals).

```protobuf
service SpaceService {
  // Пространства
  rpc CreateSpace(CreateSpaceRequest) returns (Space);
  rpc UpdateSpace(UpdateSpaceRequest) returns (Space);
  rpc UpdateSpaceMmConfig(UpdateSpaceMmConfigRequest) returns (Space); // ✓ shipped
  rpc DeleteSpace(DeleteSpaceRequest) returns (Empty);               // obsolete hard-delete handler is DB-blocked; BE-245 must replace it
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
| CreateSpace, UpdateSpace, GetSpace, ListMySpaces | ✓ | ✓ | `UpdateSpace.allow_guests` is an owner/admin (`SPACE_MANAGE_SETTINGS`) opt-in; guest admission is invite-only and fail-closed. |
| UpdateSpaceMmConfig | ✓ | ✓ | MM config on space |
| DeleteSpace | ✓ | Disabled | Obsolete owner-only hard delete is blocked at the DB boundary; BE-245 must schedule the 7-day hidden/frozen recovery window and freeze the public status mapping |
| RestoreSpace | ✗ | ✗ | Target owner-only restore during recovery window |
| SearchPublicSpaces | ✓ | ✗ | Catalog search backlog |
| Create/Update/Delete VoiceRoom | ✓ | ✓ | |
| UpsertTreeNode, RemoveTreeNode, ReorderSpaceTree | ✓ | ✓ | Tree pin fields shipped in migration `000007_tree_pin` |
| **ListSpaceTree** | ✓ | ✓ | **Omitted from earlier doc inventory** |
| Create/Update/Delete Category | ✓ | ✓ | |
| PinTreeNode, UnpinTreeNode | ✓ | ✓ | Migration `000007_tree_pin`; handlers and event payload shipped |
| CreateInvite, GetInvite, JoinByInvite | ✓ | ✓ | Guest `JoinByInvite` requires a valid invite and `space.allow_guests=true`; ban, cap, privacy/block and default-role checks remain in the ordinary join path. Composable entry policy remains backlog. |
| **RevokeInvite, ListInvites** | ✓ | ✓ | Exact `SPACE_MANAGE_INVITES` permission; owner retains the canonical Role bypass |
| JoinSpace, LeaveSpace | ✓ | ✓ | Composable AND entry policy and invite-safe verifier pipeline remain backlog — [todo/backend.md](../todo/backend.md) |
| KickMember, BanMember, UnbanMember, ListMembers, ListBans | ✓ | ✓ | |
| TimeoutMember, RemoveMemberTimeout | ✓ | ✓ | |
| TransferOwnership | ✓ | Disabled | Production still denies before dependencies. Auth proof consume/receipt lookup, Role v2, and the Space protocol-2 journal through terminal evidence, ordinary freeze and the ready-row scan are shipped foundations. Network recovery/orchestration, capability activation, public operation idempotency and the Gateway/Flutter vertical remain open. See § Ownership lifecycle principal transport. |
| AddBotMember, RemoveBotMember | ✓ | ✓ | |
| ListTemplates, CreateFromTemplate | ✓ | ✗ | |
| GetAuditLog | ✓ | ✓ backend gRPC/REST source | `created_at DESC, id DESC`; actor/action/from/to filters; 15-minute HMAC-signed snapshot cursor bound to viewer, query and key; default 50/max 100; exact `SPACE_VIEW_AUDIT_LOG`. Flutter remains backlog; source integration is not deployed proof |
| AppendAuditEvent | ✓ | ✗ | Dedicated signed Role/Chat ingestion listener and idempotent store; producer outboxes tracked separately by BE-131 |
| AreCoMembers | ✓ | ✓ | S2S co-membership check |
| SyncSpaceProSubscription | ✓ | ✓ | Subscription sync |

`ListMembers` retains current-member authorization and returns the existing member
list with an opaque `next_cursor` (empty on the last page). Ordering is
`joined_at ASC, profile_id ASC`; continuation uses both fields. Page size defaults
to 50 and is capped at 100. Previously issued UUID-only cursors remain accepted
when their boundary resolves in the same eligible Space roster; an unresolvable
or malformed cursor returns `INVALID_ARGUMENT`, rather than a false complete
empty result. Newly issued tuple cursors do not require the boundary row to remain
present. Pagination does not create a roster snapshot or bypass eligibility checks.

### Tree read authorization cut

`ListSpaceTree` reads the current viewer's effective membership, local categories,
nodes and rooms under one qualified read-only source transaction and
captures the durable Space authority revision. Ordinary and leased-community
membership use the same canonical eligibility rule. Chat-name enrichment runs
outside that scope; before returning any tree or enriched chat fields, the
handler obtains a fresh membership/revision cut. A removed viewer receives
`PERMISSION_DENIED`; a changed revision, including remove/rejoin ABA or a tree
mutation during enrichment, returns `UNAVAILABLE` without a partial response.
Both the initial read and the final recheck validate the exact migration pin,
clean migration metadata, canonical relation shape, enabled revision triggers
and unchanged function-body hashes in the same repeatable-read snapshot as
the local authority facts. This guard is unconditional, including when the
separate Federation source listener is disabled. Missing/dirty/unknown metadata,
pre-24 schemas or altered trigger/function catalogs fail closed before content
or enrichment is exposed. Qualified tree-read migration pins at this checkpoint
are 24-31 and 34; later owner migrations need explicit qualification. The
Federation source service retains its separate exact version-25 gate.
The revision remains internal positioning evidence and is not a reusable allow.

Complete tree revision coverage is supplied by the authoritative revision
migration (`000024_authority_revision`), including category/node mutations;
the earlier media-only epoch is not sufficient watcher snapshot evidence.
This current-tree read guard does not implement materialized viewer-Space
discovery, protected watcher bootstrap, a signed cursor or Voice roster delivery.
It uses authorization-read linearization and does not promise synchronous
post-revoke draining of all writers.

### Migration runbook -- fail-closed guest admission

`000016_allow_guests_fail_closed` is a forward-only security correction. It sets
`spaces.allow_guests` to `false` for every legacy row and changes the column default
to `false`; its DOWN intentionally refuses to run because the previous per-Space
open/closed state cannot be reconstructed safely.

For staging or production, take and retain a restorable `space_db` backup/PITR point
before applying this migration. Apply `000016` before enabling a Space binary that
uses the new admission path. Record the migration version, the pre-migration count
and post-migration count of `allow_guests = true` (the postcondition is zero), and
the default evidence from `information_schema.columns.column_default` (it must be
`false`). A temporary denial of a guest is safe; a temporary admission is not.

Recovery never runs the DOWN migration. Roll an application back only while retaining
the hardened schema and data when compatible. If database restoration is unavoidable,
restore the pre-migration backup into an isolated database, validate it, perform a
separately reviewed cutover, and then apply a forward security fix. Never restore over
the live source database.

The implemented scope is new Space membership through `JoinByInvite` only: a guest
needs a valid invite and explicit `space.allow_guests=true`. Disabling the flag does
not evict existing guests, revoke their memberships, or retroactively remove
downstream access. It does not implement `entry_policy`, the Space+Chat effective
access rule, Chat R3-A15, or Chat/Messaging/Realtime enforcement.

### Versioned entry policy (target)

The accepted product contract is [Space entry verification](../features/spaces.md#верификация-при-вступлении).
The implementation must replace the single legacy `entry_requirement` with a
monotonic policy revision and composable AND configuration. Questions are
machine-checked against configured answers; a wrong answer fails. Questions
never imply manual approval. Manual approve never substitutes for automatic
requirements. `SPACE_APPROVE_JOIN_REQUESTS` is the separate delegable permission
for moderator request/answer reads and approve/reject, defaulting to
Owner/Admin/Moderator. Settings, invite and ban powers are insufficient alone.

All direct, invite and approval paths use the same ordered evaluation:
ban/member-cap/guest admission, phone, captcha, questions, manual approval.
Existing visibility, privacy and account-block gates remain effective. Pending
manual review requires successful automatic stages, creates no membership and
reserves neither invite uses nor member capacity. The existing empty-body join
contract and pending `409 failed_precondition` response remain unchanged;
proof submission and applicant/moderator operations require documented additive
APIs rather than caller-controlled identity or success metadata.

Verification binds purpose, authenticated account and active profile, Space,
admission kind, exact invite when used, monotonic policy/config revision and
immutable submission identity/digest. A policy revision is never reused after
an A→B→A edit. Unknown/malformed legacy policy or question configuration must
not migrate to unrestricted admission. Legacy writers must not replace an AND
policy with a single flag. A phone value, User badge or client boolean is not
authoritative verified-phone evidence; a missing verifier remains fail-closed.

Space-owned admission state is committed in one transaction under the shared
per-Space PostgreSQL mutation fence, not only the process-local handler lock.
Re-read policy, ban, current cap, guest/visibility state, lifecycle/ownership
freeze and exact invite after acquiring the fence. Sample the database clock
after lock waits for expiry checks. Membership insertion, member_count,
invite-use increment and durable operation outcome must agree at commit;
rollback changes none. Pending approve cannot bypass a subsequent ban, expired
or revoked invite, changed policy, full Space or lifecycle freeze. Successful
retry consumes no extra use and cannot resurrect access after leave/ban/purge.

Remote Auth/Role checks are not made atomic by that local transaction. The
bounded snapshot profile below makes this limitation explicit. Default
Role assignment and membership events need durable, idempotent completion
with readiness and generation fences: a failure after local commit must not
lose the consumed admission or grant a role after leave/ban/purge. The current
post-commit `finalizeMembership` call does not prove this guarantee.

#### Policy schema, answers and compatibility

The private policy JSON has `schema_version:1`, required boolean fields
`require_phone`, `require_captcha`, `require_manual_approval`, and a required
`questions` array. Each question is
`{id:UUID,prompt:string,kind:"exact_text",expected_answer:string}`. An empty
array disables questions; otherwise every question must pass. There are at most
10 questions, with unique nonzero IDs and prompts of 1–1024 UTF-8 bytes.
Normalized configured/submitted answers are 1–256 UTF-8 bytes.

Normalize answers with Unicode NFC, then trim only outer U+0009, U+000A,
U+000D and U+0020. Preserve interior whitespace, case and punctuation; reject
other C0 controls and DEL. Comparison is exact after normalization, with no
numeric coercion, expression evaluation, regex or fuzzy/LLM grading. For an
expected `4`, outer ASCII whitespace is accepted, but `04`, `4.0`, `+4`,
`four` and a full-width digit fail. Settings explain this comparison rule.
Reject invalid UTF-8, unknown/duplicate JSON fields, unsupported schema/kind,
missing/duplicate/extra answer IDs, normalized blank answers and oversized
payloads before verification. Prompt text is never executable.

Configured answers and their digests are private: only settings-authorized
reads may return the configured values. Public Space, GetInvite, requirements,
lists and events use explicit safe DTOs. Do not serialize the private policy
through legacy `entry_questions_json`. Submitted answers are visible only to
the original applicant and authorized request reviewers; never log raw
answers, captcha tokens, invite codes or answer digests.

`SetEntryPolicy` accepts a complete private policy with UUID `operation_id`
and `expected_policy_version`. It requires `SPACE_MANAGE_SETTINGS`. An exact
operation/body replay returns its saved outcome; a changed body is
`ALREADY_EXISTS`, revision mismatch is `FAILED_PRECONDITION`, both without
mutation. Server-owned `entry_policy_version` is positive, monotonically
increasing and distinct from JSON `schema_version`. Every canonical config
change, including prompt/answer/question IDs or provider config, increments
the revision under the Space fence; a semantic no-op preserves it.

Migration maps legacy `none` to all disabled/empty questions, and legacy
`phone`, `captcha`, `manual` to the corresponding single flag. Legacy
`questions` needs a complete machine-checkable answer mapping. Unknown or
malformed values, and questions without configured answers, remain explicitly
invalid/unavailable for admission with preserved repair data. Migrated spaces
reject legacy `entry_requirement`/`entry_questions_json` writes with
`FAILED_PRECONDITION`; unrelated `UpdateSpace` fields continue to work.

#### Additive attempts and immutable submissions

The [Gateway admission routes](api-gateway.md#entry-policy-and-admission-attempts-target)
are the public projection of `SetEntryPolicy`, `StartJoinAttempt`,
`GetJoinAttempt`, `SubmitJoinAttempt`, `ListJoinRequests`, `GetJoinRequest`
and `ApproveJoinRequest`. They are target RPCs, not currently shipped methods.
Applicant exceptions to membership checks do not remove visibility, binding
or lifecycle checks. Moderator reads/approve require current membership and
`SPACE_APPROVE_JOIN_REQUESTS`. Private RPCs bind the exact method/request via
the trusted principal framework; legacy identity metadata is insufficient.

Start creates a server UUID/challenge nonce bound to authenticated account,
profile, Space, admission kind/exact invite, policy revision/config digest.
The collection window is 600 seconds from fresh database creation time;
expiry equality is expired, and retry cannot extend it. This is a collection
deadline, not a pending-review expiry. Reuse the collecting attempt or pending
request for the exact binding rather than permitting unlimited active attempts
through new operation IDs.

Each attempt accepts one immutable canonical submission. Sort answers by
question ID, normalize values and bind their digest to the exact attempt and
token digest. Never persist the raw captcha token. A changed payload for the
same attempt/operation returns `ALREADY_EXISTS`. A wrong answer terminally
fails the attempt with generic `PERMISSION_DENIED`, without per-question
correctness disclosure, pending request, membership, invite use or Role call.
A fresh attempt is allowed; no extra cooldown/lockout is implied.

Validate shape, then local gates, phone, captcha, questions and manual.
Durably claim a captcha token digest for the exact attempt before provider
verification. Concurrent exact retries do not start another verifier call;
provider uncertainty or a crash before a durable positive receipt fails
closed. An already-used provider result is never success. A saved exact pass
receipt permits retry without consuming the token again. Pending stores
normalized submitted answers and immutable pass receipts/digests, never phone
numbers or captcha tokens. The new additive API explicitly permits this
attempt/verification ledger; the old join API still permits only pending
manual review as an intermediate effect.

Old empty-body joins do not select an ambient last proof or unrelated attempt.
They may admit a policy requiring no client-supplied input, or create manual
pending after all automatic stages pass. Captcha/questions requiring client
input return `FAILED_PRECONDITION`; the caller uses the additive attempt API.

#### Freshness, final decision and recovery

Auth-owned verified-phone/account eligibility and Role-owned approver
authorization are mutable decisions. Fetch authenticated owner decisions for
the exact applicant account/profile/Space/attempt/policy at submission and
again at approval as required. A guest with phone required fails before the
phone call. Approval authenticates the current actor/session, checks current
local membership and online `SPACE_APPROVE_JOIN_REQUESTS`. Missing true
verified-phone capability remains unavailable; phone/SMS scope is unchanged.

Request mutable owner decisions immediately before the local final transaction,
using 2-second RPC deadlines. The guarded local decision must occur strictly
within 5 seconds of the earliest required owner-call start, measured with a
local monotonic clock. Recheck after lock acquisition and honor any stricter
owner validity. Equality/exceeded budget returns unavailable with no mutation;
retry fetches fresh decisions. A restart cannot reuse monotonic evidence.
This is a bounded snapshot: remote revocation after an authenticated owner
decision may race admission. Only Space-local deny gates serialize through
the common fence. No global current-at-commit remote guarantee or distributed
2PC is implied.

Captcha action/site/audience/nonce/expiry and one-use checks occur at submit.
A durable authenticated positive observation is a historical fact bound to the
immutable attempt/submission/policy, not a reusable bearer token. Delayed
manual approval uses that exact receipt without re-consuming the provider
token, even after token expiry. Policy/provider-config revision changes
invalidate it. Questions similarly retain exact config/submission/pass
bindings; approval may recompute against that same revision and cannot regrade
against a new policy or override wrong answers.

The collection deadline applies before successful automatic submission. Minimal
admission has no automatic age-based pending-review expiry. Current policy,
bindings, invite state, ban/cap/guest/visibility, identity eligibility and
lifecycle still govern approval; existing Space/account data lifecycle still
applies. Reject/cancel/cooldown and a numeric answer-retention period are
separate product extensions, not inferred requirements for the minimal
questions/manual protocol.

The operation ledger is keyed by authenticated actor profile and UUID
`operation_id`, with immutable request kind, target and canonical payload
digest. Original-actor completed retry returns only its saved outcome, with
no new checks, Role calls, counter/use/event or fresh resource projection.
After leave/ban it cannot recreate membership. The old empty-body join remains
subject to its ordinary code/transport checks. Another approve may return the
stable completed decision only to a currently authorized reviewer.

A new approve reloads the bound pending request, obtains fresh remote mutable
decisions and enters the existing scoped outer transaction for all local final
guards, snapshot budget and single pending-to-admission CAS. Exact submit retry
with a durable successful captcha receipt may retry fresh mutable checks
without provider reuse; it never changes answers/token. Concurrent processing
returns `FAILED_PRECONDITION`. Wrong answers terminally fail; an unknown
provider outcome without a durable positive receipt cannot become a pass.

Approval returns success only after authoritative admission and required Role
completion. Recovery in progress returns `UNAVAILABLE` with a stable operation
ID/status, never a second consumed invite. Preserve a durable outbox/outcome
and recover actual committed state after an ambiguous timeout. Generation and
readiness fences must prevent a late completion from granting after
leave/ban/purge; generic post-commit AssignRole plus retry is insufficient.

Runtime remains open: conservative migration, additive APIs/generated contracts, approval
authorization and transaction, Role completion, Gateway wiring and negative
concurrency/runtime evidence. No admission capability is enabled by this
documentation change.

#### Admission Role readiness and ordinary membership generations

Space stores a decided admission candidate outside `space_members`. A permanent
ordinary-member head allocates monotonically increasing positive generations
per `(space_id,profile_id)`; failures, removal and rejoin never reuse them.
Existing published ordinary members backfill explicit generation zero, which
still requires authoritative Space confirmation. Missing head/unknown source
is not a legacy-zero fallback. Independent community membership has a durable
per-profile episode: positive owner generation, independently monotonic
membership generation and a UUID allocated once per episode. Continuous valid
refresh preserves that token; roster/source revision is a heartbeat, not an
incarnation. Removal, lease equality/gap or owner-generation change invalidates
the old episode before a new higher generation/UUID. Its permanent head/floor
survives roster replacement; cleanup of an old source cannot remove a new one.
Only currently authoritative active legacy rows may initialize an explicit
migrated episode under the Space fence and fresh DB time. Expired/unknown rows
are not active. This does not classify historical ambiguous Role grants:
unproven old provenance remains unresolved rather than guessed as both sources.

`PREPARING_ROLE` is inert technical continuation after automatic/manual
authorization. It is not membership, reserves no capacity/invite use, and is
excluded from every existing ordinary membership reader. A handler-only
`ready=false` row in `space_members` is forbidden because existing Chat/Space
SQL readers would expose access. The candidate commits exact original
operation, account/profile/Space, admission kind/invite, policy/submission
binding digest and allocated generation before any Role call.

Role's protected `EnsureDefaultMemberRole` receives protocol version 1, Space,
profile, generation, stable admission operation and binding SHA-256. Role picks
the allowed current default join role and saves an immutable exact receipt;
callers cannot select role ID, permissions, Owner or a forged human actor.
`READY` means only that Role prepared its own generation-qualified projection.
It remains ineffective without matching published Space membership. Generic
AssignRole with owner metadata is not an admission protocol.

After verifying the exact READY receipt, Space gathers fresh mutable owner
decisions before taking any local mutation lock, then re-enters the shared
Space fence and all current final guards. It CASes the same candidate/head/
generation and commits published membership, member_count, exact invite use,
Role receipt, durable outcome and member-joined event together. Only published
membership is inserted into `space_members`. Final guard failure or removal
supersession invalidates the candidate and durably queues cleanup without a
member/count/use/event success. Unknown COMMIT is resolved from durable state.

Role obtains current eligibility through protected
`GetMemberAdmissionAuthority`, caller `service:role` only, binding exact Space,
profile and optionally a paired expected generation/admission operation.
Response echoes identity, explicit ordinary head/state, published generation/
operation/receipt digest, LIVE/ownership availability and separately typed
community authority. The endpoint reads Space-owned state and never calls
Role. Role queries it before its local transaction; this avoids recursive or
opposite-order locks. PREPARING permits only inert preparation; effective
ordinary rights require matching published generation/operation/receipt and
fresh owner eligibility. Unknown/error is unavailable. This remains a bounded
owner observation under the snapshot limitation above, not distributed ACID.

Leave/kick/ban/purge invalidate pending candidates under the same Space fence,
advance the permanent ordinary generation and persist typed removal work even
if no membership row exists. Account ban invalidates all pending profile aliases
for that account. Role's `RemoveMemberRoles` writes a generation floor even
without existing grants. Floor `R` defeats Ensure at `g<=R` without deleting
newer `g>R` or independently sourced community contributions. System baseline
Member cleanup is a separate typed source operation; Owner is exclusively
bootstrap/ownership/P3 authority. Physical cleanup may lag, but new effective
rights cannot rely on an orphaned ordinary contribution after Space removal.
Rejoin cannot inherit old-generation Admin/custom grants. P3 retirement
permanently dominates every ordinary generation and late retry.

Role effective role/permission/hierarchy reads and generic user/bot mutations
must qualify ordinary contributions against this live incarnation; actor and
target checks apply where required. Raw authority-source exports, policy epochs
and downstream consumers must not export an inert contribution as an active
grant. Space's own authority revision/catalog and cleanup include new head,
candidate, receipt and outbox data. Existing published-only readers, Chat's
legacy direct readers and community unions must be tested with candidate plus
published fixtures; do not introduce new cross-database reads.

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

`GetAuditLog` читает только строки запрошенного `space_id` и возвращает все поля `AuditLogEntry`. Ошибка Role Service закрывает доступ (`UNAVAILABLE`), явный deny даёт `PERMISSION_DENIED`, malformed cursor — `INVALID_ARGUMENT`. Handler передаёт actor/action/time filters в filter-bound store query; каждый cursor связывает authenticated viewer, scope, filters и page size. Остальные writers и клиентские REST/Flutter поверхности остаются в [backend backlog](../todo/backend.md).

The audit read chain consumes `pkg/principal.RequireDelegatedUser` from the
already verified Gateway delegated context. The shared reader binds canonical
non-nil account/profile UUIDs, positive session epoch and exact audience, RPC and
protobuf request hash; it never derives an actor from raw identity metadata.
For this method the protected lifecycle interceptor preserves the signed
transport metadata without manufacturing `x-voice-*` headers. The public
disclosure wrapper, membership checks, explicit profile permission/owner cores
and cursor viewer all use that typed profile. The ordinary listener still
rejects audit reads, and missing protected transport has no ordinary fallback.
Other lifecycle methods retain their explicitly unmigrated verified-first
projection. The current ordinary Space-to-Role decision transport is a separate
BE-181 residual; this cutover does not claim its S2S authorization or production
PKI rollout. See the [BE-041 execution plan](../testing/be041-verified-audit-identity-exec-plan.md).

**Invite permissions:** shipped `CreateInvite`, `RevokeInvite` and `ListInvites`
handlers use the exact `SPACE_MANAGE_INVITES` decision. An unavailable configured
Role Service fails closed. While Role is intentionally unwired, the shared
`requireSpacePermission` compatibility path permits only the current Space owner;
the A2 target does not retain this fallback after signed Role integration is
required.

<a id="phase-0-space-audit-ledger-target-not-implemented"></a>
<a id="phase-0-space-audit-ledger-accepted-target-runtime-not-implemented"></a>

## Phase-0 Space audit ledger (accepted target; bounded core implemented)

Space owns one immutable audit registry for administrative mutations. The wire
`AuditLogEntry.id`, ingestion `audit_event_id` and `audit_log.id` are the same UUID.
`action` remains an exact lower `snake_case` string so additive writers do not
break protobuf or JSON consumers. Unknown action filters are valid and return an
empty result rather than validation failure.

### Shipped bounded Space-owned core

BE-116 ships migration `000015_audit_ledger`: `audit_log` is append-only outside
the exact capability-bound legacy ownership compensation or delivered-row
365-day retention predicates; canonical JSONB `details` is capped at 4096 bytes;
every inserted row creates an `audit_outbox` row with the identical
`audit_event_id` in the same transaction. Outbox claim/retry/lease/ack retains
that ID as the consumer dedupe key. Compensation serializes on that exact outbox
row and, after any claim wait, rechecks the capability, current Space owner and
pristine outbox state under row locks before its delete can proceed. Cleanup uses
database time, removes only delivered effects older than 365 days, and is exposed
through the bounded `internal/auditretention` worker.

BE-116 deliberately has no aggregate audit purge authority. Migration
`000015_audit_ledger` blocks direct `spaces` deletion before its foreign-key
cascades can erase audit/outbox evidence, so the obsolete legacy `DeleteSpace`
RPC now fails closed. BE-005 must supply the documented terminal same-transaction
P3 purge coordinator before that guard can be replaced. The public gRPC status
mapping for this temporarily disabled legacy RPC is still a contract gap; BE-116
does not invent one.

### Closed action, writer and target registry v1

| Mutation owner | Allowed actions | Canonical target | Runtime |
|---|---|---|---|
| Space | `space_created`, `space_updated` | `space`; the same `space_id` | pending |
| Space | `ownership_transferred` | `profile`; the new owner profile | shipped |
| Space | `invite_created` | `invite`; the invite ID | pending |
| Space | `invite_revoked` | `invite`; the invite ID | shipped |
| Space | `member_kicked`, `member_timed_out`, `member_timeout_removed` | `profile`; the affected profile | shipped |
| Space | `member_banned`, `member_unbanned` | `account`; the affected account | shipped |
| Space | `category_created`, `category_updated`, `category_deleted` | `category`; the category ID | pending |
| Space | `voice_room_created`, `voice_room_updated`, `voice_room_deleted` | `voice_room`; the room ID | pending |
| Space | `tree_node_upserted`, `tree_node_removed` | `tree_node`; the node ID | shipped |
| Space | `tree_reordered` | `space`; the same `space_id` | shipped |
| Role | `role_created`, `role_updated`, `role_deleted`, `default_join_role_set` | `role`; the role ID | pending protected producer |
| Role | `roles_reordered` | `space`; the same `space_id` | pending protected producer |
| Role | `role_assigned`, `role_revoked` | `profile`; the affected profile; details bind the role | pending protected producer |
| Role | `chat_override_set`, `chat_override_removed` | `chat`; the affected Space-attached chat | pending protected producer |
| Role | `voice_room_override_set`, `voice_room_override_removed` | `voice_room`; the affected room | pending protected producer |
| Chat | `chat_created`, `chat_updated`, `chat_deleted` | `chat`; the Space-attached chat ID | pending protected producer |

Join/leave are ordinary self-service membership events and are outside audit v1.
Space delete/restore/purge facts belong to BE-245. Bot mass cleanup and
`DeleteRolesCreatedByProfile` are not implicit audit actions; adding them needs a
separate product decision. Gateway never authors an audit event.

Space-local writers commit the domain mutation, audit row and durable audit
outbox row in one `space_db` transaction. Failed/no-op mutations produce no new
audit effect. Role and Chat must commit the domain mutation plus a durable
producer outbox in their own database transaction, then deliver the fact through
protected `AppendAuditEvent(AppendAuditEventRequest)`. Composite `POST S/chats`
produces one `chat_created` only after the complete Chat+tree outcome; its internal
`UpsertTreeNode` must not create a duplicate user-visible effect.

`AppendAuditEvent` accepts only signed `service:role` or `service:chat` principals
in the single Bearer metadata credential defined by
[ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md). The principal
identifies the producer and binds the exact full RPC, `x-request-id` and SHA-256
of deterministic serialization of all request fields; unknown request fields are
rejected before handler execution. `actor_profile_id` is trusted decision data in
the request body, not a service-principal actor claim. Role/Chat derive it from
the original verified user or bot-actor context at mutation time and persist it
atomically with the outbox. Space validates its UUID and caller/action ownership,
must consume the hash-bound body value, and does not compare it to a principal
actor. No caller/source-service identity appears in the request body.

The ingestion implementation is opt-in on its own mTLS listener. Presence of any
`SPACE_AUDIT_PRINCIPAL_*` setting enables validation; partial or blank required
configuration fails startup. Required settings are `SPACE_AUDIT_PRINCIPAL_TLS_CERT_FILE`,
`SPACE_AUDIT_PRINCIPAL_TLS_KEY_FILE`, `SPACE_AUDIT_PRINCIPAL_CLIENT_CA_FILE` and
`SPACE_AUDIT_PRINCIPAL_REPLAY_REDIS_ADDR`; the optional Redis password is
`SPACE_AUDIT_PRINCIPAL_REPLAY_REDIS_PASSWORD`. `SPACE_AUDIT_PRINCIPAL_GRPC_LISTEN`
defaults to `:9098`. `DATABASE_URL` is required. This listener registers only
`AppendAuditEvent`; the ordinary listener explicitly denies that RPC before legacy
metadata handling. There is no raw actor, shared token or insecure fallback.

`S2S_JWKS_URLS_JSON` must contain exact HTTPS endpoints for `role` and `chat`;
other shared-map issuers confer no audit authority. Each trusted JWKS contains
distinct current+next RS256 keys of at least 2048 bits. Optional `S2S_JWKS_CA_FILE`
and paired `S2S_JWKS_CLIENT_CERT_FILE`/`S2S_JWKS_CLIENT_KEY_FILE` configure HTTPS
trust/client identity. Redirects and oversized JWKS responses are rejected.
`S2S_JWKS_REFRESH_AFTER`/`S2S_JWKS_HARD_EXPIRY`/`S2S_UNKNOWN_KID_COOLDOWN`
default to `30s`/`2m`/`5s`; invalid values fail startup. Redis is required at
startup and each credential's JTI is consumed atomically until expiry plus skew;
Redis failure or an unusable JWKS fails closed. These settings describe source
capability and do not establish deployment or completed PostgreSQL verification.
The accepted composite freeze/purge guard from BE-A2-COMPOSITE-CHAT-NODE c4/c5
checks its persisted COMMITTED audit/Space/Chat/creator binding under the shared
Space lifecycle transaction lock. A permanent PURGED fence rejects late delivery
before even an existing receipt can acknowledge success. Current interactive ACL
is not rerun for an already committed fact. The same guard runs after a uniqueness
collision rollback in the fresh receipt transaction. Real PostgreSQL concurrency
proof and independent review remain delivery gates; this does not activate P3.

Exact replay of the same `audit_event_id` and canonical payload returns the empty
success response; the same ID with any changed field is `ALREADY_EXISTS`.
Space-local writers never call this RPC, and Role/Chat never write directly to
`space_db`. Every request field is semantically required: UUID fields and strings
are nonempty, `details_json` is a nonempty canonical JSON object string including
`{}`, and `occurred_at` is present and a valid Timestamp. Space persists
`occurred_at` as the entry `created_at`. The request uses ordinary proto3 strings
so empty values fail validation rather than carrying presence semantics. The
empty response accepts and preserves unknown fields after known validation for
forward-compatible acknowledgement handling.

### Details registry v1

BE-234 adds the Space-owned `member_nickname_updated` action: target_type is
`profile`, target_id is the edited local member, and details is exactly
`{"changed_fields":["nickname"]}`. Neither nickname value is retained. The
nickname mutation and audit/outbox commit atomically; unchanged canonical values
produce no new fact. Migration `000026_member_nickname_audit` extends the closed
registry additively; downgrade refuses while nickname audit evidence exists.

### Member nickname RPC (BE-234-nickname-v2-transport)

`UpdateMemberNicknameRequest` has mandatory UUID `space_id` and target
`profile_id`, plus required presence of optional string `nickname`. Missing
nickname is `INVALID_ARGUMENT`; explicitly empty/Unicode White_Space-only clears
SQL NULL and the response omits nickname. The response contains authoritative
`SpaceMembership`; `joined_at`, member_count and global profile data do not change.
See [Spaces](../features/spaces.md#ник-участника-в-спейсе) for the exact 64-scalar
Unicode validation contract.

Only a verified active-profile actor may invoke the RPC. Current membership and
effective global `MEMBER_MANAGE_NICKNAMES` are required even for self; no node
override, omitted target or legacy actor metadata supplies authority. Non-owner
editing Owner is `FAILED_PRECONDITION`; other target ranks do not add a hierarchy
gate. Missing local target is `NOT_FOUND` only after caller authorization;
visible roster-only targets return `FAILED_PRECONDITION` without creating a row.
Ownership/deletion freeze is checked under the same PostgreSQL Space lock as
leave/kick and the nickname update, including same-value/clear requests. No-op
requests recheck authorization. The RPC emits no new domain/Realtime event and
adds no HTTP/UI route.

The nickname RPC is added to the existing Gateway delegated-user mTLS listener
and denied by the ordinary listener. The handler requires verified `gateway`
delegated-user principal, audience `space`, exact RPC/hash and canonical
account/active-profile UUIDs; metadata alone never supplies an actor. Existing
JWT/replay/session-epoch checks remain required. Its permission request uses the
existing protected Role connection (`OwnershipRoles`) and Space signer, with a
fresh service JWT and exact `MEMBER_MANAGE_NICKNAMES` request for the actor;
outgoing metadata contains only credential/request binding. Owner/Admin and
no-op requests also require that signed decision; missing Role connection/signer
or unavailable dependency returns `UNAVAILABLE`, never the old owner fallback.
Other existing Space and Role permission transports are unchanged. The broader
signed Space CheckPermission allowlist remains target-only outside this branch.

`target_type` is one of `space`, `invite`, `profile`, `account`, `role`, `chat`,
`voice_room`, `category`, `tree_node`; `target_id` is always a UUID. Aggregate
actions such as `space_updated`, `roles_reordered` and `tree_reordered` target the
Space itself. Server-owned typed builders enforce this allowlist:

| Action family | Allowed detail keys |
|---|---|
| update actions | `changed_fields` |
| `invite_created` | optional `max_uses`, optional `expires_at`; never the invite code |
| ban/timeout | optional normalized `reason`; timeout also has required `duration_seconds` |
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

### External audit details codec v1

This accepted wire codec applies only to the closed Role/Chat actions above;
Space-local builders and their stored JSONB are not reinterpreted. Producers and
ingress share `src/backend/pkg/spaceaudit`. Producer outboxes preserve the built
bytes; retries never rebuild them with a later codec.

The object has ASCII keys in ascending order, no whitespace/BOM/newline, escapes,
duplicate members or trailing value. Strings are unescaped ASCII tokens or
canonical lowercase hyphenated UUIDs. No null, free-form text, nested objects or
arrays except `changed_fields` are accepted. Integers are unquoted decimal
`0` or `[1-9][0-9]*`, exactly parsed as uint64; signs, leading zeroes, fractions,
exponents and overflow are invalid. Ingress emits the action-specific canonical
form and compares it with the original bytes; it rejects differences rather than
normalizing the request. JSONB normalization and the 4096-byte database check are
separate. Invalid details reserve no ID, including when an existing ID is replayed.

| Actions | Exact required details |
|---|---|
| `role_created`, `role_deleted`, `default_join_role_set`, `chat_created`, `chat_deleted` | `{}` |
| `role_updated`, `chat_updated` | `changed_fields` |
| `role_assigned`, `role_revoked` | `role_id` |
| `chat_override_set`, `chat_override_removed`, `voice_room_override_set`, `voice_room_override_removed` | `allow_mask`, `deny_mask`, `role_id` |
| `roles_reordered` | `count` |

No other or optional members are allowed in this external subset. `role_id` has no
UUID-version restriction. Both masks use the entire uint64 range, including high
and unknown future bits, zero and overlapping allow/deny bits; ingress records
facts rather than applying Role policy. Removed overrides record the deleted
row's prior masks captured in the mutation/outbox transaction. A missing row is a
no-op and emits no fact.

`changed_fields` is nonempty, unique and ASCII-sorted. Role tokens are `name`,
`permissions`, `position`; request `permissions_mask` maps to domain token
`permissions`. Chat tokens are `allow_guests`, `allow_user_main_feed`,
`avatar_url`, `name`, `slow_mode_seconds`, `threads_enabled`, `topic`. Tokens
describe actual normalized before/after value differences, not request presence;
unchanged updates emit no fact. No updated values or URLs are included. Future
fields need an explicit registry extension. `count` is a positive uint64 counting
distinct role IDs in the successfully committed reorder batch, including listed
roles retaining their position. Only an actual order change emits a fact. This
codec bound introduces no new domain quota.

### Durable external replay codec v1

The signed deterministic-protobuf request hash remains transport authentication.
Durable replay uses a separate frozen frame after authentication, recursive
unknown-field rejection and registry/field/details validation, before UUID/string,
JSONB or SQL timestamp conversion. The frame starts with ASCII domain tag
`voice.space.audit.append.v1`, then these strings in order: `audit_event_id`,
`space_id`, `actor_profile_id`, `action`, `target_type`, `target_id`,
`details_json`. Each string has a uint32 big-endian UTF-8 byte length followed by
its original validated bytes. Timestamp seconds follow as signed int64 big-endian
two's-complement, then nanos as uint32 big-endian. Missing Timestamp is invalid;
present epoch is valid. Valid protobuf dates and nanoseconds are accepted; never
convert to UnixNano. Full seconds/nanos survive in replay identity even though
the query projection remains a microsecond PostgreSQL timestamp.

SHA-256 of this frame is fingerprint version 1. The trusted `role`/`chat` origin
is compared separately. Credential, JTI, request ID and transport metadata are
excluded; retries use fresh credentials. Accepted alternate outer UUID spellings
remain different body bytes. New semantic fields require an explicit codec
version/compatibility decision. Fingerprint equality assumes SHA-256 collision
resistance; it does not claim indefinite lossless payload retention.

One immutable Space-owned ID registry is keyed globally by `audit_event_id`, not
by Space or producer. It stores Space ID, `local`/`role`/`chat` origin and external
fingerprint version/digest without actor/target/details/time payload copies.
Every local audit INSERT reserves this same namespace. Migration excludes
concurrent insertion/retention and backfills surviving old IDs as local markers;
already-deleted historical IDs cannot be reconstructed.

Audit INSERT, same-ID outbox and registry reservation commit in one transaction;
there is no committed pending receipt. External exact origin/version/fingerprint
replay succeeds without inserting even after payload retention. A different
valid body, producer, Space or local-origin collision is `ALREADY_EXISTS` and
never overwrites evidence. Uniqueness conflicts roll back the attempted
transaction before reading the committed registry in a fresh transaction.
Validation/authentication is repeated before replay lookup; invalid changed
details are `INVALID_ARGUMENT`, not replay success/conflict.

Registry entries have no ordinary TTL or audit/outbox cascade. Delivered-only
365-day retention removes payload/outbox, preserving receipts so late retries
cannot resurrect effects. Existing capability-bound compensation may remove its
pristine local audit/outbox but leaves its non-payload ID fence.

Under accepted composite terminal minimization, migration
`000029_audit_receipt_minimization` allows only the exact external fingerprint
erasure at database time `>= space_permanent_purge_fences.purged_at + 30 days`.
It clears version/digest, retaining the global ID/Space/origin nonpayload marker;
no actor/content-derived hash remains permanently. The database trigger refuses
early erasure, marker insertion, key/origin changes, restoration and deletion.
Cleanup is bounded to 1000 due rows per transaction with `SKIP LOCKED`; retries
never move the fixed deadline. A failed enclosing transaction restores all
pre-state, and migration rollback refuses to fabricate already-erased hashes.
The permanent Space fence already denies late ingestion before any receipt read,
including while physical cleanup is delayed; a marker never implies historical
success or permits resurrection. BE-005 owns terminal coordinator integration;
source capability does not activate P3, and existing hard-delete guards remain.

### Query, cursor and retention v1

`GetAuditLogRequest` now exposes optional `actor_profile_id` UUID, optional exact
nonempty `action`, inclusive `from` and exclusive `to`. Present-empty scalar
filters are `INVALID_ARGUMENT`; absent means no filter. Timestamps are valid UTC
instants and, when both are present, require `from < to`. Page size is 1..100,
default 50. Results are ordered `created_at DESC,id DESC`.

The gRPC read uses a 15-minute HMAC cursor bound to `space_id`, authenticated
viewer, access scope, normalized filters and page size, with a first-page tuple
ceiling; all replicas use the database-owned signing key and recheck ACL on every
page. `kid` is the base64url SHA-256 identifier of that singleton key; replacing
the key invalidates old cursors. Issue time and expiry stay fixed across pages.

The complete cursor v1 binds `kid`, version, `space_id`, authenticated
`viewer_profile_id`, scope `space.audit.read.v1`, normalized actor/action/from/to,
exact page size, first-page ceiling, last returned tuple, `issued_at` and
`expires_at`. UUIDs normalize to lowercase, timestamps to UTC RFC3339Nano, and
absent remains distinct from present-empty. Lifetime is 15 minutes from the first
page. Query rows are at or below the ceiling and strictly below the last tuple in
descending keyset order. Changed filters/page size, foreign Space/viewer/scope,
bad MAC, unknown key or expiry are `INVALID_ARGUMENT`; later access loss remains
the corresponding ACL denial. No legacy unsigned cursor is accepted.

While a Space is live, ordinary audit rows retain 365 days and only the shipped
Space-owned retention worker removes delivered expired rows. Accepted P3 purge is
the other allowed deletion path: ordinary rows are removed after mandatory
lifecycle facts are reduced to the sole minimal tombstone, retained until
`purged_at + 365 days`. Terminal purge is tracked by BE-005 and its Chat
continuations BE-006/BE-007; this ingestion change does not activate that path.


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
├── nickname (nullable, space-specific; UpdateMemberNickname, BE-234-nickname-v1)
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

Accepted target `BE038-ORDINARY-v1`, not a shipped guarantee: ordinary mutations
commit domain data and immutable deterministic event bytes/ID/subject in one
`space_db` transaction. Success records a durable publication obligation without
waiting for NATS. Fenced expiring claims retry indefinitely; only a matching
`chat_events` PubAck marks delivery. No global/per-Space ordering is promised:
consumers refresh current authority rather than replay state deltas. The exact
coverage, failure/recovery, audience, P3 cleanup and T1–T12 acceptance rules are
in [ordinary event delivery](../testing/space-ordinary-events-exec-plan.md).
Audit, ownership, media invalidation and P3 delivery remain distinct.

| Событие                 | Данные                          |
|-------------------------|---------------------------------|
| `space.created`         | space_id, owner_profile_id      |
| `space.updated`         | space_id, changed_fields        |
| `space.deletion_scheduled` | space_id, owner_id, purge_after |
| `space.restored`        | space_id, owner_id              |
| `space.deleted`         | space_id (only after converged irreversible purge) |
| `space.member_joined`   | space_id, profile_id            |
| `space.member_left`     | space_id, profile_id            |
| `space.member_banned`   | target additive payload: space_id, account_id, banned_by_profile_id |
| `voice.room_created`         | reserved; not currently emitted |
| `voice.room_deleted`         | reserved; not currently emitted |
| `space.tree_changed` (`change=upserted`) | space_id, node_id, kind, chat_id?, voice_room_id?, **is_pinned**, **pin_order** |
| `space.tree_changed` (`change=removed`) | space_id, node_id |
| `space.invite_created`  | space_id, invite_code           |

`SpaceUpdated.changed_fields` is an additive target. Empty legacy or unknown
keys mean full invalidation; ownership's already saved bytes are not rewritten.
Tree rows above retain the existing wire subject, not new dual-published
`space.tree_node_*` subjects. Invite codes and ban identities are privileged
internal data: no generic Space WS/push broadcast is authorized.

The separate `space.voice_room_access_invalidated` subject carries an
immutable `ChatStreamEvent` with `event_id`, `space_id`, `access_epoch` and
optional room/profile hints. Space writes its snapshot to its transactional
outbox with the authority mutation. Voice's durable `voice_space_media_chat`
consumer ACKs after current Space/Role authority is reconciled across the
active Space room index; optional hints never narrow that full pass. This typed
arm does not change the legacy Chat stream envelope or `space.updated` payload.

## BE-065 Role override target validation (accepted target)

Add `/voice.space.v1.SpaceService/ValidateRoleOverrideTarget` with package-local
`ValidateRoleOverrideTargetRequest { string space_id = 1; optional string voice_room_id = 2; }`
and empty `ValidateRoleOverrideTargetResponse`. The source implementation uses
a dedicated protected listener; runtime acceptance is separate. Configuration
is documented with the complete caller/error/consistency contract in
[Role's BE-065 contract](role-service.md#be-065-protected-override-target-validation-accepted-target);
the [execution plan](../testing/role-override-target-validation-exec-plan.md)
defines required verification.

Only a verified Role workload (`principal_type=service`, `iss=role`,
`sub=service:role`, `aud=space`, exact full RPC and deterministic whole-body hash,
including optional presence) may call. Use protected TLS/mTLS, Role JWKS/replay,
fresh JTI and canonical signed request ID matching the sole `x-request-id`, with
the standard Phase-0 <=30 s lifetime. Raw identity metadata, duplicates,
delegated-user/end-user/bot and all other services reject before store. The
handler independently requires verified Role context. Do not widen existing
Social-only or Gateway lifecycle listeners; ordinary/public listeners return
`UNAVAILABLE` before domain access. No public Gateway route is added.

Under one consistent local store/lifecycle serialization boundary, an absent
room field validates the live Space only. A present room must be a canonical
non-nil UUID (present-empty is `INVALID_ARGUMENT`) resolving to a live
`voice_rooms` row in that exact Space. Space ID also must be canonical/non-nil.
Missing Space or absent/deleted/foreign room is indistinguishable `NOT_FOUND`.
All known non-live deletion/recovery states, active ownership journal and
terminal lifecycle fences deny with `UNAVAILABLE`; DB/state failure never means
LIVE. Missing legacy fence follows existing canonical admission semantics.
Do not return owner/member projection or call Role/ACL-enriched provider APIs.

This result is a fresh point-in-time existence check, not a distributed foreign
key or reservation. Deletion after owner read may leave an inert Role reference;
actual operations still enforce current existence/lifecycle. IDs cannot be reused
or reparented, and a permitted LIVE restoration requires a new validation attempt.
Role override removal remains local cleanup even for an absent target.

## Зависимости

- **Chat Service** — создание/удаление строки текстового чата (`chats`, `group` \| `channel`); Space ведёт **`space_tree_nodes`** (`kind = text_chat`)
- **Role Service** — проверка прав при операциях (в т.ч. `chat_overrides` по `chat_id` и `voice_room_overrides`)
- **Subscription Service** — лимиты узлов дерева (текст + голос) и участников (free vs Pro)
- **User Service** — профили участников
- **Social Service** — проверка блокировок при join

### Space Pro failed-payment projection (A7 accepted target; not implemented)

Space consumes the complete revisioned `subscription.entitlement_changed`
snapshot into a durable inbox/projection in `space_db`. `ACTIVE` and
`GRACE_PERIOD` both grant Pro caps; payment recovery advances the revision and
cancels remaining Subscription-owned reminders. Only `INACTIVE` removes Pro
growth entitlement. Existing members, tree nodes and active voice participants
are not deleted or kicked; new joins/creates/admission are denied while their
count is at or above the corresponding free cap.

The transitional synchronous `SyncSpaceProSubscription` path may coexist during
cutover only after it carries the identical source revision and snapshot. It
must not race an older unversioned status over the durable event projection.
Restart, replay, stale delivery and seven-day grace evidence is defined in
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

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

Full deletion activation must preflight the complete participant capability
manifest before `SCHEDULE_PENDING` or Auth consume. Voice currently declares
`EJECT_ONLY`; see [Voice P3](voice-service.md#p3-space-lifecycle-participant-target).
Its restrictive native barrier/exact drain is local progress, not a completed
freeze receipt: self-hosted original/refreshed credentials lack durable provider
retirement proof. Keep Voice outstanding and do not start the seven-day schedule,
restore LIVE or bypass revocation through purge. A capability downgrade preserves
already committed restrictive intent and exact retry targets; no compensating
LIVE or weaker success grade is authorized. A future provider path requires
operation-bound coverage/retirement proof and qualified current/historical issuer
coverage; a boolean setting or empty roster does not satisfy this prerequisite.

Space owns the aggregate and durable participant ledger. States are `LIVE`,
`SCHEDULE_PENDING`, `FREEZE_PENDING`, `SCHEDULED`, `RESTORE_DECIDED`,
`PURGE_DECIDED`, `PURGING`, `PURGED`; generations are positive and monotonic.
The release manifest fixes eleven participants: Role, Chat, Messaging, File, Voice,
Matchmaking, Search, Subscription, Bot, Notification and Realtime. Service absence never
removes a participant: an empty result still has an immutable receipt.

Before Auth consume Space commits the immutable operation binding and
`SCHEDULE_PENDING`. After an exact receipt it enters `FREEZE_PENDING`, where
Chat first fences and captures the immutable sorted chat manifest; Messaging
imports it; Notification and Realtime import every exact Chat page; File preliminarily fences and seals exact `SPACE`, `CHAT` and
`MESSAGING` reference declarations/pages; then all eleven participants acknowledge
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
the tombstone and ready `space.deleted` outbox row commit only after all eleven
validated completion receipts. File receipt proves durable reference/GC
handoff, not physical R2 absence.

Each attempt uses exact stored bytes, a fresh 10-second RPC deadline and
unbounded exponential retry from one second to a five-minute cap with bounded
jitter. Fifteen minutes without progress alerts but never skips a participant.
The ledger uses `NOT_STARTED`, `IN_FLIGHT`, `COMPLETE`, `RETRYABLE_FAILURE`,
`CONTRACT_MISMATCH`; only an accepted receipt may set `COMPLETE`.

The store boundary applies each participant receipt under the shared Space lock:
load the current aggregate, validate the receipt, and persist evidence in one
transaction. Concurrent participant responses cannot replace each other's
progress. Exact receipt replay remains valid after the aggregate advances; a
changed receipt for an already completed participant conflicts. Restore completion
uses fresh database time under that same lock and commits `LIVE` with its stable
`space.restored` outbox row only after the eleven-participant barrier. These durable
transitions do not themselves authenticate network callers or activate the public
coordinator. Schedule reservation replay validates the original immutable binding
and loads saved progress without resetting generation or repeating current-owner
and current-name preconditions.

The saved DELETE result is the deterministic `voice.space.v1.DeleteSpaceResponse`
projection (the empty response mapped to HTTP `204`), not the current lifecycle
aggregate. Its operation becomes `COMPLETED` atomically with `SCHEDULED` and the
ready schedule event, with `completed_at = scheduled_at`. Exact response bytes
and the message-domain SHA-256 are immutable; replay cannot restart the 30-day
window. Only the locked `CompleteLifecycleSchedule` barrier may create the first
result; generic snapshot persistence cannot promote an admitted operation. A
store replay lookup validates the original account/profile/session
epoch and complete request binding before returning this result, independently
of current owner, name or retained aggregate rows. Missing or pending outcomes
never mean success; database time at or beyond `completed_at + 30 days` expires
replay. Expiry does not remove coordinator evidence needed for convergence.
Restore admission uses a distinct `RESTORE` operation ID in the shared operation
ledger; it never replaces the aggregate's original deletion operation ID. The
immutable binding covers the authenticated account/profile/session epoch and
the complete deterministic `RestoreSpaceRequest`. It needs no factor receipt.
New admission checks the current recorded owner under the shared Space lock,
then samples PostgreSQL time and commits the next recovery generation. At the
deadline it commits `PURGE_DECIDED` and fails restore; a new ID cannot join or
replace an already decided recovery operation. Exact pending retry resumes its
saved deletion ID and generation without re-running current-owner preconditions.

Only the locked ten-participant restore completion can mark that operation
`COMPLETED`. It saves the deterministic `RestoreSpaceResponse` containing the
Space projection read in the same transaction as `LIVE` and the ready restored
event, with `completed_at` equal to that event's database timestamp. Retry returns
these exact saved fields, not a later renamed or transferred Space. It validates
the original principal and full request binding before any disclosure, even if
the aggregate/Space has since been removed. Pending or missing records never
mean success; replay expires at database time `completed_at + 30 days` without
extending retention. Generic snapshot persistence cannot mint this outcome.
The public coordinator and retention cleanup worker remain unimplemented.

Ordinary store access uses the same Space lock and denies every persisted
deletion phase except `LIVE`, including pending schedule and partial restore.
`ListMySpacesPage` excludes these rows before pagination; implicit co-membership
counts only shared live Spaces. An explicit decision scope containing a frozen
Space fails closed. Missing lifecycle authority is unavailable, never an absent
fence interpreted as permission. New ownership and deletion reservations exclude
each other under the shared lock; exact saved operation replay remains available.
This is a store foundation: the public recorded-owner recovery projection and
its Gateway disclosure/status mapping remain a separate A2 vertical. It does not
activate deletion or authorize participant cleanup.

The source-disabled `internal/recoveryread` adapter prepares that read boundary:
it verifies a Gateway delegated principal for the exact `GetSpace` request, with
mandatory key, replay and live session-epoch dependencies. The store samples
recorded ownership and lifecycle state under the same Space advisory lock used
by restore/purge, and returns only `id`, `name`, `deletion_scheduled_at` and
`purge_after`. Pending schedule phases have no invented deadline; an incomplete
restore keeps the restricted projection. Non-owners, missing Spaces and `PURGED`
rows are undiscoverable. A live Space must use the ordinary authorized read,
and unavailable lifecycle authority never grants access. The adapter has no
production handler or environment wiring. Gateway/Flutter activation and the
ten-participant coordinator remain open.

The common protected wire uses `protocol_version=1`. A lifecycle-fence request
contains canonical `space_id`, `deletion_operation_id`, positive `generation`,
desired `FROZEN`/`LIVE`/`PURGE_DECIDED` and `ManifestBinding(manifest_id,
manifest_sha256,item_count)`. Its receipt adds stable `receipt_id`, fixed
`participant_id`, applied state, request/manifest hashes and database
`applied_at`. A purge request replaces desired state with `purge_decided_at`;
its receipt accepts only `COMPLETED` and records `completed_at`. Participant IDs
are Role=1, Chat=2, Messaging=3, File=4, Voice=5, Matchmaking=6, Search=7,
Subscription=8, Bot=9, Notification=10, Realtime=11; zero/unknown IDs or states are invalid.
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

### Source catalog migration qualification

`SPACE-SOURCE-CATALOG-34-v1` qualifies the protected Space authority-source
projection at exactly migration cuts 25 and 34. Every preflight and read still
checks the exact current clean migration marker, source relation shape, original
revision triggers and function-body hashes inside the same schema-locked read
snapshot. Intermediate cuts 26–31, 32/33 and unknown future cuts are not implicitly
qualified; dirty state fails closed. The tree-viewer qualification is separate.

Compared with 25, migration 26 adds the nickname audit registry; 27 adds audit
ingress registry/receipts; 28 adds private composite creation evidence/fences;
29 bounds terminal audit fingerprints; 30 adds entry-policy configuration and
operation journals; 31 serializes audit/composite IDs; 34 extends lifecycle
participant/completion constraints and terminal readiness to Realtime 11. These
changes preserve the projected membership/tree/ban/timeout/lifecycle fields,
retained source floors and original revision coverage. Private audit, policy and
operation records are not additional permission grants in this projection.
Qualification uses the real full migrator and checks ordinary/community lease
folds, revision changes, dirty/future cuts, disabled revision triggers and
changed function bodies. It does not qualify native membership 33, public P3
activation, deployment or other authority producers.

### Realtime participant11 admission boundary

BE-REALTIME-P3-DURABLE-ADMISSION-v1 adds Realtime without renumbering IDs1–10.
Migration `000034_realtime_lifecycle_participant` widens the durable participant
and compact completion constraints and terminal hard-delete predicate to11.
Ten historical completions cannot schedule, start destructive work or authorize
local hard deletion. Realtime manifest-import receipts are not strong FROZEN
receipts: the latter require exact sealed coverage and no unresolved durable
admissions across process incarnations. Dead-incarnation recovery remains
unsupported without qualified termination evidence; timeout never skips11.
This extension neither activates public P3 nor establishes old-writer coverage.
