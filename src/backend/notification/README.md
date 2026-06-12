# Notification Service

Push routing for FCM (Web + Android) and APNs (iOS): device tokens, offline delivery for `new_message` / `mention`, `match_found` foundation.

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
| `CHAT_GRPC_ADDR` | Chat member list for `MessageSent` offline push fan-out |
| `FCM_CREDENTIALS_JSON` | Firebase service account; unset = noop sender (Tier-1 degraded) |
| `APNS_KEY_ID` | Apple Push Notifications auth key id |
| `APNS_TEAM_ID` | Apple Developer team id |
| `APNS_BUNDLE_ID` | iOS app bundle id (APNs topic) |
| `APNS_AUTH_KEY` | APNs `.p8` private key PEM (or use `APNS_AUTH_KEY_PATH`) |
| `APNS_PRODUCTION` | Set to `false` for sandbox endpoint (default: production) |

When APNs credentials are missing or invalid, the service uses an APNs **noop** sender (Tier-1 degraded); `GET /health` and device registration remain available.

## Local compose

Service `notification` in `docker-compose.yml` (`--profile app`). Gateway route: `/api/v1/notifications/**`.
