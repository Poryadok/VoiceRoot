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
- Scopes: send_messages, send_dm, read_members, manage_roles, manage_channels, read_messages (privileged)
- Per-channel whitelist (бот работает только в разрешённых каналах)
- Rate limits: 5000 API req/min, 10 channel creation/day
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

  // Channel whitelist
  rpc SetChannelWhitelist(SetWhitelistRequest) returns (Empty);
  rpc GetChannelWhitelist(GetWhitelistRequest) returns (WhitelistResponse);

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
├── scopes (jsonb — ["send_messages", "read_members", ...])
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

bot_channel_whitelist
├── bot_id (FK)
├── channel_id (FK)
├── added_by (profile_id)
├── added_at
└── UNIQUE(bot_id, channel_id)

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
Event (NATS: message in whitelisted channel) ──► Bot Service
  │
  ├─► Sign payload with HMAC-SHA256
  ├─► POST to webhook_url (3 sec timeout)
  │     ├─► 200 OK with response → deliver to channel
  │     ├─► 200 OK with deferred → send "thinking..." → wait for follow-up
  │     └─► Timeout/Error → retry (3 attempts, exponential backoff)
  │
  └─► Polling mode: enqueue to bot's event stream
```

## Публикуемые события (→ NATS)

| Событие                 | Данные                               |
|-------------------------|--------------------------------------|
| `bot.registered`        | bot_id, owner_id, name               |
| `bot.command_executed`  | bot_id, command, channel_id, user_id |
| `bot.webhook_delivered` | bot_id, event_type, latency_ms       |
| `bot.webhook_failed`    | bot_id, event_type, error            |

## Зависимости

- **Messaging Service** — отправка сообщений от имени бота
- **Role Service** — проверка scopes бота в контексте канала
- **Space Service** — валидация channel whitelist
- **NATS** — получение событий для доставки ботам


