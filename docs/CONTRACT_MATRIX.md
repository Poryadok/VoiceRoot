# Матрица контрактов: Gateway → сервисы и NATS JetStream

Единая сводка **публичных** маршрутов клиента (REST/WebSocket через API Gateway) и **доменных потоков** JetStream. При изменении маршрутов или издателей/подписчиков событий обновляйте этот файл **и** каноничные разделы в [microservices/api-gateway.md](microservices/api-gateway.md) и [MICROSERVICES.md](MICROSERVICES.md) (раздел «Event Bus»), чтобы они не расходились.

Чеклист перед мержем: [DOCS_CONSISTENCY_AUDIT.md](DOCS_CONSISTENCY_AUDIT.md).

---

## Клиент → API Gateway → gRPC (REST namespaces)

Транскодинг HTTP → gRPC; детали RPC — в `docs/microservices/<service>.md`.

| HTTP prefix (`/api/v1/...`) | Целевой сервис        | Примечание |
|----------------------------|------------------------|------------|
| `auth/**`                  | Auth Service           | Публичные login/register — без JWT (см. api-gateway) |
| `users/**`                 | User Service           | |
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

| Путь   | Назначение |
|--------|------------|
| `/ws`  | Upgrade и прокси на **Realtime Service** (live-события, `s` / `resume`); см. [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) |

## Вне публичного Gateway

| Клиент / граница | Протокол | Сервис |
|------------------|----------|--------|
| Клиенты Flutter  | —        | **Federation Service** не вызывается с клиента; S2S gRPC / отдельный ingress, см. [microservices/federation-service.md](microservices/federation-service.md) |

Типичные **синхронные** gRPC между сервисами (не через Gateway) и async-паттерн для ядра чата — таблица «Tier 0: типичные связи» в [MICROSERVICES.md](MICROSERVICES.md).

---

## NATS JetStream: streams, publishers, subscribers

**Владелец схемы событий** для stream — сервис из колонки Publishers. Breaking changes в payload / `.proto` — порядок выката [REPOSITORIES.md](REPOSITORIES.md).

| Stream                 | Publishers        | Subscribers |
|------------------------|-------------------|-------------|
| `user.events`          | Auth, User        | Analytics, Social, Notification, Federation |
| `social.events`        | Social            | Analytics, Notification, Chat, Federation |
| `role.events`          | Role              | Analytics, Notification, Federation, Realtime |
| `message.events`       | Messaging         | Analytics, Notification, Search, Moderation |
| `chat.events`          | Chat, Space       | Analytics, Notification, Realtime |
| `voice.events`         | Voice             | Analytics, Notification |
| `moderation.events`    | Moderation        | Analytics, Notification, User |
| `subscription.events`  | Subscription      | Analytics, User, Space, File |
| `file.events`          | File              | Analytics, Messaging (preview update) |
| `matchmaking.events`   | Matchmaking       | Analytics, Notification, Voice, Chat |
| `story.events`         | Story             | Analytics, Notification, Matchmaking |
| `federation.events`    | Federation        | Analytics, Role, Moderation |
| `bot.events`           | Bot               | Analytics, Messaging |

Продуктовая аналитика дополнительно консьюмит subject’ы вида `analytics.*` (см. раздел «Аналитика» в [MICROSERVICES.md](MICROSERVICES.md)).
