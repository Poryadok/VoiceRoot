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
  rpc GetQuietHours(GetQuietHoursRequest) returns (GetQuietHoursResponse);
  rpc SetQuietHours(SetQuietHoursRequest) returns (Empty);

  // Internal — отправка
  rpc SendNotification(SendNotificationRequest) returns (Empty);
  rpc SendBulkNotification(SendBulkRequest) returns (Empty);

  // Federation — relay (target Auth account UUID: RelayNotificationRequest.account_id)
  rpc RelayNotification(RelayNotificationRequest) returns (Empty);
}
```

`RelayNotificationRequest.account_id` — идентификатор аккаунта (`accounts.id`), тот же смысл, что у JWT claim `user_id` после Gateway; публичные сценарии по профилю используют `profile_id` в `SendNotification` / настройках.

Актуальный gRPC-контракт: [protos/voice/notification/v1/notification.proto](../../protos/voice/notification/v1/notification.proto). В **`NotificationSettings`** время отложенного mute — **`mute_until`** (`google.protobuf.Timestamp`, UTC, поле 7); номер поля **5** зарезервирован под прежнее строковое представление (wire name `mute_until_rfc3339`). В **`RegisterDeviceRequest`** опционально **`platform_enum`** (`DevicePlatform`); при установке предпочтительно использовать его вместе со строкой `platform` для обратной совместимости.

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
├── scope_id (nullable — space_id; для channel → chats.id, type = group \| channel; для chat → chats.id, только dm)
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

Канонические wire-имена — строка `notification_type` в `SendNotificationRequest` и поле `type` в in-app WS `notification`. Feature catalog — [notifications.md](../features/notifications.md).

| Wire `notification_type` | Канал | Группировка / notes |
|--------------------------|-------|---------------------|
| **`new_message`** | push + in-app | by `chat_id` — accepted DM after requests bucket cleared |
| **`message_request`** | push + in-app | by **sender `profile_id`** (not `chat_id` until accept); stranger label — § ниже |
| `mention` | push + in-app | by `chat_id`; may bypass mute / quiet hours |
| `reply` | push + in-app | by `chat_id` |
| `reaction` | in-app only | — |
| `friend_request` | push + in-app | — |
| `match_found` | push + in-app | presence check **skipped** (spec) |
| `incoming_call` | push (VoIP) + in-app | CallKit / PushKit — feature catalog [notifications.md](../features/notifications.md) |
| `voice_member_joined` | push + in-app | presence check **skipped** (spec) |
| `system` | push + in-app | — |

**Naming:** `new_dm` — **deprecated alias**; use **`new_message`**. `SendNotificationRequest.notification_type` и in-app payload `d.type` — только канонические строки выше.

### `message_request` (stranger DM)

| Аспект | Контракт |
|--------|----------|
| **Trigger** | `message.sent` where recipient `inbox_bucket=requests` (first DM or re-contact after decline) — [text-chat.md](../features/text-chat.md) § «Запросы сообщений» |
| **Wire type** | **`message_request`** (not `new_message`) |
| **Push title** | «Незнакомец» / «Message request» + sender display name |
| **Grouping** | By sender `profile_id` until accept |
| **After accept** | Subsequent messages → `new_message` |

**Code gaps:** `message_request` absent from `delivery/types.go`; Realtime `in_app_notification_fanout.go` hardcodes `type=new_message` for all DM — [todo/backend.md](../todo/backend.md).

### Stickers and GIF (`new_message` variant)

| Aspect | Contract |
|--------|----------|
| **Wire type** | **`new_message`** (not a separate enum) |
| **Trigger** | `message.sent` with `content_type=STICKER \| GIF` |
| **Push title** | Sender display name (same as text) |
| **Push body** | Label **«Sticker»** / **«GIF»** when `content` empty; optional thumb from File presigned URL or `preview_url` |
| **In-app** | Group by `chat_id`; unread badge unless mute / `send_silent` |
| **Presence routing** | Standard `DecideRouting` |

См. [messaging-service.md](messaging-service.md) § Stickers and GIF; [notifications.md](../features/notifications.md).

## Логика доставки

```
Event (NATS) ──► Notification Service
                    │
                    ├─► Social block / privacy deny? → drop (no notification)
                    ├─► Check user settings (mute? suppress type?)
                    ├─► Check presence (online session → in-app only, offline → push)
                    ├─► Check `send_silent` on `message.sent` (push sound/badge)
                    ├─► Check quiet hours (`ApplyQuietHours` → push off; in-app on)
                    ├─► Check grouping (уже есть push для этого чата?)
                    │
                    ├─► Realtime Service (in-app, через NATS)
                    ├─► FCM / APNs (push)
                    └─► Resend (email, auth only)
```

**Policy order:** block/privacy → mute / `suppress_types` → **presence** → **`send_silent`** → **quiet hours** (mention override) → grouping → channel dispatch. `send_silent` does **not** bypass quiet hours.

### Suppress before routing (block / privacy)

| Condition | Owner | Notification |
|-----------|-------|----------------|
| **Social block** (either direction) | Messaging rejects `SendMessage` | **None** — event never reaches Notification |
| **`allow_dm` privacy deny** | Messaging rejects send | **None** |
| Per-chat / global **mute** | Notification `ApplySettings` | Suppresses configured types (default: `new_message` in-app + push) |
| **`suppress_types`** on settings row | Notification | Listed types dropped for scope |

См. [privacy.md](../features/privacy.md), [text-chat.md](../features/text-chat.md) § DM privacy.

### Presence routing

Нормативное правило (`DecideRouting` + User `GetBulkPresence` enrichment):

| Recipient `GetPresence` | In-app | Push |
|-------------------------|--------|------|
| **Online session** (`online`, `idle`, `in_call` on active WS / heartbeat) | ✓ | **✗** |
| **Offline** (`offline`, no session) | ✓ | ✓ (if settings allow) |
| **Invisible** | ✓ | ✓ (treat as offline for push) |

**Exceptions — skip presence check** (always evaluate push policy):

| Type | Reason |
|------|--------|
| `match_found` | User may be in-app without wanting to miss match alert |
| `voice_member_joined` | Voice room alerts while user in another voice context |

**Code gap:** `GRPCChecker.IsOnline` checks only `PRESENCE_ONLINE_STATUS_ONLINE`; idle/in-call not treated as online — push may fire incorrectly. MM/voice paths still apply presence contrary to skip rule — [todo/backend.md](../todo/backend.md).

Implemented in `delivery/router.go` → `DecideRouting`; message path enriches via `EnrichDecision` / `EnrichDecisions`.

### `send_silent` consumption

Wire name on `SendMessage` / `message.sent`: **`send_silent`** (bool). Notification consumer on `message.sent`:

| Channel | `send_silent=true` |
|---------|-------------------|
| **Push** | Deliver **without sound** and **without badge increment** (where platform supports) |
| **In-app** | Unread badge + in-app row **as usual** (unless mute suppresses type) |
| **Quiet hours** | `send_silent` does **not** bypass quiet hours; both apply — push still suppressed in DND window |
| **Scheduled dispatch** | Silent flag applied at **actual send** time (worker), not at schedule create |

Composer label — [text-chat.md](../features/text-chat.md) § Send options; [screen-controls.md](../design/screen-controls.md) §3.6c #1.

**Code gap:** field absent from `messaging.proto` / JetStream `message.sent` proto — [todo/backend.md](../todo/backend.md).

### Quiet hours

`ApplyQuietHours` sets **`Push=false`** during configured window; **`InApp` remains true**. Product copy: **push suppressed; in-app may still deliver** — не формулировать как «уведомления не приходят». `@mention` may bypass via `override_mentions` on quiet-hours settings (push only). Integration test — `quiet_hours_test.go`.

| Layer | During quiet hours |
|-------|-------------------|
| Push | Suppressed (except mention override) |
| In-app WS `notification` | Delivered |
| Unread badge | Updated |

См. [notifications.md](../features/notifications.md) § «Тихие часы», [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Push-уведомления.

### Read sync (cross-service — not Notification-owned)

Durable read cursor — **Messaging** `MarkRead` REST/gRPC only. WS `mark_read` / `message_read` — Realtime fan-out for same-profile tabs; **does not** replace REST persist. List unread / badges on other devices refresh after REST + `ListChats` metadata — [messaging-service.md](messaging-service.md) § MarkRead, [realtime-service.md](realtime-service.md) § Reconnect checklist, [notifications.md](../features/notifications.md) § Каналы доставки, [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § WS vs REST.

## Публикуемые события (→ NATS)

Отдельного доменного stream **`notification.events`** нет: события ниже — **телеметрия доставки**; публикация в **`analytics.notification.*`** (см. [MICROSERVICES.md](../MICROSERVICES.md) — раздел «Аналитика»).

| Событие                       | Данные                              |
|-------------------------------|-------------------------------------|
| `notification.push_sent`      | profile_id, type, platform          |
| `notification.push_delivered` | profile_id, type (delivery receipt) |
| `notification.push_clicked`   | profile_id, type, deep_link         |

## Зависимости

- **FCM** — Android и Web push
- **APNs** — iOS push и VoIP push (CallKit)
- **Resend** — email
- **Redis** — grouping state, rate limiting
- **User Service** — presence check
- **NATS** — получение событий для отправки уведомлений
- **Realtime Service** — (через NATS) in-app delivery


