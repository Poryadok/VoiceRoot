На данный момент нет архитектуры, игнорируй всё написанное здесь
## Архитектура (ARCHITECTURE.md / PLAN.md)

- [ ] **Поток JWT через gRPC-сервисы**: как ChannelDataService/MessageService проверяют токен? Через AuthService по gRPC или самостоятельно?
- [ ] **Диаграмма с LiveKit**: обновить ARCHITECTURE.md — добавить VoiceMediaService и LiveKit SFU в service diagram
- [ ] **Диаграмма с Matchmaking**: VoiceMatchmakingService не отражён в архитектуре
- [ ] **Flutter → сервисы**: на диаграмме клиент соединяется только с AuthService и WebSocketService; как он достучится до ChannelData и Message?
- [ ] **Как работает уведомление через WebSocketService**: кто его вызывает (ChannelData? Message? Matchmaking?) — нет описания flow
- [ ] **Деплой**: нет описания облачного провайдера, docker-compose для полного стека, CI/CD
- [ ] **Когда федерация**: PLAN.md не указывает фазу для federation — это Фаза 4? Позже?
- [ ] **Сторис и подписка в роадмапе**: нет в PLAN.md — когда реализовывать?

---

# Voice — Architecture

## Service Diagram

```
[Flutter Client]
      |
      | HTTP (REST)          WebSocket
      |                          |
[VoiceAuthService]    [VoiceWebSocketService]
  Java Spring Boot          Go / gRPC
  JWT auth, users       push notifications to clients
  PostgreSQL                    |
                                | gRPC
                    +-----------+-----------+
                    |                       |
        [VoiceChannelDataService]  [VoiceMessageService]
              Go / gRPC                 Go / gRPC
         channels, rooms, users    send/get messages
```

## Repositories

| Repo | Tech | Role |
|------|------|------|
| `VoiceAuthService` | Java 21, Spring Boot 3.4.1, Gradle | Auth, JWT, user management |
| `VoiceChannelDataService` | Go 1.22, gRPC | Channels, rooms, user lists |
| `VoiceMessageService` | Go 1.22, gRPC | Send/receive messages |
| `VoiceWebSocketService` | Go 1.22, gRPC | Push notifications to clients over WS |
| `VoiceChannelDataServiceProto` | protobuf | gRPC contracts for ChannelDataService |
| `VoiceMessageServiceProto` | protobuf | gRPC contracts for MessageService |
| `VoiceWebSocketServiceProto` | protobuf | gRPC contracts for WebSocketService |
| `voiceclient` | Flutter/Dart | Target client: mobile + web + desktop |

## Ports & Config

| Service | Port | Notes |
|---------|------|-------|
| VoiceAuthService | `8090` | HTTP REST, `.env` at repo root |
| VoiceAuthService DB | `5439` | PostgreSQL (`voice_users_db`) |
| VoiceWebSocketService | `24766` | gRPC, config at `config/local.yaml` |

## gRPC Contracts

### ChannelDataService (`vcds.proto`)
- `JoinChannel(userId, channelId)`
- `LeaveChannel(userId, channelId)`
- `GetRooms(userId, channelId) → [roomId]`
- `GetUserNotificationList(userId, channelId, roomId) → [userId]`

### MessageService (`vms.proto`)
- `SendMessage(userId, channelId, roomId, message)`
- `GetMessages(userId, channelId, roomId) → [MessageData]`

### WebSocketService (`vwss.proto`)
- `SendNotification(user, message)`

## Proto Code Generation

Each `*Proto` repo has a `Taskfile.yaml` — run `task` inside the proto repo to regenerate Go stubs into `gen/`.

## Key Decisions

- Flutter is the target client platform (mobile, web, desktop from one codebase)
- gRPC for all inter-service communication
- JWT auth issued by VoiceAuthService, validated per request
- Voice chat: LiveKit SFU (planned, Фаза 2)
- One global server on start, self-hosting deferred to Фаза 4

## Running Services Locally

```sh
# Auth service + its DB
cd VoiceAuthService && docker-compose up

# WebSocket service
cd VoiceWebSocketService && docker-compose up

# Go services (example)
cd VoiceChannelDataService && go run ./cmd/...
cd VoiceMessageService && go run ./cmd/...
```
