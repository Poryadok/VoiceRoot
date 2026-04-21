# Federation Service

## Обзор

S2S-федерация: подключение внешних нод, синхронизация событий, маршрутизация уведомлений. Master ↔ Node архитектура.

**Язык**: Go
**БД**: PostgreSQL `federation_db`
**Протокол**: gRPC bidirectional stream (см. `protos/voice/s2s/v1/s2s.proto`)

## Ответственность

- Регистрация федеративных нод (по запросу, не open)
- Persistent gRPC bidirectional stream для event sync
- Аутентификация пользователей нод через master
- Fallback-токены (5-10 мин TTL) для offline master
- Snapshot re-sync при reconnect (роли, баны)
- Маршрутизация уведомлений: node → master → FCM/APNs
- Мониторинг здоровья нод (heartbeat)
- Дефедерация при нарушениях ToS

## Архитектура Master ↔ Node

```
Master (Voice основной сервер):
  - Аккаунты, аутентификация
  - DM между пользователями
  - Глобальный матчмейкинг
  - Push-уведомления (FCM/APNs)

Federated Node (сторонний сервер):
  - Собственные пространства
  - Собственное хранилище файлов
  - Собственный LiveKit SFU
  - Свои лимиты подписок
```

## Scope v1

- Федеративные пространства (дерево спейса: `chats` + `voice_rooms` + единый `space_tree_nodes` — на ноде)
- Аутентификация через master
- Каталог игр (синхронизация с master)
- Жалобы и модерация

### НЕ в v1:
- DM между серверами
- Аккаунты на нодах (все на master)
- Space-level матчмейкинг

## API (gRPC)

Канонические определения — в репозитории:

- **Нода ↔ master (data plane):** [`protos/voice/s2s/v1/s2s.proto`](../../protos/voice/s2s/v1/s2s.proto) — `FederationService`: поток `EventStream(stream EventStreamRequest) returns (stream EventStreamResponse)` (нода шлёт `SubscribeRequest` / `Heartbeat` / `Ack` внутри `EventStreamRequest`, master — события в `EventStreamResponse`), плюс unary `SyncSnapshot`, `NotifyUser`, `AuthenticateUser`.
- **Управление нодами (control plane):** [`protos/voice/s2s/v1/federation_management.proto`](../../protos/voice/s2s/v1/federation_management.proto) — `FederationManagementService`: `RegisterNode`, `ApproveNode`, `DeactivateNode`, `ListNodes`, `GetNodeStatus`, `Defederate` (типы сообщений — `FederationNode`, `FederationNodeList`, …).

Ниже краткая сводка RPC (без дублирования полей сообщений):

```protobuf
// См. protos/voice/s2s/v1/s2s.proto
service FederationService {
  rpc EventStream(stream EventStreamRequest) returns (stream EventStreamResponse);
  rpc SyncSnapshot(SyncSnapshotRequest) returns (SyncSnapshotResponse);
  rpc NotifyUser(NotifyUserRequest) returns (NotifyUserResponse);
  rpc AuthenticateUser(AuthenticateUserRequest) returns (AuthenticateUserResponse);
}

// См. protos/voice/s2s/v1/federation_management.proto
service FederationManagementService {
  rpc RegisterNode(RegisterNodeRequest) returns (FederationNode);
  rpc ApproveNode(ApproveNodeRequest) returns (FederationNode);
  rpc DeactivateNode(DeactivateNodeRequest) returns (google.protobuf.Empty);
  rpc ListNodes(ListFederationNodesRequest) returns (FederationNodeList);
  rpc GetNodeStatus(GetFederationNodeStatusRequest) returns (FederationNodeStatus);
  rpc Defederate(DefederateNodeRequest) returns (google.protobuf.Empty);
}
```

## Модель данных

```
federation_nodes
├── id (UUID)
├── name
├── host (string — domain/IP)
├── port (int)
├── description
├── status (pending | active | suspended | defederated)
├── auth_token_hash (SHA-256)
├── tls_cert_fingerprint (string)
├── last_heartbeat_at
├── last_sync_at
├── registered_at
├── approved_at (nullable)
├── approved_by (admin profile_id)
└── defederated_at (nullable)

federation_events
├── id (UUID)
├── node_id (FK)
├── direction (inbound | outbound)
├── event_type (string)
├── payload (jsonb)
├── status (pending | delivered | failed)
├── created_at
└── delivered_at (nullable)

fallback_tokens
├── id (UUID)
├── user_id
├── node_id (FK)
├── token_hash (SHA-256)
├── roles (jsonb)
├── expires_at
├── created_at
└── INDEX(token_hash)
```

## Event Sync

```
Master ◄──gRPC bidirectional──► Node

Events (Master → Node):
  - RoleChanged
  - UserBanned / UserUnbanned
  - Defederated
  - GameCatalogUpdated

Events (Node → Master):
  - NotifyUser (push routing)
  - ReportCreated
  - SpaceDeleted
  - Heartbeat
```

### Reconnect flow:

> **Контекст S2S:** `last_event_id` относится к журналу событий **федеративной ноды ↔ master**, не к клиентскому WebSocket. Не смешивать с полем **`s`** и op `resume` в Gateway ([realtime-service.md](realtime-service.md)) и с курсором сообщений в Messaging.

1. Node reconnects
2. Node sends last_event_id
3. Master sends missed events (или full snapshot при длительном disconnect)

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`federation.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                        | Данные                         |
|--------------------------------|--------------------------------|
| `federation.node_connected`    | node_id, host                  |
| `federation.node_disconnected` | node_id, reason                |
| `federation.event_synced`      | node_id, event_type, direction |
| `federation.sync_failed`       | node_id, error                 |
| `federation.node_defederated`  | node_id, reason                |

## Зависимости

- **Auth Service** — валидация токенов пользователей нод
- **Notification Service** — relay push-уведомлений от нод
- **Role Service** — (через NATS) синхронизация ролей
- **Moderation Service** — (через NATS) обработка жалоб с нод


