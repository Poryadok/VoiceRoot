# Notification Service

Push routing for FCM (Web + Android): device tokens, offline delivery for `new_message` / `mention`, `match_found` foundation.

## Surfaces

- gRPC `NotificationService` on `:9090` (`NOTIFICATION_GRPC_LISTEN`)
- GET `/health` on `:8080`

## Environment

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | `notification_db` PostgreSQL |
| `NOTIFICATION_REDIS_ADDR` | Push grouping state (optional; degraded = ungrouped) |
| `NATS_URL` | `message.events` consumer (optional) |
| `USER_GRPC_ADDR` | Presence check for online vs push routing |
| `FCM_CREDENTIALS_JSON` | Firebase service account; unset = noop sender (Tier-1 degraded) |

## Local compose

Service `notification` in `docker-compose.yml` (`--profile app`). Gateway route: `/api/v1/notifications/**`.
