# Микросервисная архитектура Voice

## Обзор

Voice — распределённая система из 20 микросервисов, API Gateway и набора инфраструктурных компонентов.

| #  | Сервис                | Язык | Назначение                                         | Детали |
|----|----------------------|------|----------------------------------------------------|--------|
| 1  | API Gateway          | Go   | Маршрутизация, rate limiting, JWT-валидация         | [подробнее](microservices/api-gateway.md) |
| 2  | Auth Service         | Java | Регистрация, логин, JWT, 2FA, гостевые аккаунты    | [подробнее](microservices/auth-service.md) |
| 3  | User Service         | Go   | Профили, мульти-профили, настройки, приватность     | [подробнее](microservices/user-service.md) |
| 4  | Social Service       | Go   | Друзья, контакты, блокировки                       | [подробнее](microservices/social-service.md) |
| 5  | Chat Service         | Go   | DM и группы — создание, участники, папки           | [подробнее](microservices/chat-service.md) |
| 6  | Messaging Service    | Go   | Сообщения, треды, реакции, пины, пересылка         | [подробнее](microservices/messaging-service.md) |
| 7  | Realtime Service     | Go   | WebSocket-шлюз, fan-out событий, typing indicators | [подробнее](microservices/realtime-service.md) |
| 8  | Space Service        | Go   | Пространства, каналы, категории, инвайты           | [подробнее](microservices/space-service.md) |
| 9  | Role Service         | Go   | Роли, права, иерархия, канальные оверрайды         | [подробнее](microservices/role-service.md) |
| 10 | Voice Service        | Go   | Голос, видео, screen share, LiveKit                | [подробнее](microservices/voice-service.md) |
| 11 | File Service         | Go   | Загрузка, хранение, конвертация, антивирус         | [подробнее](microservices/file-service.md) |
| 12 | Notification Service | Go   | FCM, APNs, email, push-роутинг                    | [подробнее](microservices/notification-service.md) |
| 13 | Search Service       | Go   | Полнотекстовый поиск, глобальный поиск             | [подробнее](microservices/search-service.md) |
| 14 | Matchmaking Service  | Go   | Каталог игр, очереди, матчинг, рейтинги            | [подробнее](microservices/matchmaking-service.md) |
| 15 | Moderation Service   | Go   | Жалобы, авто-модерация, санкции, апелляции         | [подробнее](microservices/moderation-service.md) |
| 16 | Subscription Service | Go   | Биллинг, Paddle, CloudPayments, лимиты             | [подробнее](microservices/subscription-service.md) |
| 17 | Bot Service          | Go   | Реестр ботов, вебхуки, slash-команды               | [подробнее](microservices/bot-service.md) |
| 18 | Federation Service   | Go   | S2S gRPC, синхронизация, федеративные ноды         | [подробнее](microservices/federation-service.md) |
| 19 | Story Service        | Go   | Сторис, хайлайты, архив, "ищу пати"               | [подробнее](microservices/story-service.md) |
| 20 | Analytics Service    | Go   | Сбор событий, метрики, дашборды                    | [подробнее](microservices/analytics-service.md) |

## Архитектурная диаграмма

```
                         ┌─────────────────┐
                         │   Clients        │
                         │ Flutter / React  │
                         └────────┬─────────┘
                                  │
                         ┌────────▼─────────┐
                         │   API Gateway     │
                         │  (Go, REST/WS)    │
                         └──┬──┬──┬──┬──┬───┘
                            │  │  │  │  │
              ┌─────────────┘  │  │  │  └─────────────┐
              │                │  │  │                │
     ┌────────▼───┐   ┌───────▼──▼──▼───────┐  ┌─────▼──────────┐
     │ Auth (Java) │   │   Core Services     │  │ Realtime (Go)  │
     │ JWT, 2FA    │   │   (Go, gRPC)        │  │ WebSocket GW   │
     └─────────────┘   │                     │  └──────┬─────────┘
                        │ User · Social      │         │
                        │ Chat · Messaging   │    ┌────▼─────┐
                        │ Space · Role       │    │ Redis    │
                        │ Voice · File       │    │ Pub/Sub  │
                        │ Notification       │    └──────────┘
                        │ Search · MM        │
                        │ Moderation         │
                        │ Subscription       │
                        │ Bot · Story        │
                        └────────┬───────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
     ┌────────▼───┐    ┌────────▼───┐    ┌─────────▼──────┐
     │ PostgreSQL  │    │   Redis     │    │ Event Bus      │
     │ per-service │    │ cache/rate  │    │ NATS JetStream │
     └─────────────┘    └────────────┘    └────────────────┘
              │
     ┌────────▼───────────────────────────────────────┐
     │                Infrastructure                   │
     │  LiveKit · Cloudflare R2 · ClamAV · Meilisearch │
     │  Paddle · CloudPayments · Resend · FCM · APNs   │
     └─────────────────────────────────────────────────┘
              │
     ┌────────▼───┐    ┌─────────────────┐
     │ Federation  │◄──►│ External Nodes  │
     │ Service     │    │ (S2S gRPC)      │
     └─────────────┘    └─────────────────┘
              │
     ┌────────▼───┐
     │ Analytics   │
     │ ClickHouse  │
     └─────────────┘
```

## Стек технологий

| Компонент            | Технология                          |
|----------------------|-------------------------------------|
| Auth Service         | Java 25 LTS, Spring Boot 3.5, Spring Security 6 |
| Все остальные сервисы | Go 1.26+                           |
| Flutter клиент       | Flutter 3.41+ (mobile, desktop, web) |
| Web-админка          | React 19, Vite 7, TypeScript 5     |
| API Gateway          | Custom Go (chi/echo) или Kong Gateway 3.x |
| База данных          | PostgreSQL 18 (по БД на сервис)     |
| Кэш / Pub/Sub       | Redis 8 (Cluster)                  |
| Event Bus            | NATS Server 2.12+ (JetStream)      |
| Объектное хранилище  | Cloudflare R2 (S3-совместимое)     |
| Поиск (v1)           | PostgreSQL tsvector + GIN          |
| Поиск (v2+)          | Meilisearch 1.40+ → Elasticsearch 9.x |
| Аналитика (OLAP)     | ClickHouse 26.x LTS                |
| Голос/Видео          | LiveKit Server 1.10+ (SFU)         |
| Антивирус            | ClamAV                             |
| Оркестрация          | k3s v1.35+ (staging), Kubernetes 1.35 (prod) |
| CI/CD                | GitHub Actions                     |
| Мониторинг           | Prometheus 3.x + Grafana 12        |
| Трейсинг             | OpenTelemetry Collector 0.148+ + Jaeger |
| Логи                 | Grafana Loki 3.7+ (или ELK)        |

Процесс: Git и PR — [CONTRIBUTING.md](CONTRIBUTING.md); тесты и CI — [TESTING.md](TESTING.md); окружения и выкат — [DEPLOYMENT.md](DEPLOYMENT.md); репозитории и protos — [REPOSITORIES.md](REPOSITORIES.md).

## Протоколы коммуникации

```
Client ──REST/JSON──► API Gateway ──gRPC──► Микросервисы
Client ──WebSocket──► Realtime Service ──Redis Pub/Sub──► Realtime (другие инстансы)
Микросервис ──gRPC──► Микросервис (синхронные вызовы)
Микросервис ──NATS──► Микросервис (асинхронные события)
Federation ──gRPC bidirectional stream──► External Node
```

- **Client → Gateway**: REST/JSON (CRUD), WebSocket (real-time)
- **Gateway → Services**: gRPC с protobuf
- **Service ↔ Service**: gRPC (синхронно), NATS JetStream (асинхронно)
- **Realtime fan-out**: Redis Pub/Sub между инстансами WebSocket-шлюза
- **Reconnect (клиент)**: WebSocket `s` / `resume` в Realtime и догрузка сообщений курсором в Messaging — единое описание в [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) (раздел «Reconnect: WebSocket-поток и история сообщений»)
- **Federation**: gRPC bidirectional stream (см. `protos/s2s.proto`)

## Event Bus (NATS JetStream)

Асинхронная шина событий для decoupled-коммуникации между сервисами.

### Ключевые потоки (streams)

| Stream               | Publishers                     | Subscribers                                  |
|----------------------|-------------------------------|----------------------------------------------|
| `user.events`        | Auth, User                    | Analytics, Social, Notification, Federation  |
| `message.events`     | Messaging                     | Analytics, Notification, Search, Moderation  |
| `chat.events`        | Chat, Space                   | Analytics, Notification, Realtime            |
| `voice.events`       | Voice                         | Analytics, Notification                      |
| `moderation.events`  | Moderation                    | Analytics, Notification, User                |
| `subscription.events`| Subscription                  | Analytics, User, Space, File                 |
| `file.events`        | File                          | Analytics, Messaging (preview update)        |
| `matchmaking.events` | Matchmaking                   | Analytics, Notification, Voice, Chat         |
| `story.events`       | Story                         | Analytics, Notification, Matchmaking         |
| `federation.events`  | Federation                    | Analytics, Role, Moderation                  |
| `bot.events`         | Bot                           | Analytics, Messaging                         |

## Аналитика

### Источники данных

Каждый микросервис публикует события в `NATS → analytics.*` subject. Analytics Service консьюмит все потоки и записывает в ClickHouse.

### Категории событий

| Категория        | Примеры событий                                                      |
|------------------|----------------------------------------------------------------------|
| Auth             | register, login, logout, 2fa_enabled, guest_converted               |
| Users            | profile_created, profile_switched, settings_changed, presence_change |
| Social           | friend_added, friend_removed, contact_synced, user_blocked           |
| Messaging        | message_sent, message_edited, message_deleted, reaction_added        |
| Voice            | call_started, call_ended, call_duration, screen_share_started        |
| Spaces           | space_created, space_joined, space_left, channel_created             |
| Matchmaking      | search_started, match_found, match_timeout, rating_submitted         |
| Files            | file_uploaded, file_downloaded, file_converted, file_scan_result     |
| Moderation       | report_created, sanction_applied, appeal_submitted                   |
| Subscription     | plan_started, plan_cancelled, payment_success, payment_failed        |
| Stories          | story_created, story_viewed, highlight_added                         |
| Bots             | bot_registered, command_executed, webhook_delivered                   |
| Federation       | node_connected, node_disconnected, event_synced                      |
| Notifications    | push_sent, push_delivered, push_clicked                              |
| Search           | query_executed, result_clicked, zero_results                         |
| Performance      | api_latency, ws_connections, error_rate                              |

### Дашборды

- **Product**: DAU/MAU, retention (D1/D7/D30), funnel регистрации, onboarding completion
- **Engagement**: сообщения/день, голосовые минуты, matchmaking сессии, активные пространства
- **Revenue**: MRR, churn rate, conversion free→paid, ARPU
- **Health**: p50/p95/p99 latency, error rate, WebSocket connections, uptime
- **Moderation**: жалобы/день, время реакции, auto-block rate, appeals
- **Federation**: подключённые ноды, event lag, sync failures

## Владение данными (Database per Service)

Каждый сервис владеет своей базой данных. Другие сервисы обращаются к данным только через gRPC API владельца.

| Сервис              | БД / Хранилище                          |
|---------------------|----------------------------------------|
| Auth Service        | PostgreSQL `auth_db`                   |
| User Service        | PostgreSQL `user_db`, Redis (presence) |
| Social Service      | PostgreSQL `social_db`                 |
| Chat Service        | PostgreSQL `chat_db`                   |
| Messaging Service   | PostgreSQL `messaging_db`              |
| Space Service       | PostgreSQL `space_db`                  |
| Role Service        | PostgreSQL `role_db`                   |
| Voice Service       | Redis (active sessions), LiveKit       |
| File Service        | PostgreSQL `file_db`, Cloudflare R2    |
| Notification Service| PostgreSQL `notification_db`, Redis    |
| Search Service      | PostgreSQL (v1) / Meilisearch (v2)     |
| Matchmaking Service | PostgreSQL `matchmaking_db`, Redis (queues) |
| Moderation Service  | PostgreSQL `moderation_db`             |
| Subscription Service| PostgreSQL `subscription_db`           |
| Bot Service         | PostgreSQL `bot_db`                    |
| Federation Service  | PostgreSQL `federation_db`             |
| Story Service       | PostgreSQL `story_db`, R2 (медиа)     |
| Analytics Service   | ClickHouse, Redis (буфер)             |

## Масштабирование

### Горизонтальное масштабирование

Все сервисы stateless (состояние в БД/Redis/R2) → масштабируются горизонтально через Kubernetes HPA.

### Точки давления и стратегии

| Компонент         | Стратегия масштабирования                        |
|-------------------|--------------------------------------------------|
| Realtime Service  | N инстансов, Redis Pub/Sub для fan-out           |
| Messaging Service | Шардинг PostgreSQL по chat_id (при >100M сообщений) |
| Search Service    | PostgreSQL → Meilisearch → Elasticsearch; **когда переключать** — пороговая матрица в [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) |
| File Service      | R2 безлимитный egress, воркеры конвертации отдельно |
| Voice Service     | LiveKit масштабируется независимо (SFU per region) |
| Analytics Service | ClickHouse кластер, батч-запись через буфер; **когда усложнять** — [ARCHITECTURE_REQUIREMENTS.md](ARCHITECTURE_REQUIREMENTS.md) (аналитика, пороги) |
| Matchmaking       | Redis-очереди, горизонтальный матчер             |

## Отказоустойчивость

- **Circuit breaker** на всех gRPC-вызовах между сервисами
- **Retry с exponential backoff** для NATS и внешних API
- **Graceful degradation**: поиск, аналитика, сторис — некритичные, деградируют без остановки core
- **Порядок деградации и целевые SLO** по пользовательским путям, canary/rollback, владение миграциями БД — [OPERATIONS.md](OPERATIONS.md) (Tier 0: Gateway, Auth, User, Chat, Messaging, Realtime + критичный Redis/NATS; Tier 1/2 — см. там)
- **Health checks**: liveness + readiness пробы для каждого сервиса
- **Dead letter queue**: NATS DLQ для необработанных событий

## Безопасность

- TLS везде (HTTPS, WSS, gRPC с mTLS между сервисами)
- JWT-валидация на API Gateway, передача claims в gRPC metadata
- Сервисы доверяют Gateway, не принимают внешний трафик напрямую
- Rate limiting на Gateway (по конфигурации из ARCHITECTURE_REQUIREMENTS.md)
- ClamAV для загружаемых файлов
- HMAC-SHA256 для bot webhook verification
- E2E шифрование (Signal Protocol) — опциональное для DM, терминируется на клиентах

## Клиенты

| Клиент        | Технология              | Назначение            |
|---------------|-------------------------|-----------------------|
| Mobile App    | Flutter 3.41+           | Android, iOS          |
| Desktop App   | Flutter 3.41+           | Windows, macOS, Linux |
| Web App       | Flutter Web 3.41+       | Браузер               |
| Admin Panel   | React 19, Vite 7        | Модерация, аналитика  |
