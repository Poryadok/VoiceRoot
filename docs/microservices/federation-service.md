# Federation Service

Game federation remains deferred and is not advertised through Gateway. The
master now has a bounded HTTPS authority foundation for enrollment, operator
approval, certificate rotation, complete snapshots, and signed short leases.
This does not implement the legacy S2S gRPC contract, a snapshot publisher, a
Voice Node consumer, or media enforcement. The activation boundary and wire
details are in [Federation authority v1](../architecture/federation-authority-v1.md)
and the [ExecPlan](../testing/federation-authority-exec-plan.md).

## Обзор

S2S-федерация: подключение внешних нод, синхронизация событий, маршрутизация уведомлений. Master ↔ Node архитектура.

**Язык**: Go
**БД**: PostgreSQL `federation_db`; schema version 1 is embedded in the Go
service and applied transactionally under a PostgreSQL advisory lock.
**Current control plane**: dedicated TLS 1.3 HTTPS on `:9443`, mandatory
verified client certificates, separate operator DER pins, node certificate
pins, and a hash-only expiring Bearer credential. Health, readiness, and
metrics remain on `:8080`; no authority route is registered there.
**Legacy data plane**: gRPC bidirectional stream in
`protos/voice/s2s/v1/s2s.proto` remains unimplemented.

## Master authority HTTPS v1

The executable contract is versioned in
[`federation-authority-v1.md`](../architecture/federation-authority-v1.md).
It defines strict bounded JSON, canonical UUIDs, pending enrollment, explicit
operator ownership attestation, approval, immediate certificate/credential
rotation, suspension, permanent defederation, and immutable Space placement.
The operator API never fetches node endpoints. Operator pins are configured
separately from node pins; forwarded certificate headers carry no identity.
The server requires TLS 1.3 and validates the currently verified certificate
chain again for each HTTP request, including requests on a reused TLS connection.

Each placed Space has one complete typed snapshot. Publication uses a
sequential revision compare-and-set; identical bytes at the same revision are
idempotent and every other replay or gap conflicts. Nodes receive the exact
snapshot inside a signed Ed25519 envelope after presenting their registered
certificate and Bearer credential. Lease requests acknowledge the current
revision/hash with a single-use nonce; leases expire within two seconds and
nonces remain reserved through credential expiry. Node changes increment an
epoch and serialize with lease issuance using node-then-placement row locks.

The runtime requires `FEDERATION_DATABASE_URL`, `FEDERATION_TLS_CERT`,
`FEDERATION_TLS_KEY`, `FEDERATION_CLIENT_CA`, `FEDERATION_SIGNING_SEED_FILE`,
`FEDERATION_KEY_ID`, `FEDERATION_ISSUER`, `FEDERATION_ENVIRONMENT`, and
`FEDERATION_OPERATOR_CERT_SHA256`. `/ready` is database/schema readiness.
Deployment must provision `federation_db` and a service-specific login with
access limited to that database before starting the Deployment. The migration
creates only Federation-owned tables. The standard Compose `app` profile leaves
the service stopped. A separate opt-in `federation` profile passes the required
runtime settings and mounts local TLS/signing material from
`FEDERATION_LOCAL_SECRET_DIR`; unset settings or missing files make startup fail
closed. Keep all production values in the deployment secret manager.
Both Kubernetes manifests keep the Deployment at `replicas: 0`; the
`voice-federation` ClusterIP exposes `:9443` internally and `:8080` for health
and metrics only. There is no Gateway route. Keep it dormant until a Voice Node
consumer is implemented and separately activated.

When that activation is approved, `voice-app-secrets.FEDERATION_DATABASE_URL`
must use the restricted Federation login. Create the namespace-local
`voice-federation-authority` Secret with `tls.crt`, `tls.key`, `client-ca.crt`,
and `signing-seed`. The server certificate must include the `voice-federation`
DNS SAN. `signing-seed` contains the unpadded base64url encoding of exactly 32
random bytes; do not use a PEM key or reuse an environment's seed elsewhere.
The shared ConfigMap keys `FEDERATION_KEY_ID`, `FEDERATION_ISSUER`, and
`FEDERATION_OPERATOR_CERT_SHA256` must be set to the consumer's trust values and
lowercase SHA-256 DER pins. Mounts are read-only. Provision TLS and signing
material through the environment secret manager; never commit it.

Snapshots are complete effective allowlists, not partial deltas. There is no
production owning-service publisher yet, and the service does not advertise a
media capability. The future Voice Node policy projection and media verifier
must validate the envelope and fail closed; full propagation and media ejection
under the five-second budget are not acceptance claims for this foundation.

The following legacy sections describe deferred product/data-plane targets.
They are not implemented by the HTTPS authority service.

## Остальные целевые обязанности (deferred; не реализованы)

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

## Legacy S2S API (deferred; unimplemented)

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
  rpc RegisterNode(RegisterNodeRequest) returns (RegisterNodeResponse);
  rpc ApproveNode(ApproveNodeRequest) returns (ApproveNodeResponse);
  rpc DeactivateNode(DeactivateNodeRequest) returns (DeactivateNodeResponse);
  rpc ListNodes(ListNodesRequest) returns (ListNodesResponse);
  rpc GetNodeStatus(GetNodeStatusRequest) returns (GetNodeStatusResponse);
  rpc Defederate(DefederateRequest) returns (DefederateResponse);
}
```

## Legacy conceptual data model (not the current PostgreSQL schema)

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
├── account_id
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

## Target NATS events (not currently published)

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
