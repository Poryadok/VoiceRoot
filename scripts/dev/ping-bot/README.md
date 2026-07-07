# Ping bot (local dev)

Minimal app stack6 sample bot for slash `/ping` → `pong`.

## Polling mode (no public URL)

```bash
export VOICE_API_BASE_URL=http://127.0.0.1:18080
export VOICE_BOT_TOKEN=vb_...   # from Developer Portal or POST /api/v1/bots/{id}/token/regenerate
go run .
```

Set `VOICE_BOT_EPHEMERAL=true` to reply ephemerally (visible only to caller).

## Webhook mode

Use `NewWebhookHandler(webhookSecret, ephemeral)` from `webhook.go` in your own HTTP server, or point the bot manifest `webhook_url` at a tunnel.

## Tests

```bash
go test ./...
```
