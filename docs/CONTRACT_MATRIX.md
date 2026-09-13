# Матрица контрактов: Gateway → сервисы и NATS JetStream

Единая сводка **публичных** маршрутов клиента (REST/WebSocket через API Gateway) и **доменных потоков** JetStream. При изменении маршрутов или издателей/подписчиков событий обновляйте этот файл **и** каноничные разделы в [microservices/api-gateway.md](microservices/api-gateway.md) и [MICROSERVICES.md](MICROSERVICES.md) (раздел «Event Bus»), чтобы они не расходились.

Чеклист перед мержем: [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md).

---

## Клиент → API Gateway → gRPC (REST namespaces)

Транскодинг HTTP → gRPC; детали RPC — в `docs/microservices/<service>.md`.

| HTTP prefix (`/api/v1/...`) | Целевой сервис        | Примечание |
|----------------------------|------------------------|------------|
| `auth/**`                  | Auth Service           | Публичные login/register — без JWT (см. api-gateway) |
| `users/**`                 | User Service           | [user-profile.md](features/user-profile.md): `POST …/users/me/avatar/presigned-upload` → `CreateAvatarPresignedUpload` (см. [api-gateway.md](microservices/api-gateway.md)) |
| `friends/**`               | Social Service         | |
| `chats/**`                 | Chat Service           | |
| `messages/**`              | Messaging Service      | |
| `spaces/**`                | Space Service          | |
| `roles/**`                 | Role Service           | |
| `voice/**`                 | Voice Service          | |
| `files/**`                 | File Service           | |
| `notifications/**`       | Notification Service   | |
| `search/**`                | Search Service         | |
| `matchmaking/**`           | Matchmaking Service    | |
| `moderation/**`            | Moderation Service     | |
| `subscription/**`          | Subscription Service   | |
| `bots/**`                  | Bot Service            | |
| `stories/**`               | Story Service          | |
| `analytics/**`             | Analytics Service      | Только персонал (Admin); см. [api-gateway.md](microservices/api-gateway.md) |
| `version`                  | —                      | Локальная политика Gateway / конфиг версий клиента |

## WebSocket

Voice active/join responses проецируют optional `space_id` из сохранённой полной
room-привязки. WS `call_started` допускает additive `room_type`, `voice_room_id` и
`space_id`, сохраняя прежнюю аудиторию участников и поддержку legacy payload.
Правила отсутствующих или неполных полей — в
[Voice Service](microservices/voice-service.md#привязка-комнаты-в-ответах-и-событиях).

| Путь   | Назначение |
|--------|------------|
| `/ws`  | Upgrade и прокси на **Realtime Service** (live-события, `s` / `resume`); см. [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) |

## Вне публичного Gateway

| Клиент / граница | Протокол | Сервис |
|------------------|----------|--------|
| Клиенты Flutter  | —        | **Federation Service** не вызывается с клиента; S2S gRPC / отдельный ingress, см. [microservices/federation-service.md](microservices/federation-service.md) |

Типичные **синхронные** gRPC между сервисами (не через Gateway) и async-паттерн для ядра чата — таблица «Tier 0: типичные связи» в [MICROSERVICES.md](MICROSERVICES.md).

### Bot Service → S2S (примеры, [bots.md](features/bots.md))

| Caller | Callee | RPC | Триггер |
|--------|--------|-----|---------|
| Bot | Role | `CheckPermission` | `InstallBotInSpace` (`SPACE_MANAGE_BOTS`) |
| Bot | Space | `AddBotMember` / `RemoveBotMember` | install / `UninstallBotFromSpace` |
| Bot | Role | `DeleteRolesCreatedByProfile` | `UninstallBotFromSpace` — удаляет custom-роли с `created_by_profile_id = bot.actor_profile_id` |
| Bot | Role | `GetMemberRoles`, `RevokeRole` | `UninstallBotFromSpace` — снимает роли, назначенные боту в спейсе |
| Bot | Messaging | `UnpinMessagesBySenderInChats` | `UninstallBotFromSpace` — открепляет pins сообщений бота в whitelisted чатах |
| Bot | Role | `AssignRole` / `RevokeRole` | `AssignBotRole` / `RevokeBotRole` (scope `MEMBER_ASSIGN_ROLES`) |
| Bot | Messaging | (send/edit) | `SendBotMessage`, `EditBotMessage` |

Полный REST transcoding Bot API — [microservices/api-gateway.md](microservices/api-gateway.md); gRPC-only RPC (`TouchPresence`, scope runtime без REST) — [microservices/bot-service.md](microservices/bot-service.md).

---

## NATS JetStream: streams, publishers, subscribers

**Владелец схемы событий** для stream — сервис из колонки Publishers. Breaking changes в payload / `.proto` — порядок выката [REPOSITORIES.md](REPOSITORIES.md).

| Stream                 | Publishers        | Subscribers |
|------------------------|-------------------|-------------|
| `user.events`          | Auth, User        | Analytics, Social, Notification, Federation, Realtime (presence_changed → friend WS fan-out) |
| `social.events`        | Social            | Analytics, Notification, Chat, Federation |
| `role.events`          | Role              | Analytics, Notification, Federation, Realtime |
| `message.events`       | Messaging         | Analytics, Notification, Search, Moderation, Realtime |
| `chat.events`          | Chat, Space       | Analytics, Notification, Realtime |
| `voice.events`         | Voice             | Analytics, Notification, Realtime |
| `moderation.events`    | Moderation        | Analytics, Notification, User |
| `subscription.events`  | Subscription      | Analytics, User, Space, File, Notification |
| `file.events`          | File              | Analytics, Messaging (preview update) |
| `matchmaking.events`   | Matchmaking       | Analytics, Notification, Voice, Chat |
| `story.events`         | Story             | Analytics, Notification, Matchmaking |
| `federation.events`    | Federation        | Analytics, Role, Moderation |
| `bot.events`           | Bot               | Analytics, Messaging |

### Analytics telemetry (`analytics_events` stream)

| Stream              | Publishers                                      | Subscribers   |
|---------------------|-------------------------------------------------|---------------|
| `analytics_events`  | Notification, Search, Gateway, Subscription, Moderation (direct `analytics.*`) | Analytics |

Domain JetStream streams (`message_events`, `user_events`, …) are also consumed by **Analytics** via stream adapters (dual ingest). Subject pattern: `analytics.{service}.{event}`.

Продуктовая аналитика дополнительно консьюмит subject’ы вида `analytics.*` (см. раздел «Аналитика» в [MICROSERVICES.md](MICROSERVICES.md)).

## A2 Space target routes

Canonical route/schema/ACL/error/retry table:
[API Gateway A2 Space REST](microservices/api-gateway.md#a2-space-rest-contract-target).
All rows below are target contract coverage, not shipment claims.

| Route group | Owner / RPC boundary |
|---|---|
| `spaces/{space_id}/join`, `/leave`, `/transfer-ownership`; `DELETE spaces/{space_id}`; `/restore` | Space JoinSpace/LeaveSpace/TransferOwnership/DeleteSpace/RestoreSpace; delete schedules recovery |
| `spaces/{space_id}/invites/**`, `invites/{code}`, `invites/{code}/join` | Space invite management/preview/redeem; safe authenticated preview |
| `spaces/{space_id}/tree/**`, `/categories/**`, `/voice-rooms/{id}` entity CRUD | Space tree/category/room RPCs; media actions under the same room prefix still belong to Voice |
| `spaces/{space_id}/chats` | Chat CreateChat then Space UpsertTreeNode; one resumable operation |
| `spaces/{space_id}/audit-log` | Space GetAuditLog; signed filter-bound cursor |
| `auth/ownership-transfer-proof` | Auth IssueOwnershipTransferProof; consume only Space→Auth |
| `auth/space-deletion-proof` | Future Auth IssueSpaceDeletionProof; distinct space_delete purpose; consume only Space→Auth |

No public route permits direct Owner-role reassignment or either Auth consume RPC.
NATS ownership remains unchanged: Space audit/lifecycle effects use Space's
transactional outbox in the existing domain stream; this table adds no new stream.

## BE-116 Space audit protected contract (accepted target)

This is direct protected gRPC, not a new NATS audit stream. The mutation owner
first commits its domain mutation and durable producer outbox atomically.

| Caller | Callee / RPC | Allowed scope | Runtime status |
|---|---|---|---|
| Role (`service:role`) | Space `AppendAuditEvent` | Closed Role action allowlist for role definitions, assignments and overrides | proto shipped; Role outbox/publisher and Space handler not implemented |
| Chat (`service:chat`) | Space `AppendAuditEvent` | `chat_created`, `chat_updated`, `chat_deleted` for Space-attached chats | proto shipped; Chat outbox/publisher and Space handler not implemented |

The one signed Bearer principal binds exact RPC, request ID and deterministic
request hash. No caller/source-service field or raw identity metadata is accepted.
Space owns the registry and read API; Gateway never authors entries and callers
never write another service database. Exact replay is an empty successful ack;
same event ID with changed payload is `ALREADY_EXISTS`. Full action/target/details
rules are in [space-service.md](microservices/space-service.md#phase-0-space-audit-ledger-accepted-target-runtime-not-implemented).

## P3 Space lifecycle protected contracts (accepted target)

All rows below use authenticated workload identities, deterministic protocol
version 1 evidence and no public Gateway route unless stated.

| Caller | Callee / contract | Result |
|---|---|---|
| Gateway/user | Auth `IssueSpaceDeletionProof` | public issue only; password and enabled 2FA; opaque five-minute proof |
| Space | Auth `ConsumeSpaceDeletionProof`, `GetSpaceDeletionProofReceipt`, `AcknowledgeSpaceDeletionProofReceipt` | exact consume recovery and persistence acknowledgement; no other caller |
| Space | Role `RetireSpace` | permanent retirement receipt after `PURGE_DECIDED` |
| Space | Chat `ApplySpaceLifecycleFence`, manifest prepare/page, `PurgeSpace` | first freeze linearization, exact chat pages, terminal cleanup |
| Space | Messaging `ApplySpaceLifecycleFence`, manifest import/page acknowledgement, `PurgeSpace` | bounded message purge and File release handoff |
| Space | File `ApplySpaceLifecycleFence`, prepare/register/seal producer manifests, `PurgeSpace` | exact reference denial/release and durable GC handoff |
| Space | Voice, Matchmaking, Search, Subscription, Bot, Notification `ApplySpaceLifecycleFence`, `PurgeSpace` | service-owned deny/cleanup receipt for every fixed participant |
| Space/Chat/Messaging | File reference acquire/release and producer registration | caller identity fixes allowed producer/owner type |
| Messaging/Chat/Story/User (reference owner) | File `IssueFileAccessCapability` | exact live reference, authenticated subject, sorted URL/metadata surfaces, at most one hour |
| Gateway/public File routes | File URL/metadata/bulk reads | subject-bound capability required after activation; no file_id-only authority |

Participant IDs are fixed: Role=1, Chat=2, Messaging=3, File=4, Voice=5,
Matchmaking=6, Search=7, Subscription=8, Bot=9, Notification=10. Every fence,
purge request and receipt binds canonical Space/deletion IDs, positive
generation and exact root manifest; Role uses its stricter retirement wire.

| Subject / payload | Producer | Consumers / rule |
|---|---|---|
| `space.deletion_scheduled` / `SpaceDeletionScheduled` v1 | Space after full FROZEN barrier | generation-aware projections; notification only |
| `space.restored` / `SpaceRestored` v1 | Space after full LIVE barrier | generation-aware projections; notification only |
| `space.deleted` / additive `SpaceDeleted` v2 | Space after ten purge receipts plus tombstone/local purge | Realtime and projections; legacy field 1 remains `space_id` |

The events stay in `chat.events`; direct participant receipts prove convergence.
`ChatStreamEvent` retains `event_id=1`, `occurred_at=2`. NATS delivery is
at-least-once with `Nats-Msg-Id=event_id`; consumer inbox and generation state
provide logical dedup. Unknown payload/protocol and same-generation changed bytes
are contract mismatch and are not ACKed as success.
