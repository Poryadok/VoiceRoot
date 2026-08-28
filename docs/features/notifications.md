# Notifications — уведомления

## Типы уведомлений

Канонический wire enum — Notification Service `SendNotificationRequest.type` ([notification-service.md](../microservices/notification-service.md)). Feature doc uses product-facing names where they differ.

| Продукт / UX | Wire enum | Push | In-app | Status |
|--------------|-----------|------|--------|--------|
| Новое сообщение (DM / group / channel) | `new_message` | ✓ | ✓ | shipped (alias `new_dm` for DM-only paths — same handler) |
| **Запрос сообщения (незнакомец)** | `message_request` | ✓ | ✓ | spec ✓; code → [todo/backend.md](../todo/backend.md) |
| @упоминание в группе / канале | `mention` | ✓ | ✓ | shipped |
| Ответ на моё сообщение (reply) | `reply` | ✓ | ✓ | spec ✓; **not yet in code** — interim maps to `new_message` ([todo/backend.md](../todo/backend.md)) |
| Реакция на моё сообщение | `reaction` | **✗** | ✓ | shipped in-app only (no FCM/APNs) |
| Запрос в друзья | `friend_request` | ✓ | ✓ | shipped |
| Матч найден в ММ | `match_found` | ✓ | ✓ | shipped; presence check skipped |
| Системные (безопасность, обновления) | `system` | ✓ | ✓ | shipped |
| Кто-то зашёл в голосовую комнату | `voice_member_joined` | ✓ | ✓ | shipped; presence check skipped |
| Sticker / GIF (media-only message) | `new_message` | ✓ | ✓ | spec ✓; body label «Sticker» / «GIF» — [notification-service.md](../microservices/notification-service.md) § Stickers and GIF |

**Naming rule:** product copy «новое сообщение» → wire **`new_message`**. Legacy Notification DB rows / analytics may say `new_dm` — treat as alias for DM `new_message` until migration. Realtime in-app op uses `notification` with `type` matching this table.

**Sticker/GIF:** no separate wire type — grouped as **`new_message`** with truncated text body empty; push/in-app preview uses media label + optional thumb from `preview_url` / File thumb when policy allows. Same presence routing and mute rules as text messages.

### `message_request` (stranger DM)

| Аспект | Контракт |
|--------|----------|
| **Trigger** | Первое (или re-contact после decline) сообщение в DM с `inbox_bucket=requests` у получателя |
| **Push title** | «Незнакомец» (RU) / «Message request» (EN) + sender display name |
| **Push body** | Preview первого сообщения (truncated) |
| **In-app** | Badge на virtual folder «Запросы» + in-app notification row |
| **Grouping** | By sender profile (не by chat_id до accept) |
| **After accept** | Subsequent messages → ordinary `new_message` |

См. [text-chat.md](text-chat.md) § «Запросы сообщений», [friends.md](friends.md).

## Каналы доставки

| Платформа                 | Канал                                |
|---------------------------|--------------------------------------|
| Android                   | FCM                                  |
| Web                       | FCM (через Service Workers)          |
| iOS                       | APNs                                 |
| Desktop (Win/macOS/Linux) | WebSocket (приложение всегда онлайн) |

- Email — **только для авторизации** (вход, подтверждение), не для событий
- Собственный push-сервер не нужен
- **Группировка**: 1 push на чат с превью последнего сообщения и счётчиком ("Вася и ещё 4 сообщения"); обновлять существующий push, не плодить новые
- **Синхронизация прочитанного (dual-path):** unread badges и durable read cursor **не** синхронизируются через WS alone — см. таблицу ниже.

| Path | Persist | Назначение |
|------|---------|------------|
| **Primary** | REST/gRPC `Messaging.MarkRead` → `read_receipts` | Durable read cursor; badge на других устройствах — после REST persist + `ListChats` / `GetChatListMetadata` refresh |
| **Optional** | WS `mark_read` / server `message_read` — **без** persist | Fan-out на другие вкладки **того же профиля** (multi-tab sync) |

**Client rule:** открытие чата / scroll → **обязательно** REST `MarkRead`; WS `mark_read` — опционально для faster same-profile multi-tab sync. Cross-ref: [ARCHITECTURE_REQUIREMENTS.md](../ARCHITECTURE_REQUIREMENTS.md) § Доставка / WS vs REST, [messaging-service.md](../microservices/messaging-service.md) § MarkRead, [notification-service.md](../microservices/notification-service.md) § Read sync.

## Presence routing (online → in-app only)

Базовое правило (`DecideRouting` в Notification Service):

| Recipient presence | In-app (WebSocket) | Push (FCM/APNs) |
|--------------------|--------------------|-----------------|
| **Online** (`GetPresence` = online/idle/in-call on active session) | ✓ | **✗** |
| **Invisible** (shown as offline to others) | ✓ | ✓ (treat as offline for push routing) |
| **Offline** | ✓ | ✓ |

- Sender **никогда** не получает push/in-app на собственное сообщение
- **Matchmaking / voice join** — presence check **пропускается** (always push when offline policy applies); см. [todo/backend.md](../todo/backend.md)
- Quiet hours и mute применяются **после** base routing (см. ниже)

## Гранулярность настроек

Можно настраивать:
- Глобально
- Отдельно для каждого спейса
- Отдельно для каждого канала / чата
- По типу события

## Тихие часы (DND по расписанию)

- Можно задать временной диапазон (например, 23:00–8:00) + timezone
- **Push:** **suppressed** в окне quiet hours (`ApplyQuietHours` → `Push=false`)
- **In-app:** **still delivered** через Realtime WebSocket — unread badge и in-app notification row обновляются как обычно
- **@mention override:** если `override_mentions=true`, mentions **пробивают** quiet-hours push block (как mute override)

Не формулировать как «уведомления не приходят» — корректно: «push suppressed; in-app may still deliver».

## @упоминания и тишина

- Заглушённый чат: @username-упоминание **пробивает тишину** — включено по умолчанию, можно отключить в настройках
- Broadcast `@everyone` в чате: только при праве `TEXT_CHAT_MENTION_ALL_IN_CHAT`; тишину пробивает по той же логике

## Send without sound (`send_silent`)

| Layer | Wire name |
|-------|-----------|
| Composer / `SendMessageRequest` | **`send_silent`** (bool) |
| JetStream `message.sent` | **`send_silent`** |
| UI label | «Send without sound» / «Отправить без звука» |

**Notification consumption:**

| Channel | Behavior when `send_silent=true` |
|---------|----------------------------------|
| **Push** | Delivered **without sound** and **without badge increment** on some platforms; no DND break (except explicit mention-break rules) |
| **In-app** | Unread badge + in-app notification **update as usual** |
| **Grouping** | Still groups by chat |

См. [text-chat.md](text-chat.md) § Send options; producer — Messaging `message.sent`; consumer — Notification Service.

## Уведомления во время войс-чата

- Показываются как **in-app overlay** внутри приложения — войс не прерывается
- Push policy unchanged (presence routing still applies); overlay = Realtime in-app delivery path — [notification-service.md](../microservices/notification-service.md) § «@mention во время voice»
- @mention during voice may use dedicated overlay styling; does not elevate to VoIP push unless `incoming_call`
