# Notification Service

## Обзор

Маршрутизация уведомлений по каналам доставки: push (FCM/APNs), in-app (WebSocket), email.

**Язык**: Go
**БД**: PostgreSQL `notification_db`, Redis

## Ответственность

- Push-уведомления: FCM (Android, Web), APNs (iOS)
- In-app уведомления через Realtime Service (WebSocket)
- Email уведомления (только auth-related: верификация, password reset)
- Группировка push по чату (1 push, counter)
- Гранулярные настройки уведомлений (глобальные, per-space, per-channel, per-type)
- Quiet hours
- @username пробивает silence (по умолчанию)
- @mention во время voice → overlay уведомление
- Device token management (FCM/APNs registration)
- Routing для федеративных нод (node → master → FCM/APNs)
- iOS PushKit + CallKit интеграция для входящих звонков

## API (gRPC)

```protobuf
service NotificationService {
  // Device tokens
  rpc RegisterDevice(RegisterDeviceRequest) returns (Empty);
  rpc UnregisterDevice(UnregisterDeviceRequest) returns (Empty);

  // Настройки
  rpc GetNotificationSettings(GetSettingsRequest) returns (NotificationSettings);
  rpc UpdateNotificationSettings(UpdateSettingsRequest) returns (NotificationSettings);

  // Quiet hours
  rpc SetQuietHours(SetQuietHoursRequest) returns (Empty);

  // Internal — отправка
  rpc SendNotification(SendNotificationRequest) returns (Empty);
  rpc SendBulkNotification(SendBulkRequest) returns (Empty);

  // Federation — relay
  rpc RelayNotification(RelayNotificationRequest) returns (Empty);
}
```

## Модель данных

```
device_tokens
├── id (UUID)
├── profile_id (FK)
├── platform (android | ios | web | desktop)
├── token (string)
├── push_service (fcm | apns | voip_apns)
├── created_at
└── updated_at

notification_settings
├── profile_id (FK)
├── scope_type (global | space | channel | chat)
├── scope_id (nullable — space_id / channel_id / chat_id)
├── enabled (bool)
├── mute_until (nullable)
├── suppress_types (jsonb — массив type-ов для suppress)
└── UNIQUE(profile_id, scope_type, scope_id)

quiet_hours
├── profile_id (FK)
├── enabled (bool)
├── start_time (time)
├── end_time (time)
├── timezone (string)
└── override_mentions (bool — @username пробивает тишину)
```

## Типы уведомлений

| Тип               | Канал доставки      | Группировка |
|-------------------|--------------------:|-------------|
| new_dm            | push + in-app       | by chat     |
| mention           | push + in-app       | by chat     |
| reply             | push + in-app       | by chat     |
| reaction          | in-app only         | —           |
| friend_request    | push + in-app       | —           |
| match_found       | push + in-app       | —           |
| incoming_call     | push (VoIP) + in-app| —           |
| system            | push + in-app       | —           |

## Логика доставки

```
Event (NATS) ──► Notification Service
                    │
                    ├─► Check user settings (mute? quiet hours? suppress type?)
                    ├─► Check presence (online → in-app only, offline → push)
                    ├─► Check grouping (уже есть push для этого чата?)
                    │
                    ├─► Realtime Service (in-app, через NATS)
                    ├─► FCM / APNs (push)
                    └─► Resend (email, auth only)
```

## Публикуемые события (→ NATS)

| Событие                   | Данные                              |
|---------------------------|-------------------------------------|
| `notification.push_sent`  | profile_id, type, platform          |
| `notification.push_delivered`| profile_id, type (delivery receipt)|
| `notification.push_clicked`  | profile_id, type, deep_link       |

## Зависимости

- **FCM** — Android и Web push
- **APNs** — iOS push и VoIP push (CallKit)
- **Resend** — email
- **Redis** — grouping state, rate limiting
- **User Service** — presence check
- **NATS** — получение событий для отправки уведомлений
- **Realtime Service** — (через NATS) in-app delivery
