# Bot Service

## Обзор

Платформа для ботов: реестр приложений, slash-команды, webhook-доставка, авторизация.

**Язык**: Go
**БД**: PostgreSQL `bot_db`

## Ответственность

- App Manifest (YAML/JSON) — декларативное описание бота
- Регистрация и управление ботами
- Slash-команды с autocomplete параметрами
- Webhook доставка событий (production)
- Polling mode (для разработки)
- HMAC-SHA256 подпись webhook запросов
- Scopes: `TEXT_CHAT_SEND_MESSAGES`, `DM_SEND`, `SPACE_VIEW_MEMBER_LIST`, `MEMBER_ASSIGN_ROLES`, `TEXT_CHAT_CREATE_IN_SPACE`, `TEXT_CHAT_READ_HISTORY` (privileged); строки совпадают с [role-service.md](role-service.md)
- Per-chat whitelist (бот работает только в разрешённых текстовых чатах: `group` \| `channel`)
- Rate limits: 5000 API req/min, 10 созданий текстовых чатов в спейсе / день на бота (см. лимиты платформы)
- Bot token: перманентный, ручной отзыв
- Bot DM: только в ответ на сообщение пользователя (v1)
- Ephemeral и deferred responses
- 3-секундный таймаут на ответ (webhook)

## API (gRPC)

```protobuf
service BotService {
  // Управление ботами
  rpc RegisterBot(RegisterBotRequest) returns (Bot);
  rpc UpdateBot(UpdateBotRequest) returns (Bot);
  rpc DeleteBot(DeleteBotRequest) returns (Empty);
  rpc GetBot(GetBotRequest) returns (Bot);
  rpc ListBots(ListBotsRequest) returns (BotList);
  rpc RegenerateToken(RegenerateTokenRequest) returns (TokenResponse);

  // Slash-команды
  rpc RegisterCommands(RegisterCommandsRequest) returns (Empty);
  rpc GetCommands(GetCommandsRequest) returns (CommandList);

  // Webhook config
  rpc SetWebhookURL(SetWebhookURLRequest) returns (Empty);
  rpc GetWebhookURL(GetWebhookURLRequest) returns (WebhookURLResponse);

  // Chat whitelist (chats.id, group | channel)
  rpc SetChatWhitelist(SetWhitelistRequest) returns (Empty);
  rpc GetChatWhitelist(GetWhitelistRequest) returns (WhitelistResponse);

  // Bot actions (вызывается ботом через REST API)
  rpc SendBotMessage(SendBotMessageRequest) returns (Message);
  rpc EditBotMessage(EditBotMessageRequest) returns (Message);
  rpc SendEphemeral(SendEphemeralRequest) returns (Empty);
  rpc DeferResponse(DeferResponseRequest) returns (Empty);

  // Polling (dev mode)
  rpc PollEvents(PollEventsRequest) returns (stream BotEvent);
}
```

## Модель данных

```
bots
├── id (UUID)
├── owner_account_id
├── name
├── description
├── avatar_url
├── token_hash (SHA-256)
├── webhook_url (nullable)
├── webhook_secret (HMAC key)
├── is_polling_mode (bool)
├── scopes (jsonb — ["TEXT_CHAT_SEND_MESSAGES", "SPACE_VIEW_MEMBER_LIST", ...])
├── status (active | suspended)
├── created_at
└── updated_at

bot_commands
├── id (UUID)
├── bot_id (FK)
├── name (string, slash command)
├── description
├── parameters (jsonb — [{name, type, required, choices}])
├── created_at
└── updated_at

bot_chat_whitelist
├── bot_id (FK)
├── chat_id (FK) -- chats.id, type = group | channel
├── added_by (profile_id)
├── added_at
└── UNIQUE(bot_id, chat_id)

bot_event_log
├── id (UUID)
├── bot_id (FK)
├── event_type
├── payload (jsonb)
├── delivery_status (pending | delivered | failed | timeout)
├── attempts (int)
├── created_at
└── delivered_at (nullable)
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
  └─► Polling mode: enqueue to bot's event stream
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

- **Messaging Service** — отправка сообщений от имени бота
- **Role Service** — проверка scopes бота в контексте чата
- **Chat Service** — `chat_id` в whitelist = `chats.id` (`type` = `group` \| `channel`)
- **Space Service** — при ботах в спейсе: чат числится в дереве (`space_tree_nodes`, `kind=text_chat`)
- **NATS** — получение событий для доставки ботам


