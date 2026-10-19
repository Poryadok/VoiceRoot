# Bot Service

`SPACE_PRINCIPAL_JWKS_URL` accepts the canonical Space provider path
`/.well-known/jwks.json` and the existing dedicated-provider alias
`/space/jwks.json`. Both require exact HTTPS URLs without credentials, query,
fragment or alternate escaped paths. Space's dedicated JWKS provider currently
requires trusted server TLS and does not request a client certificate. The alias
already worked; canonical-path compatibility does not imply an existing
deployment outage or introduce another JWKS server.

Игровые сценарии и source audit baseline: [game-bot-interactions.md](../features/game-bot-interactions.md).
Новые card/command/opt-in contracts там помечены **proposed**; существующие RPC
и scopes этого документа не получают новые гарантии доставки автоматически.

## Обзор

### Durable native Space installation

`InstallBotInSpace` persists a `bot_installation_intents` PREPARING row before
calling the dedicated Space Bot-membership listener. The immutable request binds
operation UUID, Bot UUID, installation UUID, Space UUID, actor profile UUID and
positive int64-range Bot generation. The initiating caller must pass the current
`SPACE_MANAGE_BOTS` check; missing Role authority fails closed. Only Space's
acknowledgement permits local INSTALLED state and whitelist activation. A lost
response leaves the same operation recoverable on request retry after restart.

Cancellation persists a distinct operation UUID and the original install
operation UUID before the protected remove call. CANCELING disables local use
and blocks reinstall. Public uninstall also freezes its own per-installation
`space_uninstall` initiating decision, target and Role/Messaging obligations in
the same transaction. The native Empty acknowledgement is stored explicitly as
a Bot-local transport observation; it is not a Space receipt or global terminal
fence. Saving that observation atomically retires the local whitelist/installation
and marks native cancellation CANCELED. Public Empty acknowledges accepted native
cancellation while required typed Role/Messaging cleanup may remain pending.
The separate pending-cleanup journal preserves admission exclusion until those
obligations settle; clients provide no additional generation precondition.
Global deletion discovers these explicitly pending frozen sources after local
retirement. Pending work is recovered by the bounded
claim worker without the initiating bearer token; exact accepted bytes remain
stable and delivery credentials are refreshed. A reinstall
uses a new installation UUID and increasing Bot generation. An old internal
cancellation replay cannot erase that new installation. The public uninstall
request still addresses Bot/Space, so a new public call after reinstall targets
the current installation; it has no cross-generation client retry precondition.
Space-attached group/channel membership inherits the Space native source; this
path sends no redundant unfenced `Chat.AddMembers` request.

Before authorizing new or enabled whitelist targets, Bot performs the separate
protected `ChatScopeAuthorityService.GetBotChatScopeAuthority` read. The stored
Chat type and exact Space attachment are authoritative; caller `ChatRef.type`
is not. New admission requires an existing group/channel and LIVE lifecycle.
Validate the entire deduplicated proposed set before any native Add or durable
whitelist admission; an empty set is empty. Save exact deterministic read request
and response bytes/hashes with the native intent or whitelist mutation. Source
observations do not reserve Chat state or grant membership. Historical retries
retain their original native operation and source evidence; cleanup may reconcile
retired heads separately, never manufacture install-time provenance. Disabling
or removing an existing target does not require a fresh Chat row.

The Chat source client uses distinct `BOT_CHAT_SCOPE_*` values: `GRPC_ADDR`,
`TLS_CA_FILE`, `TLS_SERVER_NAME`, `TLS_CERT_FILE` and `TLS_KEY_FILE`, plus the
existing Bot service issuer. Each bounded call signs exact audience `chat`, new
RPC, fresh read operation, complete request hash and fresh JTI. No ordinary Chat
client, public `GetChat`, borrowed Role/Space credential or raw metadata fallback
can authorize this source fact. Existing owner/manageBots decisions remain.

The dedicated client requires all five `BOT_SPACE_INSTALLATION_*` values:
`GRPC_ADDR` (Space's dedicated listener, normally `space:9098`), `TLS_CA_FILE`,
`TLS_SERVER_NAME`, `TLS_CERT_FILE` and `TLS_KEY_FILE`. It uses verified server TLS
and a Bot client certificate, plus the existing Bot service-principal issuer.
Each call signs audience `space`, exact RPC, operation ID and deterministic full
request hash, with a fresh JWT ID per retry and a three-second call deadline.
There is no ordinary Space-client or raw-metadata fallback and no configurable
human default Role. Deployment must provide the matched protected receiver and
existing Bot issuer/JWKS configuration; these settings do not activate a stand.
Global `DeleteBot` cleanup and legacy unqualified in-flight request reconciliation
remain separate unfinished work.

### BE-131 current installation authority

`CheckBotInstallationScope` is a protected Space-only decision read on Bot's
existing Space lifecycle mTLS listener. Its signed request binds bot, Space,
actor profile, exact installation UUID and `TEXT_CHAT_CREATE_IN_SPACE`. Ordinary
and Game Integration listeners do not expose it. Bot checks its current exact
installation, actor, live bot status, current scope and local LIVE lifecycle
fence; missing/revoked/mismatched state returns `allowed=false` with the exact
requested tuple. Authority/storage failure returns `UNAVAILABLE`, never a
negative decision or permission to compensate. This read grants no user
identity and does not bypass Space's current admission checks. Space consumes
the response only after authenticating its transport and comparing every echoed
field to the immutable composite intent. No raw GetBot/ListInstalledBots
fallback is permitted. Existing Space lifecycle TLS/JWKS/replay configuration
is reused; this does not activate P3 or change generic bot scopes.

Платформа для ботов: реестр приложений, slash-команды, webhook-доставка, install в спейс, presence.

**Язык**: Go  
**БД**: PostgreSQL `bot_db`

## Ответственность

- App Manifest (YAML/JSON) — декларативное описание бота
- Регистрация и управление ботами (`slug`, `actor_profile_id`)
- Install / uninstall в спейс; per-chat whitelist и `enabled`
- Slash-команды с autocomplete; клиентские interactions (sync + defer/complete)
- Webhook доставка событий (production); polling mode (разработка)
- HMAC-SHA256 подпись webhook запросов
- Bot presence (`bot_presence`, `TouchPresence`); `online` в `ListSlashCommandsForChat` и `ListInstalledBots`
- Scopes: `TEXT_CHAT_SEND_MESSAGES`, `DM_SEND`, `SPACE_VIEW_MEMBER_LIST`, `MEMBER_ASSIGN_ROLES`, `TEXT_CHAT_CREATE_IN_SPACE`, `SPACE_MANAGE_ROLES` (privileged), `TEXT_CHAT_READ_HISTORY` (privileged); строки совпадают с [role-service.md](role-service.md)
- Rate limits: 5000 API req/min и 100 role ops/min — Gateway REST (`BotAPI`, `BotRoleOps`, `429` + `Retry-After`); 10 созданий текстовых чатов / день — Bot Service (`bot_daily_chat_creates`, `CreateBotChat`); прямой gRPC обходит Gateway limiter — см. [api-gateway.md](api-gateway.md)
- Bot-token runtime gRPC дополнительно ограничивается Bot-owned limiter. При
  настроенном `BOT_REDIS_ADDR` ошибка Redis возвращает `Unavailable` до вызова
  обработчика; запрос без проверки общей квоты не выполняется. Исчерпанная
  квота возвращает `ResourceExhausted`, после восстановления Redis запросы
  снова проверяются по общей квоте. Внешний ответ не содержит диагностику Redis.
- **gRPC access (v1):** `BOT_GRPC_GATEWAY_ONLY=true` отклоняет прямой gRPC без `x-voice-internal: true` (Gateway) или bot token / account context. Эти legacy callers остаются ограничены Bot-owned method/scope checks; прямой сетевой доступ к Bot не заменяет проверку actor. Prod NetworkPolicy — [`deploy/templates/network-policy-voice-bot.yaml`](../../deploy/templates/network-policy-voice-bot.yaml); mTLS — [`DEPLOYMENT.md`](../DEPLOYMENT.md).
- Bot token: перманентный, ручной отзыв (`RegenerateToken`)
- Bot DM: только в ответ на сообщение пользователя (v1)
- Ephemeral и deferred responses; hub deferred tokens (`MarkEventDeferred`, `RehydrateDeferred`)
- 3-секундный таймаут на ответ (webhook)

Публичный REST через Gateway: [api-gateway.md](api-gateway.md) (`/api/v1/bots/**`).

`CreateBotChat` атомарно резервирует дневной слот до вызова Chat. Ответы Chat с
`InvalidArgument`, `Unauthenticated`, `PermissionDenied` или `FailedPrecondition`
освобождают слот, поскольку эти проверки выполняются до транзакции создания.
При остальных ошибках и пустом успешном ответе слот остаётся занятым: результат
вызова может быть неоднозначным, и Bot не предполагает, что Chat ничего не
создал. `CreateBotChatRequest.request_id` — необязательный канонический UUID для
одной логической операции. Клиент, передавший его, должен повторять тот же UUID
и те же нормализованные поля запроса; Bot привязывает резерв к bot ID,
профилю-актору и хэшу запроса, а Chat получает тот же UUID. Повтор с тем же UUID и изменёнными полями
возвращает `AlreadyExists`. Повтор после неоднозначного исхода проходит в Chat
для сверки с его durable ключом и не резервирует дневную квоту повторно. Одновременные
повторы регистрируют отдельные активные попытки до RPC; известный отказ освобождает
резерв только после отказа всех активных попыток. Успешный или неоднозначный исход
делает резерв постоянным для этого ключа, чтобы поздний отказ параллельного вызова
не освободил уже использованный слот. После всех определённых отказов число
резервов уменьшается, но Bot сохраняет hash-tombstone: тот же hash можно повторить
и заново зарезервировать текущий день, а другой payload с этим UUID получает
`AlreadyExists`. Если процесс завершится до фиксации исхода попытки, резерв
остаётся занят: без подтверждения Chat освобождать его небезопасно. Без
`request_id` сохраняется прежняя совместимость и атомарное ограничение квоты,
но дедупликации повтора после timeout нет.

## API (gRPC)

Канон: `protos/voice/bot/v1/bot.proto`.

```protobuf
service BotService {
  // Registry
  rpc RegisterBot(RegisterBotRequest) returns (RegisterBotResponse);
  rpc UpdateBot(UpdateBotRequest) returns (UpdateBotResponse);
  rpc DeleteBot(DeleteBotRequest) returns (DeleteBotResponse);
  rpc GetBot(GetBotRequest) returns (GetBotResponse);
  rpc GetBotBySlug(GetBotBySlugRequest) returns (GetBotResponse);
  rpc ListBots(ListBotsRequest) returns (ListBotsResponse);
  rpc RegenerateToken(RegenerateTokenRequest) returns (RegenerateTokenResponse);

  // Commands (manifest / portal)
  rpc RegisterCommands(RegisterCommandsRequest) returns (RegisterCommandsResponse);
  rpc GetCommands(GetCommandsRequest) returns (GetCommandsResponse);

  // Webhook
  rpc SetWebhookURL(SetWebhookURLRequest) returns (SetWebhookURLResponse);
  rpc GetWebhookURL(GetWebhookURLRequest) returns (GetWebhookURLResponse);

  // Chat whitelist (per space install)
  rpc SetChatWhitelist(SetChatWhitelistRequest) returns (SetChatWhitelistResponse);
  rpc GetChatWhitelist(GetChatWhitelistRequest) returns (GetChatWhitelistResponse);

  // Bot runtime (bot token or internal)
  rpc SendBotMessage(SendBotMessageRequest) returns (SendBotMessageResponse);
  rpc EditBotMessage(EditBotMessageRequest) returns (EditBotMessageResponse);
  rpc SendEphemeral(SendEphemeralRequest) returns (SendEphemeralResponse);
  rpc DeferResponse(DeferResponseRequest) returns (DeferResponseResponse);
  rpc PollEvents(PollEventsRequest) returns (stream PollEventsResponse);

  // Manifest (Developer Portal)
  rpc ValidateManifest(ValidateManifestRequest) returns (ValidateManifestResponse);
  rpc ApplyManifest(ApplyManifestRequest) returns (ApplyManifestResponse);

  // Space lifecycle
  rpc InstallBotInSpace(InstallBotInSpaceRequest) returns (InstallBotInSpaceResponse);
  rpc UninstallBotFromSpace(UninstallBotFromSpaceRequest) returns (UninstallBotFromSpaceResponse);
  rpc ListInstalledBots(ListInstalledBotsRequest) returns (ListInstalledBotsResponse);
  rpc ListBotsInChat(ListBotsInChatRequest) returns (ListBotsInChatResponse);
  rpc SetBotChatEnabled(SetBotChatEnabledRequest) returns (SetBotChatEnabledResponse);

  // Client slash
  rpc ExecuteSlashInteraction(ExecuteSlashInteractionRequest) returns (ExecuteSlashInteractionResponse);
  rpc ListSlashCommandsForChat(ListSlashCommandsForChatRequest) returns (ListSlashCommandsForChatResponse);
  rpc CompleteInteraction(CompleteInteractionRequest) returns (CompleteInteractionResponse);
  rpc AutocompleteSlashOption(AutocompleteSlashOptionRequest) returns (AutocompleteSlashOptionResponse);

  // Presence & scopes runtime (REST via Gateway — api-gateway.md §bots)
  rpc TouchPresence(TouchPresenceRequest) returns (TouchPresenceResponse);
  rpc AssignBotRole(AssignBotRoleRequest) returns (AssignBotRoleResponse);
  rpc RevokeBotRole(RevokeBotRoleRequest) returns (RevokeBotRoleResponse);
  rpc ListSpaceMembersForBot(ListSpaceMembersForBotRequest) returns (ListSpaceMembersForBotResponse);
  rpc CreateBotChat(CreateBotChatRequest) returns (CreateBotChatResponse);
  rpc GetChatMessagesForBot(GetChatMessagesForBotRequest) returns (GetChatMessagesForBotResponse);
  rpc CreateBotRole(CreateBotRoleRequest) returns (CreateBotRoleResponse);
  rpc CompleteAutocomplete(CompleteAutocompleteRequest) returns (CompleteAutocompleteResponse);
}
```

### Ключевые поля `Bot`

| Поле | Назначение |
|------|------------|
| `actor_profile_id` | Профиль-отправитель сообщений бота (`sender_profile_id` в Messaging); используется при uninstall для Role/Messaging cleanup |
| `slug` | Уникальный публичный идентификатор; deep link `voice.gg/bots/{slug}`, `voice.app/bots/{slug}`; lookup `GetBotBySlug` / `GET /api/v1/bots/slug/{slug}` |
| `scopes_json` | JSON-массив строк scope |
| `is_polling_mode` | Dev: события через `PollEvents` вместо webhook |

### Install / uninstall

- **Install** (`InstallBotInSpace`): `SPACE_MANAGE_BOTS` через Role `CheckPermission`; `Space.AddBotMember` для `actor_profile_id`; whitelist чатов; `acknowledge_privileged_scopes` обязателен при privileged scopes.
- **Uninstall** (`UninstallBotFromSpace`): снимает whitelist/installation, protected native `Space.RemoveBotMember`; **S2S cleanup** — см. [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md):
  - Role: private `ApplyBotInstallationRemoval` с `purpose=space_uninstall` удаляет допустимые созданные роли и назначения этого Bot actor; системные/Owner роли не обходятся.
  - Messaging: private `ApplyBotChatPinRemoval` с тем же purpose снимает pins отправителя по сохранённому batch whitelisted attachments; сообщения сохраняются.
  - Обычные Role/Messaging helpers не являются fallback. Ошибка/configuration gap сохраняет отдельный pending journal. После durable native ACK возвращается Empty acceptance; незавершённый native ACK остаётся retryable `Unavailable`. Empty не означает завершение всей typed cleanup работы.
- **ListInstalledBots**: `InstalledBot.online` из `bot_presence` (тот же порог, что для slash `online`).
- `ListInstalledBots.allowed_chats` получает текущий group/channel type через
  Chat `GetChat` с исходным caller context. Тип принимается только для точного
  Chat ID в запрошенном Space. При недоступном/запрещённом lookup, неполном или
  несовпадающем ответе ID из whitelist сохраняется, optional `ChatRef.type`
  отсутствует; channel по умолчанию не подставляется. Повторяющиеся Chat ID
  гидратируются один раз на запрос; следующий запрос снова читает Chat.

### Presence

- Таблица `bot_presence (bot_id, last_seen_at)`; обновляется `TouchPresence`, poll loop и успешный webhook touch.
- `ListSlashCommandsForChat` и `ListInstalledBots` выставляют `online=false`, если heartbeat устарел.
- Публичный heartbeat: `POST /api/v1/bots/me/presence` (Gateway → `TouchPresence`).

### Deferred interactions

- `DeferResponse` / webhook `deferred: true` → `bot_event_log.delivery_status = deferred`; hub держит token до `CompleteInteraction` / `SendBotMessage` follow-up.
- `RehydrateDeferred` при старте восстанавливает tokens в hub из БД.
- `RunDeferredTTLSweeper` (interval `BOT_DEFERRED_SWEEP_INTERVAL`, default 15m) помечает stale deferred как `abandoned` после `BOT_DEFERRED_TTL` (default 24h).

## Модель данных

```
bots
├── id (UUID)
├── owner_account_id
├── name
├── description
├── avatar_url
├── slug (unique)
├── actor_profile_id          -- sender for bot messages
├── token_hash (SHA-256)
├── webhook_url (nullable)
├── webhook_secret (HMAC key)
├── is_polling_mode (bool)
├── scopes (jsonb)
├── status (live | …)
├── created_at
└── updated_at

bot_commands
├── id (UUID)
├── bot_id (FK)
├── name
├── description
├── parameters (jsonb)
├── created_at
└── updated_at

bot_space_installations
├── id (UUID)
├── bot_id (FK)
├── space_id
├── installed_by_profile_id
└── created_at

bot_chat_whitelist
├── bot_id (FK)
├── chat_id
├── space_id
├── enabled (bool)
├── added_by_profile_id
└── added_at

bot_presence
├── bot_id (PK, FK)
└── last_seen_at

bot_event_log
├── id (UUID)
├── bot_id (FK)
├── event_type
├── payload (jsonb)
├── delivery_status (pending | delivered | failed | timeout | deferred)
├── attempts
├── interaction_token
├── created_at
└── delivered_at

bot_message_deliveries
├── id (UUID; stable webhook delivery ID)
├── bot_id (FK), message_id (UUID; unique pair)
├── chat_id, payload (jsonb)
├── status (pending | delivered | canceled | failed), attempts, next_attempt_at, claimed_until
└── created_at, delivered_at
```

## Webhook доставка

`BOT_WEBHOOK_DELIVERY_ATTEMPTS` задаёт число HTTP attempts на один вызов
interaction/autocomplete delivery, включая первый (default `3`).
`BOT_WEBHOOK_RETRY_BACKOFF_MS` задаёт начальную задержку в миллисекундах
(default `100`); следующие задержки удваиваются. Пустые, нечисловые и
неположительные значения используют default. Значения читаются при каждом
вызове delivery; context cancellation прерывает ожидание следующей попытки.
Это настройки HTTP delivery, а не durable outbox lease/retry budget.

```
Event (NATS: message in whitelisted chat) ──► Bot Service
  │
  ├─► Sign payload with HMAC-SHA256
  ├─► POST to webhook_url (3 sec timeout)
  │     ├─► 200 OK with response → deliver to chat
  │     ├─► 200 OK with deferred → send "thinking..." → wait for follow-up
  │     └─► Timeout/Error → retry (3 attempts, exponential backoff)
  │
  └─► Polling mode: enqueue to bot's event stream (GET /api/v1/bots/me/interactions/poll)
```

For `message.sent`, Bot binds only the centrally provisioned JetStream push
consumer `message_events/bot_message_events`: filter `message.sent`, delivery
subject `_INBOX.voice.bot.bot_message_events`, DeliverAll and AckExplicit.
Startup fails if its configuration differs. Bot does not create a consumer.
The application connects to the local NATS leaf without credentials by default;
`BOT_NATS_CREDS_FILE` is optional. Bot producer JetStream replies use
`_INBOX.voice.bot`.

The consumer commits one durable intent per eligible `(bot_id, message_id)`
before ACK. A database failure NAKs the source message. A worker leases and
delivers recipients independently: polling inserts the visible event and marks
the intent delivered in one transaction; webhook sends a signed POST, then
marks the intent delivered. Failed recipients remain pending with retry
backoff, so another bot's polling failure cannot block a healthy webhook.
Four workers claim recipients independently, so one slow webhook cannot hold
the delivery queue. Before delivery, Bot locks the current bot and chat
whitelist rows through the POST or polling write. A disable/uninstall/URL or
secret rotation therefore commits either before delivery authorization is read
or after the in-progress delivery completes; pending revoked recipients are
canceled, and retries use the current URL and secret. Non-retryable webhook
responses (4xx except 408/429, malformed response, invalid URL) leave a durable `failed` row
for diagnosis rather than looping forever. Transient failures remain pending.
Replay after a completed delivery is inert. Webhook payload
`options.delivery_id` is stable across retries; receivers should deduplicate
that ID for an ambiguous HTTP response (remote success before local receipt).

### T51 game-event adapter (planned)

The GIS→Bot `PublishGameEvent` boundary and Bot→Messaging
`SendGameEventMessage` call are defined in the
[canonical Game Event v1 contract](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
These are future S2S RPCs, not the public Bot `SendBotMessage` path and not the
existing `message.sent` JetStream consumer. GIS owns the game-event inbox and
outbox; Bot verifies its live Bot actor, send scope and chat whitelist, then
waits for Messaging's idempotent durable message result. The S2S calls use the
service-principal contract and caller/method allowlist from
[`ARCHITECTURE_REQUIREMENTS.md`](../ARCHITECTURE_REQUIREMENTS.md). The T51
RPCs and their tests remain unimplemented.

T11 installation registration uses a separate protected HTTP proof endpoint:
`POST /internal/v1/game-integrations/bots/{bot_id}/authority`. Only the
`gameintegration` workload, authenticated with its dedicated 32-byte HMAC key,
may call this endpoint. Bot binds the exact `bot` audience, method, path,
timestamp, nonce and body digest; it rejects duplicate or stale proofs and keeps replayed nonces in
Redis for 61 seconds (covering the inclusive ±30-second timestamp window), and
returns an exact-body response MAC for the same request timestamp
and nonce. It reads `bots.owner_account_id` and `bots.status` itself, and
returns a success proof only when the supplied GIS registry owner matches the
stored owner and status is `live`. Missing key/Redis, replay storage failure,
unknown/disabled Bot and foreign owner all fail closed. The key is
`GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64`, separate from GIS↔Auth workload
credentials. This registration proof does not replace the future T51
`PublishGameEvent` service-principal RPC or make that runtime path available.
It authenticates only installation registration for the exact Bot path/body.
It does not authorize legacy Bot gRPC or mint a Bot actor token. Bot token or
account metadata, GIS `vgi1`, and T11 HMAC headers sent to the future T51 RPC
have no effect: the RPC is not registered and the call returns gRPC
`UNIMPLEMENTED`. The shared service-principal verifier also rejects both `vgi1`
credentials and T11 HMAC material as non-JWT credentials. When T51 is
implemented, its enabled-path verifier must accept only its own verified service
principal (`gameintegration` caller, `bot` audience, exact RPC allowlist) and
retain the Bot actor/scope checks in this document.

T11 workload-key rotation has no `kid` and no overlap. Change the GIS and Bot
secret-manager values as one coordinated operation. Both services fail closed
while their active keys differ; once Bot loads the replacement, it rejects old
and in-flight proofs. GIS retries a failed installation proof with the new key
and the same durable idempotency operation. No success proof or installation
binding is returned until both sides agree.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`bot.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                 | Данные                               |
|-------------------------|--------------------------------------|
| `bot.registered`        | bot_id, owner_id, name               |
| `bot.command_executed`  | bot_id, command, chat_id, user_id |
| `bot.webhook_delivered` | bot_id, event_type, latency_ms       |
| `bot.webhook_failed`    | bot_id, event_type, error            |

## Зависимости

- **Messaging Service** — отправка/редактирование сообщений; `UnpinMessagesBySenderInChats` при uninstall
- **Role Service** — `CheckPermission` (install), scopes runtime (`AssignBotRole`/`RevokeBotRole`), `DeleteRolesCreatedByProfile` при uninstall
- **Chat Service** — `chat_id` в whitelist = `chats.id` (`type` = `group` \| `channel`)
- **Space Service** — `AddBotMember` / `RemoveBotMember` для `actor_profile_id`
- **NATS** — получение событий для доставки ботам

## P3 Space lifecycle participant (target)

Bot stores a durable lifecycle fence, imported exact Chat manifest pages and
immutable receipts. `FROZEN` blocks installations, whitelist changes, bot
commands and deferred Space/chat work before bot/admin shortcuts. Restore
accepts only the next `LIVE` generation. Purge removes installations, Space and
chat whitelists, bot memberships/projections and deferred deliveries, then
persists a permanent compact `PURGED` fence. Full request/receipt bytes retain
30 days from this participant's completion; delayed lower-generation work is a
no-op.

Bot013 retains only an ordinary Bot-level `retired_cleanup_unresolved` bit when
local Purge retires actual associations whose obligations are not already
owned by an immutable global or pending-uninstall journal. The bit contains no
retired Space/chat/actor history, grants no cleanup authority and introduces no
retention clock. Owner revocation still commits, but absent source rows cannot
produce zero-target or final global `COMPLETE` while it is set. This minimum
safeguard does not resolve the separate BE-170 retired snapshot/downstream P3
coverage and post30day proof-provider gap; no clearing or compaction provider
is implied. Existing unfinished cleanup journals remain separate obligations.

The message recipient producer, webhook worker and development polling send
share the participant's Space advisory guard. They acquire all distinct actual
Space keys in ascending order before bot, whitelist and delivery/event row locks,
then read lifecycle state in a fresh statement after waiting. A missing local
head preserves sparse admission; any persisted non-`LIVE` state denies. A changed
captured whitelist binding rolls back for rediscovery, never adds another Space
lock after row locks. Installation/uninstallation and whitelist mutations take
the exclusive guard first. An admitted bounded webhook/stream send holds its
shared guard through transaction end, so freeze cannot acknowledge its receipt
until that attempt finishes. Message polling rows persist their validated Space;
legacy NULL-Space message rows must resolve current whitelist authority and
cannot bypass a known fence. Terminal canceled work cannot be rearmed by retry
bookkeeping. This is the message-path protocol; command/deferred authority,
manifest import, retention and full participant convergence remain target work.

Every listener registering BotService installs a method-specific native gRPC
transport tap for `PollEvents`. It caps the whole RPC at 25 seconds, preserving
an earlier client deadline. In pinned grpc-go 1.70 the tap context is captured
before native stream write quota initialization, so expiration terminates an
actual flow-control-blocked `Send`, rather than only a DB/application context.
The handler calls `Send` synchronously under the guard and cancels the tap budget
on exit; no send goroutine survives rollback or guard release. A successful
`Send` means transport enqueue, not receiver acknowledgement: bytes from an
already admitted send may arrive after a fence receipt. The guarantee is no new
post-fence admission and no active application send after releasing its guard.

### Protected Gateway lifecycle initiation (BE-170)

`DeleteBot`, `InstallBotInSpace`, `UninstallBotFromSpace`, `SetChatWhitelist` and
`SetBotChatEnabled` use Gateway's existing
`GATEWAY_BOT_ADMIN_AUDIT_*` mTLS client and exact delegated-user credentials.
The Bot receiver uses an independently configured mTLS listener, exact Gateway
JWKS issuer, full protobuf body/method/request binding and shared durable replay
guard. Raw or mixed identity headers are rejected. It reads the current Auth
session floor on every new public admission; a missing/deleted account or stale
epoch is unauthenticated, and unavailable floor authority is retryable and cannot
create a new owner decision. Background recovery consumes a previously committed
decision and does not manufacture a new user credential or default Role.

Receiver configuration is all-or-none:
`BOT_GATEWAY_LIFECYCLE_GRPC_LISTEN`, `BOT_GATEWAY_LIFECYCLE_TLS_CERT_FILE`,
`BOT_GATEWAY_LIFECYCLE_TLS_KEY_FILE`, `BOT_GATEWAY_LIFECYCLE_CLIENT_CA_FILE`,
`BOT_GATEWAY_PRINCIPAL_JWKS_URL`, `BOT_GATEWAY_PRINCIPAL_JWKS_CA_FILE`.
It requires the Bot signer/JWKS/replay group plus a distinct Auth client:
`BOT_AUTH_SESSION_FLOOR_GRPC_ADDR`, `BOT_AUTH_SESSION_FLOOR_TLS_CA_FILE`,
`BOT_AUTH_SESSION_FLOOR_TLS_SERVER_NAME`,
`BOT_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE`,
`BOT_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE`. Auth additionally verifies its configured
Bot URI SAN on the existing protected floor listener; test fixture identities are
not deployment defaults. This listener exposes only these five unary methods and
denies streaming RPCs. The outbound Bot signer can run without enabling the
separate GameEvent ingress; partial GameEvent configuration still fails closed.

Implementation qualification remains tracked in BE-170: a protected component
proof is not full Gateway cutover, native membership cleanup, or global deletion
completion. Empty DeleteBot success requires every required participant effect's
verified immutable completion receipt; retained installation/whitelist snapshots
are recovery input and never restore live access to a terminal Bot.

After verified owner admission, the local transaction commits the immutable
deletion root and disables the Bot before downstream cleanup. Missing retired
cleanup proof keeps that root nonterminal/retryable and the Bot disabled; it
does not roll back owner revocation or turn an empty local target set into
success. Bot-token lookup admits only live Bots. Ordinary Space uninstall uses
its separate accepted-native-cancellation protocol described above.

New public install/cancel permission decisions use the separate
`BOT_ROLE_INSTALLATION_GRPC_ADDR`, `BOT_ROLE_INSTALLATION_TLS_CA_FILE`,
`BOT_ROLE_INSTALLATION_TLS_SERVER_NAME`, `BOT_ROLE_INSTALLATION_TLS_CERT_FILE`,
`BOT_ROLE_INSTALLATION_TLS_KEY_FILE` group. It signs only the exact protected Role
`CheckPermission` request for the verified profile, Space and
`SPACE_MANAGE_BOTS`; it forwards no initiating identity metadata. A nil/failed
decision is unavailable, a definitive false is denied. Missing private transport
never falls back to ordinary `ROLE_GRPC_ADDR`. Role's actual protected listener
must enable its finite Bot lifecycle branch and trust the configured Bot JWKS;
private cleanup additionally needs its registered typed removal adapter.

The existing `PATCH /api/v1/bots/{bot_id}/chats/{chat_id}/enabled` dispatch uses
this protected client, binds the chat ID from the URL before signing and requires
the current private `SPACE_MANAGE_BOTS` decision for both enable and disable.
Enabling additionally records a fresh Chat scope observation; disabling preserves
the accepted source. Missing private Role transport cannot authorize either.

`SetChatWhitelist` retains its existing owner-only rule. Its declared protected
RPC requires the same verified Gateway/fresh-floor boundary, but no production
Gateway HTTP mapping or non-generated frontend caller currently exists. Install's
`allowed_chats` is a separate manage-bots flow, not an owner-policy setter alias.
Do not infer public HTTP availability from the protected RPC adapter.

Each accepted local chat attachment has a durable UUID allocated before an
uncertain native install call. Exact retries, overlapping whitelist updates and
disable/enable retain that lifetime; removing and adding the chat again creates
a new lifetime. This UUID identifies Bot's local attachment, not Chat's native
incarnation or historical Space membership. Existing rows without an accepted
original observation retain an explicitly unqualified origin; a later Chat read
cannot retroactively qualify it.

Deletion snapshots preserve the union of actual whitelist attachments and active
installation intents by attachment UUID, including multiple lifetimes for the
same chat. Each target retains its immutable original observation independently
from the optional current whitelist observation. Arbitrary attachment history
does not create new cleanup obligations. These records are recovery input, not
participant completion receipts. Purge-before-delete coverage and the existing
P3 evidence retention boundary remain tracked in BE-170; absence of downstream
evidence must not be interpreted as completed cleanup.
### Private Messaging cleanup caller

Global deletion or ordinary uninstall freezes the eligible per-chat attachment batch in its own Bot root
snapshot. Messaging child request bytes are sealed before transport and reused
exactly after uncertainty; only the signed service credential/JTI is refreshed.
Bot validates the typed participant receipt, every attachment target and digest,
execution revision, direct covering fields and full request/evidence binding
before recording that child as completed. A participant ACK cannot settle other
pending children or the whole root.

The dedicated client requires all five settings
`BOT_MESSAGING_CLEANUP_GRPC_ADDR`, `BOT_MESSAGING_CLEANUP_TLS_CA_FILE`,
`BOT_MESSAGING_CLEANUP_TLS_SERVER_NAME`, `BOT_MESSAGING_CLEANUP_TLS_CERT_FILE`,
and `BOT_MESSAGING_CLEANUP_TLS_KEY_FILE`, plus the existing Bot service issuer.
Missing configuration does not fall back to the ordinary Messaging connection.
The recovery worker claims due roots, validates retained byte hashes and typed
completed children, and retries independently recoverable Role/Messaging work.
It leaves the root pending while any participant is unavailable. Global Space
terminal-fence delivery, actual multi-service uninstall composition and P3/post-retention proof
coverage remain unfinished; this does not imply full readiness.
