# Voice Service

## Space communication timeout (BE-044-TIMEOUT-v1)

Accepted target: the [Space timeout matrix](../features/spaces.md#communication-timeout-be-044-timeout-v1-accepted-target)
denies fresh Space join/token/reissue and self/moderator move into a room for
the timed-out target. Room access must include fresh versioned communication
evidence from the same Space snapshot as membership/active/access_epoch. Missing,
legacy/default/malformed capability is Unavailable, never an implicit CLEAR.
Covered access uses signed protected service authority without a static-bearer
fallback. Role/Admin cannot override a timeout; no consumer-local expiry/cache
may grant admission. Leave/disconnect/cleanup remain allowed under their ACLs.

Extend the enabled access invalidation consumer: reread current whole-Space
membership, timeout and Role authority, treating event hints only as hints.
TIMED_OUT ejects the exact current media incarnation, including listen-only;
unknown authority retains the event unacked and denies new grants. Duplicate or
reordered events cannot restore an older state. Set/replace/remove use Space's
existing epoch/outbox trigger, not a second increment. Removal/expiry allows a
fresh authorized join, never automatic rejoin. Preserve the existing ejection
target (2s p95/5s max) and honest <=60s stale bearer replay limitation, with
reconcile/re-eject on stale reconnect. Do not activate R22.3/D2/D3 foundations.
Mock ejection alone does not prove the required actual two-client media stop.

Voice uses the full `VOICE_SPACE_COMMUNICATION_` client tuple (`GRPC_ADDR`,
`TLS_CA_FILE`, `CLIENT_CERT_FILE`, `CLIENT_KEY_FILE`) and its existing Voice
service signer/protected JWKS publisher. The ordinary `SPACE_GRPC_ADDR` remains
for separately authorized membership/privacy contracts; it never supplies a
communication grant. Legacy static bearer configuration is not a fallback.
Absent capability denies covered admission; partial or explicitly empty client
configuration fails startup. Fresh decision deadlines cover subsequent Role,
admission transaction and token response work. Existing source-disabled Space
move paths remain disabled; timeout enforcement does not enable transfers.

## Обзор

Оркестрация голосовых/видео-звонков и screen share через LiveKit SFU. Сам сервис не обрабатывает медиа-потоки.

**Язык**: Go
**Хранилище**: PostgreSQL (`voice_db`) — durable source of truth для room lifecycle; Redis — active-call compatibility projection и rebuildable admission/receipt mirror; LiveKit — SFU

В срезе R22.3 lifecycle остаётся **source-disabled**: Voice открывает и проверяет
`voice_db`, включая durable Redis-divergence evidence, но coordinator, lifecycle handlers, Redis bridge и external-effects
workers ещё не зарегистрированы. Отсутствующий `VOICE_DATABASE_URL` сохраняет
этот режим; заданный DSN обязан успешно подключиться, а `/ready` проверяет
базовую схему R22.2 (`000001`). D1 paths явно проверяют наличие `000002`, не
делая source-disabled readiness зависимой от новой таблицы.

## Ordinary DM/group SDK lifecycle (opt-in runtime; rollout unverified)

BE106-ORDINARY-SDK-v1 ([decision part 1](http://192.168.0.109:30990/?project=voice&task=BE-106#comment-c3),
[part 2](http://192.168.0.109:30990/?project=voice&task=BE-106#comment-c4))
activates only the ordinary DM/group lifecycle. The native journal/coordinator,
handler/read/token routing, exact conversion-source cleanup and persisted recovery
and event relay are wired by `VOICE_ORDINARY_SDK_ENABLED=true`. This runtime does
not register R22, Space watcher/roster, GAME_SESSION, MatchSquad
or federation paths. Their existing dispatch/ownership guards remain authoritative.
The resumable implementation plan is
[BE-106 ExecPlan](../../.agent/execplans/be106-ordinary-lifecycle.md).

### Startup and controlled cutover

Before enabling, operators must stop every legacy ordinary writer, close its
previous SDK rooms and finish/reconcile legacy call/account ownership. This is an
explicit controlled drain, not automatic adoption. Set
`VOICE_ORDINARY_LEGACY_DRAINED=true` only with that drain evidence. Startup requires
the attestation, migration `000012_ordinary_calls`, PostgreSQL, durable Redis,
LiveKit credentials, NATS, User and Chat configuration. It scans active ordinary
Redis records and rejects any without matching native operation provenance;
foreign resource owners retain their existing dispatch. The scan is not a fence
against concurrently serving legacy processes. Rollout must quiesce them first.

With the flag absent/false, a nonempty native journal refuses startup rather than
routing native projections through legacy tuple-only mutation. `/ready` checks
the native schema while enabled. Recovery/outbox workers use persisted leases;
unfinished creation is scheduled after its bounded preparation window, and old
terminal room names continue reconciliation indefinitely. This source delivery
does not establish drain completion, enabled deployment or actual SFU acceptance.

### Admission and authority

One ordinary PostgreSQL journal owns immutable operation/room IDs, committed
membership generations, exact shared account-fence ownership, effects and event
outbox. Redis is a rebuildable compatibility projection. DM room
`voice-dm-<room UUID>` is ensured before RINGING; group room
`voice-group-<room UUID>` before ACTIVE. UUIDs/names are never reused. Ring expiry
is committed RINGING time plus 30 seconds; Accept requires `now < expires_at`.
Internal PREPARING is not a new public CallStatus.

Trusted caller/Chat/privacy/account and SDK-conversion prechecks precede mutation.
Pending admission reserves capacity/account/profile slots durably before external
effects; those fences cannot expire/reassign while an SDK effect may have run.
Pending admission is visible as busy to conversion quiescence, but is not logical
membership and cannot authorize tokens or participant facts. Preserve the existing
post-reservation SDK-conversion recheck. No database transaction spans network IO.
Finalization CAS revalidates the same operation/generation, room status/version,
deadline and exact fence ownership, then commits membership and stable event
intents together. Ordinary read/token paths consult this authority before Redis;
pending, closing or unavailable authority fails closed.

The shared serialization protocol is
[BE106-SDK-CONVERSION-SERIALIZATION-v1](http://192.168.0.109:30990/?project=voice&task=BE-106#comment-c10).
Ordinary transactions lock their room first, then the complete account/profile
identity union using sorted, deduplicated actual signed PostgreSQL hash keys in
two-int advisory namespace `0x564F4943`, before mapping/fence writes. Conversion
locks its operation then the same identity barrier and performs only nonlocking
native-source reads, never ordinary room row locks. Both use READ COMMITTED;
ownership reads are separate statements after acquiring the barrier. Pending or
fenced conversion denies the source profile and every profile of the target
account. Activated conversion retains only retired source-profile denial. Cleanup
is always allowed. Immutable receipt provenance binds ordinary cleanup to exact
room, operation and membership generation. Missing authority is not empty source.
Space/Game writers need their own participation before global conversion safety
can be asserted.

The native A3 consumer seam is
[BE011-SOURCE-SEAM-v1](http://192.168.0.109:30990/?project=voice&task=BE-011#comment-c2).
Keep a separate roster counter from the all-mutation journal CAS. Each committed
profile join/leave gets a distinct consecutive roster version and frozen event
in the same transaction, ordered by UUID bytes within a batch; DM Accept advances
twice atomically. Ringing, pending/no-op admission, SDK cleanup and retries do not
advance this counter. Store each admitted actor's own verified session epoch,
never an epoch reconstructed from Auth floors. A common private reader accepts
one read-only REPEATABLE READ transaction across native sources and takes no
locks. Source coverage, Space writer integration and the trusted ordinary
reconnect producer remain separate activation requirements.

SDK create retry uses the stored intent/name. Before READY wins, uncertainty,
cancellation, failure or restart chooses ABORTING and compensates that exact name;
a late create cannot ring, issue tokens or revive it. If READY already won before
response loss, preserve the accepted call. Public Start has no new idempotency
field/promise: pending/live repeats retain conflict semantics; GetActiveCall
recovers an accepted call after response loss. Unjournalled legacy Redis rooms
need validated explicit adoption or controlled drain, never synthetic identity.

### Termination, recovery and media identities

One room/membership transaction elects termination, denies new admission/token
issuance, invalidates pending admissions and records committed-member snapshots,
terminal event intents and exact cleanup ownership. Decline applies only to
RINGING. Either DM participant may End/Leave; preserve existing group End authority.
Non-last group Leave removes only self; last Leave versus Join is serialized at
this same transaction boundary. Failed tentative admissions produce no member
facts, while committed transitions retain their facts after later projection,
cleanup, response or NATS failure. The shared fact contract is
[BE102-COMMITTED-ADMISSION-v1](http://192.168.0.109:30990/?project=voice&task=BE-102#comment-c5).

CloseRoom is required for decline, expiry, DM End/Leave, group End and final Leave;
NotFound proves absence, timeout/unavailable does not. Keep CLOSING/fences and
retry on uncertainty, returning UNAVAILABLE/deadline rather than cleanup success.
Confirmed cleanup precedes exact-generation fence release and required Redis
projection convergence. Non-final Leave journals LEAVING and removes the stored
media identity. Public terminal decisions/authorized replay remain recoverable
after membership projection removal. Stale operations cannot overwrite a winner.

Each real ordinary admission freezes opaque media identity
`ov:<profile UUID>:<membership UUID>`; refresh retains it, leave/rejoin changes it.
Public roster/events keep profile IDs and client mapping resolves media identity
back to the profile. Late old removal cannot eject the newer admission. Space,
MatchSquad and Game identity formats stay unchanged. Shared account fences use
ordinary operation/generation CAS without commandeering Space operation IDs.

Persist retry scheduling/worker ownership and use bounded fresh contexts. Restart
aborts unfinished preparation, resumes cleanup, repairs committed projection and
outbox, expires ringing by CAS and preserves valid ACTIVE calls. Unknown commit
results resolve by durable readback, never guessed rollback or TTL takeover.
Outbox replay retains original event IDs/payload/recipients/order; success means
the accepted journal/SDK/projection boundary and durable intent, not NATS delivery.

DeleteRoom/RemoveParticipant prove current disconnect, not permanent bearer-token
revocation. The accepted guarantee is durable application admission denial plus
confirmed current cleanup, with ongoing reconciliation of retired names/identities
against stale reconnect/late effects. Compact terminal tombstones are not collected
solely at JWT expiry; an old bearer may briefly reconnect before ejection. Strict
deny-before-connect is a separate SFU admission-verifier decision. Source tests,
handshake and signaling are not two-client real-media acceptance.

## Ответственность

- DM-звонки (1:1 голос/видео)
- Групповые звонки / временные комнаты у текстовых групп (текстовая группа — до 500 участников; каждая временная voice room — до 32 участников)
- Голосовые комнаты в спейсах (`voice_rooms`, до 32 free / 128 paid)
- Screen share (desktop/window/tab + system audio)
- Генерация LiveKit токенов для клиентов
- Управление LiveKit-комнатами (создание, закрытие)
- Voice state tracking (кто в какой комнате, mute/deafen статус)
- Commander mode (broadcasting + ducking)
- Raise hand
- PTT / VAD mode
- Ограничение: один активный voice на профиль
- Множественные screen share потоки (до 3 одновременно)

DM and standalone group-call LiveKit JWTs carry room-scoped `roomJoin=true`,
`canPublish=true`, and `canSubscribe=true` grants explicitly. An explicit
`canPublish=false` supplied by the admission path remains a publish denial;
it does not prevent listening. This preserves the ordinary call media defaults
without relying on omitted LiveKit claims. No room administration grants are
issued, and existing membership checks, participant identity and expiry remain
authoritative. Space-room grants follow the separate Role-owned policy below.

### Subscription enforcement projection (A7 accepted target; not implemented)

Voice consumes `subscription.entitlement_changed` for Space aggregates into a
durable `voice_db` inbox/projection with source revision and `entitled_until`.
`ACTIVE`/`GRACE_PERIOD` allow the Space Pro 128-participant cap; `INACTIVE` or
deadline equality uses the free 32 cap. A higher recovery snapshot cannot be
regressed by stale failure/expiry. Existing participants are not kicked when Pro
ends, but new admission is denied while count is at/above the free cap.

Personal paid stream quality comes only from Auth's trusted short-lived claims
containing `subscription_tier`, `subscription_revision`, and
`subscription_entitled_until`. At equality Voice makes a protected entitlement
decision through Subscription-owned `ResolveEntitlementAtBoundary`, passing the
aggregate key and claim revision as `minimum_revision`. Its budget is
`min(500ms, remaining request deadline)`; timeout/auth/unavailable response
denies only paid quality/cap for that decision and triggers reconciliation
instead of extending it or breaking the base/free path. The current unversioned `GetSpaceSubscription` lookup
is a migration path and cannot race over the revisioned projection after
activation. Snapshot/replay tests are specified in
[subscription-lifecycle-convergence-exec-plan.md](../testing/subscription-lifecycle-convergence-exec-plan.md).

### Trust dependency diagnostics

Voice emits `voice_trust_dependency_failure` before its existing fail-closed
error mapping discards dependency detail. `stage` identifies `jwks_request`,
`jwks_status`, `jwks_body`, `jwks_parse`, `jwks_keys`, `replay_write`, or the Auth
floor boundary (`auth_floor`, `auth_floor_call`, `auth_floor_sign`,
`auth_floor_response`). `cause` is a closed label such as `timeout`,
`tls_unknown_authority`, `tls_hostname`, `tls_certificate`, `dns`,
`connection_refused`, `connection_reset`, or a bounded gRPC status class;
`duration_ms` measures that dependency attempt. `http_status` preserves only
100–599 for a non-success JWKS response and is zero for other failures or an
invalid status. Logs exclude raw error text,
URLs, credentials, account/profile IDs, request bodies and key material.

The two-second dependency budgets, TLS verification, replay checks and fresh
Auth session-floor authority remain unchanged. A cold Go JWKS request can fail
while a later warm-cache request passes; neither a successful BusyBox GET nor
one successful audio attempt proves the original failure's cause or stable
post-restart acceptance. The 2026-10-15 initial and post-restart audio failures
both reached Voice and returned generic verifier `Unavailable` after 2000 ms,
with a successful audio attempt between them. Those historical logs do not
distinguish JWKS, replay Redis and Auth-floor failures. Preserve all outcomes;
use the dependency event from a subsequent controlled acceptance attempt to
identify the failing stage before changing trust or transport configuration.

### Ordinary participant NATS facts (BE-102 accepted contract)

For ordinary DM and standalone group calls, a participant is a committed member
of an ACTIVE logical call. Invitations, token issuance, pending reservations and
SFU connectivity are not membership. Group Start commits one joined fact for its
initiator; absent-to-present Join commits one. Existing-member Join, failed or
compensated admission and state changes commit none. Leave commits one left;
the last leave does not emit a second terminal left. Whole-call termination
commits one left per remaining membership in its winning snapshot. DM ringing,
rejection and expiry emit none; the winning accept joins both profiles atomically,
and active End/Leave leaves both once. Terminal/no-op attempts emit none. A real
leave/rejoin creates a new membership generation.

BE106-ORDINARY-SDK-v1 PostgreSQL transition/outbox is the single authority:
effects are encoded in the same transaction as final committed membership.
Never derive them from tentative Redis state or a handler read/write pair.
Ambiguous/post-commit failures preserve saved operations and event IDs; fresh
delivery retries do not rebuild facts or compensate committed membership.

`VoiceParticipantJoined`/`VoiceParticipantLeft` carry only UUID `room_id` and
`profile_id`; `VoiceStreamEvent` carries persisted UUID `event_id` and original
`occurred_at`. Subjects are exactly `voice.participant_joined` and
`voice.participant_left` on `voice_events`. There is no recipient list, roster
version, new WS operation or push. Realtime, Notification and Analytics ACK these
unsupported facts without fanout or synthetic analytics records. Existing
`call_started` and `voice.member_joined` behavior stays unchanged. This does not
activate Phase0/Space roster, managed rooms, MatchSquad or federation, and source
presence alone is not deployment or media delivery proof.

## API (gRPC)

Источник истины: [protos/voice/calls/v1/calls.proto](../../protos/voice/calls/v1/calls.proto) (`VoiceService`). Важные ответы: **`GetJoinTokenResponse`** — поля `jwt` и `expires_at` (`google.protobuf.Timestamp`, UTC); **`GetVoiceStatesResponse`** — `repeated VoiceParticipantState participants` (без промежуточной обёртки-списка).

```protobuf
service VoiceService {
  rpc StartCall(StartCallRequest) returns (StartCallResponse);
  rpc AcceptCall(AcceptCallRequest) returns (AcceptCallResponse);
  rpc DeclineCall(DeclineCallRequest) returns (DeclineCallResponse);
  rpc JoinCall(JoinCallRequest) returns (JoinCallResponse);
  rpc LeaveCall(LeaveCallRequest) returns (LeaveCallResponse);
  rpc EndCall(EndCallRequest) returns (EndCallResponse);
  rpc JoinVoiceRoom(JoinVoiceRoomRequest) returns (JoinVoiceRoomResponse);
  rpc LeaveVoiceRoom(LeaveVoiceRoomRequest) returns (LeaveVoiceRoomResponse);
  rpc MoveToVoiceRoom(MoveToVoiceRoomRequest) returns (MoveToVoiceRoomResponse);
  rpc MoveVoiceRoomParticipant(MoveVoiceRoomParticipantRequest) returns (MoveVoiceRoomParticipantResponse);
  rpc GetJoinToken(GetJoinTokenRequest) returns (GetJoinTokenResponse);
  rpc UpdateVoiceState(UpdateVoiceStateRequest) returns (UpdateVoiceStateResponse);
  rpc GetVoiceStates(GetVoiceStatesRequest) returns (GetVoiceStatesResponse);
  rpc GetActiveCall(GetActiveCallRequest) returns (GetActiveCallResponse);
  rpc StartScreenShare(StartScreenShareRequest) returns (StartScreenShareResponse);
  rpc StopScreenShare(StopScreenShareRequest) returns (StopScreenShareResponse);
  rpc SetCommanderMode(SetCommanderModeRequest) returns (SetCommanderModeResponse);
  rpc SetBroadcasting(SetBroadcastingRequest) returns (SetBroadcastingResponse);
  rpc RaiseHand(RaiseHandRequest) returns (RaiseHandResponse);
  rpc LowerHand(LowerHandRequest) returns (LowerHandResponse);
  rpc GrantFloor(GrantFloorRequest) returns (GrantFloorResponse);
  rpc RevokeFloor(RevokeFloorRequest) returns (RevokeFloorResponse);
}
```

## Модель данных (`voice_db` + Redis)

`voice_db` содержит семь lifecycle tables: room instances, memberships,
operations, effects, media-epoch denials, outbox и orthogonal
`voice_lifecycle_redis_divergences`. Open divergence evidence проверяется до
Redis access и до admission нового operation ID. Для `decided` mismatch Voice
атомарно увеличивает fence, снимает lease и применяет существующий business
transition в `quarantined`; `completed` receipt остаётся неизменным; orphan не
получает выдуманный subject. Исправление/исчезновение Redis не закрывает
incident. Resolution остаётся отдельным будущим protected operator flow.

### A3 membership schema (source-disabled storage contract)

Migration `000003_matchmaking_membership` expands room instances to `call`,
`group_voice` and `voice_room`. Ordinary calls/groups bind a Chat UUID; Space
rooms bind the existing Space/logical-room UUID pair. Immutable `purpose` is
`ORDINARY` or `MATCH_SQUAD`. A match room uses `group_voice` and retains its match
owner ID, creation operation, manifest hash, creation receipt ID and Chat creation
receipt ID together with its Chat UUID. No ordinary resource can be relabelled
as a match resource by updating these fields.

### T31 managed game-session rooms

This is the target contract, not evidence that all listed APIs exist at the T31
base. `ProvisionGameSessionRoom` is implemented at base
`c31a8e0b771f99d00f00a760a2c6e2f7ec67cbcd`. Managed-grant admission checking
and the close path below remain required T31 implementation work; the close
protobuf/RPC, durable CLOSING/CLOSED receipt and media fencer/store integration
are absent there. T31 includes the Voice proto, server, Postgres schema/migration,
and focused service integration tests for these additions.

`000004_game_session_rooms` adds `GAME_SESSION`, a `group_voice` room with a
NULL `owner_id`. This purpose is separate from `MATCH_SQUAD`, whose owner remains
required. GIS identity and player profiles never become room owners. Each managed
room binds the authenticated application/environment, GIS `session_id`, exact
external resource kind/key, Chat ID and Chat creation operation ID to one Voice
operation, request hash, Voice creation receipt and immutable response. The Chat creation operation
ID identifies Chat's durable creation receipt; Voice does not infer or create a
Chat resource.

The optional private listener is enabled only by a complete
`VOICE_GAME_PRINCIPAL_*` configuration. Startup fails if TLS server identity,
dedicated client CA, HTTPS GIS JWKS, replay Redis, Voice DB, or migrations 4 and 5 are
unavailable. It requires verified mTLS and exposes only
`GameSessionProvisioningService/ProvisionGameSessionRoom`. The request's
`operation_id` is the `x-request-id`; its service principal must have exact
issuer `gameintegration`, audience `voice`, exact RPC and deterministic
protobuf request hash. A JTI can authorize only one attempt.

Room, resource mapping and immutable Voice receipt are committed in one
PostgreSQL transaction. The same operation and deterministic input returns the
stored original response after process restart; changed input or a second
operation for the same application/environment/resource conflicts. Voice stores
`session_id` with the immutable operation request and receipt; replaying the
same resource with another GIS session ID conflicts. Voice passes the stored ID
to `CheckGameSessionGrant` with application/environment, room ID, and the
authenticated profile. A Role grant is valid only while its Role ledger row is
active and becomes invalid immediately after durable revoke; T31 has no grant
expiry clock. Provisioning
does not add members, issue media tokens or authorize admission. Players continue
through the existing user-authorized Voice path, which checks current Chat
membership/admission before token issuance. Under T31, managed-session
`JoinCall` and `GetJoinToken` additionally call Role's private
`CheckGameSessionGrant` for the verified profile, application/environment,
session ID and Voice room before issuing or refreshing a media token. Missing,
revoked or mismatched grant denies admission even if Chat
membership or a warm CallStore entry exists. On a Voice call-store cache miss,
Voice rebuilds an ownerless managed-room projection from the durable mapping,
then checks live Chat membership before adding the authenticated profile. This
lets an active managed room recover after Redis loss without an invented
initiator. Managed rooms do not occupy the ordinary active-Chat-call index, stay
active when the last player leaves, and cannot be ended through player `EndCall`;
the GIS provisioning endpoint does not expose lifecycle mutation methods.

### T31 managed-room close receipt

The close API described in this section is a required new owner implementation;
it is not present at the cited base.

`VoiceGRPC` receives an injectable `ManagedGameSessionGrantChecker` dependency
with method `CheckGameSessionGrant(ctx, applicationID, environmentID, sessionID,
voiceRoomID, profileID string) error`. Missing dependency fails closed for
managed-room admission. Both cold projection and warm CallStore paths call it
for JoinCall and GetJoinToken.
The in-package typed setter
`setManagedGameSessionGrantChecker(ManagedGameSessionGrantChecker)` exists for
focused service tests; production startup injects the real Voice-to-Role client
through the same interface.

Voice enables GIS room provisioning and the Role grant checker as a complete
configuration pair. GIS admission uses `VOICE_GAME_PRINCIPAL_GRPC_LISTEN`,
`VOICE_GAME_PRINCIPAL_TLS_CERT_FILE`, `VOICE_GAME_PRINCIPAL_TLS_KEY_FILE`,
`VOICE_GAME_PRINCIPAL_CLIENT_CA_FILE`, `VOICE_GAME_PRINCIPAL_JWKS_URL`,
`VOICE_GAME_PRINCIPAL_JWKS_CA_FILE`, and
`VOICE_GAME_PRINCIPAL_REPLAY_REDIS_ADDR`. The trusted client CA is dedicated to
the GIS caller. Role lookup uses `VOICE_ROLE_GRPC_ADDR`,
`VOICE_ROLE_TLS_CA_FILE`, `VOICE_ROLE_CLIENT_CERT_FILE`, and
`VOICE_ROLE_CLIENT_KEY_FILE`; its client identity is distinct from GIS and is
signed by Role's configured client CA. Partial configuration prevents Voice
startup. The Voice service principal signer uses
`VOICE_PRINCIPAL_PRIVATE_KEY_FILE`, `VOICE_PRINCIPAL_KID`,
`VOICE_PRINCIPAL_NEXT_PRIVATE_KEY_FILE`, `VOICE_PRINCIPAL_NEXT_KID`,
`VOICE_PRINCIPAL_JWKS_TLS_CERT_FILE`, and
`VOICE_PRINCIPAL_JWKS_TLS_KEY_FILE`. The HTTPS
`/internal/v1/principal/jwks.json` listener publishes both public current and
next RSA keys; Voice signs only with the current KID. Current and next KIDs and
keys must be distinct. The Role caller sends only the current-key assertion for
`CheckGameSessionGrant`.

The Voice-to-Role client uses a separate private mTLS listener and signed
workload assertion with exact issuer `voice`, audience `role`, exact RPC
`RoleService/CheckGameSessionGrant`, request UUID in `x-request-id`, and
deterministic protobuf SHA-256 request hash. This Voice principal cannot call
Role Apply/Revoke. GIS uses its distinct `gameintegration` principal for those
mutations; it never impersonates Voice or a user.

The T31 owner API adds private
`GameSessionProvisioningService/CloseGameSessionRoom`. Its protobuf request
contains `operation_id`, `application_id`, `environment_id`, `session_id`, `resource`,
`chat_id`, and `chat_creation_operation_id`; it identifies the same immutable
owner tuple as provisioning. Its response contains those IDs, `room_id`,
`status`, `close_receipt_id`, `request_hash`, `closing_at`, `media_fenced_at`,
and `closed_at`. The GIS mTLS principal is exact issuer `gameintegration`,
audience `voice`, exact RPC and deterministic protobuf request hash. Voice
stores the close request hash and immutable receipt in the same PostgreSQL
transaction that changes the managed room to `CLOSING`; this commit immediately
fences new admission. The Voice media fencer then stops/ejects existing media
and atomically changes the room to `CLOSED`, recording `closed_at`,
`media_fenced_at`, and the immutable close receipt. `CloseGameSessionRoom`
returns only after that receipt is committed. Exact retries resume a durable
`CLOSING` operation or return its original receipt; changed bytes under the
same operation ID conflict. Closing an already closed resource with its
original close operation returns that receipt; a different close operation
for that resource conflicts. A restart resumes fencing from the durable
`CLOSING` row using the same operation ID.

The close commit immediately fences new `JoinCall` and `GetJoinToken` admission,
including warm CallStore entries; those paths consult durable owner state and
Role grants before issuing a token. Each media-fence attempt is bounded to five
seconds. If an attempt fails, `CLOSING` continues to fence admission and an
exact retry resumes the same operation; the successful attempt records
`media_fenced_at` after its fence completes. The typed
`ManagedGameSessionMediaFencer.FenceManagedGameSession(ctx, roomID, operationID)`
dependency is injected into the provisioning handler. The winning operation ID
is passed on every retry so the media owner deduplicates the fence effect. The
active-media recording fixture in the Postgres integration test verifies this
path. Voice never deletes the managed room or its
Chat. Provision and close receipts remain queryable for the T32
terminal receipt window; Voice retains the non-content resource fence after
receipt expiry.

The store-level integration seam is typed as
`PostgresStore.CloseGameSessionRoom(ctx, request, mediaFencer)`. It commits
`CLOSING` plus the exact protobuf request hash before invoking the fencer,
passes the winning operation ID to the fencer for idempotency, then commits
`CLOSED` plus one immutable receipt with `closing_at`, `media_fenced_at`, and
`closed_at`. Exact retry during `CLOSING` resumes fencing; retry after `CLOSED`
returns the same receipt without a second fencer call. The close Postgres
integration test provisions a room, seeds active media in a recording fencer,
closes it, asserts the active media stopped and `media_fenced_at` is within five
seconds of the successful fencer attempt's start, confirms the closed room is
no longer joinable, and replays the same receipt. It also fails a first fence,
waits past five seconds, and verifies that an exact retry can finish the same
durable `CLOSING` operation. The gRPC handler separately binds the authenticated principal to the
exact request ID and deterministic protobuf hash before calling this seam.

Memberships record the verified `account_id`, positive `session_epoch` and
`JOINING|JOINED|RECONNECTING|LEAVING|LEFT|EJECTED` state. `RECONNECTING` alone has
`reconnect_started_at` and `reconnect_deadline`, with a positive interval no longer
than 30 seconds. Repeating a reconnect update in the same media epoch cannot
extend that interval. Identity cannot be cleared or account ownership changed;
the session epoch cannot regress or change within the same media epoch.
Space-specific access/Role epochs remain mandatory for Space memberships and
are absent for ordinary call/group memberships.

Legacy membership rows retain NULL identity/state, with no synthetic account or
epoch backfill. Dedicated storage reads reject those unknown rows; they are not
evidence of SOLO or an eligible roster. Existing R22 APIs keep their migration-1
compatibility; the new read model requires migration 3 separately. Reads expose
stored evidence only and do not establish current Auth session validity.

This schema does not activate snapshot RPC, roster publication, protected
writers or runtime registration. Activation must first replace legacy writers
and implement one transaction boundary for membership, roster version and outbox,
plus current-session validation and expired reconnect handling. DOWN takes
exclusive locks and rejects expanded identity or room data; legacy-only rows can
be rolled back without discarding evidence. Verification and remaining gates:
[voice-mm-membership-exec-plan.md](../testing/voice-mm-membership-exec-plan.md).

The current `RedisCallStore` layout below is the compatibility store used by
existing active-call paths. Keys use the configured prefix (default `voice:`).
The call document is the record read and mutated within this compatibility
store; the other keys are lookup indexes or move receipts. These keys do not
represent the `voice_db`-backed lifecycle projection described above. That
lifecycle path remains source-disabled, and its Redis bridge/projection has not
been registered.

| Key (default prefix) | Redis value | Use |
|---|---|---|
| `voice:call:{room_id}` | JSON `Call` document | Stores the call record; its fields are shown below. |
| `voice:session:{profile_id}` | `room_id` string | Profile-to-call lookup for a ringing or active call. This is a pointer, not a session object. |
| `voice:active_chat:{chat_id}` | `room_id` string | Active group-voice lookup by Chat ID; managed game-session rooms are excluded. |
| `voice:active_voice_room:{voice_room_id}` | `room_id` string | Active Space voice-room lookup by voice-room ID. |
| `voice:voice_move_operation:{actor_profile_id}:{operation_id}` | JSON `{"request": ..., "result": ...}` | Idempotency receipt for a voice-room participant move. |

The `Call` document shape is:

```text
{
  room_id, livekit_room_name, chat_id,
  managed_game_session?, application_id?, environment_id?, session_id?,
  voice_room_id?, space_id?, session_kind?,
  initiator_profile_id, callee_profile_id, media_kind, status,
  started_at, expires_at, ended_at?,
  states: {
    <profile_id>: {
      profile_id, is_muted, is_deafened, is_video_on, is_screen_sharing,
      is_commander, hand_raised, has_floor, is_broadcasting
    }
  },
  screen_shares?: [{ profile_id, stream_id }]
}
```

Question-marked fields are omitted when empty or zero. The profile, Chat, and
voice-room keys above are indexes to this document; they are not copies of its
participant state.

Written call documents, lookup indexes, and move receipts have a 24-hour TTL.
The implementation scans `voice:call:*` documents to find expired ringing
calls; there are no `voice:room:*` objects or participant/screen-share set keys
in this store.

### Привязка комнаты в ответах и событиях

`CallSession.space_id` (active session) и `VoiceSession.space_id` (join response)
передают сохранённый при проверенном join серверный Space ID. Новое поле присутствует
только у `voice_room` с полной парой валидных UUID `voice_room_id` / `space_id`.
Старые записи без полной пары остаются читаемыми; Space не выводится из session ID,
LiveKit name или данных клиента. Существующие поля ответа сохраняют совместимость.

`CallStarted` / WS `call_started` дополнительно передают `room_type`
(`call`, `group_voice`, `voice_room`), а для полной room-привязки — оба ID.
Realtime сохраняет прежних получателей `profile_ids`; legacy события без новых
полей допустимы. `voice_member_joined` уже передаёт эту пару, его аудитория не меняется.

Это persisted locator, **не разрешение** и не доказательство текущего membership.
`GetActiveCall` не добавляет обращения к Space; выдача или повторная выдача media grant
по-прежнему требует свежей проверки через существующий token flow.

Redis compatibility state above does not replace durable lifecycle authority.
Where the `voice_db` lifecycle path is enabled, durable room lifecycle and
divergence evidence govern admission and reconciliation; the compatibility
keys above are not that authoritative projection. Process instances remain
horizontally scalable because durable state is externalized.

## Интеграция с LiveKit

```
Voice Service ──LiveKit Server SDK──► LiveKit SFU
Client ──LiveKit Client SDK──► LiveKit SFU (media streams)
```

- Voice Service создаёт/удаляет комнаты через LiveKit Server SDK
- Voice Service генерирует JWT-токены для клиентов
- Клиенты подключаются напрямую к LiveKit для медиа-потоков

For mapped game-node Space rooms, the opt-in `VOICE_FEDERATED_MEDIA_CONFIG`
client preserves the existing canonical Space/membership/Role and profile fence
checks, resolves the exact current master route/application binding, obtains a
private signed credential through a distinct Voice mTLS role, and exchanges
only that credential at the registered HTTPS node media edge. The node's SFU
secret stays local. Only an explicit version-1 not-hosted result permits hosted
token issuance; denial, ambiguity, stale policy, generic HTTP error or unavailable
authority fails closed. See [authority contract](../architecture/federation-authority-v1.md)
and [runtime setup](../../docker/voice-node/authority/README.md). Owning-service
projection, complete bundle and qualified capacity acceptance remain open.

- WebRTC signaling (`offer`/`answer`/`ICE`) идёт внутри LiveKit SDK; собственный signaling через Realtime/Gateway не вводится в Фазе 2
- Кодеки: Opus (32 kbps audio), VP8/VP9 (video)
- LiveKit Simulcast для screen share (адаптивное качество)

## Phase-0 Space-room media, roster и lifecycle (partial implementation; full target remains open)

R22.3 PostgreSQL evidence/classification остается source-disabled. Отдельный
BE255 implementation slice добавляет Space-room join/leave admission, короткий
LiveKit grant, durable Space/Role invalidation delivery и Voice reconciliation
с точным удалением LiveKit incarnation. Каждый подписанный Voice-user RPC до
вызова handler сверяет epoch делегированного principal с текущим Auth floor;
epoch также сохраняется в grant.

Active Space-media reconciliation reads the current signed Auth floor once per
account in each full `ReconcileAll` sweep across Spaces, then compares every
profile's own issued session epoch independently. Standalone `ReconcileSpace`
calls start their own fresh pass. A Space watermark restart discards all shared
floor values before retrying; a new sweep retains no previous values.
Retries retain the existing per-Space bound and do not rescan completed Spaces;
the shared floor values do not form an atomic cross-Space snapshot.
Unavailable, missing, or nonpositive authority leaves
reconciliation incomplete and the invalidation unacknowledged; stale epochs or
an authoritative missing account retain exact-incarnation ejection. Admission
recovery and signed Voice-user RPCs retain their fresh authority checks.

This does not complete Phase-0: LiveKit-only reconnect eligibility after an
Auth epoch change remains a release gate. SLA удаления 2 s p95 / 5 s max остается ограниченным потерей
membership/Role rights и не распространяется на Auth-only epoch change. Roster projection,
move handlers, Redis v2 encoding и replay deadline bridge semantics (D2/D3)
также остаются отдельными gates. Hosted Linux proof относится только к явно
выбранному BE255 media consumer и не означает завершение R22.3/BE255 целиком.

The private Space lifecycle listener opts in only through
`VOICE_SPACE_LIFECYCLE_PRINCIPAL_*` settings. Shared `S2S_JWKS_*` settings alone
do not enable it; they also serve the separate user-principal listener. Any
lifecycle-specific setting, including an empty value, still requires complete
lifecycle TLS, replay, and trusted JWKS configuration or startup fails. When both
listeners are enabled, their listen addresses must differ. The combined local/CI
fixtures retain lifecycle port 9092 and use user-principal port 9093, with
Gateway's matching private upstream; neither port is published to the host.

Space media admission resolves Role-owned room grants only through Role's
private mTLS listener, using the Voice workload signer and a fresh
request-bound principal for each `ResolveVoiceRoomGrants` call. The request
binds the canonical Space, room, and profile IDs; Role's returned policy epoch
is retained with the media grant. `VOICE_ROLE_GRPC_ADDR` and its complete
`VOICE_ROLE_TLS_*` client tuple enable this consumer independently of GIS
provisioning. Incomplete settings fail startup; there is no ordinary-listener
fallback.

The user-principal listener validates local TLS configuration and replay Redis
before startup, but starts with an empty Gateway JWKS cache. Gateway waits for
this listener before serving its own key endpoint, so requiring an initial key
fetch would deadlock their startup. Listener readiness does not assert that
Gateway keys are already available: protected RPC verification fetches a complete
valid key set before accepting any principal and returns `UNAVAILABLE` during a
key endpoint outage. Invalid keys still deny access. Background refresh, the
issuer-wide unknown-key cooldown, hard expiry, replay prevention and Auth epoch
checks retain the shared Phase-0 rules.

Account-wide Voice reservation and transfer checks take the caller account and
profile from the verified delegated principal on this listener, then compare
against User's profile ownership. Legacy identity headers cannot override or
repair an invalid delegated identity. The existing legacy call listener retains
its metadata-based identity path when no verified principal is present.

**Public surface.** Gateway exposes Space room actions under
`/api/v1/spaces/{space_id}/voice-rooms/{voice_room_id}`: `POST /join`,
`POST /leave`, `GET /roster`, `POST /move` (self; destination `voice_room_id`),
and `POST /participants/{profile_id}/move` (moderator). `space_id` is path-bound
and never asserted by the client in a body/header. All actions use the Gateway
derived delegated-user principal; no client-provided profile or Space identity is
authoritative. `MoveToVoiceRoom` is self-only; the distinct
`MoveVoiceRoomParticipant` RPC accepts an explicit target and never treats a
client-provided actor as authoritative.

For this surface: unauthenticated is `401 unauthenticated`; absent Space/room is
`404 not_found` only after a caller is entitled to discover it; a caller without
Space membership/discovery receives `404 not_found`; a discoverable room denied by
policy receives `403 permission_denied`; stale/inactive room, self-move impossible
or conflicting active operation gives `409 failed_precondition`; resolver/Role/
LiveKit dependency failure is `503 unavailable`. Every deny has no room session,
roster or event side effect. Gateway freezes these `error_code` values instead of
relaying arbitrary gRPC text. Mutations carry a client UUID `operation_id`; Voice
keeps a 24-hour keyed ledger `(actor_profile_id, operation_id, method, canonical
request)` for identical replay and rejects changed request as `409 failed_precondition`.

**LiveKit grant.** A Space-room grant is a 60-second LiveKit JWT bound to server
identity, `profile_id`, `room_id`, an authorization `epoch`, and the explicit
join/publish/subscribe grants determined by Role. Voice re-resolves access before
every issue/reissue. On membership, `VOICE_JOIN` or `VOICE_SPEAK` loss, self-hosted
LiveKit actively ejects the current participant/media within **2 s p95** and
**5 s max**. Voice denies a fresh grant/reissue after the authorization change and
the client retries reconnect for at most 30 s only while its authenticated session
and epoch are unchanged. Removing a participant on self-hosted LiveKit does not
cryptographically revoke issued bearers. LiveKit can refresh a connected client's
token beyond the original Voice-issued ≤60-second expiry; that expiry is not a
bound on every refreshed bearer. Without a LiveKit-side verifier there is no
deny-before-connect or zero-transient-media guarantee. Reconciliation must retain
exact retired identities beyond the original expiry and re-eject observed stale
reconnects within the same SLA; a five-second periodic scan alone does not prove
2 s p95 / 5 s max. Strict SFU admission verification remains separate scope.
No cloud media movement is a target:
user grants remain self-hosted by default.

### Moderator mute (accepted target; BE-273)

The accepted `BE273-MODERATOR-MUTE-v1` decision defines the following additions;
the RPC, schema, source grants and live proof are not implemented by this
documentation change. Scope is self-hosted canonical Space voice rooms. DM,
standalone group, MatchSquad, GameSession and federation policies are unchanged.
Do not activate full-roster/watchers/moves or source-disabled R22.3/D2/D3 here.

**State and authority.** Voice persists `moderator_muted` keyed by
`(space_id, voice_room_id, target_profile_id)`, independent of transport/admission
generation. It survives disconnect, leave/rejoin, room empty/reopen, restart,
Role changes and account-session refresh; another room/profile is unaffected.
Only authorized clear or permanent logical room/profile/Space deletion removes
the restriction. Keep `self_is_muted` distinct: either changed moderation flag
sets/leaves it true. Clearing never opens a microphone, activates a device,
grants a floor or grants `VOICE_SPEAK`. While restricted all audio-enabling paths,
including self-unmute and broadcast/floor-related enabling, deny. Self-mute,
deafen, leave and independently authorized non-audio actions remain available.

The actor is only the verified Gateway delegated principal with fresh Auth
epoch/profile ownership, current Space membership/discovery,
`VOICE_MUTE_OTHERS` in the exact canonical room and `SPACE_VIEW_MEMBER_LIST`
before selecting/disclosing the target. Actor media presence is unnecessary.
Owner bypass remains Role-owned; Voice adds no hierarchy/immunity rule.
`actor_profile_id == target_profile_id` denies, including self-clear by a
moderator. The target must be a current Space member. A new mute requires a
committed `ACTIVE` participant in this room; tentative admission does not count.
Clear may address an offline current member with an existing restriction.
Repeating the current desired state is a no-op, with no new media effect/event.
BE-044 timeout remains an additional admission/media deny, including listeners;
it does not itself remove moderator administration rights.

**Protected surface.** Add `SetVoiceRoomParticipantMute` on the existing signed
Voice user listener and
`POST /api/v1/spaces/{space_id}/voice-rooms/{voice_room_id}/participants/{target_profile_id}/mute`.
Path IDs are canonical and body contains required presence-aware boolean
`moderator_muted`, UUID `operation_id`, and `expected_moderation_version` (zero
only for never-created state). No body actor/account/media identity is trusted.
Authorized state reads disclose additive `moderator_muted`, `self_is_muted` and
`moderation_version`; occupancy-only output never exposes them or a target ID.
Compatibility `is_muted` means effective audio mute; old client patches cannot
clear the durable restriction. A receipt contains `operation_id`,
`target_profile_id`, `moderator_muted`, `self_is_muted` (true when changed),
`moderation_version` and `enforcement_completed_at`. It is not a token or proof
of permanent bearer revocation.

Use the existing 24-hour `(actor_profile_id, operation_id, method, canonical
request)` ledger. Exact retry resumes/replays the same operation; changed input
conflicts. Monotonic moderation-version CAS orders opposing moderator commands;
one pending transition per target/key gives competing transitions `409` until
convergence. A desired-state no-op returns the current version without effect.
Completed replay still requires current authenticated actor/discovery, never
restores superseded state and creates no new effect. After ledger expiry the
version CAS still rejects historical updates against a later state.

Errors follow the existing surface: `400 invalid_argument` for missing/invalid
fields, `401 unauthenticated`, generic `404 not_found` for undiscoverable,
nonmember or absent resources without target disclosure, `403 permission_denied`
for explicit policy deny/self-target/self-unmute, `409 failed_precondition` for
stale version, non-active target on first mute, inactive room or conflicting
pending transition, and `503 unavailable` for unknown dependency/capability/media
outcome. Pre-decision denial has no restriction/roster/grant/event side effect.
Post-commit failure preserves the durable decision and resumes by the same ID.

**Audio sources.** Extend the same protected Role room-grant projection with
versioned explicit video/share-source capability, store the issued effective
source vector in the Space admission, and resolve fresh on join/reissue and
Role invalidation. Unknown/legacy capabilities cannot authorize unproven sources.

| Source | Required current permission and local condition |
|---|---|
| microphone | `VOICE_SPEAK` and not moderator-muted |
| screen-share audio | `VOICE_SPEAK`, `VOICE_SCREEN_SHARE` and not moderator-muted |
| camera | `VOICE_VIDEO` |
| screen-share video | `VOICE_SCREEN_SHARE` |

Membership/Auth/timeout/`VOICE_JOIN` and existing screen-share admission/limits
still intersect the vector. Set `canPublishSources` to the explicit nonempty
allowed vector; empty vector means `canPublish=false`, never true with omitted
allowlist. Microphone mute must not become permanent all-track denial. Track
source/kind validation must reject audio labelled as an allowed video source on
the pinned SFU; this is not content inspection of audio encoded into video.
Neither `MutePublishedTrack` nor modifying Role permissions prevents republish.

**Transaction, enforcement and recovery.** Serialize moderation with the existing
durable Space room-head/admission authority. One transaction stores desired
flag/version, actor/target, operation receipt/pending effect, stable state-change
intent and captured current admission tuple: trusted account/profile/Auth epoch,
Space/logical room, call room/generation, admission operation/generation and
exact LiveKit room/identity. No database transaction spans a network call.

Any changed flag retires that exact current media incarnation using existing
`BeginRoomLeave` / revoker / `CompleteRoomLeave` and generation-bound fence
release. This temporarily interrupts all current target media, including video
and listening, before fresh authorized admission with the new source vector;
it is not a persistent video/listen ban. Other targets/rooms stay independent.
No unordered in-place `UpdateParticipant` relaxation is introduced. Late
operations may touch only retired identities, never a replacement generation.

Join/reissue finalization checks the moderation version in that same serialized
authority: a grant begun before mute cannot later commit unrestricted media.
Unresolved old cleanup blocks new target admission; no unconditional rollback or
second fence. Confirmed removal/NotFound and exact release permit fresh
self-muted admission. Clearing also requires fresh admission/grant when media
existed, never automatic mic activation. The existing ≤30-second same-session
client retry policy is not a new auto-rejoin feature.

Immediately trigger durable cleanup; measure active ejection from decision
commit at 2 s p95 / 5 s max and use the same bound for observed stale reconnect.
Success means desired state committed plus confirmed cleanup/projection receipt;
confirmed target absence suffices. SDK timeout/cancellation/transport failure is
unknown, never proof of absence. Post-commit SDK/DB/Redis failure leaves the
pending decision/effect intact, blocks target grants and returns `503` or the
transport deadline/cancellation. Same-operation retries and independently bounded
recovery contexts with backoff converge after crash/response loss. Recovery must
not recreate permanently deleted resources or eject a newer admission.

Extend the existing Space reconciler with exact retired identities even after
active roster/admission changes. Never reuse those identities or drop tombstones
merely at original JWT expiry; connected-client refreshed bearers remain a
reconciliation risk. There is no second lifecycle coordinator or ordinary-call
membership outbox. Stable Voice state-change intent deduplicates one moderation
version and uses only existing authorized state delivery, with no occupancy-only
identity/state disclosure; NATS failure never rolls back the restriction.
Auth/Space/Role epochs, moderation version, roster epoch and media generation
remain separate values. Source/grant, deterministic concurrency/failure tests and
pinned actual-media proof are specified in the
[ExecPlan](../testing/space-moderator-mute-exec-plan.md).

**Roster.** `SPACE_VIEW_MEMBER_LIST` permits full room roster fields
`profile_id`, display snapshot, muted/deafened/video/speaking state; a Space member
without it receives a redacted occupancy response (`occupant_count`, no profile or
state fields), not a member-list disclosure. The same audience rule applies to
REST snapshot and Realtime events. Voice resolves current membership and Role
permission before publishing explicit full and occupancy recipient sets; failure
is fail-closed and Realtime never infers watchers. Snapshot carries the Voice-owned
`roster_epoch`, disclosure `projection` (`full` or `occupancy`), that projection's
monotonic `projection_version`, and an HMAC-signed filter-bound cursor.

The full counter advances on join/leave and disclosed voice/screen state changes.
The occupancy counter advances only when `occupant_count` changes; a full-only
mutation emits no aggregate event and creates no aggregate gap. A room incarnation
or authority change that can alter a viewer's disclosure class starts a new
positive `roster_epoch`; this is not LiveKit `media_epoch`, Space access generation
or Role policy epoch. Clients buffer only their live projection during snapshot,
deduplicate by UUID `event_id`, then apply contiguous
`(room_id, roster_epoch, projection, projection_version)` values. A gap, reconnect,
epoch/projection change or invalid cursor requires the matching new snapshot.
Exact payload redaction and active-tab fan-out are specified in
[realtime-service.md](realtime-service.md#phase-0-space-room-roster-fan-out-target-не-реализовано).


Доменный поток JetStream: **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

Existing participant lifecycle publishers provide the authoritative recipient
set consumed by Realtime; Realtime does not infer recipients from a room, chat or
Space. The envelope UUID `event_id` is the client dedupe key and `occurred_at` is
diagnostic only. Exact per-operation recipients, public WS fields, malformed
event handling and reconnect ordering are frozen in
[realtime-service.md](realtime-service.md#voice-participant-lifecycle-fan-out-shipped-compatibility-contract).
This compatibility stream is not the Phase-0 Space watcher/roster stream: the
latter requires audience-specific disclosure plus room authorization epoch and
monotonic roster version before it can ship.

| Событие                      | Данные                                  |
|------------------------------|-----------------------------------------|
| `voice.call_started`         | room_id, initiator_id, type             |
| `voice.call_incoming`        | room_id, chat_id, initiator_profile_id, callee_profile_id, media_kind, expires_at |
| `voice.call_accepted`        | room_id, accepted_by_profile_id, profile_ids |
| `voice.call_declined`        | room_id, declined_by_profile_id, profile_ids |
| `voice.call_missed`          | room_id, initiator_profile_id, callee_profile_id |
| `voice.call_ended`           | room_id, duration_seconds               |
| `voice.participant_joined`   | room_id, profile_id                     |
| `voice.participant_left`     | room_id, profile_id                     |
| `voice.state_changed`        | profile_id, changes (mute/deafen/video) |
| `voice.screen_share_started` | room_id, profile_id                     |
| `voice.screen_share_stopped` | room_id, profile_id                     |

## Публикуемые события (→ NATS)

## Зависимости

- **LiveKit** — SFU для медиа
- **Chat Service** — валидация участников DM/group при звонке
- **Space Service** — валидация доступа к **голосовой комнате** (`voice_room_id`)
- **Role Service** — проверка прав (`VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_VIDEO`, …)
- **Notification Service** — (через NATS) входящий звонок → push
- **Redis** — хранение активных сессий
- **PostgreSQL `voice_db`** — durable lifecycle и divergence evidence; Redis
  outage/divergence fail closed и не выбирает memory или legacy Redis-only authority

## Масштабирование

Voice process instances stateless — масштабируются горизонтально. Durable lifecycle и divergence evidence находятся в `voice_db`; Redis rebuildable. LiveKit масштабируется независимо (SFU per region для low-latency).

## P3 Space lifecycle participant (target)

### Native admission workflow ownership

Migration `000015_space_admission_ownership` adds
`voice_space_media_admission_owners`. A fresh admission creates a random private
owner token and owner epoch `1` in the transaction before its first durable room
head claim. Ownership is separate from immutable admission operation/media,
room and P3 lifecycle generations. Takeover never regenerates event IDs,
payloads, account-fence tuples or room identities.

Authority expires on PostgreSQL time after 30 seconds. The executing foreground
or recovery workflow renews every five seconds using a two-second DB budget;
renewal cannot revive an expired/replaced owner. This detection window covers
multiple bounded recovery effects and six renewal opportunities. It never
extends the foreground's existing two-second Space communication decision
deadline, cached grants, bearer expiry or provider admission authority. Failed
or uncertain renewal cancels forward work. Workflow completion/cancellation
relinquishes only the exact token/epoch; failed acknowledgement leaves the
durable journal and bounded lease for later readback/recovery.

Every forward journal mutation and owner-driven abort/fence-release/cleanup
completion verifies the current unexpired token/epoch under the existing Space
barrier, room and sorted identity lock order. It first acquires the owner row's
exclusive lock, then checks token, epoch and fresh PostgreSQL time in a separate
statement. A valid transaction retains that lock through commit and may finish
after its lease deadline while waiting for downstream rows; takeover serializes
after it. A new transaction or renewal must acquire the lock and check freshness
again, so a lock wait cannot preserve a pre-lock lease observation or revive an
expired owner. SQL authority does not carry into a separate Redis effect or ACK
transaction. Recovery enumeration is only a
candidate list: healthy owners are filtered before the page limit. An atomic
claim rechecks current eligibility and state under the Space barrier and room
head, then rotates the token and increments its epoch. Expired/relinquished
PREPARED/FENCED work becomes ABORTING; COMMITTED-unapplied work uses its freshly
read state and authority to roll forward or deny. A stale PREPARED snapshot
cannot select an abort after a later commit. A committed creator can restore
its own missing shell from its immutable journal; other joiners still require
the finalized creator/survivor witness. Orphan heads without an admission use
the same locked owner-expiry arbitration, never a fixed head age.

Redis has a separate `space_admission_dispatch:<operation UUID>` gate containing
a fresh incarnation, RESTORING/READY phase, owner epoch and allow/deny state.
Dispatch tickets bind both the Space lifecycle gate and this admission gate.
After observing a missing/replaced gate, maintenance installs RESTORING and
reads fresh PostgreSQL authority before publishing READY. Redis projection
transactions WATCH and verify the original admission incarnation/epoch at the
sink, including retries; delayed callbacks cannot adopt a newer gate. Recovery
first durably takes ownership, then establishes admission DENY before exact
projection drain, account-fence release and cleanup completion. PostgreSQL
records the matching `dispatch_fenced_epoch`; an unknown Redis result cannot
produce that acknowledgement. A lost gate remains fail-closed and terminal
abort cannot restore forward dispatch.

Owned revocation uses explicit `BeginOwnedRoomLeave`/`CompleteOwnedRoomLeave`
arguments and an owner-bound sink loader. Missing or replaced ownership fails
before Redis mutation and before an idempotent terminal return. Verified public
leave, authority-loss cleanup and P3 captured drain use separate restrictive
APIs: they persist REVOKING and obtain an exact admission tuple with the current
owner epoch, without acquiring or borrowing a writer token. The full expected
account/profile/Space/room/incarnation/identity tuple is compared under canonical
locks before any restrictive SQL mutation. Both fencers validate their explicit
owner or restrictive target before allocating even a missing Redis gate, then
reload authority after gate observation. Owned DENY APIs reject forward LIVE
state. Their loader can
only publish DENY for that persisted restrictive target. A takeover invalidates
the old target and acknowledgement; cleanup must explicitly obtain a fresh
target and re-fence. Redis revocation transactions check the captured DENY nonce
and epoch as well as the immutable tuple in the same WATCH/EXEC. Removing owner
context never selects the restrictive path, and old owned calls cannot fall
back to it. P3 retains its existing captured-target and provider retirement
proof requirements.

Native startup requires the ownership schema and rejects any unfinished/live
admission or nonterminal head without matching ownership metadata. Before
cutover, fence every pre-ownership writer and explicitly reconcile historical
rows; no invented backfill, age-based adoption or mixed-writer serving is
supported. The existing Redis snapshot rollback/old-writable-endpoint recovery
boundary applies to both Space and admission gates: fence infrastructure writes
and rotate/rebuild from PostgreSQL before exposure. This source contract does
not establish rollout, activation, provider retirement or acceptance.

The native A3 roster outbox relay is dormant in production composition. Native
transactions still save their frozen facts and counters, but ordinary Voice,
Space media and `NATS_URL` configuration do not lease, publish or mark these facts
delivered. Existing participant/admission compatibility delivery is unchanged.
A3 relay activation requires a separately qualified canonical broker authority,
scoped publisher rights and durable consumer/inbox/source-coverage package; a v5
readiness result or PubAck alone does not authorize that composition change.

Voice adds a durable PostgreSQL lifecycle fence/operation receipt even though
its media projection is primarily Redis. `FROZEN` atomically fences admission,
actively ejects every Space session and revokes outstanding grants; ordinary
join/token/command paths fail closed on uncertain fence state. `LIVE` accepts
only the next valid generation. `PURGE_DECIDED` is irreversible and removes or
terminally fences Space room/session/grant rows and Redis keys. It binds the
root manifest without inventing chat-owned work. Full request/receipt bytes
retain 30 days from this participant's completion; the compact max-generation
`PURGED` fence is permanent.

BE-265 current-provider correction: a local restrictive fence or successful
participant removal is not a completed `FROZEN` receipt. Completion requires
complete native/legacy grant coverage, settled exact cleanup and durable provider
rejection of all old original/refreshed credentials, including pending handshakes,
or a proved empty credential history. Current self-hosted removal supplies no
such retirement capability. Voice applies the restrictive barrier and immediate
exact drain, but keeps the operation pending without a receipt. Full deletion
activation requires a qualified provider retirement path; `LIVE` and `PURGED`
cannot bypass this barrier. Original Voice60s expiry, empty Redis/roster and
DeleteRoom/NotFound are not future-admission proof.

The current native journal participates in the shared two-int PostgreSQL Space
barrier (`0x53504345`, `hashtext(canonical Space UUID)`), before room resources and
sorted account/profile identity locks. Forward admission/commit/projection/grant
authorization checks its saved lifecycle generation; cleanup remains permitted
for exact captured owners. The restrictive transaction seals typed native/head
and legacy targets before changing native states. Retries drain saved identities
even without Redis. Redis projection writers require one per-Space gate containing
a random incarnation, RESTORING/READY phase, lifecycle generation and LIVE/DENY.
Missing gates install RESTORING with a fresh incarnation before a fresh PostgreSQL
barrier read; publication compares that exact incarnation. Dispatch reads READY
before fresh native authorization, then atomically checks the same incarnation,
generation and exact native tuple. A delayed callback cannot acquire a replacement
incarnation or initialize LIVE. Metadata and cleanup cannot recreate active indexes;
exact cleanup preserves another incarnation's owned pointers. Existing
COMMITTED journal intent covers projection dispatch before its applied marker.
Grant authorization saves exact claims/provider binding and absolute Voice expiry
before signing; retry never extends that expiry. No expiry bounds SFU refresh.

This gate covers deletion, eviction, partial loss, flush and process retry on one
authoritative Redis history. Restoring an old snapshot with the same READY
incarnation, asynchronous failover losing an acknowledged DENY, or a writable old
endpoint requires infrastructure recovery to fence writes and rotate/rebuild gates
from PostgreSQL before exposure. The nonce does not itself prove that operational
prerequisite; current qualification must retain this boundary.

`Service.RetirementCapability` and the self-hosted adapter declare `EJECT_ONLY`
(missing adapter/config is `UNKNOWN`). There is no production retirement proof
provider or environment switch that can claim full support. Historical issuers
and old unguarded receipts remain unproven: migration does not rewrite their
bytes or invent credential coverage. Before any future full activation, inventory
them and reconcile all issuer/writer versions; reject mixed deployments. Retain
unresolved targets/grants and deny state beyond receipt retention. Provider
qualification, protected DEV/STG checks and independent acceptance remain open.

# MatchSquad room provider

Voice exposes the typed `MatchSquadVoiceService` only on its dedicated protected
listener. `VOICE_MATCH_SQUAD_PRINCIPAL_*` is disabled only when every variable
in that namespace is absent. Partial configuration, missing Voice lifecycle
schema, unavailable JWKS or replay Redis, invalid TLS material, and listener
startup failure prevent the capability from starting. The listener uses
mutual TLS and accepts only the exact create and teardown methods with a
request-bound `service:matchmaking` principal. It also accepts the exact
`CompactMatchSquadRoom` method for that same principal; no ordinary listener
exposes it.

`voice_room_instances` owns the current MatchSquad resource and its
`active`/`closing`/`closed` state. `voice_match_squad_operations` binds the
creation and teardown operation IDs to exact request and receipt bytes, the
Matchmaking receipt, participant manifest, and permanent replay/terminal
fences. A repeated operation with matching bytes returns its stored receipt;
changed bytes conflict. The resource is never reopened after teardown begins.

Redis call records and LiveKit rooms are repairable effects using the same
room UUID and deterministic `match-squad-<room UUID>` LiveKit name. A Redis
MatchSquad marker is a projection label only. It does not authenticate a
caller or grant membership, and ordinary call join, media-token, leave, and
end operations reject marked rooms. Redis profile indexes are not authority;
projection repair cannot replace a conflicting active profile lock or an
open divergent call document. Teardown commits its completed receipt only
after the database resource is closed and Redis/LiveKit effects are confirmed
or absent.

Delegated self-membership is available only on the separate
`MatchSquadMemberService` listener configured by
`VOICE_MATCH_SQUAD_MEMBER_PRINCIPAL_*`. It accepts only Join, GetJoinToken, and
Leave with a Gateway delegated-user principal bound to the exact method,
request ID, and request hash. The listener requires mutual TLS, HTTPS JWKS,
replay protection, and a current Auth `auth:session:min_epoch:<account>` value.
Voice rechecks that session and the User-owned profile-to-account mapping
around durable admission decisions; forwarded profile or session metadata is
not authority.

The stored creation roster is only eligibility. A verified actor separately
acquires one current `voice_room_memberships` generation; Voice never fills
the active membership projection from the whole roster. MatchSquad command
receipts, generation-bound effects, and each issued token expiry are retained
in `voice_match_squad_member_operations`,
`voice_match_squad_member_effects`, and
`voice_match_squad_member_grants`. Redis `call:<room UUID>` and
`session:<profile UUID>` records and the LiveKit room are projections of that
same durable room and member generation.

GetJoinToken requires the exact current JOINED generation and records its
expiry before returning a token capped at 60 seconds and the actor's remaining
validity. Self Leave removes only that actor's projections and stays LEAVING
until the latest bearer expires and both effects are confirmed; a DB-clock
repair then marks that generation LEFT and releases its account fence. This
member completion does not delete the room or complete Matchmaking's aggregate.
Aggregate teardown separately closes admission, waits for grants and pending
effects to drain, confirms final LiveKit room absence, and only then stores
the terminal teardown receipt.

Full operation bytes remain stored until Matchmaking's durable teardown
aggregate is complete and its PostgreSQL authority clock reaches
`aggregate_completed_at + 30 days`. Matchmaking then sends one immutable
protected compaction command binding the aggregate, match, room, creation and
teardown receipts, request hashes, manifest, and both authority timestamps.
Voice checks that binding against its own closed, effects-confirmed database
resource and commits the exact command and stable compact receipt in one SQL
transaction while clearing only the full create/teardown payload bytes. Voice
does not infer eligibility from its own clock or perform Redis/LiveKit effects
during compaction. The compact command/receipt and permanent ownership,
operation, resource, and digest fences remain; matching compact-command replay
returns the exact stored receipt, while old create/teardown replay gets a
terminal precondition instead of recreating or fabricating historic receipts.
The migration refuses DOWN while any operation or permanent fence remains.
