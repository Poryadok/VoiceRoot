# Voice Project — Root Context

## Architecture Overview

Discord-like voice/chat application. Microservices backend + Flutter client.

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

> React Native client (`VoiceClient_fromUT`) was a discarded attempt — removed.

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
