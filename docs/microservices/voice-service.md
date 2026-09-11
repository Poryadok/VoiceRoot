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
- Групповые звонки / временные комнаты у текстовых групп (до 500 участников группы; лимит в комнате — см. [voice-chat.md](../features/voice-chat.md))
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
authoritative. The final proto must add a distinct moderator-move RPC; it must not
reuse self move with an asserted actor.

For this surface: unauthenticated is `401 unauthenticated`; absent Space/room is
`404 not_found` only after a caller is entitled to discover it; a caller without
Space membership/discovery receives `404 not_found`; a discoverable room denied by
policy receives `403 permission_denied`; stale/inactive room, self-move impossible
or conflicting active operation gives `409 failed_precondition`; resolver/Role/
LiveKit dependency failure is `503 unavailable`. Every deny has no room session,
roster or event side effect. Gateway freezes these `error_code` values instead of
relaying arbitrary gRPC text. Mutations carry a client UUID `operation_id`; Voice
keeps a 24-hour keyed ledger `(actor_profile_id, operation_id, method, canonical
request)` for identical replay and rejects changed request as `409 already_exists`.

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
REST snapshot and Realtime events. Snapshot carries room authorization epoch,
monotonic roster `version` and HMAC-signed filter-bound cursor. Clients buffer
live events during snapshot, then apply contiguous versions; a gap/invalid cursor
requires a new snapshot.


Доменный поток JetStream: **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

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
