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

```
voice:session:{profile_id} → {
  room_id, room_type (call|voice_room|group_voice),
  chat_id, voice_room_id, space_id,
  is_muted, is_deafened, is_video_on,
  is_screen_sharing, is_commander,
  hand_raised, has_floor, is_broadcasting, joined_at
}

voice:room:{room_id} → {
  type, chat_id, voice_room_id, space_id,
  participant_count, max_participants,
  created_at, livekit_room_name
}

voice:room:{room_id}:participants → Set[profile_id]
voice:room:{room_id}:screen_shares → Set[{profile_id, stream_id}]
```

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

Эти Redis-ключи — projection, которую можно перестроить из durable lifecycle
данных в `voice_db`, если operation не заблокирована open divergence evidence;
Redis не является источником истины room lifecycle. Process instances остаются
горизонтально масштабируемыми, потому что durable state вынесен наружу.

## Интеграция с LiveKit

```
Voice Service ──LiveKit Server SDK──► LiveKit SFU
Client ──LiveKit Client SDK──► LiveKit SFU (media streams)
```

- Voice Service создаёт/удаляет комнаты через LiveKit Server SDK
- Voice Service генерирует JWT-токены для клиентов
- Клиенты подключаются напрямую к LiveKit для медиа-потоков
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
