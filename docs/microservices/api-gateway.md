# API Gateway

## Обзор

Единая точка входа для всех клиентских запросов. Маршрутизирует REST и WebSocket трафик к внутренним сервисам, применяет сквозные политики безопасности и rate limiting.

**Язык**: Go
**Фреймворк**: chi / echo (custom) или Kong

## Ответственность

- Маршрутизация HTTP/REST запросов к соответствующим сервисам; текущая Go-реализация — HTTP reverse proxy, REST → gRPC transcoding добавляется вместе с целевыми сервисами
- Проксирование WebSocket-соединений к Realtime Service
- JWT-валидация (проверка access token, извлечение claims) и чтение Redis blacklist для отозванных access token
- T056-P1 session-epoch enforcement: чтение Auth-owned Redis floor и fail-closed strict-проверка
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
POST /api/v1/realtime/ws-ticket → Gateway (short-lived WS ticket; JWT в заголовке)
/ws                      → Realtime Service (WebSocket upgrade; `Authorization` или `?ticket=`)
```

**WebSocket auth:** нативные клиенты — `Authorization: Bearer` на upgrade. Браузер — `POST /api/v1/realtime/ws-ticket` (JWT только в REST), затем `/ws?ticket=…` (opaque, single-use, Redis TTL ~60s). Legacy `?access_token=` на `/ws` поддерживается для совместимости; web-клиент не использует. См. [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md).


**[user-profile.md](../features/user-profile.md) — presigned аватар (R2, User Service, без File Service):** `POST /api/v1/users/me/avatar/presigned-upload` (JWT). Тело JSON: `content_type`, `content_length`; `profile_id` опционален (по умолчанию активный профиль из JWT → `X-Voice-Profile-Id`). Ответ — поля `upload_url`, `http_method`, `required_headers`, `expires_at`, `public_url` / `object_key` для последующего `PUT` в R2 и сохранения URL через `PATCH /api/v1/users/me` (`UpdateProfile.avatar_url`). Обход REST: тот же контракт по **gRPC** `UserService.CreateAvatarPresignedUpload` на User Service (внутренний ingress, непубличные клиенты), если edge Gateway недоступен.

**Phase-0 Space voice-room target routes (not implemented):** the canonical Space
room surface is owned by Voice and is listed in
[voice-service.md](voice-service.md#phase-0-space-room-media-roster-и-lifecycle-target-не-реализовано).
Gateway binds `space_id` and `voice_room_id` from the path, derives the actor only
from verified JWT, attaches its delegated-user credential, redacts it in logs, and
preserves the frozen `401/404/403/409/503` + `error_code` disclosure mapping. It
must not forward current client `space.id` assertions as authority.

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

**[text-chat.md](../features/text-chat.md) — Stickers and GIF (Chat catalog + provider search):** namespace under **`/api/v1/chats/**`** sticker/GIF routes transcoded to **Chat Service** ([chat-service.md](chat-service.md) § Sticker packs). **Not yet in Gateway code.**

| Method | Route | gRPC | Query / body |
|--------|-------|------|--------------|
| `GET` | `/api/v1/sticker-packs` | `ListInstalledStickerPacks` | — |
| `GET` | `/api/v1/sticker-packs/{pack_id}` | `GetStickerPack` | — |
| `POST` | `/api/v1/sticker-packs/{pack_id}/install` | `InstallStickerPack` | — |
| `DELETE` | `/api/v1/sticker-packs/{pack_id}` | `UninstallStickerPack` | user packs only (`is_system` → 403) |
| `POST` | `/api/v1/sticker-packs/user` | `CreateUserStickerPack` | `title` |
| `POST` | `/api/v1/sticker-packs/{pack_id}/stickers` | `AddStickersToUserPack` | per-sticker after File `ConfirmUpload` |
| `GET` | `/api/v1/gifs/search` | `SearchGifs` | `q`, `limit`, **`cursor`** (opaque; Chat-owned `next_cursor`) |
| `GET` | `/api/v1/gifs/trending` | `GetTrendingGifs` | optional `cursor` |

Send after pick — **Messaging** `POST /api/v1/messages/...` (not File attach). File upload for pack assets — `/api/v1/files/**` with `intent=sticker`.

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
4. В strict-режиме после успешной проверки non-empty `jti` читает Auth-owned session-epoch floor; `session_epoch` обязан быть положительным integer и быть не меньше floor
5. Извлекает claims: `sub`/`user_id`, `profile_id`, `roles`, `subscription_tier`, `jti`, `session_epoch`
6. Сохраняет проверенные claims и upstream JWT в существующем auth context при проксировании `/ws` в Realtime; отдельный downstream header для `session_epoch` этим контрактом не вводится
7. Публичные endpoints (login, register, OTP, version, health, metrics) — без JWT

### Phase-0 delegated transport (target)

Для защищённого downstream gRPC Gateway выдаёт отдельный delegated-user bearer,
а не пересылает client JWT или `X-Voice-*` identity headers. В metadata остаются
только один `authorization: Bearer <delegated principal>` и один `x-request-id`.
Credential содержит verified account/profile, положительный `session_epoch`, exact
`aud`/full `rpc`, deterministic protobuf `request_hash`, `request_id`, `iat`,
`nbf`, `exp`, `jti`, `kid`; его TTL не более 30 s и не длиннее остатка client
session. Consumer проверяет JWKS, request binding и Auth epoch fail-closed до
handler. Общие transport, temporal и rotation правила —
[ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md#phase-0-межсервисные-и-edge-principals-target-внедряется-по-сервисам).

### T056-P1: session epoch

Auth DB хранит `accounts.session_epoch` как durable source of truth и выдаёт его
новым access JWT положительным integer claim. В Compose после Auth migration,
startup seed и issuance preparation включён strict Gateway. Он обязан проверить
claim и Redis minimum-epoch floor: token допускается только при
`session_epoch >= floor`. После успешной JWT проверки Gateway проверяет
non-empty `jti` blacklist **до** floor. Отсутствующий/неположительный claim
даёт `invalid_token`; отсутствующий или повреждённый floor и любая ошибка/timeout
Redis дают fail-closed `auth_unavailable`; floor нельзя считать равным `1`.
Floor читается без Gateway prefix override из Auth-owned ключа
`auth:session:min_epoch:<account_id>`: положительный `int64`, без TTL, read-only
для Gateway, через общий `GATEWAY_REDIS_ADDR`/`GATEWAY_REDIS_PASSWORD`; один
запрос ограничен 2 секундами.

`GATEWAY_SESSION_EPOCH_STRICT` парсится строго: unset означает compatibility,
точные литералы `false` и `true` включают соответственно compatibility и strict;
пустая строка, whitespace, иной регистр и прочие значения — startup config error.
Strict требует `GATEWAY_REDIS_ADDR` до создания HTTP server, но не делает Redis
Ping на startup. Для JWKS validator Gateway передаёт
`WithSessionEpochRequired(strict)`: в compatibility legacy JWT без claim допустим,
но присутствующий malformed/non-positive claim всё равно invalid.

Compatibility допустим только в явно настроенном окружении вне доказанного strict
deployment; Compose strict proof не завершает rollout во всех окружениях. При обычном `/ws`, выпуске
`POST /api/v1/realtime/ws-ticket` и потреблении одноразового ticket Gateway
применяет ту же JWT → `jti` → floor проверку. Consume атомарно забирает ticket,
после чего заново валидирует сохранённый `record.UpstreamToken` и строит claims
из этой свежей проверки, а не из snapshot в ticket. Ticket уже spent при любом
последующем deny; повтор даёт `invalid_ticket`. Client `Authorization` при
ticket-upgrade не может его переопределить: upstream получает сохранённый token.
Та же проверка обязательна для обычного REST JWT. `jti` остаётся per-session
blacklist-механизмом и не заменяется epoch.

Для dev/tests допускается `GATEWAY_AUTH_MODE=static` + `GATEWAY_STATIC_TOKENS_JSON`; production должен использовать JWKS.
В JWKS-режиме `GATEWAY_REDIS_ADDR` обязателен для проверки `jti`: без него Gateway
не пропускает защищённый запрос с `jti` и отвечает `503 auth_unavailable`. Явный
`static` режим для dev/tests может работать без Redis.

## Конфигурация Gateway

| Переменная | Назначение |
|------------|------------|
| `GATEWAY_JWKS_URL` | JWKS endpoint Auth Service |
| `GATEWAY_JWT_ISSUER`, `GATEWAY_JWT_AUDIENCE` | Проверка `iss` и `aud` |
| `GATEWAY_REDIS_ADDR`, `GATEWAY_REDIS_PASSWORD` | Redis для rate limit и JWT blacklist |
| `GATEWAY_SESSION_EPOCH_STRICT` | Только точное `true` включает strict; unset/точное `false` — compatibility; прочее не даёт Gateway стартовать |
| `GATEWAY_JWT_BLACKLIST_PREFIX` | Prefix blacklist ключей; default `jwt:blacklist:` |
| `GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR`, `GATEWAY_PRINCIPAL_ACTIVE_KID` | Phase-0 Gateway issuer: secret-mounted каталог с ровно двумя unencrypted PKCS#8 RSA private keys `<kid>.pem` (active + peer). `ACTIVE_KID` выбирает signing key; JWKS публикует оба sorted public keys. Неполная/некорректная конфигурация или legacy aliases `S2S_SIGNING_KEY_PEM` / `S2S_SIGNING_KID` не дают Gateway стартовать. Выпуск delegated credentials в downstream routes пока **not wired**. |
| `S2S_JWKS_URLS_JSON`, `S2S_JWKS_REFRESH_AFTER`, `S2S_JWKS_HARD_EXPIRY`, `S2S_UNKNOWN_KID_COOLDOWN` | **Target, not wired:** будущие Phase-0 issuer JWKS endpoints и bounded verifier cache; общий contract с downstream services. Текущий Gateway эти vars не читает, до wiring они unused. |
| `GATEWAY_TRUSTED_PROXY_CIDRS` | CIDR/IP список proxy, от которых принимается `X-Forwarded-For` |
| `GATEWAY_CORS_ALLOWED_ORIGINS` | CSV allowlist browser origins; default deny |
| `GATEWAY_REST_UPSTREAMS_JSON` / `GATEWAY_<NAMESPACE>_UPSTREAM_URL` | REST upstream routes |
| `GATEWAY_GRPC_UPSTREAMS_JSON` / `GATEWAY_<NAMESPACE>_GRPC_ADDR` | JSON-объект непустых gRPC-адресов по namespace; значение per-namespace перекрывает карту. Непустая malformed-карта, `null`, нестроковое или пустое/whitespace значение — startup error до создания HTTP listener. |
| `GATEWAY_REALTIME_UPSTREAM_URL` | `/ws` upstream Realtime Service |
| `GATEWAY_VERSION_CONFIGS_JSON`, `GATEWAY_FORCE_UPDATE_JSON` | Version policy |

## Зависимости

- **Redis** — rate limiting (sliding window), чтение JWT blacklist и T056-P1 minimum-epoch floor. В strict-режиме ошибки/отсутствие floor fail-closed; зона ответственности с **Auth Service**: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) (раздел «Redis: API Gateway и Auth Service»).
- **Auth Service** — JWT public key (ротация через JWKS endpoint)
- **Version config store** — таблица `client_versions` (или эквивалентный конфиг-стор) для `/api/v1/version`

## Метрики (→ Analytics)

- `gateway_request_count` — Prometheus counter по route group, method, status code
- `gateway_request_latency_ms_sum` — суммарная latency в миллисекундах по route group, method, status code
- `gateway_ratelimit_hit` — заблокированные запросы по группе лимита
- `gateway.ws.connections` — текущие WebSocket соединения

## Масштабирование

Stateless, масштабируется горизонтально. За внешним Load Balancer (L4/L7).

## A2 Space REST contract (target)

This section freezes the next Gateway/Flutter contract. Existing route paths and
response wrappers are preserved; the complete authorization, retry, lifecycle and
audit semantics below are **target, not implemented**. New endpoints must remain
unexposed until their service contracts and negative transport tests land.
Space room **media** actions use the separate
[Voice contract](voice-service.md#phase-0-space-room-media-roster-и-lifecycle-target-не-реализовано);
room entity CRUD below still belongs to Space.

### Common wire and authorization rules

All routes below require a valid user JWT, including invite preview. Gateway
uses verified account/profile/session epoch to issue the downstream delegated
principal; raw identity headers and body actor fields confer no authority.
Space checks canonical resource ownership and the route-specific ACL before any
new mutation. Join/redeem and invite preview do not require existing membership;
completed operation replay uses the original authenticated actor and saved binding,
not a new membership/owner check. Restore uses recorded ownership while frozen.
A path ID is authoritative: a supplied matching body ID is accepted for existing
protobuf clients, but a different body ID is `400 invalid_argument`; a referenced
node/category/invite/room from another Space is never operated on.

JSON uses protobuf `snake_case` field names, UUID strings, RFC3339 UTC timestamps
and existing response wrappers. Required fields are listed below; `?` means
optional. Unknown fields, invalid UUID/timestamp, duplicate query keys or invalid
integer ranges are `400 invalid_argument`. Empty optional strings are not IDs.
Responses serialize the named protobuf type using the existing Gateway rules;
`204` has no body. `Space`, `Invite`, `Category`, `VoiceRoom`, `SpaceTreeNode`,
`SpaceMembership` and `AuditLogEntry` are defined by
[Space proto](../../protos/voice/space/v1/space.proto); target additions are explicit.

Common errors use `{"error_code":"<code>","message":"<safe text>"}`: `401
unauthenticated` for absent/invalid session; `404 not_found` for missing or
undiscoverable Space/resource (including foreign-Space resource IDs); `403
permission_denied` for a discoverable action denied by policy; `409
failed_precondition` for frozen/purged Space or invalid lifecycle/member state;
`409 already_exists` for a reused operation key with different request; `429
resource_exhausted` for a rate limit; `503 unavailable` for Auth/Role/store/service
failure. Never expose gRPC internals or proof-factor distinctions. Failure before
commit changes no membership, role, tree, audit or event state. A lost response
after commit is recovered using the operation rules below.

Except join/redeem, code-holder preview, recorded-owner restore and completed
operation replay, protected reads/mutations require current membership plus the
table ACL. Owner
recognition and permission evaluation remain Role/Space contracts; an unavailable
Role does not grant access. Frozen Space is hidden to members; the owner can read
only `{space:{id,name,deletion_scheduled_at,purge_after}}` through existing
`GET /api/v1/spaces/{space_id}` and restore. For a frozen Space, non-owners receive `404 not_found`; the recorded owner
receives `409 failed_precondition` for normal operations other than the limited
read and restore. Completed replay returns only its saved outcome and never fresh
resource data; it cannot grant a new disclosure or mutation. No normal tree/invite/audit read is
allowed while frozen. Audit tombstone retention does not create public access.

### Lifecycle

`S` below is `/api/v1/spaces/{space_id}`. Each new lifecycle mutation requires a
UUID `operation_id` in JSON. `POST S/leave` remains compatible with its existing
empty-body form; clients should supply the optional idempotency header below.

| Method/path | Space RPC / ACL | JSON request | Success |
|---|---|---|---|
| `POST S/join` | `JoinSpace`; canonical visibility/entry policy, no invite-only bypass | Empty body | `200 {space_membership: SpaceMembership}` |
| `POST S/leave` | `LeaveSpace`; current member, owner must transfer first | Empty body | `204` |
| `POST S/transfer-ownership` | `TransferOwnership`; current owner, target is existing member | `{new_owner_profile_id,proof,operation_id}` | `204` |
| `DELETE S` | `DeleteSpace` target schedule; current owner | `{confirmation_name,proof,operation_id}` | `204` (scheduled, never immediate purge) |
| `POST S/restore` | `RestoreSpace` target; recorded owner before purge_after | `{operation_id}` | `200 {space: Space}` |

The transfer issue route is `POST /api/v1/auth/ownership-transfer-proof` →
Auth `IssueOwnershipTransferProof`, JWT required. Request:
`{space_id,new_owner_profile_id,operation_id,password,totp_code?,backup_code?}`;
response `200 {proof,expires_at}`. At most one second-factor field is supplied;
Auth requires it when 2FA is enabled. Auth-only storage/consume semantics are in
[auth-service.md](auth-service.md#ownership-transfer-step-up-proof-target-contract-not-implemented).
`ConsumeOwnershipTransferProof` is Space-only and has no public route.
Gateway redacts credentials and proof everywhere and never persists either.

Deletion uses a **different purpose-bound proof**, specified in
[Space lifecycle](space-service.md#a2-public-lifecycle-and-retry-contract-target).
The UI must type the exact current Space name and complete Auth factors; restore
does not require a fresh factor proof. At/after `purge_after`, restore returns
`409 failed_precondition` to the recorded owner; others receive `404 not_found`.
Concurrent restore/purge serialize so only one lifecycle transition can win.

### Invites

Management prefix is `S/invites`; lookup/redeem retain `/api/v1/invites/{code}`.
Invite code is an opaque path segment, not a UUID. No credentials/codes are logged.

| Method/path | RPC / ACL | Request | Success |
|---|---|---|---|
| `POST S/invites` | `CreateInvite`; `SPACE_MANAGE_INVITES` | `{max_uses?,expires_at?}`; max_uses integer >0, expiry in future; absence means no corresponding limit | `200 {invite: Invite}` |
| `GET S/invites` | `ListInvites`; `SPACE_MANAGE_INVITES` | No query/body | `200 {invite_list:{invites:[Invite]}}` |
| `DELETE S/invites/{invite_id}` | `RevokeInvite`; `SPACE_MANAGE_INVITES`, canonical invite belongs to S | Empty body | `204` |
| `GET /api/v1/invites/{code}` | `GetInvite`; authenticated holder of valid code, no membership prerequisite | No query/body | `200 {invite:{code,space_id,expires_at?}}` safe preview |
| `POST /api/v1/invites/{code}/join` | `JoinByInvite`; canonical entry policy and limits | Empty body | `200 {space_membership: SpaceMembership}` |

Invalid/expired/revoked/exhausted code always gives `404 not_found` without
revealing which check failed. Preview never exposes creator, usage counters,
member identities or the administrative invite ID; the safe projection replaces
current full-Invite disclosure only when the target capability is enabled.
Manage-list retains its existing unpaged wrapper, ordered `created_at DESC,id
DESC`, includes revoked/expired invites and never silently truncates. Pagination
requires a separately versioned additive contract, not an undocumented cursor.
Redeem consumes a use only after all entry requirements and approval succeed;
a current member retry does not consume another use. Pending approval returns
`409 failed_precondition` and no membership; the durable pending request is the
only permitted intermediate effect of canonical entry-policy evaluation.

### Tree, category and room entities

Tree mutations require `TEXT_CHAT_CREATE_IN_SPACE`, the existing Space tree-manage
permission (including pin); read requires membership. Chat creation additionally
applies Chat's own create/limit policy. Child IDs must resolve to the path Space.

| Method/path | RPC | JSON request | Success |
|---|---|---|---|
| `GET S/tree` | `ListSpaceTree` | None | `200 {categories:[Category],nodes:[SpaceTreeNode],voice_rooms:[VoiceRoom]}` |
| `POST S/categories` | `CreateCategory` | `{name,sort_order?}` | `200 {category: Category}` |
| `PATCH S/categories/{category_id}` | `UpdateCategory` | `{name?,sort_order?}`; at least one | `200 {category: Category}` |
| `DELETE S/categories/{category_id}` | `DeleteCategory` | Empty | `204` |
| `POST S/voice-rooms` | `CreateVoiceRoom` | `{name}` | `200 {voice_room: VoiceRoom}` |
| `PATCH S/voice-rooms/{voice_room_id}` | `UpdateVoiceRoom` | `{name}` | `200 {voice_room: VoiceRoom}` |
| `DELETE S/voice-rooms/{voice_room_id}` | `DeleteVoiceRoom` | Empty | `204` |
| `POST S/tree/nodes` | `UpsertTreeNode` | `{node_id?,category_id?,kind,linked_chat?,voice_room_id?,sort_order?,is_system?}` | `200 {space_tree_node: SpaceTreeNode}` |
| `DELETE S/tree/nodes/{node_id}` | `RemoveTreeNode` | Empty | `204` |
| `POST S/tree/reorder` | `ReorderSpaceTree` | `{ordered_node_ids:[UUID]}` | `204` |
| `POST S/tree/nodes/{node_id}/pin` | `PinTreeNode` | Empty | `200 {space_tree_node: SpaceTreeNode}` |
| `DELETE S/tree/nodes/{node_id}/pin` | `UnpinTreeNode` | Empty | `200 {space_tree_node: SpaceTreeNode}` |
| `POST S/chats` | Chat `CreateChat`, then Space `UpsertTreeNode` | Existing `CreateChatRequest` JSON excluding actor and path-bound space_id | `200 {space_tree_node: SpaceTreeNode}` |

Names are nonempty after trimming and retain service-defined limits. Sort values
are int32 >=0. Node kind is `text_chat` with `linked_chat:{id,type?}` or
`voice_room` with `voice_room_id`, never both. New public nodes cannot request
`is_system:true`; existing system nodes cannot be removed or converted. Reorder
contains each current node once without duplicates and preserves the pin-group
contract; stale/incomplete sets fail `409 failed_precondition`. Tree is an
unpaged complete snapshot in canonical category/pin/sort order. Category deletion
moves its nodes to root without deleting chats/rooms. Room deletion removes its
tree node and follows the Voice cleanup contract. Chat->node creation must retain
a durable operation record across both calls so retry cannot create a duplicate
Chat; a failed node link is retried/compensated before success is exposed.

### Audit read

`GET S/audit-log` → `GetAuditLog`, membership and `SPACE_VIEW_AUDIT_LOG`.
Query: `actor_profile_id?` (UUID), `action?` (exact nonempty action string),
`from?` inclusive and `to?` exclusive (RFC3339 UTC, from < to), `page_size?`
(integer 1..100, default50), `cursor?` (opaque). Unknown action is a valid filter
with no matching rows. Success:
`200 {audit_log_list:{entries:[AuditLogEntry],next_cursor:"..."}}`;
an empty result uses `entries:[]`; the final page preserves all its returned
entries and sets only `next_cursor` to the empty string. Order is
`created_at DESC,id DESC`. `details_json` remains a redacted JSON **string**,
not a second nested wire type. Filtering never bypasses Space/actor visibility.

First page fixes an upper `(created_at,id)` snapshot ceiling. Subsequent pages use
the same filters and page size; an HMAC-signed cursor binds Space, actor/access
scope, normalized filters, ceiling, last key and expiry. Lifetime is 15 minutes
from first page; current ACL is rechecked each page. Changed filter/page size,
invalid/expired cursor is `400 invalid_argument`; access loss still denies even
with a valid cursor. No cursor creates access to a frozen/purged Space. Durable
ledger/outbox/retention semantics remain in
[space-service.md](space-service.md#phase-0-space-audit-ledger-target-not-implemented).

### Retry and activation

Every non-lifecycle mutation above (existing or newly exposed), plus existing
join/leave, accepts optional `Idempotency-Key: <UUID>`;
new lifecycle routes use the mandatory body `operation_id`. Gateway copies the
normalized key into a target protobuf `operation_id` before signing the deterministic
request hash; it is not forwarded as an unsigned identity/operation metadata header.
If both are present
they must match. Without a key existing routes retain their characterized retry
behavior and clients must reconcile with reads before retrying creates. For new
non-lifecycle endpoints without a key, repeated PATCH applies the supplied values
under current ACL; repeated entity DELETE after the resource is absent returns
404 not_found; pinning an already pinned node or unpinning an unpinned node is a
200 no-op returning the current node, without a new audit/event effect. A missing
node still returns 404. New no-key requests never bypass current ACL or frozen
state checks. Target
Flutter always supplies a key. Space stores a durable canonical request digest
and outcome for 30 days under `(actor_profile_id,operation_id)`, binding method,
path Space and complete payload; proof uses a cryptographic digest, never plaintext.
Identical completed replay by the original verified actor returns its saved
outcome without new side effects; changed body/method/Space is `409 already_exists`.
Concurrent in-progress replay returns `409 failed_precondition` and can be retried
with the same key. Dependency failures leave resumable state, never a false success.
Clients do not reuse keys after expiry and reconcile unknown outcomes rather than
blindly reissue a destructive action. Transfer preserves its exact durable Auth
receipt contract; successful retry works after the actor ceases to be owner.

These changes require proto additions for lifecycle/proof/filter fields, signed
transport binding for the protobuf operation fields, durable store/outbox migrations,
Gateway schemas/error mapping and client contract tests. Legacy and target routes
must not form an alternative authorization bypass: enable each vertical only
after caller cutover and UI/REST/gRPC negative tests on the same SHA. This document
freezes implementation inputs; it does not make the current endpoints production ready.
