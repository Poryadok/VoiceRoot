# API Gateway

## Обзор

Единая точка входа для всех клиентских запросов. Маршрутизирует REST и WebSocket трафик к внутренним сервисам, применяет сквозные политики безопасности и rate limiting.

**Язык**: Go
**Фреймворк**: chi / echo (custom) или Kong

## Ответственность

- Маршрутизация HTTP/REST запросов к соответствующим сервисам; текущая Go-реализация — HTTP reverse proxy, REST → gRPC transcoding добавляется вместе с целевыми сервисами
- Проксирование WebSocket-соединений к Realtime Service
- JWT-валидация (проверка access token, извлечение claims) и чтение Redis blacklist для отозванных access token
- Rate limiting по правилам из конфигурации
- CORS, request logging, Prometheus metrics (`/metrics`)
- Версионирование API (`/api/v1/...`)
- Проверка версии клиента (endpoint `/api/v1/version`, ответ 426 при force_update; Gateway — canonical owner этой политики)
- TLS termination
- Request ID генерация и propagation (для трейсинга)

## Rate Limiting

Правила (из ARCHITECTURE_REQUIREMENTS.md):

| Endpoint группа       | Лимит         | Окно   |
|-----------------------|---------------|--------|
| Auth (login/register) | 5 запросов    | 15 мин |
| OTP                   | 3 запроса     | 10 мин |
| Messages (send)       | 5 сообщений   | 5 сек  |
| File upload           | 10 загрузок   | 1 час  |
| Space creation        | 5 пространств | 1 день |
| Bot API               | 5000 запросов | 1 мин  |

В реализации Gateway группа **File upload** также покрывает `POST /api/v1/users/me/avatar/presigned-upload` (выдача presigned PUT для статичного аватара — [user-profile.md](../features/user-profile.md); см. ниже).

Реализация: Redis sliding window counter. Для публичных маршрутов ключ строится по IP; `X-Forwarded-For` учитывается только от доверенных proxy из `GATEWAY_TRUSTED_PROXY_CIDRS`. Для защищённых маршрутов ключ строится по `user_id`.

## Маршрутизация

```
/api/v1/auth/**          → Auth Service
/api/v1/users/**         → User Service
/api/v1/friends/**       → Social Service
/api/v1/chats/**         → Chat Service
/api/v1/messages/**      → Messaging Service
/api/v1/spaces/**        → Space Service
/api/v1/roles/**         → Role Service
/api/v1/voice/**         → Voice Service
/api/v1/files/**         → File Service
/api/v1/notifications/** → Notification Service
/api/v1/search/**        → Search Service
/api/v1/matchmaking/**   → Matchmaking Service
/api/v1/moderation/**    → Moderation Service
/api/v1/subscription/**  → Subscription Service
/api/v1/bots/**          → Bot Service
/api/v1/stories/**       → Story Service
/api/v1/analytics/**     → Analytics Service (только персонал; см. раздел ниже)
/api/v1/version          → Локальный конфиг (version check)
/ws                      → Realtime Service (WebSocket upgrade)
```

**[user-profile.md](../features/user-profile.md) — presigned аватар (R2, User Service, без File Service):** `POST /api/v1/users/me/avatar/presigned-upload` (JWT). Тело JSON: `content_type`, `content_length`; `profile_id` опционален (по умолчанию активный профиль из JWT → `X-Voice-Profile-Id`). Ответ — поля `upload_url`, `http_method`, `required_headers`, `expires_at`, `public_url` / `object_key` для последующего `PUT` в R2 и сохранения URL через `PATCH /api/v1/users/me` (`UpdateProfile.avatar_url`). Обход REST: тот же контракт по **gRPC** `UserService.CreateAvatarPresignedUpload` на User Service (внутренний ingress, непубличные клиенты), если edge Gateway недоступен.

**[voice-chat.md](../features/voice-chat.md) — DM-звонки через Voice Service + LiveKit:** namespace `POST/GET /api/v1/voice/**` транскодится в `VoiceService` ([voice-service.md](voice-service.md)). Клиент не отправляет WebRTC `offer/answer/ICE` в Gateway: media signaling идёт внутри LiveKit SDK; Gateway управляет только lifecycle и выдачей токена. Минимальные публичные маршруты:

| Method | Route | gRPC | Тело / параметры |
|--------|-------|------|------------------|
| `POST` | `/api/v1/voice/calls` | `StartCall` | `linked_chat`, `callee_profile_id`, `media_kind` (`audio` \| `video`) |
| `POST` | `/api/v1/voice/calls/{room_id}/accept` | `AcceptCall` | — |
| `POST` | `/api/v1/voice/calls/{room_id}/decline` | `DeclineCall` | — |
| `POST` | `/api/v1/voice/calls/{room_id}/join` | `JoinCall` | — |
| `POST` | `/api/v1/voice/calls/{room_id}/leave` | `LeaveCall` | — |
| `POST` | `/api/v1/voice/calls/{room_id}/end` | `EndCall` | — |
| `GET` | `/api/v1/voice/calls/active` | `GetActiveCall` | — |
| `GET` | `/api/v1/voice/calls/{room_id}/token` | `GetJoinToken` | — |
| `PATCH` | `/api/v1/voice/calls/{room_id}/state` | `UpdateVoiceState` | `is_muted`, `is_deafened`, `is_video_on` |
| `GET` | `/api/v1/voice/calls/{room_id}/states` | `GetVoiceStates` | — |

**Не через этот REST-префикс:** [Federation Service](federation-service.md) (S2S gRPC, отдельный ingress / mTLS). Публичные Flutter-клиенты не вызывают Analytics.

**[bots.md](../features/bots.md) — Bot API:** namespace `GET/POST/PATCH/DELETE /api/v1/bots/**` транскодится в `BotService` ([bot-service.md](bot-service.md)). Реализация: `transcode_bots.go`. Два режима auth:

- **JWT** (`Authorization: Bearer …`) — портал, клиент, install/uninstall, slash autocomplete/interactions.
- **Bot token** (`Authorization: Bot <token>`) — маршруты `…/bots/me/**` (polling, defer/complete interaction, send/edit message).

| Method | Route | gRPC | Auth | Тело / query |
|--------|-------|------|------|----------------|
| `POST` | `/api/v1/bots` | `RegisterBot` | JWT | `name`, `description`, `scopes_json` |
| `GET` | `/api/v1/bots` | `ListBots` | JWT | — |
| `GET` | `/api/v1/bots/{bot_id}` | `GetBot` | JWT | — |
| `PATCH` | `/api/v1/bots/{bot_id}` | `UpdateBot` | JWT | partial `Bot` fields |
| `DELETE` | `/api/v1/bots/{bot_id}` | `DeleteBot` | JWT | — |
| `GET` | `/api/v1/bots/slug/{slug}` | `GetBotBySlug` | JWT | публичный lookup по `slug` (deep link `voice.gg` / `voice.app`) |
| `POST` | `/api/v1/bots/{bot_id}/token/regenerate` | `RegenerateToken` | JWT | — |
| `POST` | `/api/v1/bots/{bot_id}/webhook-secret/regenerate` | `RegenerateWebhookSecret` | JWT | one-shot `webhook_secret` |
| `POST` | `/api/v1/bots/{bot_id}/manifest` | `ApplyManifest` | JWT | `manifest_yaml` |
| `POST` | `/api/v1/bots/manifest/validate` | `ValidateManifest` | JWT | `manifest_yaml` |
| `GET` | `/api/v1/bots/{bot_id}/webhook` | `GetWebhookURL` | JWT | — |
| `PATCH` | `/api/v1/bots/{bot_id}/webhook` | `SetWebhookURL` | JWT | `url` |
| `POST` | `/api/v1/bots/{bot_id}/spaces/{space_id}/install` | `InstallBotInSpace` | JWT | `allowed_chats`, `acknowledge_privileged_scopes` |
| `DELETE` | `/api/v1/bots/{bot_id}/spaces/{space_id}` | `UninstallBotFromSpace` | JWT | — |
| `GET` | `/api/v1/bots/spaces/{space_id}/installed` | `ListInstalledBots` | JWT | `InstalledBot.online` из `bot_presence` |
| `GET` | `/api/v1/bots/chats/{chat_id}` | `ListBotsInChat` | JWT | `space_id`, `chat_type` (default `CHAT_TYPE_CHANNEL`) |
| `PATCH` | `/api/v1/bots/{bot_id}/chats/{chat_id}/enabled` | `SetBotChatEnabled` | JWT | `enabled`, optional `chat`, `space_id` |
| `GET` | `/api/v1/bots/commands` | `ListSlashCommandsForChat` | JWT | `chat_id`, `chat_type`; `online` всегда в JSON (`EmitUnpopulated`) |
| `POST` | `/api/v1/bots/interactions` | `ExecuteSlashInteraction` | JWT | `chat`, `bot_id`, `command_name`, `options_json` |
| `POST` | `/api/v1/bots/autocomplete` | `AutocompleteSlashOption` | JWT | `chat`, `bot_id`, `command_name`, `option_name`, `focused_value`, `options_json` |
| `GET` | `/api/v1/bots/me/interactions/poll` | `PollEvents` (stream → JSON array) | Bot token | — |
| `POST` | `/api/v1/bots/me/interactions/defer` | `DeferResponse` | Bot token | `interaction_token` |
| `POST` | `/api/v1/bots/me/interactions/complete` | `CompleteInteraction` | Bot token | `interaction_token`, `content`, `is_ephemeral`, `deferred` |
| `POST` | `/api/v1/bots/me/messages` | `SendBotMessage` | Bot token | `chat`, `content`, optional `thread_parent_id`, `interaction_token` |
| `POST` | `/api/v1/bots/me/messages/ephemeral` | `SendEphemeral` | Bot token | `chat`, `target_profile_id`, `content` |
| `PATCH` | `/api/v1/bots/me/messages/{message_id}` | `EditBotMessage` | Bot token | `content` |
| `POST` | `/api/v1/bots/me/presence` | `TouchPresence` | Bot token | — |
| `GET` | `/api/v1/bots/me/spaces/{space_id}/members` | `ListSpaceMembersForBot` | Bot token | optional `cursor` |
| `POST` | `/api/v1/bots/me/spaces/{space_id}/roles/assign` | `AssignBotRole` | Bot token | `profile_id`, `role_id` |
| `POST` | `/api/v1/bots/me/spaces/{space_id}/roles/revoke` | `RevokeBotRole` | Bot token | `profile_id`, `role_id` |
| `POST` | `/api/v1/bots/me/chats` | `CreateBotChat` | Bot token | `space_id`, `name`, `chat_type` |
| `GET` | `/api/v1/bots/me/chats/{chat_id}/messages` | `GetChatMessagesForBot` | Bot token | `chat_type`, optional `cursor` |
| `POST` | `/api/v1/bots/me/roles` | `CreateBotRole` | Bot token | `space_id`, `name`, `permissions_mask`, `position` |
| `POST` | `/api/v1/bots/me/autocomplete/complete` | `CompleteAutocomplete` | Bot token | `request_id`, `choices` |

**Rate limits (BOT-C):** `BotRoleOps` — 100/min per bot token on `roles/assign`, `roles/revoke`, `POST /me/roles`.

## Маршруты персонала (Admin API)

`/api/v1/analytics/**` — для **React Admin Panel** и внутренних операторов. После валидации JWT Gateway проверяет, что в claims есть роль персонала (набор имён и источник истины — Auth / Role; например платформенный staff и/или доступ к модераторской панели). Без этого — **403 Forbidden**. Все вызовы с чувствительными отчётами и **export** должны писаться в audit log (subject, маршрут, время) на стороне Analytics или общего аудита.

## Канонический формат клиентских API-доков

`api-gateway.md` фиксирует публичные namespace/route-группы. Табличная сводка маршрутов ↔ целевых сервисов и потоков NATS — [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md). Детальная предметная семантика описывается в документах целевых сервисов (`docs/microservices/*`).

Для каждого публичного endpoint документация должна содержать:
- HTTP method + route + auth requirement
- request/response schema (обязательные поля и типы)
- error model (status code + `error_code`)
- pagination/курсоры (где нужно)
- idempotency/повтор запроса (где нужно)

## Аутентификация

1. Клиент отправляет `Authorization: Bearer <access_token>`
2. Gateway валидирует JWT через JWKS (`GATEWAY_JWKS_URL`), issuer и audience (`GATEWAY_JWT_ISSUER`, `GATEWAY_JWT_AUDIENCE`)
3. Проверяет Redis blacklist по `jti` (`GATEWAY_JWT_BLACKLIST_PREFIX`, по умолчанию `jwt:blacklist:`)
4. Извлекает claims: `sub`/`user_id`, `profile_id`, `roles`, `subscription_tier`, `jti`
5. Передаёт claims downstream сервисам через `X-Voice-*` headers
6. Публичные endpoints (login, register, OTP, version, health, metrics) — без JWT

Для dev/tests допускается `GATEWAY_AUTH_MODE=static` + `GATEWAY_STATIC_TOKENS_JSON`; production должен использовать JWKS.

## Конфигурация Gateway

| Переменная | Назначение |
|------------|------------|
| `GATEWAY_JWKS_URL` | JWKS endpoint Auth Service |
| `GATEWAY_JWT_ISSUER`, `GATEWAY_JWT_AUDIENCE` | Проверка `iss` и `aud` |
| `GATEWAY_REDIS_ADDR`, `GATEWAY_REDIS_PASSWORD` | Redis для rate limit и JWT blacklist |
| `GATEWAY_JWT_BLACKLIST_PREFIX` | Prefix blacklist ключей; default `jwt:blacklist:` |
| `GATEWAY_TRUSTED_PROXY_CIDRS` | CIDR/IP список proxy, от которых принимается `X-Forwarded-For` |
| `GATEWAY_CORS_ALLOWED_ORIGINS` | CSV allowlist browser origins; default deny |
| `GATEWAY_REST_UPSTREAMS_JSON` / `GATEWAY_<NAMESPACE>_UPSTREAM_URL` | REST upstream routes |
| `GATEWAY_REALTIME_UPSTREAM_URL` | `/ws` upstream Realtime Service |
| `GATEWAY_VERSION_CONFIGS_JSON`, `GATEWAY_FORCE_UPDATE_JSON` | Version policy |

## Зависимости

- **Redis** — rate limiting (sliding window), чтение JWT blacklist. Зона ответственности с **Auth Service**: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (раздел «Redis: API Gateway и Auth Service»).
- **Auth Service** — JWT public key (ротация через JWKS endpoint)
- **Version config store** — таблица `client_versions` (или эквивалентный конфиг-стор) для `/api/v1/version`

## Метрики (→ Analytics)

- `gateway_request_count` — Prometheus counter по route group, method, status code
- `gateway_request_latency_ms_sum` — суммарная latency в миллисекундах по route group, method, status code
- `gateway_ratelimit_hit` — заблокированные запросы по группе лимита
- `gateway.ws.connections` — текущие WebSocket соединения

## Масштабирование

Stateless, масштабируется горизонтально. За внешним Load Balancer (L4/L7).

