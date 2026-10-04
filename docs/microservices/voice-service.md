# Voice Service

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

## Phase-0 Space-room media, roster и lifecycle (target; не реализовано)

R22.3 предоставляет только source-disabled PostgreSQL evidence/classification
foundation. Coordinator, handlers, LiveKit/NATS adapters, registration, grant
issuance и public readiness остаются не реализованы; Redis v2 encoding и replay
deadline bridge semantics (D2/D3) также остаются отдельными gates.

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
and epoch are unchanged. A previously issued bearer JWT can still reconnect until
its ≤60-second expiry: without a LiveKit-side token verifier this is not a
cryptographic deny-before-connect guarantee. Such stale reconnect is best-effort
reconciliation followed by the same eject SLA. No cloud media movement is a target:
user grants remain self-hosted by default.

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

Voice adds a durable PostgreSQL lifecycle fence/operation receipt even though
its media projection is primarily Redis. `FROZEN` atomically fences admission,
actively ejects every Space session and revokes outstanding grants; ordinary
join/token/command paths fail closed on uncertain fence state. `LIVE` accepts
only the next valid generation. `PURGE_DECIDED` is irreversible and removes or
terminally fences Space room/session/grant rows and Redis keys. It binds the
root manifest without inventing chat-owned work. Full request/receipt bytes
retain 30 days from this participant's completion; the compact max-generation
`PURGED` fence is permanent.
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
