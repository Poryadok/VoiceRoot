# Bot Service

Go service for the Voice bot platform: registry, slash commands, webhooks, space install, and runtime APIs.

**Canonical spec:** [docs/microservices/bot-service.md](../../../docs/microservices/bot-service.md)

## Capabilities

- **Registry** — register/update/delete bots, slug lookup, token regeneration, manifest validate/apply
- **Install lifecycle** — install/uninstall in a space, per-chat whitelist, privileged-scope acknowledgement
- **Slash & interactions** — command registration, client slash execution, defer/complete, ephemeral responses, autocomplete (webhook and polling)
- **Webhook delivery** — HMAC-SHA256 signed POSTs with retries; polling mode for local development
- **Runtime (bot token)** — send/edit messages, ephemeral replies, presence heartbeat, scoped role/member/chat APIs
- **Rate limits** — 10 text chat creates per bot per day (`CreateBotChat`); Gateway REST limits for API and role ops (see bot-service doc)
- **Deferred interactions** — hub tokens, rehydration on startup, TTL sweeper for abandoned deferred rows

## API surface

- **gRPC:** `protos/voice/bot/v1/bot.proto` (`BotService`)
- **REST (public):** via API Gateway — `/api/v1/bots/**` (see [api-gateway.md](../../../docs/microservices/api-gateway.md))
- **Health:** `GET /health` → `{"service":"bot","status":"ok"}`

## Data & dependencies

- **PostgreSQL** `bot_db` — bots, commands, installations, whitelist, presence, event log, daily chat-create counters
- **Downstream gRPC** — Messaging, Role, Chat, Space, User (actor profile provisioning)
- **NATS** — inbound events for webhook/poll delivery; publishes `bot.events` domain stream

## Local development

From repo root, see [docs/DEPLOYMENT.md](../../../docs/DEPLOYMENT.md) and `docker compose --profile app`. Bot migrations: `make compose-migrate-bot`.

For sandbox Game Integration Bot binding, configure the same dedicated 32-byte
base64 key as GIS in `GAME_INTEGRATION_BOT_WORKLOAD_KEY_B64` and configure
`BOT_REDIS_ADDR` (plus `BOT_REDIS_PASSWORD` when Redis requires authentication).
Generate a local-only key with `openssl rand -base64 32`; do not commit it or
reuse the GIS↔Auth key. Without the key, Redis replay store, or Bot database,
the internal authority proof endpoint fails closed. There is no development-key
fallback.
