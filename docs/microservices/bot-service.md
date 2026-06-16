# Bot Service

## Обзор

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
- Scopes: `TEXT_CHAT_SEND_MESSAGES`, `DM_SEND`, `SPACE_VIEW_MEMBER_LIST`, `MEMBER_ASSIGN_ROLES`, `TEXT_CHAT_CREATE_IN_SPACE`, `TEXT_CHAT_READ_HISTORY` (privileged); строки совпадают с [role-service.md](role-service.md)
- Rate limits (целевые): 5000 API req/min, 10 созданий текстовых чатов в спейсе / день на бота — см. [features/bots.md](../features/bots.md); enforcement — открытый пункт BOT-C
- Bot token: перманентный, ручной отзыв (`RegenerateToken`)
- Bot DM: только в ответ на сообщение пользователя (v1)
- Ephemeral и deferred responses; hub deferred tokens (`MarkEventDeferred`, `RehydrateDeferred`)
- 3-секундный таймаут на ответ (webhook)

Публичный REST через Gateway: [api-gateway.md](api-gateway.md) (`/api/v1/bots/**`).

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

  // Presence & scopes runtime (gRPC-only; REST — см. TODO BOT-C)
  rpc TouchPresence(TouchPresenceRequest) returns (TouchPresenceResponse);
  rpc AssignBotRole(AssignBotRoleRequest) returns (AssignBotRoleResponse);
  rpc RevokeBotRole(RevokeBotRoleRequest) returns (RevokeBotRoleResponse);
  rpc ListSpaceMembersForBot(ListSpaceMembersForBotRequest) returns (ListSpaceMembersForBotResponse);
  rpc CreateBotChat(CreateBotChatRequest) returns (CreateBotChatResponse);
  rpc GetChatMessagesForBot(GetChatMessagesForBotRequest) returns (GetChatMessagesForBotResponse);
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
- **Uninstall** (`UninstallBotFromSpace`): снимает whitelist/installation, `Space.RemoveBotMember`; **S2S cleanup** — см. [CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md):
  - Role: `DeleteRolesCreatedByProfile` (роли с `created_by_profile_id = actor_profile_id`), затем `GetMemberRoles` + `RevokeRole` для назначенных ролей бота
  - Messaging: `UnpinMessagesBySenderInChats` по whitelisted `chat_id` и `sender_profile_id = actor_profile_id`
- **ListInstalledBots**: `InstalledBot.online` из `bot_presence` (тот же порог, что для slash `online`).

### Presence

- Таблица `bot_presence (bot_id, last_seen_at)`; обновляется `TouchPresence`, poll loop и успешный webhook touch.
- `ListSlashCommandsForChat` и `ListInstalledBots` выставляют `online=false`, если heartbeat устарел.

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
```

## Webhook доставка

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
