# Voice Service

## Обзор

Оркестрация голосовых/видео-звонков и screen share через LiveKit SFU. Сам сервис не обрабатывает медиа-потоки.

**Язык**: Go
**Хранилище**: Redis (активные сессии), LiveKit (SFU)

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
  rpc RaiseHand(RaiseHandRequest) returns (RaiseHandResponse);
  rpc LowerHand(LowerHandRequest) returns (LowerHandResponse);
}
```

## Модель данных (Redis)

```
voice:session:{profile_id} → {
  room_id, room_type (call|voice_room|group_voice),
  chat_id, voice_room_id, space_id,
  is_muted, is_deafened, is_video_on,
  is_screen_sharing, is_commander,
  hand_raised, joined_at
}

voice:room:{room_id} → {
  type, chat_id, voice_room_id, space_id,
  participant_count, max_participants,
  created_at, livekit_room_name
}

voice:room:{room_id}:participants → Set[profile_id]
voice:room:{room_id}:screen_shares → Set[{profile_id, stream_id}]
```

## Интеграция с LiveKit

```
Voice Service ──LiveKit Server SDK──► LiveKit SFU
Client ──LiveKit Client SDK──► LiveKit SFU (media streams)
```

- Voice Service создаёт/удаляет комнаты через LiveKit Server SDK
- Voice Service генерирует JWT-токены для клиентов
- Клиенты подключаются напрямую к LiveKit для медиа-потоков
- Кодеки: Opus (32 kbps audio), VP8/VP9 (video)
- LiveKit Simulcast для screen share (адаптивное качество)

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`voice.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                      | Данные                                  |
|------------------------------|-----------------------------------------|
| `voice.call_started`         | room_id, initiator_id, type             |
| `voice.call_ended`           | room_id, duration_seconds               |
| `voice.participant_joined`   | room_id, profile_id                     |
| `voice.participant_left`     | room_id, profile_id                     |
| `voice.state_changed`        | profile_id, changes (mute/deafen/video) |
| `voice.screen_share_started` | room_id, profile_id                     |
| `voice.screen_share_stopped` | room_id, profile_id                     |

## Зависимости

- **LiveKit** — SFU для медиа
- **Chat Service** — валидация участников DM/group при звонке
- **Space Service** — валидация доступа к **голосовой комнате** (`voice_room_id`)
- **Role Service** — проверка прав (`VOICE_JOIN`, `VOICE_SPEAK`, `VOICE_VIDEO`, …)
- **Notification Service** — (через NATS) входящий звонок → push
- **Redis** — хранение активных сессий

## Масштабирование

Voice Service stateless — масштабируется горизонтально. LiveKit масштабируется независимо (SFU per region для low-latency).


